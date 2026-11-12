package reportcore

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"unicode/utf8"
)

type SiteUnitQuickVegetationEntry struct {
	PlotNumber string
	Species    vprocore.ProjectMetadataCell
	Cover      float64
	Layer      int
	VegRowID   string
	SURowID    string
}

type SiteUnitQuickVegetationGroup struct {
	PlotNumber string
	Species    vprocore.ProjectMetadataCell
	MyCover    float64
	EntryCount int
}

type SiteUnitQuickVegetation struct {
	Project     string
	SU          string
	Memberships []VegetationReportMembership
	Entries     []SiteUnitQuickVegetationEntry
	Groups      []SiteUnitQuickVegetationGroup
}

// QuickVegRecords inserts nine raw covers per physical SU join; unlike
// CreateTempVeg, it never MAX-reduces duplicate Veg rows before insertion.
func PrepareSiteUnitQuickVegetation(ctx context.Context, project, suName string, orderBy int, veg, su vprocore.ProjectMetadataTable) (SiteUnitQuickVegetation, error) {
	fail := func(err error) (SiteUnitQuickVegetation, error) { return SiteUnitQuickVegetation{}, err }
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
	columns, err := vprocore.SiteUnitTransferColumns(veg, coverNames...)
	if err != nil {
		return fail(fmt.Errorf("Summary QuickVeg physical Veg schema: %w", err))
	}
	suColumns, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return fail(fmt.Errorf("Summary QuickVeg physical SU schema: %w", err))
	}
	result := SiteUnitQuickVegetation{Project: project, SU: suName,
		Memberships: []VegetationReportMembership{}, Entries: []SiteUnitQuickVegetationEntry{},
		Groups: []SiteUnitQuickVegetationGroup{}}
	memberships := map[string][]string{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		plot, unit := row.Cells[suColumns["PlotNumber"]], row.Cells[suColumns["SiteUnit"]]
		for _, cell := range []vprocore.ProjectMetadataCell{plot, unit} {
			if _, err := vprocore.VegetationReportIdentity(cell); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg SU row %s: %w", row.RowID, err))
			}
		}
		result.Memberships = append(result.Memberships, VegetationReportMembership{
			RowID: row.RowID, PlotNumber: vprocore.CloneSiteUnitCell(plot), SiteUnit: vprocore.CloneSiteUnitCell(unit)})
		if plot.Text != nil {
			memberships[*plot.Text] = append(memberships[*plot.Text], row.RowID)
		}
	}
	type groupKey struct{ plot, species string }
	type accumulation struct {
		species vprocore.ProjectMetadataCell
		sum     *big.Rat
		count   int
	}
	groups := map[groupKey]*accumulation{}
	for layer, column := range coverNames[2:] {
		for _, row := range veg.Rows {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			plot, err := vprocore.VegetationReportIdentity(row.Cells[columns["PlotNumber"]])
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s PlotNumber: %w", row.RowID, err))
			}
			if plot == nil || len(memberships[*plot]) == 0 {
				continue
			}
			value, err := vprocore.VegetationReportNumber(row.Cells[columns[column]])
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s %s: %w", row.RowID, column, err))
			}
			if value == nil {
				continue
			}
			if err := LifeformSummaryText(row.Cells[columns["PlotNumber"]], 8); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg inserted Plotnumber: %w", err))
			}
			species := row.Cells[columns["Species"]]
			if err := LifeformSummaryText(species, 8); err != nil {
				return fail(fmt.Errorf("Summary QuickVeg inserted Species: %w", err))
			}
			cover, err := LifeformSummarySingle(value)
			if err != nil {
				return fail(fmt.Errorf("Summary QuickVeg Veg row %s %s: %w", row.RowID, column, err))
			}
			key := groupKey{*plot, VegetationTextKey(species)}
			group := groups[key]
			if group == nil {
				group = &accumulation{species: vprocore.CloneSiteUnitCell(species), sum: new(big.Rat)}
				groups[key] = group
			}
			for _, suRowID := range memberships[*plot] {
				result.Entries = append(result.Entries, SiteUnitQuickVegetationEntry{
					PlotNumber: *plot, Species: vprocore.CloneSiteUnitCell(species), Cover: cover,
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
		result.Groups = append(result.Groups, SiteUnitQuickVegetationGroup{
			PlotNumber: key.plot, Species: vprocore.CloneSiteUnitCell(group.species), MyCover: cover, EntryCount: group.count})
	}
	return result, nil
}
