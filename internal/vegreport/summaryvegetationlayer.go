package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	ReferenceOrigins  []reportcore.VegetationLifeformReferenceOrigin
	RawVegRowIDs      []string
	Cover             vprocore.ProjectMetadataCell
}

type summaryVegetationLayerRow struct {
	Layer        vprocore.ProjectMetadataCell
	Species      vprocore.ProjectMetadataCell
	CommonName   *vprocore.ProjectMetadataCell `json:"CommonName,omitempty"`
	RawSpecies   *vprocore.ProjectMetadataCell `json:"RawSpecies,omitempty"`
	Order        vprocore.ProjectMetadataCell
	Covers       []vprocore.ProjectMetadataCell
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
	veg, su, master, personal, layers vprocore.ProjectMetadataTable, average int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerDisplay(ctx, project, suName, veg, su, master, personal, layers, average, 6)
}

func prepareSummaryVegetationLayerDisplay(ctx context.Context, project, suName string,
	veg, su, master, personal, layers vprocore.ProjectMetadataTable, average, display int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerOptions(ctx, project, suName, veg, su, master, personal, layers, average, display, 0)
}

func prepareSummaryVegetationLayerOptions(ctx context.Context, project, suName string,
	veg, su, master, personal, layers vprocore.ProjectMetadataTable, average, display, showClass int) (summaryVegetationLayerReport, error) {
	return prepareSummaryVegetationLayerReferences(ctx, project, suName, veg, su, master, personal, layers,
		average, display, showClass, nil)
}

func prepareSummaryVegetationLayerReferences(ctx context.Context, project, suName string,
	veg, su, master, personal, layers vprocore.ProjectMetadataTable, average, display, showClass int,
	references *summaryVegetationReferencePlan) (summaryVegetationLayerReport, error) {
	fail := func(err error) (summaryVegetationLayerReport, error) { return summaryVegetationLayerReport{}, err }
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, su, layers)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation layer source preparation: %w", err))
	}
	if err := reportcore.LifeformSummaryText(vprocore.ProjectMetadataCell{Storage: "text", Text: &project}, 20); err != nil {
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
		if err := reportcore.LifeformSummaryText(vprocore.ProjectMetadataCell{Storage: "text", Text: &row.PlotNumber}, 7); err != nil {
			return fail(fmt.Errorf("summary vegetation TempReportVeg PlotNumber: %w", err))
		}
		if err := reportcore.LifeformSummaryText(row.Species, 8); err != nil {
			return fail(fmt.Errorf("summary vegetation TempReportVeg Species: %w", err))
		}
		// TempReportVeg inserts all fifteen MAX fields, even fields not expanded
		// by the selected LayerCode rows.
		for _, cover := range row.Covers {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			number, err := vprocore.VegetationReportNumber(cover)
			if err != nil {
				return fail(err)
			}
			if number != nil {
				if _, err := reportcore.LifeformSummarySingle(number); err != nil {
					return fail(fmt.Errorf("summary vegetation TempReportVeg SINGLE: %w", err))
				}
			}
		}
		sourceIDs[sourceKey{row.PlotNumber, reportcore.VegetationTextKey(row.Species)}] = row.SourceRowIDs
	}
	stage := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "Plotnumber"}, {Name: "Layer"}, {Name: "Species"}, {Name: "Cover"},
	}, Rows: []vprocore.ProjectMetadataRow{}}
	origins := map[string][]string{}
	conversionLayers := map[string]string{}
	for i, observation := range prepared.Observations {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		number, err := vprocore.VegetationReportNumber(observation.Cover)
		if err != nil {
			return fail(err)
		}
		value, err := reportcore.LifeformSummarySingle(number)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer staged SINGLE: %w", err))
		}
		// The rational assignment helper cannot retain the sign of source zero.
		if observation.Cover.Real != nil && *observation.Cover.Real == 0 {
			value = *observation.Cover.Real
		}
		id := strconv.Itoa(i + 1)
		stage.Rows = append(stage.Rows, vprocore.ProjectMetadataRow{RowID: id, Cells: []vprocore.ProjectMetadataCell{
			{Storage: "text", Text: &observation.PlotNumber}, {Storage: "text", Text: &observation.Layer},
			vprocore.CloneSiteUnitCell(observation.Species), {Storage: "real", Real: &value},
		}})
		origins[id] = sourceIDs[sourceKey{observation.PlotNumber, reportcore.VegetationTextKey(observation.Species)}]
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
	raw, su, stage, master, personal, layers vprocore.ProjectMetadataTable, average int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationLayerDisplay(ctx, raw, su, stage, master, personal, layers, average, 6)
}

func planSummaryVegetationLayerDisplay(ctx context.Context,
	raw, su, stage, master, personal, layers vprocore.ProjectMetadataTable, average, display int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationLayerOptions(ctx, raw, su, stage, master, personal, layers, average, display, 0)
}

