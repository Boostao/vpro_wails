package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type vegetationNoneUnitOccurrence struct {
	EnvRowID          string
	MembershipRowID   string
	QualityOccurrence *int
	PlotNumber        ProjectMetadataCell
	SiteUnit          ProjectMetadataCell
}

func prepareLongVegetationNoneUnits(ctx context.Context, project, suName string,
	veg, su, layers, env ProjectMetadataTable, quality *vegetationQualitySelection) (VegetationReportPreparation, []vegetationNoneUnitOccurrence, error) {
	fail := func(err error) (VegetationReportPreparation, []vegetationNoneUnitOccurrence, error) {
		return VegetationReportPreparation{}, nil, err
	}
	occurrences, err := planLongVegetationNoneUnits(ctx, env, su, quality)
	if err != nil {
		return fail(err)
	}
	counts := map[string]int{}
	for _, occurrence := range occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		counts[occurrence.MembershipRowID]++
	}
	filtered := su
	filtered.Rows = []ProjectMetadataRow{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if counts[row.RowID] != 0 {
			filtered.Rows = append(filtered.Rows, row)
		}
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, filtered, layers)
	if err != nil {
		return fail(err)
	}
	for i := range prepared.Memberships {
		unit := "All plots"
		prepared.Memberships[i].SiteUnit = ProjectMetadataCell{Storage: "text", Text: &unit}
	}
	prepared.noneUnitMembershipCounts = counts
	return prepared, occurrences, nil
}

// WriteBlankSiteUnits joins Env to the current list, which QCLV may have
// replaced with weighted occurrences. These are derived joins, not physical SU IDs.
func planLongVegetationNoneUnits(ctx context.Context, env, su ProjectMetadataTable,
	quality *vegetationQualitySelection) ([]vegetationNoneUnitOccurrence, error) {
	if ctx == nil {
		return nil, errors.New("None report units require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ec, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("None report units physical Env: %w", err)
	}
	sc, err := siteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("None report units selected SU: %w", err)
	}
	envIDs, members := map[string]ProjectMetadataRow{}, map[string]ProjectMetadataRow{}
	literals := map[string]string{}
	identity := func(cell ProjectMetadataCell) (*string, error) {
		value, err := vegetationReportIdentity(cell)
		if err != nil || value == nil {
			return value, err
		}
		for _, char := range *value {
			if char == 0 || char > 127 {
				return nil, errors.New("None report unit joins require calibrated ASCII plot identities without NUL")
			}
		}
		key := strings.ToLower(*value)
		if previous, exists := literals[key]; exists && previous != *value {
			return nil, errors.New("None report unit joins have unverified distinct case-equivalent plot identities; no literal join was guessed")
		}
		literals[key] = *value
		return value, nil
	}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, err := identity(row.Cells[ec["PlotNumber"]]); err != nil {
			return nil, fmt.Errorf("None report Env row %s: %w", row.RowID, err)
		}
		envIDs[row.RowID] = row
	}
	type joinMember struct {
		id      string
		quality *int
	}
	byPlot := map[string][]joinMember{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot, err := identity(row.Cells[sc["PlotNumber"]])
		if err != nil {
			return nil, fmt.Errorf("None report SU row %s: %w", row.RowID, err)
		}
		if _, err := vegetationReportIdentity(row.Cells[sc["SiteUnit"]]); err != nil {
			return nil, fmt.Errorf("None report SU row %s SiteUnit: %w", row.RowID, err)
		}
		members[row.RowID] = row
		if quality == nil && plot != nil {
			byPlot[*plot] = append(byPlot[*plot], joinMember{id: row.RowID})
		}
	}
	if quality != nil {
		for i, occurrence := range quality.Occurrences {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			member, present := members[occurrence.Membership.RowID]
			parent, envPresent := envIDs[occurrence.EnvRowID]
			if !present || !envPresent ||
				!reflect.DeepEqual(member.Cells[sc["PlotNumber"]], occurrence.Membership.PlotNumber) ||
				!reflect.DeepEqual(member.Cells[sc["SiteUnit"]], occurrence.Membership.SiteUnit) ||
				!reflect.DeepEqual(parent.Cells[ec["PlotNumber"]], occurrence.Membership.PlotNumber) ||
				occurrence.Membership.PlotNumber.Storage != "text" {
				return nil, errors.New("None report quality occurrence is not an original selected SU/Env join")
			}
			index := i
			byPlot[*occurrence.Membership.PlotNumber.Text] = append(byPlot[*occurrence.Membership.PlotNumber.Text],
				joinMember{id: occurrence.Membership.RowID, quality: &index})
		}
	}
	result := []vegetationNoneUnitOccurrence{}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		plot := row.Cells[ec["PlotNumber"]]
		if plot.Storage == "null" {
			continue
		}
		for _, member := range byPlot[*plot.Text] {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			unit := "All plots"
			var occurrence *int
			if member.quality != nil {
				index := *member.quality
				occurrence = &index
			}
			result = append(result, vegetationNoneUnitOccurrence{EnvRowID: row.RowID, MembershipRowID: member.id,
				QualityOccurrence: occurrence, PlotNumber: cloneSiteUnitCell(plot),
				SiteUnit: ProjectMetadataCell{Storage: "text", Text: &unit}})
		}
	}
	return result, nil
}
