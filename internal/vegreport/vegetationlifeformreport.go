package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
)

type vegetationLifeformObservation struct {
	PlotNumber string
	Species    vprocore.ProjectMetadataCell
	Layer      vprocore.ProjectMetadataCell
	Cover      vprocore.ProjectMetadataCell
}

type vegetationLifeformPreparation struct {
	Observations []vegetationLifeformObservation
}

func vegetationLifeformInsertionLayer(cell vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataCell, error) {
	return vegetationLifeformInsertionLayerWithInteger(cell, vprocore.VegetationLifeformInteger)
}

func vegetationLifeformInsertionLayerWithInteger(cell vprocore.ProjectMetadataCell,
	readInteger func(vprocore.ProjectMetadataCell) (*int, error)) (vprocore.ProjectMetadataCell, error) {
	value, err := readInteger(cell)
	if err != nil {
		return vprocore.ProjectMetadataCell{}, err
	}
	// SQL Format(NULL, '00') yields empty text before Null2Question.
	formatted := ""
	if value != nil {
		formatted = fmt.Sprintf("%02d", *value)
		if *value < 0 {
			formatted = "-" + fmt.Sprintf("%02d", -*value)
		}
	}
	return vegetationLifeformNull2Question(vprocore.ProjectMetadataCell{Storage: "text", Text: &formatted})
}

func vegetationLifeformNull2Question(cell vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataCell, error) {
	value, err := vprocore.VegetationReportIdentity(cell)
	if err != nil {
		return vprocore.ProjectMetadataCell{}, err
	}
	if value != nil && *value == "" {
		question := "?"
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &question}, nil
	}
	return vprocore.CloneSiteUnitCell(cell), nil
}

// BuildLifeFormTable's active insert (before GoTo MyExit). TempReportVeg MAX
// covers are assigned SINGLE first; SetTo99 has a second SINGLE assignment
// at its parameter boundary, and EntryDat_Veg.Cover has the final assignment.
func prepareLongVegetationLifeforms(ctx context.Context, prepared VegetationReportPreparation,
	union vprocore.ProjectMetadataTable) (vegetationLifeformPreparation, error) {
	return prepareLongVegetationLifeformsWithInteger(ctx, prepared, union, vprocore.VegetationLifeformInteger)
}