func planSummaryVegetationLayerOptions(ctx context.Context,
	raw, su, stage, master, personal, layers vprocore.ProjectMetadataTable, average, display, showClass int) (summaryVegetationLayerReport, error) {
	return planSummaryVegetationStage(ctx, raw, su, stage, master, personal, layers, average, display, showClass,
		summaryVegetationStageProjection{joinField: "Layer1234567", joinLimit: 2, labelField: "Layer", labelLimit: 3})
}

type summaryVegetationStageProjection struct {
	joinField       string
	joinLimit       int
	labelField      string
	labelLimit      int
	nullGroup       bool
	numericOrder    bool
	references      *summaryVegetationReferencePlan
	constancy       bool
	unanimousCommon bool
}

func planSummaryVegetationStage(ctx context.Context,
	raw, su, stage, master, personal, layers vprocore.ProjectMetadataTable, average, display, showClass int,
	projection summaryVegetationStageProjection) (summaryVegetationLayerReport, error) {
	fail := func(err error) (summaryVegetationLayerReport, error) { return summaryVegetationLayerReport{}, err }
	if err := validateSummaryVegetationDisplayOptions(ctx, average, display, showClass); err != nil {
		return fail(err)
	}
	if projection.unanimousCommon && (projection.constancy || projection.references == nil ||
		!projection.references.Review.ShowCommon || !projection.numericOrder || display != 6 || showClass != 0) {
		return fail(errors.New("summary vegetation unanimous First(common name) requires a non-Constancy effective common Display6 projection"))
	}
	units, err := prepareSummaryVegetationUnits(ctx, raw, su)
	if err != nil {
		return fail(err)
	}
	var references reportcore.VegetationLifeformReferences
	showCommon := false
	if projection.references != nil {
		references = projection.references.References
		showCommon = projection.references.Review.ShowCommon
	} else {
		references, err = reportcore.PrepareVegetationLifeformReferences(ctx, master, personal)
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer references: %w", err))
		}
	}
	sc, err := vprocore.SiteUnitTransferColumns(stage, "Plotnumber", "Layer", "Species", "Cover")
	if err != nil {
		return fail(fmt.Errorf("summary vegetation staged schema: %w", err))
	}
	lc, err := vprocore.SiteUnitTransferColumns(layers, projection.joinField, projection.labelField)
	if err != nil {
		return fail(fmt.Errorf("summary vegetation layer mapping schema: %w", err))
	}
	type layerMapping struct {
		id    string
		label vprocore.ProjectMetadataCell
	}
	mappings := map[string]layerMapping{}
	for _, row := range layers.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		code, err := vprocore.VegetationReportIdentity(row.Cells[lc[projection.joinField]])
		if err != nil {
			return fail(fmt.Errorf("summary vegetation layer mapping row %s: %w", row.RowID, err))
		}
		for name, limit := range map[string]int{projection.joinField: projection.joinLimit, projection.labelField: projection.labelLimit} {
			if err := reportcore.LifeformSummaryText(row.Cells[lc[name]], limit); err != nil {
				return fail(fmt.Errorf("summary vegetation layer mapping row %s %s: %w", row.RowID, name, err))
			}
		}
		if code != nil {
			if _, exists := mappings[*code]; exists {
				return fail(fmt.Errorf("summary vegetation layer mapping has duplicate literal %s; original unique index is unavailable", projection.joinField))
			}
			label := vprocore.ProjectMetadataCell{Storage: "null"}
			if !projection.nullGroup {
				label = vprocore.CloneSiteUnitCell(row.Cells[lc[projection.labelField]])
			}
			mappings[*code] = layerMapping{row.RowID, label}
		}
	}
	rc, err := vprocore.SiteUnitTransferColumns(references.Table, reportcore.VegetationLifeformReferenceColumns...)
	if err != nil {
		return fail(err)
	}
	refRows := map[string][]vprocore.ProjectMetadataRow{}
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
			if err := reportcore.LifeformSummaryText(row.Cells[rc[name]], limit); err != nil {
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
	uc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	members := map[string][]vprocore.ProjectMetadataRow{}
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
		row          summaryVegetationLayerRow
		covers       map[string][]vprocore.ProjectMetadataCell
		sourceCommon string
	}
	groups := map[groupKey]*group{}
	pivot := map[string]bool{}
	for _, row := range stage.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		for name, limit := range map[string]int{"Plotnumber": 8, "Layer": 3, "Species": 8} {
			if err := reportcore.LifeformSummaryText(row.Cells[sc[name]], limit); err != nil {
				return fail(fmt.Errorf("summary vegetation stage row %s %s: %w", row.RowID, name, err))
			}
		}
		plot := row.Cells[sc["Plotnumber"]].Text
		species := row.Cells[sc["Species"]]
		if plot == nil || *plot == "" || species.Text != nil && *species.Text == "" {
			return fail(fmt.Errorf("summary vegetation stage row %s requires nonempty Plotnumber and nonempty-or-NULL Species; no source omission or truncation", row.RowID))
		}
		cover := row.Cells[sc["Cover"]]
		if _, err := aggregateSummaryVegetationCovers(ctx, []vprocore.ProjectMetadataCell{cover}, 1); err != nil {
			return fail(fmt.Errorf("summary vegetation stage row %s: %w", row.RowID, err))
		}
		mapping := layerMapping{label: vprocore.ProjectMetadataCell{Storage: "null"}}
		if code := row.Cells[sc["Layer"]].Text; code != nil {
			if found, exists := mappings[*code]; exists {
				mapping = found
			}
		}
		matches := []vprocore.ProjectMetadataRow{}
		if species.Text != nil {
			matches = refRows[*species.Text]
		}
		if len(matches) == 0 {
			matches = []vprocore.ProjectMetadataRow{{}}
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
				common := vprocore.ProjectMetadataCell{Storage: "null"}
				origins := []reportcore.VegetationLifeformReferenceOrigin{}
				if ref.RowID != "" {
					if scientific := ref.Cells[rc["ScientificName"]]; scientific.Storage != "null" {
						name = scientific
					}
					origins = append(origins, references.Origins[ref.RowID]...)
					common = ref.Cells[rc["EnglishName"]]
				}
				label := mapping.label
				key := groupKey{layer: reportcore.VegetationTextKey(label), species: reportcore.VegetationTextKey(name)}
				var number *int
				if projection.constancy {
					label = vprocore.ProjectMetadataCell{Storage: "null"}
					number = numericGroups[ref.RowID]
					key.layer, key.rawSpecies = "null", reportcore.VegetationTextKey(species)
					if number != nil {
						value := strconv.Itoa(*number)
						label = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
						key.layer = "integer:" + value
					}
				}
				if showCommon && !projection.unanimousCommon {
					key.common = reportcore.VegetationTextKey(common)
				}
				g := groups[key]
				if g != nil && projection.unanimousCommon && g.sourceCommon != reportcore.VegetationTextKey(common) {
					return fail(fmt.Errorf("summary vegetation First(common name) is ambiguous for literal group %q species %q; all contributing common names must match, including NULL/empty", key.layer, key.species))
				}
				if g == nil {
					g = &group{row: summaryVegetationLayerRow{Layer: vprocore.CloneSiteUnitCell(label),
						Species: vprocore.CloneSiteUnitCell(name), Witnesses: []summaryVegetationLayerWitness{}, numericGroup: number},
						covers: map[string][]vprocore.ProjectMetadataCell{}}
					groups[key] = g
					if projection.constancy {
						raw := vprocore.CloneSiteUnitCell(species)
						g.row.RawSpecies = &raw
					}
					if showCommon {
						value := vprocore.CloneSiteUnitCell(common)
						if projection.unanimousCommon {
							g.sourceCommon = reportcore.VegetationTextKey(common)
							// Original First projects empty TEXT as NULL; provenance remains unchanged.
							if value.Text != nil && *value.Text == "" {
								value = vprocore.ProjectMetadataCell{Storage: "null"}
							}
						}
						g.row.CommonName = &value
					}
				}
				pivot[unit] = true
				g.covers[unit] = append(g.covers[unit], vprocore.CloneSiteUnitCell(cover))
				g.row.Witnesses = append(g.row.Witnesses, summaryVegetationLayerWitness{
					StageRowID: row.RowID, MembershipID: member.RowID, Unit: unit,
					LayerMapID: mapping.id, ReferenceOrigins: origins, RawVegRowIDs: []string{},
					Cover: vprocore.CloneSiteUnitCell(cover),
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
		g.row.Covers = []vprocore.ProjectMetadataCell{}
		g.row.Order = vprocore.ProjectMetadataCell{Storage: "null"}
		for _, unit := range result.Columns {
			if len(g.covers[unit]) == 0 {
				g.row.Covers = append(g.row.Covers, vprocore.ProjectMetadataCell{Storage: "null"})
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
			g.row.Covers = append(g.row.Covers, vprocore.CloneSiteUnitCell(cell.Value))
			if cell.Order.Text != nil {
				if projection.numericOrder {
					value, err := strconv.ParseFloat(*cell.Order.Text, 64)
					if err != nil {
						return fail(fmt.Errorf("summary vegetation numeric Order: %w", err))
					}
					if g.row.Order.Real == nil || value > *g.row.Order.Real {
						g.row.Order = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
					}
				} else if g.row.Order.Text == nil || *cell.Order.Text > *g.row.Order.Text {
					g.row.Order = vprocore.CloneSiteUnitCell(cell.Order)
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
		} else if reportcore.VegetationTextKey(a.Layer) != reportcore.VegetationTextKey(b.Layer) {
			if a.Layer.Text == nil || b.Layer.Text == nil {
				return a.Layer.Text == nil
			}
			return *a.Layer.Text < *b.Layer.Text
		}
		if reportcore.VegetationTextKey(a.Species) != reportcore.VegetationTextKey(b.Species) {
			if a.Species.Text == nil || b.Species.Text == nil {
				return a.Species.Text == nil
			}
			return *a.Species.Text < *b.Species.Text
		}
		if projection.constancy && reportcore.VegetationTextKey(*a.RawSpecies) != reportcore.VegetationTextKey(*b.RawSpecies) {
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
