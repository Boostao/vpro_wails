package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
)

type siteUnitSpeciesListOptions struct {
	OrderBy             int
	CoverCalculation    int
	AndOr               int
	PresenceGreaterThan int
	CoverGreaterThan    int
}

type siteUnitSpeciesListRow struct {
	Species         string
	ScientificName  ProjectMetadataCell
	EnglishName     ProjectMetadataCell
	CodeType        *ProjectMetadataCell
	Cover           string
	Presence        string
	PhysicalValues  int
	Included        bool
	ReferenceRowIDs []string
}

type siteUnitSpeciesListGroup struct {
	Index   int
	Caption string
	Rows    []siteUnitSpeciesListRow
}

type siteUnitSpeciesListUnit struct {
	Code   string
	NPlots int
	Groups []siteUnitSpeciesListGroup
}

func validateSiteUnitSpeciesListOptions(options siteUnitSpeciesListOptions) error {
	if options.OrderBy != 1 && options.OrderBy != 2 {
		return errors.New("Summary species-list strata requires the separate physical LayerCode preparation")
	}
	if options.CoverCalculation != 1 && options.CoverCalculation != 2 || options.AndOr != 1 && options.AndOr != 2 {
		return errors.New("Summary species-list requires explicit all/present cover calculation and AND/OR choices")
	}
	for _, threshold := range []int{options.PresenceGreaterThan, options.CoverGreaterThan} {
		if threshold < -32768 || threshold > 32767 {
			return errors.New("Summary species-list threshold exceeds source INTEGER range")
		}
	}
	return nil
}

