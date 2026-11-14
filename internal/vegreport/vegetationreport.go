package vegreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type VegetationReportLayer struct {
	RowID       string `json:"rowId"`
	Layer       string `json:"layer"`
	CoverColumn string `json:"coverColumn"`
}

type VegetationReportReducedRow struct {
	PlotNumber   string                         `json:"plotNumber"`
	Species      vprocore.ProjectMetadataCell   `json:"species"`
	SourceRowIDs []string                       `json:"sourceRowIds"`
	Covers       []vprocore.ProjectMetadataCell `json:"covers"`
}

type VegetationReportObservation struct {
	PlotNumber string                       `json:"plotNumber"`
	Species    vprocore.ProjectMetadataCell `json:"species"`
	LayerRowID string                       `json:"layerRowId"`
	Layer      string                       `json:"layer"`
	Cover      vprocore.ProjectMetadataCell `json:"cover"`
}

type VegetationReportPreparation struct {
	Project                  string                                  `json:"project"`
	SU                       string                                  `json:"su"`
	CoverColumns             []string                                `json:"coverColumns"`
	Memberships              []reportcore.VegetationReportMembership `json:"memberships"`
	Layers                   []VegetationReportLayer                 `json:"layers"`
	ReducedRows              []VegetationReportReducedRow            `json:"reducedRows"`
	Observations             []VegetationReportObservation           `json:"observations"`
	qualityMembershipCounts  map[string]int
	noneUnitMembershipCounts map[string]int
	selectedUnitOccurrences  []vegetationSelectedUnitOccurrence
	projectUnitInputs        []vegetationProjectUnitInput
}

type vegetationReportEntries struct {
	CoverColumns []string
	Layers       []VegetationReportLayer
	ReducedRows  []VegetationReportReducedRow
	Observations []VegetationReportObservation
}

func longVegetationCoverColumns() []string {
	return []string{"Cover1", "Cover2", "Cover3", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c",
		"Cover6", "Cover7", "Cover8", "Cover9", "Cover10", "TotalA", "TotalB"}
}

// This is CreateTempVeg plus layer-mode ConvertProVeg2V2, not the report's
// subsequent SU/reference joins, species filters or statistical calculations.
func prepareLongVegetation(ctx context.Context, project, suName string, veg, su, layers vprocore.ProjectMetadataTable) (VegetationReportPreparation, error) {
	fail := func(err error) (VegetationReportPreparation, error) { return VegetationReportPreparation{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project) ||
		!utf8.ValidString(suName) || strings.ContainsRune(project+suName, 0) {
		return fail(errors.New("Long Vegetation preparation requires explicit valid project/SU identities"))
	}
	result := VegetationReportPreparation{Project: project, SU: suName, CoverColumns: longVegetationCoverColumns(),
		Memberships: []reportcore.VegetationReportMembership{}, Layers: []VegetationReportLayer{},
		ReducedRows: []VegetationReportReducedRow{}, Observations: []VegetationReportObservation{}}
	vegColumns, err := vprocore.SiteUnitTransferColumns(veg, append([]string{"PlotNumber", "Species"}, result.CoverColumns...)...)
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation physical Veg schema: %w", err))
	}
	suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation selected SU schema: %w", err))
	}
	layerColumns, err := vprocore.SiteUnitTransferColumns(layers, "LayerText")
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation LayerCode schema: %w", err))
	}
	scope := map[string]bool{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[suColumns["PlotNumber"]], row.Cells[suColumns["SiteUnit"]]
		for _, cell := range []vprocore.ProjectMetadataCell{plot, unit} {
			if _, err := vprocore.VegetationReportIdentity(cell); err != nil {
				return fail(fmt.Errorf("Long Vegetation SU row %s: %w", row.RowID, err))
			}
		}
		result.Memberships = append(result.Memberships, reportcore.VegetationReportMembership{RowID: row.RowID,
			PlotNumber: vprocore.CloneSiteUnitCell(plot), SiteUnit: vprocore.CloneSiteUnitCell(unit)})
		if plot.Text != nil {
			scope[*plot.Text] = true
		}
	}
	entries, err := reduceLongVegetationEntries(ctx, veg, layers, vegColumns, layerColumns, scope)
	if err != nil {
		return fail(err)
	}
	result.CoverColumns, result.Layers = entries.CoverColumns, entries.Layers
	result.ReducedRows, result.Observations = entries.ReducedRows, entries.Observations
	sort.Slice(result.Memberships, func(i, j int) bool { return result.Memberships[i].RowID < result.Memberships[j].RowID })
	return result, nil
}