func prepareLongVegetationLifeformsWithInteger(ctx context.Context, prepared VegetationReportPreparation,
	union vprocore.ProjectMetadataTable, readInteger func(vprocore.ProjectMetadataCell) (*int, error)) (vegetationLifeformPreparation, error) {
	fail := func(err error) (vegetationLifeformPreparation, error) { return vegetationLifeformPreparation{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	columns, err := reportcore.VegetationLifeformReferenceSourceSchemaWithInteger(ctx, union, "Lifeform", readInteger)
	if err != nil {
		return fail(err)
	}
	index, err := vprocore.SiteUnitTransferIndex(union, columns["Code"])
	if err != nil {
		return fail(err)
	}
	coverIndex := map[string]int{}
	for i, name := range prepared.CoverColumns {
		if _, duplicate := coverIndex[name]; duplicate {
			return fail(errors.New("lifeform preparation has duplicate cover columns"))
		}
		coverIndex[name] = i
	}
	included := longVegetationCoverColumns()[:12]
	for _, name := range included {
		if _, present := coverIndex[name]; !present {
			return fail(fmt.Errorf("lifeform preparation requires %s", name))
		}
	}
	type aggregate struct {
		observation vegetationLifeformObservation
		sum         *big.Rat
	}
	groups := map[string]*aggregate{}
	for _, row := range prepared.ReducedRows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if len(row.Covers) != len(prepared.CoverColumns) {
			return fail(errors.New("lifeform preparation requires complete reduced covers"))
		}
		if _, err := vprocore.VegetationReportIdentity(row.Species); err != nil {
			return fail(err)
		}
		sum := new(big.Rat)
		for i, cell := range row.Covers {
			number, err := vprocore.VegetationReportNumber(cell)
			if err != nil {
				return fail(err)
			}
			if number == nil {
				continue
			}
			first, err := reportcore.LifeformSummarySingle(number)
			if err != nil {
				return fail(fmt.Errorf("TempReportVeg %s: %w", prepared.CoverColumns[i], err))
			}
			for _, name := range included {
				if prepared.CoverColumns[i] == name {
					sum.Add(sum, new(big.Rat).SetFloat64(first))
				}
			}
		}
		parameter, err := reportcore.LifeformSummarySingle(sum)
		if err != nil {
			return fail(fmt.Errorf("SetTo99 SINGLE parameter: %w", err))
		}
		capped := parameter
		if capped > 99.9 {
			capped = 99.9
		}
		refs := []vprocore.ProjectMetadataRow{}
		if row.Species.Text != nil {
			refs = index[*row.Species.Text]
		}
		if len(refs) == 0 {
			refs = []vprocore.ProjectMetadataRow{{}}
		}
		for _, ref := range refs {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			form := vprocore.ProjectMetadataCell{Storage: "null"}
			if len(ref.Cells) > 0 {
				codeType := ref.Cells[columns["Codetype"]]
				if codeType.Text != nil && (*codeType.Text == "s" || *codeType.Text == "S") {
					continue
				}
				form = ref.Cells[columns["Lifeform"]]
			}
			layer, err := vegetationLifeformInsertionLayerWithInteger(form, readInteger)
			if err != nil {
				return fail(err)
			}
			if err := reportcore.LifeformSummaryText(layer, 3); err != nil {
				return fail(fmt.Errorf("EntryDat_Veg.Layer: %w", err))
			}
			key := strconv.Quote(row.PlotNumber) + "/" + reportcore.VegetationTextKey(layer) + "/" + reportcore.VegetationTextKey(row.Species)
			group := groups[key]
			if group == nil {
				group = &aggregate{observation: vegetationLifeformObservation{PlotNumber: row.PlotNumber,
					Species: vprocore.CloneSiteUnitCell(row.Species), Layer: layer}, sum: new(big.Rat)}
				groups[key] = group
			}
			group.sum.Add(group.sum, new(big.Rat).SetFloat64(capped))
		}
	}
	result := vegetationLifeformPreparation{Observations: []vegetationLifeformObservation{}}
	for _, key := range vprocore.SortedEnvironmentReportKeys(groups) {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		group := groups[key]
		final, err := reportcore.LifeformSummarySingle(group.sum)
		if err != nil {
			return fail(fmt.Errorf("EntryDat_Veg.Cover SINGLE: %w", err))
		}
		group.observation.Cover = vprocore.ProjectMetadataCell{Storage: "real", Real: &final}
		result.Observations = append(result.Observations, group.observation)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

// The bridge feeds already joined observations into the proven layer planner,
// without duplicating its physical SU, EnglishName, constant-list or statistics
// kernel. Internal group tokens never escape: report rows retain INTEGER/NULL.
func planLongVegetationLifeforms(ctx context.Context, prepared VegetationReportPreparation,
	master, union, layers vprocore.ProjectMetadataTable, options longVegetationOptions) (vegetationLayerReport, error) {
	fail := func(err error) (vegetationLayerReport, error) { return vegetationLayerReport{}, err }
	if options.NoneGrouping {
		return fail(errors.New("lifeform planner cannot apply None grouping"))
	}
	inserted, err := prepareLongVegetationLifeforms(ctx, prepared, union)
	if err != nil {
		return fail(err)
	}
	mc, err := reportcore.VegetationLifeformReferenceSchema(ctx, master)
	if err != nil {
		return fail(err)
	}
	lc, err := vprocore.SiteUnitTransferColumns(layers, "Layer1234567")
	if err != nil {
		return fail(err)
	}
	mi, err := vprocore.SiteUnitTransferIndex(master, mc["Code"])
	if err != nil {
		return fail(err)
	}
	li, err := vprocore.SiteUnitTransferIndex(layers, lc["Layer1234567"])
	if err != nil {
		return fail(err)
	}
	bridge := prepared
	bridge.Observations = []VegetationReportObservation{}
	bridgeMaster := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Code"}, {Name: "ScientificName"}, {Name: "EnglishName"}, {Name: "Codetype"}}, Rows: []vprocore.ProjectMetadataRow{}}
	bridgeLayers := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Layer1234567"}, {Name: "Layer"}}, Rows: []vprocore.ProjectMetadataRow{}}
	groups := map[string]vprocore.ProjectMetadataCell{}
	diagnostics := []vegetationLayerDiagnostic{}
	for _, observation := range inserted.Observations {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		refs := []vprocore.ProjectMetadataRow{}
		if observation.Species.Text != nil {
			refs = mi[*observation.Species.Text]
		}
		if len(refs) == 0 {
			refs = []vprocore.ProjectMetadataRow{{}}
			diagnostics = append(diagnostics, vegetationLayerDiagnostic{"missing_master_species", reportcore.VegetationTextKey(observation.Species), 1})
		} else if len(refs) > 1 {
			diagnostics = append(diagnostics, vegetationLayerDiagnostic{"master_species_multiplicity", reportcore.VegetationTextKey(observation.Species), len(refs)})
		}
		layerRefs := []vprocore.ProjectMetadataRow{}
		if observation.Layer.Text != nil {
			layerRefs = li[*observation.Layer.Text]
		}
		if len(layerRefs) == 0 {
			layerRefs = []vprocore.ProjectMetadataRow{{}}
			diagnostics = append(diagnostics, vegetationLayerDiagnostic{"missing_layer_label", reportcore.VegetationTextKey(observation.Layer), 1})
		} else if len(layerRefs) > 1 {
			diagnostics = append(diagnostics, vegetationLayerDiagnostic{"layer_label_multiplicity", reportcore.VegetationTextKey(observation.Layer), len(layerRefs)})
		}
		for _, ref := range refs {
			form, english := vprocore.ProjectMetadataCell{Storage: "null"}, vprocore.ProjectMetadataCell{Storage: "null"}
			name := observation.Species
			if len(ref.Cells) > 0 {
				codeType := ref.Cells[mc["Codetype"]]
				if codeType.Text != nil && (*codeType.Text == "S" || *codeType.Text == "s") {
					continue
				}
				form, english = ref.Cells[mc["Lifeform"]], ref.Cells[mc["EnglishName"]]
				if scientific := ref.Cells[mc["ScientificName"]]; scientific.Text != nil {
					name = scientific
				}
			}
			groupKey := "null"
			if form.Integer != nil {
				groupKey = "integer:" + *form.Integer
			}
			if _, present := groups[groupKey]; !present {
				groups[groupKey] = vprocore.CloneSiteUnitCell(form)
				label := vprocore.ProjectMetadataCell{Storage: "null"}
				if form.Integer != nil {
					label = vprocore.ProjectMetadataCell{Storage: "text", Text: &groupKey}
				}
				id := strconv.Itoa(len(bridgeLayers.Rows) + 1)
				token := groupKey
				bridgeLayers.Rows = append(bridgeLayers.Rows, vprocore.ProjectMetadataRow{RowID: id,
					Cells: []vprocore.ProjectMetadataCell{{Storage: "text", Text: &token}, label}})
			}
			for range layerRefs {
				id := strconv.Itoa(len(bridgeMaster.Rows) + 1)
				token := "joined:" + id
				code := vprocore.ProjectMetadataCell{Storage: "null"}
				if name.Text != nil {
					code = vprocore.ProjectMetadataCell{Storage: "text", Text: &token}
					bridgeMaster.Rows = append(bridgeMaster.Rows, vprocore.ProjectMetadataRow{RowID: id,
						Cells: []vprocore.ProjectMetadataCell{code, vprocore.CloneSiteUnitCell(name), vprocore.CloneSiteUnitCell(english), {Storage: "null"}}})
				}
				bridge.Observations = append(bridge.Observations, VegetationReportObservation{
					PlotNumber: observation.PlotNumber, Species: code, Layer: groupKey, Cover: vprocore.CloneSiteUnitCell(observation.Cover)})
			}
		}
	}
	report, err := planLongVegetationLayers(ctx, bridge, bridgeMaster, bridgeLayers, options)
	if err != nil {
		return fail(err)
	}
	for i := range report.Units {
		for j := range report.Units[i].Rows {
			row := &report.Units[i].Rows[j]
			key := "null"
			if row.Layer.Text != nil {
				key = *row.Layer.Text
			}
			row.Layer = vprocore.CloneSiteUnitCell(groups[key])
		}
		sort.SliceStable(report.Units[i].Rows, func(a, b int) bool {
			left, right := report.Units[i].Rows[a], report.Units[i].Rows[b]
			lf, _ := vprocore.VegetationLifeformInteger(left.Layer)
			rf, _ := vprocore.VegetationLifeformInteger(right.Layer)
			if lf == nil || rf == nil {
				if lf != nil || rf != nil {
					return lf == nil
				}
			} else if *lf != *rf {
				return *lf < *rf
			}
			if !options.ConstantSpeciesList && options.Order == "presence" && *left.Presence != *right.Presence {
				return *left.Presence > *right.Presence
			}
			return reportcore.VegetationTextKey(left.Species) < reportcore.VegetationTextKey(right.Species)
		})
	}
	// Synthetic bridge missing-reference diagnostics are not source evidence.
	sourceDiagnostics := diagnostics
	for _, item := range report.Diagnostics {
		if item.Code != "missing_master_species" && item.Code != "missing_layer_label" {
			sourceDiagnostics = append(sourceDiagnostics, item)
		}
	}
	unique := map[string]vegetationLayerDiagnostic{}
	for _, item := range sourceDiagnostics {
		unique[item.Code+"\x00"+item.Identity] = item
	}
	report.Diagnostics = []vegetationLayerDiagnostic{}
	for _, key := range vprocore.SortedEnvironmentReportKeys(unique) {
		report.Diagnostics = append(report.Diagnostics, unique[key])
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return report, nil
}