func planSiteUnitSpeciesList(ctx context.Context, input siteUnitQuickVegetationInput,
	options siteUnitSpeciesListOptions) ([]siteUnitSpeciesListUnit, error) {
	if ctx == nil {
		return nil, errors.New("Summary species-list requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateSiteUnitSpeciesListOptions(options); err != nil {
		return nil, err
	}
	if input.Quick.Project != input.Environment.Report.Project || input.Quick.SU != input.Environment.Report.SU {
		return nil, errors.New("Summary species-list preparation and Environment ownership differ")
	}
	columns, err := vegetationLifeformReferenceSchema(ctx, input.References.Table)
	if err != nil {
		return nil, err
	}
	type reference struct {
		row  ProjectMetadataRow
		form *int
	}
	references := map[string][]reference{}
	for _, row := range input.References.Table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		code, kind := row.Cells[columns["Code"]], row.Cells[columns["Codetype"]]
		if code.Text == nil || kind.Text == nil || *kind.Text == "s" || *kind.Text == "S" {
			continue
		}
		form, err := vegetationLifeformInteger(row.Cells[columns["Lifeform"]])
		if err != nil {
			return nil, err
		}
		references[*code.Text] = append(references[*code.Text], reference{row, form})
	}
	result := []siteUnitSpeciesListUnit{}
	seen := map[string]bool{}
	for _, unit := range input.Environment.Report.Units {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if seen[unit.Code] || len(unit.Plots) < 1 {
			return nil, errors.New("Summary species-list requires unique units and positive owned plot counts")
		}
		seen[unit.Code] = true
		scope := map[string]int{}
		for _, membership := range input.Quick.Memberships {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if membership.SiteUnit.Text != nil && *membership.SiteUnit.Text == unit.Code && membership.PlotNumber.Text != nil {
				scope[*membership.PlotNumber.Text]++
			}
		}
		type key struct {
			index                        int
			species, scientific, english string
			kind                         string
		}
		type accumulation struct {
			row   siteUnitSpeciesListRow
			sum   *big.Rat
			count int
			refs  map[string]bool
		}
		values := map[key]*accumulation{}
		add := func(plot string, species ProjectMetadataCell, cover float64, layer int) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if math.IsNaN(cover) || math.IsInf(cover, 0) {
				return errors.New("Summary species-list cover must be finite")
			}
			if species.Text == nil || scope[plot] == 0 || options.OrderBy == 1 && (layer < 1 || layer > 7) {
				return nil
			}
			for _, ref := range references[*species.Text] {
				if err := ctx.Err(); err != nil {
					return err
				}
				index := layer
				if options.OrderBy == 2 {
					if ref.form == nil || *ref.form < 0 || *ref.form > 13 {
						continue
					}
					index = *ref.form
				}
				scientific, english, kind := ref.row.Cells[columns["ScientificName"]], ref.row.Cells[columns["EnglishName"]], ref.row.Cells[columns["Codetype"]]
				groupKey := key{index, *species.Text, vegetationTextKey(scientific), vegetationTextKey(english), ""}
				if options.OrderBy == 2 {
					groupKey.kind = vegetationTextKey(kind)
				}
				value := values[groupKey]
				if value == nil {
					value = &accumulation{row: siteUnitSpeciesListRow{
						Species: *species.Text, ScientificName: cloneSiteUnitCell(scientific),
						EnglishName:     cloneSiteUnitCell(english),
						ReferenceRowIDs: []string{}}, sum: new(big.Rat), refs: map[string]bool{}}
					values[groupKey] = value
					if options.OrderBy == 2 {
						codeType := cloneSiteUnitCell(kind)
						value.row.CodeType = &codeType
					}
				}
				weight := scope[plot]
				if weight > int(^uint(0)>>1)-value.count {
					return errors.New("Summary species-list physical value count overflows")
				}
				value.sum.Add(value.sum, new(big.Rat).Mul(new(big.Rat).SetFloat64(cover), new(big.Rat).SetInt64(int64(weight))))
				value.count += weight
				if !value.refs[ref.row.RowID] {
					value.row.ReferenceRowIDs = append(value.row.ReferenceRowIDs, ref.row.RowID)
					value.refs[ref.row.RowID] = true
				}
			}
			return nil
		}
		if options.OrderBy == 1 {
			for _, entry := range input.Quick.Entries {
				if err := add(entry.PlotNumber, entry.Species, entry.Cover, entry.Layer); err != nil {
					return nil, err
				}
			}
		} else {
			for _, group := range input.Quick.Groups {
				if err := add(group.PlotNumber, group.Species, group.MyCover, 0); err != nil {
					return nil, err
				}
			}
		}
		keys := make([]key, 0, len(values))
		for groupKey := range values {
			keys = append(keys, groupKey)
		}
		sort.Slice(keys, func(i, j int) bool {
			left, right := keys[i], keys[j]
			if left.index != right.index {
				return left.index < right.index
			}
			for _, pair := range [][2]string{{left.species, right.species}, {left.scientific, right.scientific}, {left.english, right.english}} {
				if pair[0] != pair[1] {
					return pair[0] < pair[1]
				}
			}
			return left.kind < right.kind
		})
		next := siteUnitSpeciesListUnit{Code: unit.Code, NPlots: len(unit.Plots), Groups: []siteUnitSpeciesListGroup{}}
		for _, groupKey := range keys {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			value := values[groupKey]
			denominator := len(unit.Plots)
			if options.CoverCalculation == 2 {
				denominator = value.count
			}
			cover, _ := new(big.Rat).Quo(value.sum, new(big.Rat).SetInt64(int64(denominator))).Float64()
			presenceRatio := new(big.Rat).SetFrac64(int64(value.count), int64(len(unit.Plots)))
			presence, _ := presenceRatio.Mul(presenceRatio, new(big.Rat).SetInt64(100)).Float64()
			if math.IsInf(cover, 0) || math.IsNaN(cover) || math.IsInf(presence, 0) || math.IsNaN(presence) {
				return nil, errors.New("Summary species-list summary exceeds finite source values")
			}
			value.row.Cover, value.row.Presence = siteUnitFixedNumber(cover, 2, 1), siteUnitFixedNumber(presence, 1, 1)
			formattedCover, okCover := new(big.Rat).SetString(value.row.Cover)
			formattedPresence, okPresence := new(big.Rat).SetString(value.row.Presence)
			if !okCover || !okPresence {
				return nil, errors.New("Summary species-list formatting is not a complete numeric value")
			}
			coverPass := formattedCover.Cmp(new(big.Rat).SetInt64(int64(options.CoverGreaterThan))) > 0
			presencePass := formattedPresence.Cmp(new(big.Rat).SetInt64(int64(options.PresenceGreaterThan))) > 0
			value.row.Included = coverPass && presencePass
			if options.AndOr == 2 {
				value.row.Included = coverPass || presencePass
			}
			value.row.PhysicalValues = value.count
			sort.Strings(value.row.ReferenceRowIDs)
			if len(next.Groups) == 0 || next.Groups[len(next.Groups)-1].Index != groupKey.index {
				caption := fmt.Sprintf("Layer %d", groupKey.index)
				if options.OrderBy == 2 {
					caption = siteUnitLifeformCaptions[groupKey.index]
				}
				next.Groups = append(next.Groups, siteUnitSpeciesListGroup{groupKey.index, caption, []siteUnitSpeciesListRow{}})
			}
			last := &next.Groups[len(next.Groups)-1]
			last.Rows = append(last.Rows, value.row)
		}
		result = append(result, next)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
