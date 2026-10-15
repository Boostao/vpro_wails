package main

import (
	"context"
	"errors"
	"reflect"
)

func vegetationWorkbookSelectedUnitSummary(ctx context.Context, summary vegetationWorkbookSummary,
	env, admin, su ProjectMetadataTable, names []string, report LongVegetationReport) (vegetationWorkbookSummary, error) {
	fail := func(err error) (vegetationWorkbookSummary, error) { return vegetationWorkbookSummary{}, err }
	if ctx == nil {
		return fail(errors.New("selected report unit summary requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(report.SelectedUnits) == 0 || len(report.SelectedUnits) > 1048572 || len(report.NoneUnits) != 0 {
		return fail(errors.New("selected report unit summary requires nonempty bounded exclusive derived witnesses"))
	}
	var quality *vegetationQualitySelection
	if report.Quality != nil {
		quality = &vegetationQualitySelection{}
		for _, row := range report.Quality.Occurrences {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			quality.Occurrences = append(quality.Occurrences, vegetationQualityOccurrence{
				Membership: VegetationReportMembership{RowID: row.MembershipID, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit},
				EnvRowID:   row.EnvRowID, AdminRowID: row.AdminRowID,
			})
		}
	}
	expected, err := planSelectedUnitProjection(ctx, env, admin, su, names, quality, true)
	if err != nil {
		return fail(err)
	}
	if len(expected) != len(report.SelectedUnits) {
		return fail(errors.New("selected report unit summary lost complete original SU/Env/Admin join witnesses"))
	}
	physical := map[string]vegetationWorkbookMembership{}
	for _, member := range summary.Memberships {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if _, duplicate := physical[member.RowID]; duplicate {
			return fail(errors.New("selected report unit summary repeats original membership identity"))
		}
		physical[member.RowID] = member
	}
	summary.Memberships = make([]vegetationWorkbookMembership, 0, len(expected))
	summary.SelectedPlotRows = 0
	for i, row := range expected {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		witness := report.SelectedUnits[i]
		if witness.MembershipID != row.MembershipRowID || !reflect.DeepEqual(witness.EnvRowID, row.EnvRowID) ||
			!reflect.DeepEqual(witness.AdminRowID, row.AdminRowID) || !reflect.DeepEqual(witness.PlotNumber, row.PlotNumber) ||
			!reflect.DeepEqual(witness.SiteUnit, row.SiteUnit) {
			return fail(errors.New("selected report unit summary differs from independently read physical field projection"))
		}
		member, present := physical[row.MembershipRowID]
		if !present || !reflect.DeepEqual(member.PlotNumber, row.PlotNumber) {
			return fail(errors.New("selected report unit summary lost original physical membership"))
		}
		member.PlotNumber, member.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
		member.Quality, member.NoneUnit = nil, nil
		witness.EnvRowID, witness.AdminRowID = cloneCatalogueValue(row.EnvRowID), cloneCatalogueValue(row.AdminRowID)
		witness.PlotNumber, witness.SiteUnit = cloneSiteUnitCell(row.PlotNumber), cloneSiteUnitCell(row.SiteUnit)
		member.SelectedUnit = &witness
		summary.Memberships = append(summary.Memberships, member)
		if row.PlotNumber.Storage != "null" {
			summary.SelectedPlotRows++
		}
	}
	return summary, nil
}