func reduceLongVegetationEntries(ctx context.Context, veg, layers vprocore.ProjectMetadataTable,
	vegColumns, layerColumns map[string]int, scope map[string]bool) (vegetationReportEntries, error) {
	fail := func(err error) (vegetationReportEntries, error) { return vegetationReportEntries{}, err }
	result := vegetationReportEntries{CoverColumns: longVegetationCoverColumns(),
		Layers: []VegetationReportLayer{}, ReducedRows: []VegetationReportReducedRow{},
		Observations: []VegetationReportObservation{}}
	for _, row := range layers.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		layer, err := vprocore.VegetationReportIdentity(row.Cells[layerColumns["LayerText"]])
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation LayerCode row %s: %w", row.RowID, err))
		}
		if layer == nil {
			continue
		}
		column := ""
		for _, cover := range result.CoverColumns {
			if strings.EqualFold(cover, "Cover"+*layer) {
				column = cover
				break
			}
		}
		if column == "" {
			return fail(fmt.Errorf("Long Vegetation LayerCode row %s has unsupported literal LayerText %q; no mapping guessed", row.RowID, *layer))
		}
		result.Layers = append(result.Layers, VegetationReportLayer{row.RowID, *layer, column})
	}
	type groupKey struct {
		plot        string
		species     string
		nullSpecies bool
	}
	groups := map[groupKey]*VegetationReportReducedRow{}
	for _, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, err := vprocore.VegetationReportIdentity(row.Cells[vegColumns["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation Veg row %s PlotNumber: %w", row.RowID, err))
		}
		if plot == nil || !scope[*plot] {
			continue
		}
		species, err := vprocore.VegetationReportIdentity(row.Cells[vegColumns["Species"]])
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation Veg row %s Species: %w", row.RowID, err))
		}
		key := groupKey{plot: *plot, nullSpecies: species == nil}
		if species != nil {
			key.species = *species
		}
		group := groups[key]
		if group == nil {
			group = &VegetationReportReducedRow{PlotNumber: *plot, Species: vprocore.CloneSiteUnitCell(row.Cells[vegColumns["Species"]]),
				SourceRowIDs: []string{}, Covers: make([]vprocore.ProjectMetadataCell, len(result.CoverColumns))}
			for i := range group.Covers {
				group.Covers[i] = vprocore.ProjectMetadataCell{Storage: "null"}
			}
			groups[key] = group
		}
		group.SourceRowIDs = append(group.SourceRowIDs, row.RowID)
		for i, column := range result.CoverColumns {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			cell := row.Cells[vegColumns[column]]
			number, err := vprocore.VegetationReportNumber(cell)
			if err != nil {
				return fail(fmt.Errorf("Long Vegetation Veg row %s %s: %w", row.RowID, column, err))
			}
			if number == nil {
				continue
			}
			current, err := vprocore.VegetationReportNumber(group.Covers[i])
			if err != nil {
				return fail(err)
			}
			// Numeric ties retain a deterministic original storage value rather
			// than rounding signed64 integers through float64.
			preferTie := cell.Storage < group.Covers[i].Storage ||
				cell.Storage == "real" && group.Covers[i].Storage == "real" && math.Signbit(*group.Covers[i].Real) && !math.Signbit(*cell.Real)
			if current == nil || number.Cmp(current) > 0 || number.Cmp(current) == 0 && preferTie {
				group.Covers[i] = vprocore.CloneSiteUnitCell(cell)
			}
		}
	}
	for _, group := range groups {
		sort.Strings(group.SourceRowIDs)
		result.ReducedRows = append(result.ReducedRows, *group)
	}
	sort.Slice(result.ReducedRows, func(i, j int) bool {
		a, b := result.ReducedRows[i], result.ReducedRows[j]
		if a.PlotNumber != b.PlotNumber {
			return a.PlotNumber < b.PlotNumber
		}
		if a.Species.Storage != b.Species.Storage {
			return a.Species.Storage < b.Species.Storage
		}
		return a.Species.Text != nil && *a.Species.Text < *b.Species.Text
	})
	sort.Slice(result.Layers, func(i, j int) bool { return result.Layers[i].RowID < result.Layers[j].RowID })
	coverIndex := map[string]int{}
	for i, column := range result.CoverColumns {
		coverIndex[column] = i
	}
	for _, row := range result.ReducedRows {
		for _, layer := range result.Layers {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			cover := row.Covers[coverIndex[layer.CoverColumn]]
			if cover.Storage != "null" {
				result.Observations = append(result.Observations, VegetationReportObservation{row.PlotNumber,
					vprocore.CloneSiteUnitCell(row.Species), layer.RowID, layer.Layer, vprocore.CloneSiteUnitCell(cover)})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func PreviewLongVegetationLayers(s *vprocore.ContextService, ctx context.Context, contextID string) (vegetationLayerReport, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (vegetationLayerReport, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (vegetationLayerReport, error) {
			values, err := plots.Projects.Preferences.Snapshot()
			if err != nil {
				return vegetationLayerReport{}, err
			}
			options, err := DecodeOwnedLongVegetationOptions(s, ctx, plots, tx, values)
			if err != nil {
				return vegetationLayerReport{}, err
			}
			if err := CheckLongVegetationGrouping(s, options); err != nil {
				return vegetationLayerReport{}, err
			}
			source, err := readLongVegetationReportSource(ctx, owner, tx, options)
			return source.Report, err
		})
	})
}

func readLongVegetationLayers(ctx context.Context, plots *vprocore.PlotService, options longVegetationOptions) (result vegetationLayerReport, resultErr error) {
	owner := plots.Projects.Sqlite
	if owner.Selection.SU == "None" {
		return result, errors.New("Long Vegetation requires an explicitly selected SU, not an all-project fallback")
	}
	if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
		return result, err
	}
	defer owner.Mu.Unlock()
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return result, err
	}
	tx, err := owner.BeginReadSnapshot(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			resultErr = errors.Join(resultErr, err)
		}
		if resultErr != nil {
			result = vegetationLayerReport{}
		}
	}()
	source, err := readLongVegetationReportSource(ctx, owner, tx, options)
	if err != nil {
		return result, err
	}
	result = source.Report
	if err := vprocore.ProfileOwnedFiles(owner); err != nil {
		return result, err
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
