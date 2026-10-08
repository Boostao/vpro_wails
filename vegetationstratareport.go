package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type vegetationStratumObservation struct {
	PlotNumber   string
	Species      ProjectMetadataCell
	Layer        string
	Cover        ProjectMetadataCell
	SourceRowIDs []string
}

// Private typed-numeric draft only: Access Nz/Val and SINGLE conversion are not
// calibrated. Do not wire this into the report yet. Layer is an insertion code,
// not a physical reference ID; the later Strata join is separate.
func prepareLongVegetationStrata(ctx context.Context, prepared VegetationReportPreparation) ([]vegetationStratumObservation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if prepared.Project == "" || prepared.SU == "" || prepared.SU == "None" ||
		!utf8.ValidString(prepared.Project+prepared.SU) || strings.ContainsRune(prepared.Project+prepared.SU, 0) {
		return nil, errors.New("strata preparation requires explicit valid project/SU identities")
	}
	expected := longVegetationCoverColumns()
	if len(prepared.CoverColumns) != len(expected) {
		return nil, errors.New("strata preparation requires the original fifteen MAX-reduced cover columns")
	}
	columns := map[string]int{}
	for i, column := range expected {
		if prepared.CoverColumns[i] != column {
			return nil, errors.New("strata preparation requires the original ordered cover columns")
		}
		columns[column] = i
	}
	result := []vegetationStratumObservation{}
	seen := map[string]bool{}
	for _, row := range prepared.ReducedRows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !utf8.ValidString(row.PlotNumber) || strings.ContainsRune(row.PlotNumber, 0) {
			return nil, errors.New("strata preparation contains a malformed plot identity")
		}
		if _, err := vegetationReportIdentity(row.Species); err != nil {
			return nil, fmt.Errorf("strata species identity: %w", err)
		}
		key := row.PlotNumber + "\x00" + vegetationTextKey(row.Species)
		if seen[key] {
			return nil, errors.New("strata preparation requires distinct MAX-reduced plot/species rows")
		}
		seen[key] = true
		if len(row.Covers) != len(expected) || len(row.SourceRowIDs) == 0 {
			return nil, errors.New("strata preparation lost cover columns or physical source provenance")
		}
		ids := map[string]bool{}
		for _, id := range row.SourceRowIDs {
			if id == "" || !utf8.ValidString(id) || strings.ContainsRune(id, 0) || ids[id] {
				return nil, errors.New("strata preparation contains invalid/duplicate physical source IDs")
			}
			ids[id] = true
		}
		for i, cover := range row.Covers {
			if _, err := vegetationReportNumber(cover); err != nil {
				return nil, fmt.Errorf("strata plot %q %s: %w", row.PlotNumber, expected[i], err)
			}
		}
		appendObservation := func(layer string, cover ProjectMetadataCell) {
			result = append(result, vegetationStratumObservation{row.PlotNumber,
				cloneSiteUnitCell(row.Species), layer, cloneSiteUnitCell(cover), append([]string{}, row.SourceRowIDs...)})
		}
		for _, stratum := range []struct {
			layer, total string
			covers       []string
		}{
			{"1", "TotalA", []string{"Cover1", "Cover2", "Cover3"}},
			{"4", "TotalB", []string{"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c"}},
		} {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			cover := row.Covers[columns[stratum.total]]
			if cover.Storage == "null" {
				sum := 0.0
				for _, column := range stratum.covers {
					number, err := vegetationReportNumber(row.Covers[columns[column]])
					if err != nil {
						return nil, err
					}
					if number != nil {
						value, _ := number.Float64()
						sum += value
						if math.IsInf(sum, 0) || math.IsNaN(sum) {
							return nil, fmt.Errorf("strata plot %q %s fallback sum is not finite", row.PlotNumber, stratum.total)
						}
					}
				}
				if sum > 99 {
					sum = 99
				}
				cover = ProjectMetadataCell{Storage: "real", Real: &sum}
			}
			number, err := vegetationReportNumber(cover)
			if err != nil {
				return nil, err
			}
			if number.Sign() > 0 {
				appendObservation(stratum.layer, cover)
			}
		}
		for _, layer := range []string{"6", "7"} {
			cover := row.Covers[columns["Cover"+layer]]
			if cover.Storage != "null" {
				appendObservation(layer, cover)
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
