package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
)

type vegetationLayerRow struct {
	Layer       ProjectMetadataCell
	Species     ProjectMetadataCell
	EnglishName ProjectMetadataCell
	MatchedName ProjectMetadataCell
	Presence    *float64
	MeanCover   *float64
	Plots       []vegetationCrosstabPlot
}

type vegetationLayerUnit struct {
	Code               ProjectMetadataCell
	LongName           *string
	NameStatus         string
	NameCandidates     []EnvironmentReportName
	NumPlots           int
	MembershipIDs      []string
	ProjectUnitIndexes []int
	Rows               []vegetationLayerRow
}

type vegetationLayerDiagnostic struct {
	Code, Identity string
	Count          int
}

type vegetationLayerReport struct {
	Project, SU, Title string
	SelectedFieldType  string
	Units              []vegetationLayerUnit
	Diagnostics        []vegetationLayerDiagnostic
	Quality            *vegetationQualitySelection
	NoneUnits          []vegetationNoneUnitOccurrence
	SelectedUnits      []vegetationSelectedUnitOccurrence
	ProjectUnits       []vegetationProjectUnitInput
}

func vegetationTextKey(cell ProjectMetadataCell) string {
	if cell.Text == nil {
		return "null"
	}
	return "text:" + *cell.Text
}

