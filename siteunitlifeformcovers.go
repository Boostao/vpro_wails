package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
)

type siteUnitLifeformCover struct {
	Lifeform      int
	Caption       string
	Value         string
	PhysicalPlots int
	DistinctPlots int
	ValueCount    int
	Minimum       *float64
	Mean          *float64
	Maximum       *float64
}

type siteUnitLifeformCoverUnit struct {
	Code   string
	NPlots int
	Rows   []siteUnitLifeformCover
}

func planSiteUnitLifeformCovers(ctx context.Context, input siteUnitQuickVegetationInput) ([]siteUnitLifeformCoverUnit, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input.Quick.Project != input.Environment.Report.Project || input.Quick.SU != input.Environment.Report.SU {
		return nil, errors.New("Summary lifeform cover preparation and environment ownership differ")
	}
	columns, err := vegetationLifeformReferenceSchema(ctx, input.References.Table)
	if err != nil {
		return nil, err
	}
	references := map[string][]int{}
	for _, row := range input.References.Table.Rows {
		form, err := vegetationLifeformInteger(row.Cells[columns["Lifeform"]])
		if err != nil {
			return nil, err
		}
		code := row.Cells[columns["Code"]].Text
		if code != nil && form != nil && *form >= 0 && *form <= 13 {
			references[*code] = append(references[*code], *form)
		}
	}
	result := []siteUnitLifeformCoverUnit{}
	seen := map[string]bool{}
	for _, unit := range input.Environment.Report.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[unit.Code] || len(unit.Plots) < 1 {
			return nil, errors.New("Summary lifeform cover units require unique identities and positive owned plot counts")
		}
		seen[unit.Code] = true
		scope := map[string]int{}
		physical := 0
		for _, row := range input.Quick.Memberships {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if row.SiteUnit.Text != nil && *row.SiteUnit.Text == unit.Code && row.PlotNumber.Text != nil {
				scope[*row.PlotNumber.Text]++
				physical++
			}
		}
		type accumulation struct {
			plots map[string]bool
			sum   *big.Rat
			count int
			min   float64
			max   float64
		}
		values := make([]accumulation, 14)
		for form := range values {
			values[form] = accumulation{plots: map[string]bool{}, sum: new(big.Rat)}
		}
		for _, group := range input.Quick.Groups {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if math.IsNaN(group.MyCover) || math.IsInf(group.MyCover, 0) {
				return nil, fmt.Errorf("Summary lifeform cover group %s is not finite", group.PlotNumber)
			}
			if group.Species.Text == nil || scope[group.PlotNumber] == 0 {
				continue
			}
			for _, form := range references[*group.Species.Text] {
				value := &values[form]
				value.plots[group.PlotNumber] = true
				if value.count == 0 || group.MyCover < value.min {
					value.min = group.MyCover
				}
				if value.count == 0 || group.MyCover > value.max {
					value.max = group.MyCover
				}
				weight := scope[group.PlotNumber]
				cover := new(big.Rat).SetFloat64(group.MyCover)
				cover.Mul(cover, new(big.Rat).SetInt64(int64(weight)))
				value.sum.Add(value.sum, cover)
				value.count += weight
			}
		}
		next := siteUnitLifeformCoverUnit{Code: unit.Code, NPlots: len(unit.Plots), Rows: []siteUnitLifeformCover{}}
		for form, value := range values {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row := siteUnitLifeformCover{Lifeform: form, Caption: siteUnitLifeformCaptions[form],
				PhysicalPlots: physical, DistinctPlots: len(value.plots), ValueCount: value.count}
			if value.count > 0 {
				minimum := value.min
				if len(value.plots) < physical {
					minimum = 0
				}
				mean, _ := new(big.Rat).Quo(value.sum, new(big.Rat).SetInt64(int64(len(unit.Plots)))).Float64()
				maximum := value.max
				row.Minimum, row.Mean, row.Maximum = &minimum, &mean, &maximum
			}
			row.Value, err = formatSiteUnitLifeformCover(row)
			if err != nil {
				return nil, err
			}
			next.Rows = append(next.Rows, row)
		}
		result = append(result, next)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
