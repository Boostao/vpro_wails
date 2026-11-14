package vegreport

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func selectedUnitConfigForChoice(values vprocore.ConfigValues, choice int) (vprocore.ConfigValues, error) {
	if choice < 1 || choice > 3 {
		return nil, errors.New("selected report unit approval requires source unit choice 1, 2 or 3")
	}
	if _, err := vprocore.ConfigInt(values, "ReportOptions", "LVUnitGroups", 1, 3); err != nil {
		return nil, err
	}
	section, err := vprocore.ConfigSection(values, "ReportOptions")
	if err != nil {
		return nil, err
	}
	result := maps.Clone(values)
	options := maps.Clone(section)
	options["LVUnitGroups"] = choice
	result["ReportOptions"] = options
	return result, nil
}

func SelectedUnitAvailability(s *vprocore.ContextService, ctx context.Context, plots *vprocore.PlotService, tx *sql.Tx) (bool, error) {
	if err := s.RequireSelectedListReportScope(plots.Projects.ContextID); err != nil {
		return false, err
	}
	entry := s.SelectedUnitSelections.Read(plots.Projects.ContextID)
	if entry.Build == nil {
		return false, nil
	}
	tables, err := readSelectedUnitSelectionTables(ctx, plots.Projects.Sqlite, tx)
	if err != nil {
		return false, err
	}
	values, err := plots.Projects.Preferences.Snapshot()
	if err != nil {
		return false, err
	}
	hash, err := selectedUnitSelectionHash(plots.Projects.ContextID, plots.Projects.Sqlite, tables, entry.Fields, values)
	if err != nil {
		return false, err
	}
	return hash == entry.Build.ApprovalHash, nil
}

func DecodeOwnedLongVegetationOptions(s *vprocore.ContextService, ctx context.Context, plots *vprocore.PlotService,
	tx *sql.Tx, values vprocore.ConfigValues) (longVegetationOptions, error) {
	if err := s.RequireSelectedListReportScope(plots.Projects.ContextID); err != nil {
		return longVegetationOptions{}, err
	}
	group, err := vprocore.ConfigInt(values, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return longVegetationOptions{}, err
	}

	if group != 2 {
		return decodeLongVegetationOptions(values)
	}
	fields, err := ApprovedSelectedUnitFields(s, ctx, plots.Projects.ContextID, plots.Projects.Sqlite, tx, values)
	if err != nil {
		return longVegetationOptions{}, err
	}
	ordinary, err := selectedUnitConfigForChoice(values, 1)
	if err != nil {
		return longVegetationOptions{}, err
	}
	options, err := decodeLongVegetationOptions(ordinary)
	if err != nil {
		return longVegetationOptions{}, err
	}
	options.SelectedUnitFields = fields
	return options, nil
}

func validateSelectedUnitDestination(row vegetationSelectedUnitOccurrence) error {
	for _, field := range []struct {
		name string
		cell vprocore.ProjectMetadataCell
		max  int
	}{{"PlotNumber", row.PlotNumber, 7}, {"SiteUnit", row.SiteUnit, 255}} {
		value, err := vprocore.VegetationReportIdentity(field.cell)
		if err != nil {
			return fmt.Errorf("selected report unit destination %s: %w", field.name, err)
		}
		if value != nil {
			if *value == "" || strings.ContainsRune(*value, 0) {
				return fmt.Errorf("selected report unit destination %s refuses empty/NUL text; no derived row was dropped or normalized", field.name)
			}
			if err := vprocore.ValidateChildPhysicalText("selected report unit destination "+field.name, *value, field.max); err != nil {
				return err
			}
		}
	}
	return nil
}

