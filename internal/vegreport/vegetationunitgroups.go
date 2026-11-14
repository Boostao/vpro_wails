package vegreport

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func addLongVegetationUnitNames(ctx context.Context, report *vegetationLayerReport, master vprocore.ProjectMetadataTable) error {
	columns, err := vprocore.SiteUnitTransferColumns(master, "SiteSeries", "SiteSeriesLongName")
	if err != nil {
		return fmt.Errorf("Long Vegetation master name schema: %w", err)
	}
	rows, err := vprocore.SiteUnitTransferIndex(master, columns["SiteSeries"])
	if err != nil {
		return err
	}
	for i := range report.Units {
		if err := ctx.Err(); err != nil {
			return err
		}
		unit := &report.Units[i]
		code, err := vprocore.VegetationReportIdentity(unit.Code)
		if err != nil {
			return err
		}
		unit.NameCandidates = []vprocore.EnvironmentReportName{}
		if code == nil {
			empty := ""
			unit.LongName, unit.NameStatus = &empty, "unassigned"
			continue
		}
		names, err := reportcore.ResolveReportUnitNames(ctx, rows[*code], columns["SiteSeriesLongName"], "Long Vegetation")
		if err != nil {
			return err
		}
		unit.LongName, unit.NameStatus, unit.NameCandidates = names.LongName, names.Status, names.Candidates
		if names.Status != "unique" {
			report.Diagnostics = append(report.Diagnostics, vegetationLayerDiagnostic{
				Code: "unit_name_" + names.Status, Identity: reportcore.VegetationTextKey(unit.Code), Count: len(names.Candidates),
			})
		}
	}
	return ctx.Err()
}

func longVegetationCodeReferences(ctx context.Context, source vprocore.ProjectMetadataTable, options longVegetationOptions) (vprocore.ProjectMetadataTable, longVegetationOptions, error) {
	if err := ctx.Err(); err != nil {
		return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	if !options.ShowSpeciesCode {
		return source, options, nil
	}
	columns, err := vprocore.SiteUnitTransferColumns(source, "Code", "EnglishName")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
	}
	result := vprocore.ProjectMetadataTable{
		Columns: append([]vprocore.ProjectMetadataColumn{}, source.Columns...),
		Rows:    make([]vprocore.ProjectMetadataRow, 0, len(source.Rows)),
	}
	for _, row := range source.Rows {
		if err := ctx.Err(); err != nil {
			return vprocore.ProjectMetadataTable{}, longVegetationOptions{}, err
		}
		cells := make([]vprocore.ProjectMetadataCell, len(row.Cells))
		for i, cell := range row.Cells {
			cells[i] = vprocore.CloneSiteUnitCell(cell)
		}
		cells[columns["EnglishName"]] = vprocore.CloneSiteUnitCell(row.Cells[columns["Code"]])
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	options.ShowEnglishName, options.ShowSpeciesCode = true, false
	return result, options, nil
}

func CheckLongVegetationGrouping(_ *vprocore.ContextService, options longVegetationOptions) error {
	if options.ShowEnglishName && options.ShowSpeciesCode || options.SelectedField != "" && (options.ShowEnglishName || options.ShowSpeciesCode) {
		return errors.New("Long Vegetation supplementary fields cannot combine English name, Code or a selected field")
	}
	if options.NoneGrouping && (options.LifeformGrouping || options.StrataGrouping) ||
		options.LifeformGrouping && options.StrataGrouping {
		return errors.New("Long Vegetation grouping cannot combine None, Lifeform or Strata")
	}
	return nil
}

type vegetationNoneUnitOccurrence struct {
	EnvRowID          string
	MembershipRowID   string
	QualityOccurrence *int
	PlotNumber        vprocore.ProjectMetadataCell
	SiteUnit          vprocore.ProjectMetadataCell
}

func prepareLongVegetationNoneUnits(ctx context.Context, project, suName string,
	veg, su, layers, env vprocore.ProjectMetadataTable, quality *vegetationQualitySelection) (VegetationReportPreparation, []vegetationNoneUnitOccurrence, error) {
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
	filtered.Rows = []vprocore.ProjectMetadataRow{}
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
		prepared.Memberships[i].SiteUnit = vprocore.ProjectMetadataCell{Storage: "text", Text: &unit}
	}
	prepared.noneUnitMembershipCounts = counts
	return prepared, occurrences, nil
}

// WriteBlankSiteUnits joins Env to the current list, which QCLV may have
// replaced with weighted occurrences. These are derived joins, not physical SU IDs.
func planLongVegetationNoneUnits(ctx context.Context, env, su vprocore.ProjectMetadataTable,
	quality *vegetationQualitySelection) ([]vegetationNoneUnitOccurrence, error) {
	if ctx == nil {
		return nil, errors.New("None report units require a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	ec, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return nil, fmt.Errorf("None report units physical Env: %w", err)
	}
	sc, err := vprocore.SiteUnitTransferColumns(su, "PlotNumber", "SiteUnit")
	if err != nil {
		return nil, fmt.Errorf("None report units selected SU: %w", err)
	}
	envIDs, members := map[string]vprocore.ProjectMetadataRow{}, map[string]vprocore.ProjectMetadataRow{}
	literals := map[string]string{}
	identity := func(cell vprocore.ProjectMetadataCell) (*string, error) {
		value, err := vprocore.VegetationReportIdentity(cell)
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
		if _, err := vprocore.VegetationReportIdentity(row.Cells[sc["SiteUnit"]]); err != nil {
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
				QualityOccurrence: occurrence, PlotNumber: vprocore.CloneSiteUnitCell(plot),
				SiteUnit: vprocore.ProjectMetadataCell{Storage: "text", Text: &unit}})
		}
	}
	return result, nil
}

func vegetationWorkbookNoneUnitSummary(ctx context.Context, summary vegetationWorkbookSummary,
	env vprocore.ProjectMetadataTable, report LongVegetationReport) (vegetationWorkbookSummary, error) {
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
	columns, err := vprocore.SiteUnitTransferColumns(env, "PlotNumber")
	if err != nil {
		return fail(err)
	}
	environments := map[string]vprocore.ProjectMetadataCell{}
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
			!reflect.DeepEqual(occurrence.SiteUnit, vprocore.ProjectMetadataCell{Storage: "text", Text: &allPlots}) {
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
		member.PlotNumber = vprocore.CloneSiteUnitCell(member.PlotNumber)
		member.SiteUnit = vprocore.CloneSiteUnitCell(occurrence.SiteUnit)
		member.Quality = nil
		copy := occurrence
		copy.QualityOccurrence = sqlutil.CloneCatalogueValue(occurrence.QualityOccurrence)
		copy.PlotNumber, copy.SiteUnit = vprocore.CloneSiteUnitCell(occurrence.PlotNumber), vprocore.CloneSiteUnitCell(occurrence.SiteUnit)
		member.NoneUnit = &copy
		summary.Memberships = append(summary.Memberships, member)
		summary.SelectedPlotRows++
	}
	return summary, nil
}
