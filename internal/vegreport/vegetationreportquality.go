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
	"strings"
)

type vegetationQualityCriterion struct {
	Minimum     string
	IncludeNull bool
}

type vegetationQualityOccurrence struct {
	Membership reportcore.VegetationReportMembership
	EnvRowID   string
	AdminRowID string
	ListRowIDs [3]*string
	Values     [3]vprocore.ProjectMetadataCell
}

type vegetationQualityReference struct {
	RowID     string
	Item      vprocore.ProjectMetadataCell
	ListName  vprocore.ProjectMetadataCell
	ItemOrder vprocore.ProjectMetadataCell
}

type vegetationQualitySelection struct {
	ThresholdRowIDs       [3][]string
	Occurrences           []vegetationQualityOccurrence
	ExcludedMembershipIDs []string
	References            []vegetationQualityReference
}

func prepareQualityLongVegetation(ctx context.Context, project, suName string, veg, su, layers, env, admin, lists vprocore.ProjectMetadataTable,
	criteria [3]vegetationQualityCriterion) (VegetationReportPreparation, vegetationQualitySelection, error) {
	fail := func(err error) (VegetationReportPreparation, vegetationQualitySelection, error) {
		return VegetationReportPreparation{}, vegetationQualitySelection{}, err
	}
	selection, err := planLongVegetationQuality(ctx, su, env, admin, lists, criteria)
	if err != nil {
		return fail(err)
	}
	weights := map[string]int{}
	for _, occurrence := range selection.Occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		weights[occurrence.Membership.RowID]++
	}
	filtered := su
	filtered.Rows = []vprocore.ProjectMetadataRow{}
	for _, row := range su.Rows {
		if weights[row.RowID] > 0 {
			filtered.Rows = append(filtered.Rows, row)
		}
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, filtered, layers)
	if err != nil {
		return fail(err)
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	prepared.qualityMembershipCounts = weights
	return prepared, selection, nil
}