func prepareLongVegetationSelectedUnits(ctx context.Context, project, suName string,
	veg, su, layers, env, admin vprocore.ProjectMetadataTable, names []string,
	quality *vegetationQualitySelection) (VegetationReportPreparation, []vegetationSelectedUnitOccurrence, error) {
	fail := func(err error) (VegetationReportPreparation, []vegetationSelectedUnitOccurrence, error) {
		return VegetationReportPreparation{}, nil, err
	}
	occurrences, err := planSelectedUnitProjection(ctx, env, admin, su, names, quality, true)
	if err != nil {
		return fail(err)
	}
	included := map[string]bool{}
	for _, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		included[row.MembershipRowID] = true
	}
	filtered := su
	filtered.Rows = []vprocore.ProjectMetadataRow{}
	for _, row := range su.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if included[row.RowID] {
			filtered.Rows = append(filtered.Rows, row)
		}
	}
	prepared, err := prepareLongVegetation(ctx, project, suName, veg, filtered, layers)
	if err != nil {
		return fail(err)
	}
	prepared.selectedUnitOccurrences = make([]vegetationSelectedUnitOccurrence, len(occurrences))
	for i, row := range occurrences {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		row.EnvRowID, row.AdminRowID = sqlutil.CloneCatalogueValue(row.EnvRowID), sqlutil.CloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		prepared.selectedUnitOccurrences[i] = row
	}
	return prepared, occurrences, nil
}

func selectedUnitReportMemberships(ctx context.Context, prepared VegetationReportPreparation) ([]reportcore.VegetationReportMembership, error) {
	if prepared.selectedUnitOccurrences == nil {
		return prepared.Memberships, nil
	}
	if prepared.noneUnitMembershipCounts != nil || prepared.qualityMembershipCounts != nil {
		return nil, errors.New("selected report units cannot apply None/quality occurrence weights")
	}
	original := map[string]reportcore.VegetationReportMembership{}
	for _, member := range prepared.Memberships {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if _, exists := original[member.RowID]; exists {
			return nil, errors.New("selected report units require unique original membership identities")
		}
		original[member.RowID] = member
	}
	type witness struct {
		member, env, admin string
	}
	seen := map[witness]bool{}
	result := make([]reportcore.VegetationReportMembership, 0, len(prepared.selectedUnitOccurrences))
	for _, row := range prepared.selectedUnitOccurrences {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		member, exists := original[row.MembershipRowID]
		if !exists || !reflect.DeepEqual(member.PlotNumber, row.PlotNumber) {
			return nil, errors.New("selected report unit derivation lost original physical membership")
		}
		if err := validateSelectedUnitDestination(row); err != nil {
			return nil, err
		}
		key := witness{member: row.MembershipRowID}
		if (row.EnvRowID == nil) != (row.AdminRowID == nil) {
			return nil, errors.New("selected report unit derivation requires both original Env/Admin witnesses or neither")
		}
		if row.EnvRowID == nil {
			if row.SiteUnit.Storage != "null" {
				return nil, errors.New("selected report unit unmatched membership cannot invent a label")
			}
		} else {
			if row.PlotNumber.Storage != "text" {
				return nil, errors.New("selected report unit matched parents require original non-NULL plot")
			}
			for _, id := range []*string{row.EnvRowID, row.AdminRowID} {
				number, err := strconv.ParseInt(*id, 10, 64)
				if err != nil || strconv.FormatInt(number, 10) != *id {
					return nil, errors.New("selected report unit derivation requires exact physical parent identities")
				}
			}
			key.env, key.admin = *row.EnvRowID, *row.AdminRowID
		}
		if seen[key] {
			return nil, errors.New("selected report unit derivation repeats an original SU/Env/Admin join")
		}
		seen[key] = true
		result = append(result, reportcore.VegetationReportMembership{RowID: member.RowID,
			PlotNumber: vprocore.CloneSiteUnitCell(row.PlotNumber), SiteUnit: vprocore.CloneSiteUnitCell(row.SiteUnit)})
	}
	return result, nil
}

func vegetationWorkbookSelectedUnitSummary(ctx context.Context, summary vegetationWorkbookSummary,
	env, admin, su vprocore.ProjectMetadataTable, names []string, report LongVegetationReport) (vegetationWorkbookSummary, error) {
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
				Membership: reportcore.VegetationReportMembership{RowID: row.MembershipID, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit},
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
		member.PlotNumber, member.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		member.Quality, member.NoneUnit = nil, nil
		witness.EnvRowID, witness.AdminRowID = sqlutil.CloneCatalogueValue(row.EnvRowID), sqlutil.CloneCatalogueValue(row.AdminRowID)
		witness.PlotNumber, witness.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		member.SelectedUnit = &witness
		summary.Memberships = append(summary.Memberships, member)
		if row.PlotNumber.Storage != "null" {
			summary.SelectedPlotRows++
		}
	}
	return summary, nil
}
