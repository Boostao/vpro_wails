package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
)

type vegetationCrosstabKey struct {
	Group   ProjectMetadataCell
	Species ProjectMetadataCell
}

type vegetationCrosstabInput struct {
	Key        vegetationCrosstabKey
	PlotNumber string
	Cover      ProjectMetadataCell
}

type vegetationCrosstabOptions struct {
	Average              string
	Ungrouped            bool
	ConstantSpeciesList  bool
	Unfiltered           bool
	PresenceGreaterThan  float64
	MeanCoverGreaterThan float64
}

type vegetationCrosstabPlot struct {
	PlotNumber string
	Cover      *float64
}

type vegetationCrosstabRow struct {
	Key       vegetationCrosstabKey
	Presence  *float64
	MeanCover *float64
	Plots     []vegetationCrosstabPlot
}

// Inputs are already joined/transformed observations. The caller must supply
// the independently counted physical SU denominator, not a vegetation count.
func calculateVegetationCrosstab(ctx context.Context, inputs []vegetationCrosstabInput, numPlots int,
	options vegetationCrosstabOptions, constantKeys []vegetationCrosstabKey) ([]vegetationCrosstabRow, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if numPlots <= 0 || options.Average != "all-plots" && options.Average != "observations" {
		return nil, errors.New("vegetation crosstab requires positive physical SU denominator and explicit supported average")
	}
	for _, value := range []float64{options.PresenceGreaterThan, options.MeanCoverGreaterThan} {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.New("vegetation crosstab thresholds must be finite")
		}
	}
	if !options.ConstantSpeciesList && len(constantKeys) > 0 {
		return nil, errors.New("constant species keys require explicit constant-list mode")
	}
	if options.Unfiltered && options.ConstantSpeciesList {
		return nil, errors.New("unfiltered intermediate crosstab cannot apply a constant-list join")
	}
	type identity struct{ group, species string }
	keyIdentity := func(key vegetationCrosstabKey) (identity, error) {
		if options.Ungrouped && key.Group.Storage != "null" {
			return identity{}, errors.New("ungrouped vegetation crosstab requires absent NULL group metadata")
		}
		values := []string{}
		for _, cell := range []ProjectMetadataCell{key.Group, key.Species} {
			value, err := vegetationReportIdentity(cell)
			if err != nil {
				return identity{}, fmt.Errorf("vegetation crosstab key: %w", err)
			}
			tagged := "null"
			if value != nil {
				tagged = "text:" + *value
			}
			values = append(values, tagged)
		}
		return identity{values[0], values[1]}, nil
	}
	type aggregate struct {
		key   vegetationCrosstabKey
		sum   *big.Rat
		count int64
		plots map[string]*big.Rat
	}
	groups := map[identity]*aggregate{}
	plots := map[string]bool{}
	for _, input := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// Empty text remains a literal pivot identity, distinct from NULL.
		if _, err := vegetationReportIdentity(ProjectMetadataCell{Storage: "text", Text: &input.PlotNumber}); err != nil {
			return nil, err
		}
		id, err := keyIdentity(input.Key)
		if err != nil {
			return nil, err
		}
		number, err := vegetationReportNumber(input.Cover)
		if err != nil {
			return nil, fmt.Errorf("vegetation crosstab cover: %w", err)
		}
		group := groups[id]
		if group == nil {
			group = &aggregate{key: vegetationCrosstabKey{cloneSiteUnitCell(input.Key.Group), cloneSiteUnitCell(input.Key.Species)},
				sum: new(big.Rat), plots: map[string]*big.Rat{}}
			groups[id] = group
		}
		plots[input.PlotNumber] = true
		if number != nil {
			if group.plots[input.PlotNumber] == nil {
				group.plots[input.PlotNumber] = new(big.Rat)
			}
			group.plots[input.PlotNumber].Add(group.plots[input.PlotNumber], number)
			group.sum.Add(group.sum, number)
			group.count++
		}
	}
	selected := map[identity]vegetationCrosstabKey{}
	if options.ConstantSpeciesList {
		// ExcelSppList is a separate whole-scope list, not this unit's keys.
		for _, key := range constantKeys {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			id, err := keyIdentity(key)
			if err != nil {
				return nil, err
			}
			if _, duplicate := selected[id]; duplicate {
				return nil, errors.New("constant species list contains duplicate grouped identities")
			}
			selected[id] = key
		}
	} else {
		for id, group := range groups {
			selected[id] = group.key
		}
	}
	ids := make([]identity, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if ids[i].group != ids[j].group {
			return ids[i].group < ids[j].group
		}
		return ids[i].species < ids[j].species
	})
	pivotNames := sortedEnvironmentReportKeys(plots)
	finite := func(number *big.Rat) (*float64, error) {
		value, _ := number.Float64()
		if math.IsInf(value, 0) || math.IsNaN(value) {
			return nil, errors.New("vegetation crosstab calculation exceeds finite output range; no partial result")
		}
		return &value, nil
	}
	result := []vegetationCrosstabRow{}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		key := selected[id]
		row := vegetationCrosstabRow{Key: vegetationCrosstabKey{cloneSiteUnitCell(key.Group), cloneSiteUnitCell(key.Species)},
			Plots: []vegetationCrosstabPlot{}}
		group := groups[id]
		// Source constant-list equality joins do not match SQL NULL keys.
		if options.ConstantSpeciesList && (key.Species.Storage == "null" ||
			!options.Ungrouped && key.Group.Storage == "null") {
			group = nil
		}
		if group != nil {
			presence := float64(len(group.plots)) / float64(numPlots)
			row.Presence = &presence
			if group.count > 0 {
				denominator := int64(numPlots)
				if options.Average == "observations" {
					denominator = group.count
				}
				var err error
				row.MeanCover, err = finite(new(big.Rat).Quo(group.sum, new(big.Rat).SetInt64(denominator)))
				if err != nil {
					return nil, err
				}
			}
			for _, plot := range pivotNames {
				cell := vegetationCrosstabPlot{PlotNumber: plot}
				if cover := group.plots[plot]; cover != nil {
					var err error
					cell.Cover, err = finite(cover)
					if err != nil {
						return nil, err
					}
				}
				row.Plots = append(row.Plots, cell)
			}
		} else {
			for _, plot := range pivotNames {
				row.Plots = append(row.Plots, vegetationCrosstabPlot{PlotNumber: plot})
			}
		}
		// Source applies strict thresholds only outside constant-list mode.
		if options.Unfiltered || options.ConstantSpeciesList || row.MeanCover != nil &&
			*row.MeanCover > options.MeanCoverGreaterThan && *row.Presence > options.PresenceGreaterThan/100 {
			result = append(result, row)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
