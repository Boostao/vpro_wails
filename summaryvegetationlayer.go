package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

type summaryVegetationLayerWitness struct {
	StageRowID        string
	ConversionLayerID string
	MembershipID      string
	Unit              string
	LayerMapID        string
	ReferenceOrigins  []vegetationLifeformReferenceOrigin
	RawVegRowIDs      []string
	Cover             ProjectMetadataCell
}

type summaryVegetationLayerRow struct {
	Layer        ProjectMetadataCell
	Species      ProjectMetadataCell
	CommonName   *ProjectMetadataCell `json:"CommonName,omitempty"`
	RawSpecies   *ProjectMetadataCell `json:"RawSpecies,omitempty"`
	Order        ProjectMetadataCell
	Covers       []ProjectMetadataCell
	Witnesses    []summaryVegetationLayerWitness
	numericGroup *int
}

type summaryVegetationLayerReport struct {
	Units   []summaryVegetationUnit
	Columns []string
	Rows    []summaryVegetationLayerRow
}

// This literal read model deliberately excludes Access regional collation and
// options/callbacks. Raw preparation omits NULL covers before the source joins.
func prepareSummaryVegetationLayer(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerDisplay(ctx, project, suName, veg, su, master, personal, layers, average, 6)
}

func prepareSummaryVegetationLayerDisplay(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, display int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerOptions(ctx, project, suName, veg, su, master, personal, layers, average, display, 0)
}

func prepareSummaryVegetationLayerOptions(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, display, showClass int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerReferences(ctx, project, suName, veg, su, master, personal, layers,
		average, display, showClass, nil)
}

func prepareSummaryVegetationLayerReferences(ctx context.Context, project, suName string,
	veg, su, master, personal, layers ProjectMetadataTable, average, display, showClass int,
	references *summaryVegetationReferencePlan) (summaryVegetationLayerReport, error) {
	fail := func(err error) (summaryVegetationLayerReport, error) { return summaryVegetationLayerReport{}, err }
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, su, layers)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation layer source preparation: %w", err))
	}
	if err := lifeformSummaryText(ProjectMetadataCell{Storage: "text", Text: &project}, 20); err != nil {
		return fail(fmt.Errorf("summary vegetation staged ProjectID: %w", err))
	}
	type sourceKey struct{ plot, species string }
	sourceIDs := map[sourceKey][]string{}
	for _, row := range prepared.ReducedRows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if row.PlotNumber == "" || row.Species.Text == nil || *row.Species.Text == "" {
			return fail(errors.New("summary vegetation TempReportVeg requires nonempty PlotNumber/Species; no source omission"))
		}
		if err := lifeformSummaryText(ProjectMetadataCell{Storage: "text", Text: &row.PlotNumber}, 7); err != nil {
			return fail(fmt.Errorf("summary vegetation TempReportVeg PlotNumber: %w", err))
		}
		if err := lifeformSummaryText(row.Species, 8); err != nil {
			return fail(fmt.Errorf("summary vegetation TempReportVeg Species: %w", err))
		}
		// TempReportVeg inserts all fifteen MAX fields, even fields not expanded
		// by the selected LayerCode rows.
		for _, cover := range row.Covers {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			number, err := vegetationReportNumber(cover)
			if err != nil {
				return fail(err)
			}
			if number != nil {
				if _, err := lifeformSummarySingle(number); err != nil {
					return fail(fmt.Errorf("summary vegetation TempReportVeg SINGLE: %w", err))
				}
			}
		}
		sourceIDs[sourceKey{row.PlotNumber, vegetationTextKey(row.Species)}] = row.SourceRowIDs
	}
	stage := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Plotnumber"}, {Name: "Layer"}, {Name: "Species"}, {Name: "Cover"},
	}, Rows: []ProjectMetadataRow{}}
	origins := map[string][]string{}
	conversionLayers := map[string]string{}
	for i, observation := range prepared.Observations {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		number, err := vegetationReportNumber(observation.Cover)
		if err != nil {
			return fail(err)
		}
		value, err := lifeformSummarySingle(number)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer staged SINGLE: %w", err))
		}
		// The rational assignment helper cannot retain the sign of source zero.
		if observation.Cover.Real != nil && *observation.Cover.Real == 0 {
			value = *observation.Cover.Real
		}
		id := strconv.Itoa(i + 1)
		stage.Rows = append(stage.Rows, ProjectMetadataRow{RowID: id, Cells: []ProjectMetadataCell{
			{Storage: "text", Text: &observation.PlotNumber}, {Storage: "text", Text: &observation.Layer},
			cloneSiteUnitCell(observation.Species), {Storage: "real", Real: &value},
		}})
		origins[id] = sourceIDs[sourceKey{observation.PlotNumber, vegetationTextKey(observation.Species)}]
		conversionLayers[id] = observation.LayerRowID
	}
	result, err := planSummaryVegetationStage(ctx, veg, su, stage, master, personal, layers, average, display, showClass,
		summaryVegetationStageProjection{joinField: "Layer1234567", joinLimit: 2, labelField: "Layer", labelLimit: 3,
			references: references})
	if err != nil {
		return fail(err)
	}
	for i := range result.Rows {
		for j := range result.Rows[i].Witnesses {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			witness := &result.Rows[i].Witnesses[j]
			witness.RawVegRowIDs = append([]string{}, origins[witness.StageRowID]...)
			witness.ConversionLayerID = conversionLayers[witness.StageRowID]
		}
	}
	return result, nil
}

