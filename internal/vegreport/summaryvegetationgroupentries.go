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
	"reflect"
	"strconv"
)

// EntryDat_Veg is a new typed read model, not a rewrite of the source table.
func prepareSummaryVegetationGroupedEntries(ctx context.Context, prepared VegetationReportPreparation,
	union vprocore.ProjectMetadataTable, grouping int) (vprocore.ProjectMetadataTable, error) {
	return prepareSummaryVegetationGroupedEntriesWithInteger(ctx, prepared, union, grouping, vprocore.VegetationLifeformInteger)
}

func prepareSummaryVegetationGroupedEntriesWithInteger(ctx context.Context, prepared VegetationReportPreparation,
	union vprocore.ProjectMetadataTable, grouping int, readInteger func(vprocore.ProjectMetadataCell) (*int, error)) (vprocore.ProjectMetadataTable, error) {
	fail := func(err error) (vprocore.ProjectMetadataTable, error) { return vprocore.ProjectMetadataTable{}, err }
	if ctx == nil {
		return fail(errors.New("summary vegetation grouped entries require a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if grouping != 12 && grouping != 13 && grouping != 14 {
		return fail(errors.New("summary vegetation grouped entries require GroupAndOrder12,13 or14"))
	}
	if prepared.Project == "" || prepared.SU == "" || prepared.SU == "None" {
		return fail(errors.New("summary vegetation grouped entries require an explicit normal SU/project"))
	}
	for _, identity := range []struct {
		name  string
		value string
		limit int
	}{{"ProjectID", prepared.Project, 20}, {"SU", prepared.SU, 0}} {
		cell := vprocore.ProjectMetadataCell{Storage: "text", Text: &identity.value}
		if _, err := vprocore.VegetationReportIdentity(cell); err != nil {
			return fail(fmt.Errorf("summary vegetation grouped %s: %w", identity.name, err))
		}
		if identity.limit > 0 {
			if err := reportcore.LifeformSummaryText(cell, identity.limit); err != nil {
				return fail(fmt.Errorf("summary vegetation grouped %s: %w", identity.name, err))
			}
		}
	}
	if !reflect.DeepEqual(prepared.CoverColumns, longVegetationCoverColumns()) {
		return fail(errors.New("summary vegetation grouped entries require original ordered fifteen MAX cover columns"))
	}
	held := prepared
	held.ReducedRows = []VegetationReportReducedRow{}
	seen := map[string]bool{}
	for _, row := range prepared.ReducedRows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if row.PlotNumber == "" || row.Species.Storage != "text" || row.Species.Text == nil || *row.Species.Text == "" {
			return fail(errors.New("summary vegetation TempReportVeg requires nonempty PlotNumber/Species"))
		}
		for _, field := range []struct {
			name  string
			cell  vprocore.ProjectMetadataCell
			limit int
		}{{"PlotNumber", vprocore.ProjectMetadataCell{Storage: "text", Text: &row.PlotNumber}, 7}, {"Species", row.Species, 8}} {
			if err := reportcore.LifeformSummaryText(field.cell, field.limit); err != nil {
				return fail(fmt.Errorf("summary vegetation TempReportVeg %s: %w", field.name, err))
			}
		}
		key := strconv.Quote(row.PlotNumber) + "/" + reportcore.VegetationTextKey(row.Species)
		if seen[key] || len(row.SourceRowIDs) == 0 || len(row.Covers) != len(held.CoverColumns) {
			return fail(errors.New("summary vegetation TempReportVeg requires distinct complete MAX rows with provenance"))
		}
		seen[key] = true
		ids := map[string]bool{}
		for _, id := range row.SourceRowIDs {
			if id == "" || ids[id] {
				return fail(errors.New("summary vegetation TempReportVeg has empty/duplicate physical source IDs"))
			}
			if _, err := vprocore.VegetationReportIdentity(vprocore.ProjectMetadataCell{Storage: "text", Text: &id}); err != nil {
				return fail(err)
			}
			ids[id] = true
		}
		copyRow := VegetationReportReducedRow{PlotNumber: row.PlotNumber, Species: vprocore.CloneSiteUnitCell(row.Species),
			SourceRowIDs: append([]string{}, row.SourceRowIDs...), Covers: []vprocore.ProjectMetadataCell{}}
		for i, cell := range row.Covers {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			number, err := vprocore.VegetationReportNumber(cell)
			if err != nil {
				return fail(err)
			}
			staged := vprocore.ProjectMetadataCell{Storage: "null"}
			if number != nil {
				value, err := reportcore.LifeformSummarySingle(number)
				if err != nil {
					return fail(fmt.Errorf("summary vegetation TempReportVeg %s SINGLE: %w", held.CoverColumns[i], err))
				}
				staged = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
			}
			copyRow.Covers = append(copyRow.Covers, staged)
		}
		held.ReducedRows = append(held.ReducedRows, copyRow)
	}
	stage := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "Plotnumber", DeclaredType: "TEXT(8)"}, {Name: "Layer", DeclaredType: "TEXT(3)"},
		{Name: "Species", DeclaredType: "TEXT(8)"}, {Name: "Cover", DeclaredType: "SINGLE"},
		{Name: "ProjectID", DeclaredType: "TEXT(20)"},
	}, Rows: []vprocore.ProjectMetadataRow{}}
	appendRow := func(plot string, layer, species, cover, project vprocore.ProjectMetadataCell) {
		stage.Rows = append(stage.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(len(stage.Rows) + 1),
			Cells: []vprocore.ProjectMetadataCell{{Storage: "text", Text: &plot}, vprocore.CloneSiteUnitCell(layer),
				vprocore.CloneSiteUnitCell(species), vprocore.CloneSiteUnitCell(cover), vprocore.CloneSiteUnitCell(project)}})
	}
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	switch grouping {
	case 12:
		strata, err := prepareLongVegetationStrataSingle(ctx, held)
		if err != nil {
			return fail(err)
		}
		for _, row := range strata.Observations {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			appendRow(row.PlotNumber, vprocore.ProjectMetadataCell{Storage: "text", Text: &row.Layer}, row.Species, row.Cover, null)
		}
	case 13:
		lifeforms, err := prepareLongVegetationLifeformsWithInteger(ctx, held, union, readInteger)
		if err != nil {
			return fail(err)
		}
		for _, row := range lifeforms.Observations {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			appendRow(row.PlotNumber, row.Layer, row.Species, row.Cover, null)
		}
	case 14:
		included := map[string]bool{}
		for i := 1; i <= 7; i++ {
			included["Cover"+strconv.Itoa(i)] = true
		}
		for _, row := range held.ReducedRows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			sum := new(big.Rat)
			for i, cell := range row.Covers {
				if !included[held.CoverColumns[i]] {
					continue
				}
				number, err := vprocore.VegetationReportNumber(cell)
				if err != nil {
					return fail(err)
				}
				if number != nil {
					sum.Add(sum, number)
				}
			}
			if sum.Sign() <= 0 {
				continue
			}
			if sum.Cmp(big.NewRat(100, 1)) > 0 {
				sum.SetInt64(100)
			}
			value, err := reportcore.LifeformSummarySingle(sum)
			if err != nil {
				return fail(fmt.Errorf("summary vegetation None EntryDat_Veg.Cover SINGLE: %w", err))
			}
			appendRow(row.PlotNumber, null, row.Species, vprocore.ProjectMetadataCell{Storage: "real", Real: &value},
				vprocore.ProjectMetadataCell{Storage: "text", Text: &prepared.Project})
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return stage, nil
}