func planLongVegetationLayers(ctx context.Context, prepared VegetationReportPreparation,
	species, layers ProjectMetadataTable, options longVegetationOptions) (vegetationLayerReport, error) {
	fail := func(err error) (vegetationLayerReport, error) { return vegetationLayerReport{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if prepared.projectUnitInputs != nil && (len(prepared.Memberships) != 0 ||
		prepared.selectedUnitOccurrences != nil || prepared.noneUnitMembershipCounts != nil || prepared.qualityMembershipCounts != nil) {
		return fail(errors.New("project report units cannot carry selected-SU memberships or occurrence weights"))
	}
	if prepared.Project == "" || prepared.SU == "" || options.Average != "all-plots" && options.Average != "observations" ||
		options.Order != "species" && options.Order != "presence" {
		return fail(errors.New("layer planner requires owned preparation and supported explicit options"))
	}
	for _, value := range []float64{options.PresenceGreaterThan, options.MeanCoverGreaterThan} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return fail(errors.New("layer planner thresholds must be finite"))
		}
	}
	sc, err := siteUnitTransferColumns(species, "Code", "ScientificName", "EnglishName", "Codetype")
	if err != nil {
		return fail(err)
	}
	lc, err := siteUnitTransferColumns(layers, "Layer1234567", "Layer")
	if err != nil {
		return fail(err)
	}
	for _, source := range []struct {
		table    ProjectMetadataTable
		columns  map[string]int
		required []string
	}{{species, sc, []string{"Code", "ScientificName", "EnglishName", "Codetype"}},
		{layers, lc, []string{"Layer1234567", "Layer"}}} {
		for _, row := range source.table.Rows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			for _, name := range source.required {
				if _, err := vegetationReportIdentity(row.Cells[source.columns[name]]); err != nil {
					return fail(fmt.Errorf("layer report reference row %s %s: %w", row.RowID, name, err))
				}
			}
		}
	}
	si, err := siteUnitTransferIndex(species, sc["Code"])
	if err != nil {
		return fail(err)
	}
	li, err := siteUnitTransferIndex(layers, lc["Layer1234567"])
	if err != nil {
		return fail(err)
	}
	report := vegetationLayerReport{Project: prepared.Project, SU: prepared.SU, Title: options.Title,
		Units: []vegetationLayerUnit{}, Diagnostics: []vegetationLayerDiagnostic{}}
	type displayKey struct{ layer, species, english string }
	type joined struct {
		key   vegetationCrosstabKey
		name  ProjectMetadataCell
		plot  string
		cover ProjectMetadataCell
	}
	type unitState struct {
		unit   vegetationLayerUnit
		inputs []joined
	}
	units, memberships := map[string]*unitState{}, map[string][]string{}
	physicalCounts := map[string]int{}
	qualityDiagnostics := []vegetationLayerDiagnostic{}
	reportMemberships, err := selectedUnitReportMemberships(ctx, prepared)
	if err != nil {
		return fail(err)
	}
	selectedIDs := map[string]map[string]bool{}
	selectedCounts := map[string]int{}
	selectedOrder := []string{}
	for _, member := range reportMemberships {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for _, cell := range []ProjectMetadataCell{member.PlotNumber, member.SiteUnit} {
			if _, err := vegetationReportIdentity(cell); err != nil {
				return fail(err)
			}
		}
		id := vegetationTextKey(member.SiteUnit)
		if units[id] == nil {
			units[id] = &unitState{unit: vegetationLayerUnit{Code: cloneSiteUnitCell(member.SiteUnit), MembershipIDs: []string{}, Rows: []vegetationLayerRow{}},
				inputs: []joined{}}
		}
		state := units[id]
		if prepared.selectedUnitOccurrences == nil {
			state.unit.MembershipIDs = append(state.unit.MembershipIDs, member.RowID)
		} else {
			if selectedCounts[member.RowID] == 0 {
				selectedOrder = append(selectedOrder, member.RowID)
			}
			selectedCounts[member.RowID]++
			if selectedIDs[id] == nil {
				selectedIDs[id] = map[string]bool{}
			}
			if !selectedIDs[id][member.RowID] {
				state.unit.MembershipIDs = append(state.unit.MembershipIDs, member.RowID)
				selectedIDs[id][member.RowID] = true
			}
		}
		weight := 1
		if prepared.noneUnitMembershipCounts != nil {
			if prepared.qualityMembershipCounts != nil {
				return fail(errors.New("None report unit preparation must not apply quality weights twice"))
			}
			weight = prepared.noneUnitMembershipCounts[member.RowID]
			if weight <= 0 {
				return fail(errors.New("None report unit preparation lost its derived Env/current-list join count"))
			}
			if weight > 1 {
				qualityDiagnostics = append(qualityDiagnostics, vegetationLayerDiagnostic{"none_unit_join_multiplicity", member.RowID, weight})
			}
		}
		if prepared.qualityMembershipCounts != nil {
			weight = prepared.qualityMembershipCounts[member.RowID]
			if weight <= 0 {
				return fail(errors.New("quality-qualified preparation lost a physical membership weight"))
			}
			if weight > 1 {
				qualityDiagnostics = append(qualityDiagnostics, vegetationLayerDiagnostic{"quality_reference_multiplicity", member.RowID, weight})
			}
		}
		// Named PlotCount counts SiteUnit; unassigned DCount counts PlotNumber.
		if member.SiteUnit.Text != nil || member.PlotNumber.Text != nil {
			state.unit.NumPlots += weight
		}
		if member.PlotNumber.Text != nil {
			if prepared.selectedUnitOccurrences == nil || selectedCounts[member.RowID] == 1 {
				physicalCounts[*member.PlotNumber.Text]++
			}
			for i := 0; i < weight; i++ {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				memberships[*member.PlotNumber.Text] = append(memberships[*member.PlotNumber.Text], id)
			}
		}
	}
	for _, id := range selectedOrder {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if selectedCounts[id] > 1 {
			qualityDiagnostics = append(qualityDiagnostics, vegetationLayerDiagnostic{"selected_unit_join_multiplicity", id, selectedCounts[id]})
		}
	}
	if prepared.projectUnitInputs != nil {
		report.ProjectUnits, err = validateProjectUnitInputs(ctx, prepared.projectUnitInputs)
		if err != nil {
			return fail(err)
		}
		for i, row := range report.ProjectUnits {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			id := vegetationTextKey(row.SiteUnit)
			if units[id] == nil {
				units[id] = &unitState{unit: vegetationLayerUnit{Code: cloneSiteUnitCell(row.SiteUnit),
					MembershipIDs: []string{}, ProjectUnitIndexes: []int{}, Rows: []vegetationLayerRow{}}, inputs: []joined{}}
			}
			state := units[id]
			state.unit.ProjectUnitIndexes = append(state.unit.ProjectUnitIndexes, i)
			if row.SiteUnit.Text != nil || row.PlotNumber.Text != nil {
				state.unit.NumPlots++
			}
			if row.PlotNumber.Text != nil {
				physicalCounts[*row.PlotNumber.Text]++
				memberships[*row.PlotNumber.Text] = append(memberships[*row.PlotNumber.Text], id)
			}
		}
	}
	active := map[string]bool{}
	for _, row := range prepared.ReducedRows {
		for _, id := range memberships[row.PlotNumber] {
			active[id] = true
		}
	}
	constants := map[displayKey]joined{}
	diagnostics := map[string]vegetationLayerDiagnostic{}
	diagnose := func(code, identity string, count int) {
		diagnostics[code+"\x00"+identity] = vegetationLayerDiagnostic{code, identity, count}
	}
	for _, diagnostic := range qualityDiagnostics {
		diagnose(diagnostic.Code, diagnostic.Identity, diagnostic.Count)
	}
	for plot, count := range physicalCounts {
		if count > 1 {
			code := "physical_membership_multiplicity"
			if prepared.projectUnitInputs != nil {
				code = "project_unit_join_multiplicity"
			}
			diagnose(code, plot, count)
		}
	}
	for _, observation := range prepared.Observations {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if _, err := vegetationReportIdentity(observation.Species); err != nil {
			return fail(err)
		}
		if number, err := vegetationReportNumber(observation.Cover); err != nil || number == nil {
			if err == nil {
				err = errors.New("layer preparation observations must contain non-NULL cover")
			}
			return fail(err)
		}
		refs := []ProjectMetadataRow{}
		if observation.Species.Text != nil {
			refs = si[*observation.Species.Text]
		}
		if len(refs) == 0 {
			refs = []ProjectMetadataRow{{}}
			diagnose("missing_master_species", vegetationTextKey(observation.Species), 1)
		} else if len(refs) > 1 {
			diagnose("master_species_multiplicity", *observation.Species.Text, len(refs))
		}
		layerRefs := li[observation.Layer]
		if len(layerRefs) == 0 {
			layerRefs = []ProjectMetadataRow{{}}
			diagnose("missing_layer_label", observation.Layer, 1)
		} else if len(layerRefs) > 1 {
			diagnose("layer_label_multiplicity", observation.Layer, len(layerRefs))
		}
		for _, ref := range refs {
			name, english := observation.Species, ProjectMetadataCell{Storage: "null"}
			if len(ref.Cells) > 0 {
				codeType := ref.Cells[sc["Codetype"]]
				if codeType.Text != nil && (*codeType.Text == "S" || *codeType.Text == "s") {
					continue
				}
				if scientific := ref.Cells[sc["ScientificName"]]; scientific.Text != nil {
					name = scientific
				}
				if options.ShowEnglishName {
					english = ref.Cells[sc["EnglishName"]]
				}
			}
			if options.selectedFieldGrouping != nil && len(memberships[observation.PlotNumber]) != 0 {
				if err := options.selectedFieldGrouping.observeProjected(ctx, name, english); err != nil {
					return fail(err)
				}
			}
			for _, layerRef := range layerRefs {
				layer := ProjectMetadataCell{Storage: "null"}
				if len(layerRef.Cells) > 0 {
					layer = layerRef.Cells[lc["Layer"]]
				}
				value := joined{key: vegetationCrosstabKey{cloneSiteUnitCell(layer), cloneSiteUnitCell(name)},
					name: cloneSiteUnitCell(english), plot: observation.PlotNumber, cover: observation.Cover}
				key := displayKey{vegetationTextKey(layer), vegetationTextKey(name), vegetationTextKey(english)}
				for _, id := range memberships[observation.PlotNumber] {
					units[id].inputs = append(units[id].inputs, value)
					constants[key] = value
				}
			}
		}
	}
	blankPlots := func(names []string) []vegetationCrosstabPlot {
		result := []vegetationCrosstabPlot{}
		for _, name := range names {
			result = append(result, vegetationCrosstabPlot{PlotNumber: name})
		}
		return result
	}
	cloneRow := func(row vegetationLayerRow) vegetationLayerRow {
		row.Layer, row.Species = cloneSiteUnitCell(row.Layer), cloneSiteUnitCell(row.Species)
		row.EnglishName, row.MatchedName = cloneSiteUnitCell(row.EnglishName), cloneSiteUnitCell(row.MatchedName)
		if row.Presence != nil {
			value := *row.Presence
			row.Presence = &value
		}
		if row.MeanCover != nil {
			value := *row.MeanCover
			row.MeanCover = &value
		}
		row.Plots = append([]vegetationCrosstabPlot{}, row.Plots...)
		for i := range row.Plots {
			if row.Plots[i].Cover != nil {
				value := *row.Plots[i].Cover
				row.Plots[i].Cover = &value
			}
		}
		return row
	}
	for _, id := range sortedEnvironmentReportKeys(units) {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		state := units[id]
		// Orphan-only named units are absent from SnRecords' inner Veg join.
		if state.unit.NumPlots == 0 || state.unit.Code.Text != nil && !active[id] {
			count := len(state.unit.MembershipIDs)
			if prepared.projectUnitInputs != nil {
				count = len(state.unit.ProjectUnitIndexes)
			}
			diagnose("excluded_unit_without_vegetation", id, count)
			continue
		}
		buckets, names := map[string][]vegetationCrosstabInput{}, map[string]ProjectMetadataCell{}
		pivots := map[string]bool{}
		for _, input := range state.inputs {
			key := vegetationTextKey(input.name)
			buckets[key] = append(buckets[key], vegetationCrosstabInput{input.key, input.plot, input.cover})
			names[key] = input.name
			pivots[input.plot] = true
		}
		pivotNames := sortedEnvironmentReportKeys(pivots)
		calculated := []vegetationLayerRow{}
		for _, key := range sortedEnvironmentReportKeys(buckets) {
			rows, err := calculateVegetationCrosstab(ctx, buckets[key], state.unit.NumPlots,
				vegetationCrosstabOptions{Average: options.Average, Unfiltered: true}, nil)
			if err != nil {
				return fail(err)
			}
			for _, row := range rows {
				plots := blankPlots(pivotNames)
				for i := range plots {
					for _, cell := range row.Plots {
						if plots[i].PlotNumber == cell.PlotNumber {
							plots[i].Cover = cell.Cover
						}
					}
				}
				calculated = append(calculated, vegetationLayerRow{row.Key.Group, row.Key.Species,
					cloneSiteUnitCell(names[key]), cloneSiteUnitCell(names[key]), row.Presence, row.MeanCover, plots})
			}
		}
		if options.ConstantSpeciesList {
			for key, constant := range constants {
				matched := 0
				for _, row := range calculated {
					// EnglishName participates in grouping but NOT the final join.
					if constant.key.Group.Text != nil && constant.key.Species.Text != nil &&
						key.layer == vegetationTextKey(row.Layer) && key.species == vegetationTextKey(row.Species) {
						row.EnglishName = cloneSiteUnitCell(constant.name)
						state.unit.Rows = append(state.unit.Rows, cloneRow(row))
						matched++
					}
				}
				if matched == 0 {
					state.unit.Rows = append(state.unit.Rows, vegetationLayerRow{Layer: cloneSiteUnitCell(constant.key.Group),
						Species: cloneSiteUnitCell(constant.key.Species), EnglishName: cloneSiteUnitCell(constant.name),
						MatchedName: ProjectMetadataCell{Storage: "null"}, Plots: blankPlots(pivotNames)})
				} else if matched > 1 {
					diagnose("constant_list_name_fanout", key.layer+"/"+key.species, matched)
				}
			}
		} else {
			for _, row := range calculated {
				if row.MeanCover != nil && row.Presence != nil && *row.MeanCover > options.MeanCoverGreaterThan &&
					*row.Presence > options.PresenceGreaterThan/100 {
					state.unit.Rows = append(state.unit.Rows, row)
				}
			}
		}
		sort.Slice(state.unit.Rows, func(i, j int) bool {
			a, b := state.unit.Rows[i], state.unit.Rows[j]
			if (!options.NoneGrouping || options.ConstantSpeciesList) && vegetationTextKey(a.Layer) != vegetationTextKey(b.Layer) {
				return vegetationTextKey(a.Layer) < vegetationTextKey(b.Layer)
			}
			if !options.ConstantSpeciesList && options.Order == "presence" && *a.Presence != *b.Presence {
				return *a.Presence > *b.Presence
			}
			for _, pair := range [][2]ProjectMetadataCell{{a.Species, b.Species}, {a.EnglishName, b.EnglishName}, {a.MatchedName, b.MatchedName}} {
				if vegetationTextKey(pair[0]) != vegetationTextKey(pair[1]) {
					return vegetationTextKey(pair[0]) < vegetationTextKey(pair[1])
				}
			}
			if options.NoneGrouping {
				return vegetationTextKey(a.Layer) < vegetationTextKey(b.Layer)
			}
			return false
		})
		sort.Strings(state.unit.MembershipIDs)
		report.Units = append(report.Units, state.unit)
	}
	for _, key := range sortedEnvironmentReportKeys(diagnostics) {
		report.Diagnostics = append(report.Diagnostics, diagnostics[key])
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return report, nil
}