// Stage is the physical EntryDat_Veg boundary. Its NULL covers establish pivot
// columns, unlike NULL raw covers omitted by ConvertProVeg2V2.
func planSummaryVegetationLayer(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationLayerDisplay(ctx, raw, su, stage, master, personal, layers, average, 6)
}

func planSummaryVegetationLayerDisplay(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, display int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationLayerOptions(ctx, raw, su, stage, master, personal, layers, average, display, 0)
}

func planSummaryVegetationLayerOptions(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, display, showClass int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationStage(ctx, raw, su, stage, master, personal, layers, average, display, showClass,
		summaryVegetationStageProjection{joinField: "Layer1234567", joinLimit: 2, labelField: "Layer", labelLimit: 3})
}

type summaryVegetationStageProjection struct {
	joinField    string
	joinLimit    int
	labelField   string
	labelLimit   int
	nullGroup    bool
	numericOrder bool
	references   *summaryVegetationReferencePlan
	constancy    bool
}

func planSummaryVegetationStage(ctx context.Context,
	raw, su, stage, master, personal, layers ProjectMetadataTable, average, display, showClass int,
	projection summaryVegetationStageProjection) (summaryVegetationLayerReport, error) {
	fail := func(err error) (summaryVegetationLayerReport, error) { return summaryVegetationLayerReport{}, err }
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	units, err := prepareSummaryVegetationUnits(ctx, raw, su)
	if err != nil {
		return fail(err)
	}
	var references vegetationLifeformReferences
	showCommon := false
	if projection.references != nil {
		references = projection.references.References
		showCommon = projection.references.Review.ShowCommon
	} else {
		references, err = prepareVegetationLifeformReferences(ctx, master, personal)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer references: %w", err))
		}
	}
	sc, err := siteUnitTransferColumns(stage, "Plotnumber", "Layer", "Species", "Cover")
	if err != nil {
		return fail(fmt.Errorf("summary vegetation staged schema: %w", err))
	}
	lc, err := siteUnitTransferColumns(layers, projection.joinField, projection.labelField)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation layer mapping schema: %w", err))
	}
	type layerMapping struct {
		id    string
		label ProjectMetadataCell
	}
	mappings := map[string]layerMapping{}
	for _, row := range layers.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		code, err := vegetationReportIdentity(row.Cells[lc[projection.joinField]])
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer mapping row %s: %w", row.RowID, err))
		}
		for name, limit := range map[string]int{projection.joinField: projection.joinLimit, projection.labelField: projection.labelLimit} {
			if err := lifeformSummaryText(row.Cells[lc[name]], limit); err != nil {
				return fail(fmt.Errorf("summary vegetation layer mapping row %s %s: %w", row.RowID, name, err))
			}
		}
		if code != nil {
			if _, exists := mappings[*code]; exists {
				return fail(fmt.Errorf("summary vegetation layer mapping has duplicate literal %s; original unique index is unavailable", projection.joinField))
			}
			label := ProjectMetadataCell{Storage: "null"}
			if !projection.nullGroup {
				label = cloneSiteUnitCell(row.Cells[lc[projection.labelField]])
			}
			mappings[*code] = layerMapping{row.RowID, label}
		}
	}
	rc, err := siteUnitTransferColumns(references.Table, vegetationLifeformReferenceColumns...)
	if err != nil {
		return fail(err)
	}
	refRows := map[string][]ProjectMetadataRow{}
	numericGroups := map[string]*int{}
	for _, row := range references.Table.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		codeLimit, typeLimit := 8, 1
		if projection.references != nil {
			codeLimit, typeLimit = 255, 255
		}
		for name, limit := range map[string]int{"Code": codeLimit, "ScientificName": 255, "EnglishName": 255, "Codetype": typeLimit} {
			if err := lifeformSummaryText(row.Cells[rc[name]], limit); err != nil {
				return fail(fmt.Errorf("summary vegetation layer reference row %s %s: %w", row.RowID, name, err))
			}
		}
		if projection.constancy {
			if row.RowID == "" {
				return fail(errors.New("summary vegetation Constancy reference requires a physical projection identity"))
			}
			if _, exists := numericGroups[row.RowID]; exists {
				return fail(errors.New("summary vegetation Constancy has duplicate reference projection identities"))
			}
			number, err := summaryVegetationMasterLongInteger(row.Cells[rc["Lifeform"]])
			if err != nil {
				return fail(fmt.Errorf("summary vegetation Constancy reference row %s: %w", row.RowID, err))
			}
			numericGroups[row.RowID] = number
		}
		if code := row.Cells[rc["Code"]].Text; code != nil {
			refRows[*code] = append(refRows[*code], row)
		}
	}
	denominators := map[string]int64{}
	for _, unit := range units {
		denominators[unit.Code] = int64(unit.NumPlots)
	}
	uc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	members := map[string][]ProjectMetadataRow{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[uc["PlotNumber"]].Text, row.Cells[uc["SiteUnit"]].Text
		if plot != nil && unit != nil && denominators[*unit] > 0 {
			members[*plot] = append(members[*plot], row)
		}
	}
	type groupKey struct{ layer, species, common, rawSpecies string }
	type group struct {
		row    summaryVegetationLayerRow
		covers map[string][]ProjectMetadataCell
	}
	groups := map[groupKey]*group{}
	pivot := map[string]bool{}
	for _, row := range stage.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for name, limit := range map[string]int{"Plotnumber": 8, "Layer": 3, "Species": 8} {
			if err := lifeformSummaryText(row.Cells[sc[name]], limit); err != nil {
				return fail(fmt.Errorf("summary vegetation stage row %s %s: %w", row.RowID, name, err))
			}
		}
		plot := row.Cells[sc["Plotnumber"]].Text
		species := row.Cells[sc["Species"]]
		if plot == nil || *plot == "" || species.Text != nil && *species.Text == "" {
			return fail(fmt.Errorf("summary vegetation stage row %s requires nonempty Plotnumber and nonempty-or-NULL Species; no source omission or truncation", row.RowID))
		}
		cover := row.Cells[sc["Cover"]]
		if _, err := aggregateSummaryVegetationCovers(ctx, []ProjectMetadataCell{cover}, 1); err != nil {
			return fail(fmt.Errorf("summary vegetation stage row %s: %w", row.RowID, err))
		}
		mapping := layerMapping{label: ProjectMetadataCell{Storage: "null"}}
		if code := row.Cells[sc["Layer"]].Text; code != nil {
			if found, exists := mappings[*code]; exists {
				mapping = found
			}
		}
		matches := []ProjectMetadataRow{}
		if species.Text != nil {
			matches = refRows[*species.Text]
		}
		if len(matches) == 0 {
			matches = []ProjectMetadataRow{{}}
		}
		for _, member := range members[*plot] {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			unit := *member.Cells[uc["SiteUnit"]].Text
			for _, ref := range matches {
				if err := ctx.Err(); err != nil {
					return fail(err)
				}
				name := species
				common := ProjectMetadataCell{Storage: "null"}
				origins := []vegetationLifeformReferenceOrigin{}
				if ref.RowID != "" {
					if scientific := ref.Cells[rc["ScientificName"]]; scientific.Storage != "null" {
						name = scientific
					}
					origins = append(origins, references.Origins[ref.RowID]...)
					common = ref.Cells[rc["EnglishName"]]
				}
				label := mapping.label
				key := groupKey{layer: vegetationTextKey(label), species: vegetationTextKey(name)}
				var number *int
				if projection.constancy {
					label = ProjectMetadataCell{Storage: "null"}
					number = numericGroups[ref.RowID]
					key.layer, key.rawSpecies = "null", vegetationTextKey(species)
					if number != nil {
						value := strconv.Itoa(*number)
						label = ProjectMetadataCell{Storage: "integer", Integer: &value}
						key.layer = "integer:" + value
					}
				}
				if showCommon {
					key.common = vegetationTextKey(common)
				}
				g := groups[key]
				if g == nil {
					g = &group{row: summaryVegetationLayerRow{Layer: cloneSiteUnitCell(label),
						Species: cloneSiteUnitCell(name), Witnesses: []summaryVegetationLayerWitness{}, numericGroup: number},
						covers: map[string][]ProjectMetadataCell{}}
					groups[key] = g
					if projection.constancy {
						raw := cloneSiteUnitCell(species)
						g.row.RawSpecies = &raw
					}
					if showCommon {
						value := cloneSiteUnitCell(common)
						g.row.CommonName = &value
					}
				}
				pivot[unit] = true
				g.covers[unit] = append(g.covers[unit], cloneSiteUnitCell(cover))
				g.row.Witnesses = append(g.row.Witnesses, summaryVegetationLayerWitness{
					StageRowID: row.RowID, MembershipID: member.RowID, Unit: unit,
					LayerMapID: mapping.id, ReferenceOrigins: origins, RawVegRowIDs: []string{},
					Cover: cloneSiteUnitCell(cover),
				})
			}
		}
	}
	result := summaryVegetationLayerReport{Units: units, Columns: []string{}, Rows: []summaryVegetationLayerRow{}}
	for code := range pivot {
		result.Columns = append(result.Columns, code)
	}
	sort.Strings(result.Columns)
	for _, g := range groups {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		g.row.Covers = []ProjectMetadataCell{}
		g.row.Order = ProjectMetadataCell{Storage: "null"}
		for _, unit := range result.Columns {
			if len(g.covers[unit]) == 0 {
				g.row.Covers = append(g.row.Covers, ProjectMetadataCell{Storage: "null"})
				continue
			}
			input, err := aggregateSummaryVegetationCovers(ctx, g.covers[unit], denominators[unit])
			if err != nil {
				return fail(fmt.Errorf("summary vegetation unit %q aggregate: %w", unit, err))
			}
			var cell summaryVegetationDisplayCell
			if showClass == 0 {
				cell, err = calculateSummaryVegetationDisplay(ctx, input, average, display)
			} else {
				cell, err = calculateSummaryVegetationClassDisplay(ctx, input, average, display, showClass)
			}
			if err != nil {
				return fail(fmt.Errorf("summary vegetation unit %q cell: %w", unit, err))
			}
			g.row.Covers = append(g.row.Covers, cloneSiteUnitCell(cell.Value))
			if cell.Order.Text != nil {
				if projection.numericOrder {
					value, err := strconv.ParseFloat(*cell.Order.Text, 64)
					if err != nil {
						return fail(fmt.Errorf("summary vegetation numeric Order: %w", err))
					}
					if g.row.Order.Real == nil || value > *g.row.Order.Real {
						g.row.Order = ProjectMetadataCell{Storage: "real", Real: &value}
					}
				} else if g.row.Order.Text == nil || *cell.Order.Text > *g.row.Order.Text {
					g.row.Order = cloneSiteUnitCell(cell.Order)
				}
			}
		}
		if display == 1 || display == 3 || g.row.Order.Storage != "null" {
			result.Rows = append(result.Rows, g.row)
		}
	}
	sort.Slice(result.Rows, func(i, j int) bool {
		a, b := result.Rows[i], result.Rows[j]
		if projection.constancy {
			if a.numericGroup == nil || b.numericGroup == nil {
				if a.numericGroup != nil || b.numericGroup != nil {
					return a.numericGroup == nil
				}
			} else if *a.numericGroup != *b.numericGroup {
				return *a.numericGroup < *b.numericGroup
			}
		} else if vegetationTextKey(a.Layer) != vegetationTextKey(b.Layer) {
			if a.Layer.Text == nil || b.Layer.Text == nil {
				return a.Layer.Text == nil
			}
			return *a.Layer.Text < *b.Layer.Text
		}
		if vegetationTextKey(a.Species) != vegetationTextKey(b.Species) {
			if a.Species.Text == nil || b.Species.Text == nil {
				return a.Species.Text == nil
			}
			return *a.Species.Text < *b.Species.Text
		}
		if projection.constancy && vegetationTextKey(*a.RawSpecies) != vegetationTextKey(*b.RawSpecies) {
			if a.RawSpecies.Text == nil || b.RawSpecies.Text == nil {
				return a.RawSpecies.Text == nil
			}
			return *a.RawSpecies.Text < *b.RawSpecies.Text
		}
		if showCommon {
			if a.CommonName.Text == nil || b.CommonName.Text == nil {
				return a.CommonName.Text == nil && b.CommonName.Text != nil
			}
			return *a.CommonName.Text < *b.CommonName.Text
		}
		return false
	})
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
