package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"
)

type siteUnitQuickVegetationEntry struct {
	PlotNumber string
	Species    ProjectMetadataCell
	Cover      float64
	Layer      int
	VegRowID   string
	SURowID    string
}

type siteUnitQuickVegetationGroup struct {
	PlotNumber string
	Species    ProjectMetadataCell
	MyCover    float64
	EntryCount int
}

type siteUnitQuickVegetation struct {
	Project     string
	SU          string
	Memberships []VegetationReportMembership
	Entries     []siteUnitQuickVegetationEntry
	Groups      []siteUnitQuickVegetationGroup
}

// QuickVegRecords inserts nine raw covers per physical SU join; unlike
// CreateTempVeg, it never MAX-reduces duplicate Veg rows before insertion.
func prepareSiteUnitQuickVegetation(ctx context.Context, project, suName string, orderBy int, veg, su ProjectMetadataTable) (siteUnitQuickVegetation, error) {
	fail := func(err error) (siteUnitQuickVegetation, error) { return siteUnitQuickVegetation{}, err }
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if project == "" || suName == "" || suName == "None" || !utf8.ValidString(project+suName) ||
		strings.ContainsRune(project+suName, 0) {
		return fail(errors.New("Summary QuickVeg requires explicit valid project/SU identities"))
	}
	if orderBy != 1 && orderBy != 2 {
		return fail(errors.New("Summary QuickVeg strata grouping requires its separate physical LayerCode join"))
	}
	coverNames := []string{"PlotNumber", "Species"}
	for i := 1; i <= 9; i++ {
		coverNames = append(coverNames, fmt.Sprintf("Cover%d", i))
	}
	columns, err := siteUnitTransferColumns(veg, coverNames...)
	if err != nil {
		return fail(fmt.Errorf("Summary QuickVeg physical Veg schema: %w", err))
	}
	suColumns, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Summary QuickVeg physical SU schema: %w", err))
	}
	result := siteUnitQuickVegetation{Project: project, SU: suName,
		Memberships: []VegetationReportMembership{}, Entries: []siteUnitQuickVegetationEntry{},
		Groups: []siteUnitQuickVegetationGroup{}}
	memberships := map[string][]string{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[suColumns["PlotNumber"]], row.Cells[suColumns["SiteUnit"]]
		for _, cell := range []ProjectMetadataCell{plot, unit} {
			if _, err := vegetationReportIdentity(cell); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg SU row %s: %w", row.RowID, err))
			}
		}
		result.Memberships = append(result.Memberships, VegetationReportMembership{
			RowID: row.RowID, PlotNumber: cloneSiteUnitCell(plot), SiteUnit: cloneSiteUnitCell(unit)})
		if plot.Text != nil {
			memberships[*plot.Text] = append(memberships[*plot.Text], row.RowID)
		}
	}
	type groupKey struct{ plot, species string }
	type accumulation struct {
		species ProjectMetadataCell
		sum     *big.Rat
		count   int
	}
	groups := map[groupKey]*accumulation{}
	for layer, column := range coverNames[2:] {
		for _, row := range veg.Rows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			plot, err := vegetationReportIdentity(row.Cells[columns["PlotNumber"]])
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s PlotNumber: %w", row.RowID, err))
			}
			if plot == nil || len(memberships[*plot]) == 0 {
				continue
			}
			value, err := vegetationReportNumber(row.Cells[columns[column]])
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s %s: %w", row.RowID, column, err))
			}
			if value == nil {
				continue
			}
			if err := lifeformSummaryText(row.Cells[columns["PlotNumber"]], 8); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg inserted Plotnumber: %w", err))
			}
			species := row.Cells[columns["Species"]]
			if err := lifeformSummaryText(species, 8); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg inserted Species: %w", err))
			}
			cover, err := lifeformSummarySingle(value)
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s %s: %w", row.RowID, column, err))
			}
			key := groupKey{*plot, vegetationTextKey(species)}
			group := groups[key]
			if group == nil {
				group = &accumulation{species: cloneSiteUnitCell(species), sum: new(big.Rat)}
				groups[key] = group
			}
			for _, suRowID := range memberships[*plot] {
				result.Entries = append(result.Entries, siteUnitQuickVegetationEntry{
					PlotNumber: *plot, Species: cloneSiteUnitCell(species), Cover: cover,
					Layer: layer + 1, VegRowID: row.RowID, SURowID: suRowID})
				group.sum.Add(group.sum, new(big.Rat).SetFloat64(cover))
				group.count++
			}
		}
	}
	keys := make([]groupKey, 0, len(groups))
	for key := range groups {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].plot != keys[j].plot {
			return keys[i].plot < keys[j].plot
		}
		return keys[i].species < keys[j].species
	})
	for _, key := range keys {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		group := groups[key]
		cover := float64(99)
		if group.sum.Cmp(big.NewRat(99, 1)) <= 0 {
			cover, _ = group.sum.Float64()
		}
		result.Groups = append(result.Groups, siteUnitQuickVegetationGroup{
			PlotNumber: key.plot, Species: cloneSiteUnitCell(group.species), MyCover: cover, EntryCount: group.count})
	}
	return result, nil
}