// QCLV ranks against DataQuality, not the oppositely ordered editor list.
// Keep reference fanout explicit; these are not new physical SU records.
func planLongVegetationQuality(ctx context.Context, su, env, admin, lists vprocore.ProjectMetadataTable,
	criteria [3]vegetationQualityCriterion) (vegetationQualitySelection, error) {
	fail := func(err error) (vegetationQualitySelection, error) { return vegetationQualitySelection{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	fields := [3]string{"SitePlotQuality", "VegPlotQuality", "SoilPlotQuality"}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(err)
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return fail(err)
	}
	ac, err := vprocore.SiteUnitTransferColumns(admin, "Plot", fields[0], fields[1], fields[2])
	if err != nil {
		return fail(err)
	}
	lc, err := vprocore.SiteUnitTransferColumns(lists, "Item", "ListName", "ItemOrder")
	if err != nil {
		return fail(err)
	}
	envIndex, err := vprocore.SiteUnitTransferIndex(env, ec["PlotNumber"])
	if err != nil {
		return fail(err)
	}
	adminIndex, err := vprocore.SiteUnitTransferIndex(admin, ac["Plot"])
	if err != nil {
		return fail(err)
	}
	cache := map[string][]vprocore.ProjectMetadataRow{}
	used := map[string]vprocore.ProjectMetadataRow{}
	referencesFor := func(code string) ([]vprocore.ProjectMetadataRow, error) {
		if rows, present := cache[code]; present {
			return rows, ctx.Err()
		}
		matches := []vprocore.ProjectMetadataRow{}
		for _, row := range lists.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			item, err := vprocore.VegetationReportIdentity(row.Cells[lc["Item"]])
			if err != nil {
				return nil, err
			}
			if item != nil && strings.EqualFold(*item, code) {
				matches = append(matches, row)
				used[row.RowID] = row
			}
		}
		cache[code] = matches
		return matches, nil
	}
	result := vegetationQualitySelection{Occurrences: []vegetationQualityOccurrence{}, ExcludedMembershipIDs: []string{}}
	var thresholds [3]*big.Rat
	for i, criterion := range criteria {
		result.ThresholdRowIDs[i] = []string{}
		references, err := referencesFor(criterion.Minimum)
		if err != nil {
			return fail(err)
		}
		for _, row := range references {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			domain, err := vprocore.VegetationReportIdentity(row.Cells[lc["ListName"]])
			if err != nil {
				return fail(err)
			}
			if domain == nil || !strings.EqualFold(*domain, "DataQuality") {
				continue
			}
			value, err := vprocore.VegetationReportNumber(row.Cells[lc["ItemOrder"]])
			if err != nil {
				return fail(fmt.Errorf("%s threshold row %s ItemOrder: %w", fields[i], row.RowID, err))
			}
			if value == nil || !value.IsInt() || value.Cmp(big.NewRat(-32768, 1)) < 0 || value.Cmp(big.NewRat(32767, 1)) > 0 {
				return fail(fmt.Errorf("%s threshold row %s requires an exact source Integer ItemOrder; VBA rounding is not implemented", fields[i], row.RowID))
			}
			if thresholds[i] != nil && thresholds[i].Cmp(value) != 0 {
				return fail(fmt.Errorf("%s threshold %q has conflicting DataQuality definitions; no first row was chosen", fields[i], criterion.Minimum))
			}
			thresholds[i] = value
			result.ThresholdRowIDs[i] = append(result.ThresholdRowIDs[i], row.RowID)
		}
		if thresholds[i] == nil {
			return fail(fmt.Errorf("%s threshold %q has no DataQuality definition", fields[i], criterion.Minimum))
		}
	}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[sc["PlotNumber"]], row.Cells[sc["SiteUnit"]]
		for _, cell := range []vprocore.ProjectMetadataCell{plot, unit} {
			if _, err := vprocore.VegetationReportIdentity(cell); err != nil {
				return fail(err)
			}
		}
		var sources, parents []vprocore.ProjectMetadataRow
		if plot.Text != nil {
			sources, parents = envIndex[*plot.Text], adminIndex[*plot.Text]
		}
		if len(sources) > 1 || len(parents) > 1 {
			return fail(errors.New("plot-quality qualification requires unambiguous physical ENV/Admin links; no DISTINCT projection was guessed"))
		}
		before := len(result.Occurrences)
		if len(sources) == 1 && len(parents) == 1 {
			var matches [3][]*string
			for i, field := range fields {
				code, err := vprocore.VegetationReportIdentity(parents[0].Cells[ac[field]])
				if err != nil {
					return fail(fmt.Errorf("Admin row %s %s: %w", parents[0].RowID, field, err))
				}
				var references []vprocore.ProjectMetadataRow
				if code != nil {
					references, err = referencesFor(*code)
					if err != nil {
						return fail(err)
					}
				}
				if len(references) == 0 {
					if criteria[i].IncludeNull {
						matches[i] = append(matches[i], nil)
					}
					continue
				}
				for _, reference := range references {
					if err := ctx.Err(); err != nil {
						return fail(err)
					}
					domain, err := vprocore.VegetationReportIdentity(reference.Cells[lc["ListName"]])
					if err != nil {
						return fail(err)
					}
					// The source filters ListName after joining the whole list.
					// A match only in another domain is not a missing reference.
					if domain != nil && !strings.EqualFold(*domain, "DataQuality") {
						continue
					}
					order, err := vprocore.VegetationReportNumber(reference.Cells[lc["ItemOrder"]])
					if err != nil {
						return fail(fmt.Errorf("quality reference row %s ItemOrder: %w", reference.RowID, err))
					}
					if order == nil && criteria[i].IncludeNull || order != nil && order.Cmp(thresholds[i]) >= 0 {
						id := reference.RowID
						matches[i] = append(matches[i], &id)
					}
				}
			}
			for _, site := range matches[0] {
				for _, veg := range matches[1] {
					for _, soil := range matches[2] {
						if err := ctx.Err(); err != nil {
							return fail(err)
						}
						ids := [3]*string{}
						values := [3]vprocore.ProjectMetadataCell{}
						for i, id := range [3]*string{site, veg, soil} {
							values[i] = vprocore.CloneSiteUnitCell(parents[0].Cells[ac[fields[i]]])
							if id != nil {
								value := *id
								ids[i] = &value
							}
						}
						result.Occurrences = append(result.Occurrences, vegetationQualityOccurrence{
							Membership: reportcore.VegetationReportMembership{RowID: row.RowID, PlotNumber: vprocore.CloneSiteUnitCell(plot), SiteUnit: vprocore.CloneSiteUnitCell(unit)},
							EnvRowID:   sources[0].RowID, AdminRowID: parents[0].RowID, ListRowIDs: ids, Values: values})
					}
				}
			}
		}
		if len(result.Occurrences) == before {
			result.ExcludedMembershipIDs = append(result.ExcludedMembershipIDs, row.RowID)
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	result.References = make([]vegetationQualityReference, 0, len(used))
	for _, row := range used {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		result.References = append(result.References, vegetationQualityReference{row.RowID,
			vprocore.CloneSiteUnitCell(row.Cells[lc["Item"]]), vprocore.CloneSiteUnitCell(row.Cells[lc["ListName"]]),
			vprocore.CloneSiteUnitCell(row.Cells[lc["ItemOrder"]])})
	}
	sort.Slice(result.References, func(i, j int) bool { return result.References[i].RowID < result.References[j].RowID })
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
