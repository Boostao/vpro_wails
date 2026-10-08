package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"
)

type VegetationReportMembership struct {
	RowID      string              `json:"rowId"`
	PlotNumber ProjectMetadataCell `json:"plotNumber"`
	SiteUnit   ProjectMetadataCell `json:"siteUnit"`
}

type VegetationReportLayer struct {
	RowID       string `json:"rowId"`
	Layer       string `json:"layer"`
	CoverColumn string `json:"coverColumn"`
}

type VegetationReportReducedRow struct {
	PlotNumber   string                `json:"plotNumber"`
	Species      ProjectMetadataCell   `json:"species"`
	SourceRowIDs []string              `json:"sourceRowIds"`
	Covers       []ProjectMetadataCell `json:"covers"`
}

type VegetationReportObservation struct {
	PlotNumber string              `json:"plotNumber"`
	Species    ProjectMetadataCell `json:"species"`
	LayerRowID string              `json:"layerRowId"`
	Layer      string              `json:"layer"`
	Cover      ProjectMetadataCell `json:"cover"`
}

type VegetationReportPreparation struct {
	Project                 string                        `json:"project"`
	SU                      string                        `json:"su"`
	CoverColumns            []string                      `json:"coverColumns"`
	Memberships             []VegetationReportMembership  `json:"memberships"`
	Layers                  []VegetationReportLayer       `json:"layers"`
	ReducedRows             []VegetationReportReducedRow  `json:"reducedRows"`
	Observations            []VegetationReportObservation `json:"observations"`
	qualityMembershipCounts map[string]int
}

func longVegetationCoverColumns() []string {
	return []string{"Cover1", "Cover2", "Cover3", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c",
		"Cover6", "Cover7", "Cover8", "Cover9", "Cover10", "TotalA", "TotalB"}
}

// This is CreateTempVeg plus layer-mode ConvertProVeg2V2, not the report's
// subsequent SU/reference joins, species filters or statistical calculations.
func prepareLongVegetation(ctx context.Context, project, suName string, veg, su, layers ProjectMetadataTable) (VegetationReportPreparation, error) {
	fail := func(err error) (VegetationReportPreparation, error) { return VegetationReportPreparation{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project) ||
		!utf8.ValidString(suName) || strings.ContainsRune(project+suName, 0) {
		return fail(errors.New("Long Vegetation preparation requires explicit valid project/SU identities"))
	}
	result := VegetationReportPreparation{Project: project, SU: suName, CoverColumns: longVegetationCoverColumns(),
		Memberships: []VegetationReportMembership{}, Layers: []VegetationReportLayer{},
		ReducedRows: []VegetationReportReducedRow{}, Observations: []VegetationReportObservation{}}
	vegColumns, err := siteUnitTransferColumns(veg, append([]string{"PlotNumber", "Species"}, result.CoverColumns...)...)
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation physical Veg schema: %w", err))
	}
	suColumns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation selected SU schema: %w", err))
	}
	layerColumns, err := siteUnitTransferColumns(layers, "LayerText")
	if err != nil {
		return fail(fmt.Errorf("Long Vegetation LayerCode schema: %w", err))
	}
	scope := map[string]bool{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[suColumns["PlotNumber"]], row.Cells[suColumns["SiteUnit"]]
		for _, cell := range []ProjectMetadataCell{plot, unit} {
			if _, err := vegetationReportIdentity(cell); err != nil {
				return fail(fmt.Errorf("Long Vegetation SU row %s: %w", row.RowID, err))
			}
		}
		result.Memberships = append(result.Memberships, VegetationReportMembership{row.RowID,
			cloneSiteUnitCell(plot), cloneSiteUnitCell(unit)})
		if plot.Text != nil {
			scope[*plot.Text] = true
		}
	}
	for _, row := range layers.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		layer, err := vegetationReportIdentity(row.Cells[layerColumns["LayerText"]])
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
		plot, err := vegetationReportIdentity(row.Cells[vegColumns["PlotNumber"]])
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation Veg row %s PlotNumber: %w", row.RowID, err))
		}
		if plot == nil || !scope[*plot] {
			continue
		}
		species, err := vegetationReportIdentity(row.Cells[vegColumns["Species"]])
		if err != nil {
			return fail(fmt.Errorf("Long Vegetation Veg row %s Species: %w", row.RowID, err))
		}
		key := groupKey{plot: *plot, nullSpecies: species == nil}
		if species != nil {
			key.species = *species
		}
		group := groups[key]
		if group == nil {
			group = &VegetationReportReducedRow{PlotNumber: *plot, Species: cloneSiteUnitCell(row.Cells[vegColumns["Species"]]),
				SourceRowIDs: []string{}, Covers: make([]ProjectMetadataCell, len(result.CoverColumns))}
			for i := range group.Covers {
				group.Covers[i] = ProjectMetadataCell{Storage: "null"}
			}
			groups[key] = group
		}
		group.SourceRowIDs = append(group.SourceRowIDs, row.RowID)
		for i, column := range result.CoverColumns {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			cell := row.Cells[vegColumns[column]]
			number, err := vegetationReportNumber(cell)
			if err != nil {
				return fail(fmt.Errorf("Long Vegetation Veg row %s %s: %w", row.RowID, column, err))
			}
			if number == nil {
				continue
			}
			current, err := vegetationReportNumber(group.Covers[i])
			if err != nil {
				return fail(err)
			}
			// Numeric ties retain a deterministic original storage value rather
			// than rounding signed64 integers through float64.
			preferTie := cell.Storage < group.Covers[i].Storage ||
				cell.Storage == "real" && group.Covers[i].Storage == "real" && math.Signbit(*group.Covers[i].Real) && !math.Signbit(*cell.Real)
			if current == nil || number.Cmp(current) > 0 || number.Cmp(current) == 0 && preferTie {
				group.Covers[i] = cloneSiteUnitCell(cell)
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
	sort.Slice(result.Memberships, func(i, j int) bool { return result.Memberships[i].RowID < result.Memberships[j].RowID })
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
					cloneSiteUnitCell(row.Species), layer.RowID, layer.Layer, cloneSiteUnitCell(cover)})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}

func vegetationReportIdentity(cell ProjectMetadataCell) (*string, error) {
	if _, err := metadataCellValue(cell); err != nil {
		return nil, err
	}
	if cell.Storage != "text" && cell.Storage != "null" {
		return nil, errors.New("report identities require original text/NULL without coercion")
	}
	return cell.Text, nil
}

func vegetationReportNumber(cell ProjectMetadataCell) (*big.Rat, error) {
	value, err := metadataCellValue(cell)
	if err != nil {
		return nil, err
	}
	switch value := value.(type) {
	case nil:
		return nil, nil
	case int64:
		return new(big.Rat).SetInt64(value), nil
	case float64:
		return new(big.Rat).SetFloat64(value), nil
	default:
		return nil, errors.New("report cover requires original finite numeric/NULL storage; no historical text/blob was converted")
	}
}
