package main

import (
	"context"
	"errors"
	"reflect"
)

func vegetationWorkbookNoneUnitSummary(ctx context.Context, summary vegetationWorkbookSummary,
	env ProjectMetadataTable, report LongVegetationReport) (vegetationWorkbookSummary, error) {
	fail := func(err error) (vegetationWorkbookSummary, error) { return vegetationWorkbookSummary{}, err }
	if ctx == nil {
		return fail(errors.New("Long Vegetation None summary requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if len(report.NoneUnits) == 0 || len(report.NoneUnits) > 1048572 {
		return fail(errors.New("Long Vegetation None summary requires nonempty bounded derived join evidence"))
	}
	columns, err := siteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return fail(err)
	}
	environments := map[string]ProjectMetadataCell{}
	for _, row := range env.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		environments[row.RowID] = row.Cells[columns["PlotNumber"]]
	}
	physical := map[string]vegetationWorkbookMembership{}
	for _, member := range summary.Memberships {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		physical[member.RowID] = member
	}
	type witness struct {
		env, member string
		quality     int
	}
	seen := map[witness]bool{}
	summary.Memberships = make([]vegetationWorkbookMembership, 0, len(report.NoneUnits))
	summary.SelectedPlotRows = 0
	allPlots := "All plots"
	for _, occurrence := range report.NoneUnits {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		member, present := physical[occurrence.MembershipID]
		plot, envPresent := environments[occurrence.EnvRowID]
		if !present || !envPresent || occurrence.PlotNumber.Storage != "text" ||
			!reflect.DeepEqual(plot, occurrence.PlotNumber) || !reflect.DeepEqual(member.PlotNumber, occurrence.PlotNumber) ||
			!reflect.DeepEqual(occurrence.SiteUnit, ProjectMetadataCell{Storage: "text", Text: &allPlots}) {
			return fail(errors.New("Long Vegetation None summary derived occurrence differs from original physical Env/SU membership"))
		}
		index := -1
		if report.Quality == nil {
			if occurrence.QualityOccurrence != nil {
				return fail(errors.New("Long Vegetation None summary has unexpected quality link"))
			}
		} else {
			if occurrence.QualityOccurrence == nil || *occurrence.QualityOccurrence < 0 ||
				*occurrence.QualityOccurrence >= len(report.Quality.Occurrences) {
				return fail(errors.New("Long Vegetation None summary lacks valid original quality link"))
			}
			index = *occurrence.QualityOccurrence
			quality := report.Quality.Occurrences[index]
			if quality.MembershipID != member.RowID || !reflect.DeepEqual(quality.PlotNumber, member.PlotNumber) ||
				!reflect.DeepEqual(quality.SiteUnit, member.SiteUnit) {
				return fail(errors.New("Long Vegetation None summary quality link differs from original SU membership"))
			}
		}
		key := witness{occurrence.EnvRowID, occurrence.MembershipID, index}
		if seen[key] {
			return fail(errors.New("Long Vegetation None summary repeats the same derived join witness"))
		}
		seen[key] = true
		member.PlotNumber = cloneSiteUnitCell(member.PlotNumber)
		member.SiteUnit = cloneSiteUnitCell(occurrence.SiteUnit)
		member.Quality = nil
		copy := occurrence
		copy.QualityOccurrence = cloneCatalogueValue(occurrence.QualityOccurrence)
		copy.PlotNumber, copy.SiteUnit = cloneSiteUnitCell(occurrence.PlotNumber), cloneSiteUnitCell(occurrence.SiteUnit)
		member.NoneUnit = &copy
		summary.Memberships = append(summary.Memberships, member)
		summary.SelectedPlotRows++
	}
	return summary, nil
}
