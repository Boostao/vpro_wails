package vegreport

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type vegetationProjectUnitInput struct {
	EnvRowID   string
	AdminRowID *string
	PlotNumber vprocore.ProjectMetadataCell
	SiteUnit   vprocore.ProjectMetadataCell
}

func validateProjectUnitInputs(ctx context.Context, inputs []vegetationProjectUnitInput) ([]vegetationProjectUnitInput, error) {
	seen := map[struct{ env, admin string }]bool{}
	identity := selectedUnitIdentityReader()
	result := make([]vegetationProjectUnitInput, 0, len(inputs))
	var selected *bool
	for _, row := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		hasAdmin := row.AdminRowID != nil
		if selected != nil && *selected != hasAdmin {
			return nil, errors.New("project report units cannot mix Env-only and Env/Admin provenance")
		}
		selected = &hasAdmin
		key := struct{ env, admin string }{env: row.EnvRowID}
		ids := []string{row.EnvRowID}
		if hasAdmin {
			key.admin = *row.AdminRowID
			ids = append(ids, *row.AdminRowID)
			if row.PlotNumber.Storage != "text" {
				return nil, errors.New("project selected unit requires an original non-NULL joined plot")
			}
		} else if row.SiteUnit.Storage != "text" || row.SiteUnit.Text == nil || *row.SiteUnit.Text != "All plots" {
			return nil, errors.New("project None unit requires its source All plots label")
		}
		for _, id := range ids {
			value, err := strconv.ParseInt(id, 10, 64)
			if err != nil || strconv.FormatInt(value, 10) != id {
				return nil, errors.New("project report units require exact original physical signed64 identities")
			}
		}
		if seen[key] {
			return nil, errors.New("project report units repeat an original physical tuple")
		}
		seen[key] = true
		if _, err := identity(row.PlotNumber); err != nil {
			return nil, err
		}
		label, err := vprocore.VegetationReportIdentity(row.SiteUnit)
		if err != nil {
			return nil, err
		}
		if label != nil {
			if err := vprocore.ValidateChildPhysicalText("project report unit label", *label, 255); err != nil {
				return nil, err
			}
		}
		row.AdminRowID = sqlutil.CloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		result = append(result, row)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func projectReportPreparation(ctx context.Context, project vegetationProjectPreparation) (VegetationReportPreparation, error) {
	if ctx == nil {
		return VegetationReportPreparation{}, errors.New("project report kernel requires a context")
	}
	if err := ctx.Err(); err != nil {
		return VegetationReportPreparation{}, err
	}
	if project.Project == "" || project.SourceSU == "" || !utf8.ValidString(project.Project+project.SourceSU) ||
		strings.ContainsRune(project.Project+project.SourceSU, 0) ||
		(project.SelectedUnits == nil) == (project.NoneUnits == nil) ||
		project.NoneUnits != nil && project.SourceSU != "None" {
		return VegetationReportPreparation{}, errors.New("project report kernel requires one explicit exclusive original unit source")
	}
	inputs := []vegetationProjectUnitInput{}
	for _, row := range project.SelectedUnits {
		if err := ctx.Err(); err != nil {
			return VegetationReportPreparation{}, err
		}
		admin := row.AdminRowID
		inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, &admin, row.PlotNumber, row.SiteUnit})
	}
	for _, row := range project.NoneUnits {
		if err := ctx.Err(); err != nil {
			return VegetationReportPreparation{}, err
		}
		inputs = append(inputs, vegetationProjectUnitInput{row.EnvRowID, nil, row.PlotNumber, row.SiteUnit})
	}
	validated, err := validateProjectUnitInputs(ctx, inputs)
	if err != nil {
		return VegetationReportPreparation{}, err
	}
	return VegetationReportPreparation{Project: project.Project, SU: project.SourceSU,
		CoverColumns: project.Entries.CoverColumns, Layers: project.Entries.Layers,
		ReducedRows: project.Entries.ReducedRows, Observations: project.Entries.Observations,
		Memberships: []reportcore.VegetationReportMembership{}, projectUnitInputs: validated}, nil
}

type vegetationProjectWorkbookSummary struct {
	Header         vegetationWorkbookSummaryHeader
	SourceKind     string
	SourcePlotRows int
	ProjectTuples  []vprocore.LongVegetationProjectUnit
}

func prepareVegetationProjectWorkbookSummary(ctx context.Context, createdDate string,
	env, admin, descriptions vprocore.ProjectMetadataTable, scope LongVegetationReportScope,
	source vegetationLayerReport) (vegetationProjectWorkbookSummary, error) {
	fail := func(err error) (vegetationProjectWorkbookSummary, error) {
		return vegetationProjectWorkbookSummary{}, err
	}
	header, err := prepareVegetationWorkbookSummaryHeader(ctx, createdDate, env, descriptions)
	if err != nil {
		return fail(err)
	}
	if !scope.BuildCurrent || scope.Build == nil || scope.Project != source.Project || scope.SU != source.SU {
		return fail(errors.New("project summary requires its current independent original scope Build"))
	}
	report, err := longVegetationProjectTransport(ctx, source, scope.SourceKind)
	if err != nil {
		return fail(err)
	}
	full := []vprocore.LongVegetationProjectUnit{}
	switch scope.SourceKind {
	case vprocore.VegetationProjectFieldsScope:
		rows, err := planLongVegetationProjectSelectedUnits(ctx, env, admin, scope.Fields)
		if err != nil {
			return fail(err)
		}
		for _, row := range rows {
			id := row.AdminRowID
			full = append(full, vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: &id, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit})
		}
	case vprocore.VegetationProjectAllScope:
		rows, err := planLongVegetationProjectNoneUnits(ctx, env)
		if err != nil {
			return fail(err)
		}
		for _, row := range rows {
			full = append(full, vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: nil, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit})
		}
	default:
		return fail(errors.New("project summary requires explicit original source2 or source3"))
	}
	if !reflect.DeepEqual(full, scope.Build.ProjectTuples) {
		return fail(errors.New("project summary physical field projection differs from complete original Build tuples"))
	}
	plots := map[string]bool{}
	if report.Quality != nil {
		for _, occurrence := range report.Quality.Occurrences {
			plots[reportcore.VegetationTextKey(occurrence.PlotNumber)] = true
		}
	}
	result := vegetationProjectWorkbookSummary{Header: header, SourceKind: scope.SourceKind, ProjectTuples: []vprocore.LongVegetationProjectUnit{}}
	for _, row := range full {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if report.Quality != nil && !plots[reportcore.VegetationTextKey(row.PlotNumber)] {
			continue
		}
		row.AdminRowID = sqlutil.CloneCatalogueValue(row.AdminRowID)
		row.PlotNumber, row.SiteUnit = vprocore.CloneSiteUnitCell(row.PlotNumber), vprocore.CloneSiteUnitCell(row.SiteUnit)
		result.ProjectTuples = append(result.ProjectTuples, row)
		if row.PlotNumber.Storage != "null" {
			result.SourcePlotRows++
		}
	}
	if !reflect.DeepEqual(result.ProjectTuples, report.ProjectUnits) {
		return fail(errors.New("project summary output differs from its exact original-SU qualified-plot subset"))
	}
	return result, nil
}

func longVegetationProjectTransport(ctx context.Context, source vegetationLayerReport, sourceKind string) (LongVegetationReport, error) {
	if ctx == nil {
		return LongVegetationReport{}, errors.New("project report transport requires a context")
	}
	if err := ctx.Err(); err != nil {
		return LongVegetationReport{}, err
	}
	if sourceKind != vprocore.VegetationProjectFieldsScope && sourceKind != vprocore.VegetationProjectAllScope {
		return LongVegetationReport{}, errors.New("project report transport requires its explicit original source kind")
	}
	if source.Project == "" || source.SU == "" || !utf8.ValidString(source.Project+source.SU) ||
		strings.ContainsRune(source.Project+source.SU, 0) ||
		source.ProjectUnits == nil || len(source.ProjectUnits) == 0 || len(source.ProjectUnits) > 1048572 ||
		source.SelectedUnits != nil || source.NoneUnits != nil ||
		sourceKind == vprocore.VegetationProjectAllScope && source.SU != "None" ||
		source.Quality != nil && (sourceKind == vprocore.VegetationProjectAllScope || source.SU == "None") {
		return LongVegetationReport{}, errors.New("project report transport requires exclusive original project provenance and actual quality authority")
	}
	inputs, err := validateProjectUnitInputs(ctx, source.ProjectUnits)
	if err != nil {
		return LongVegetationReport{}, err
	}
	if (inputs[0].AdminRowID != nil) != (sourceKind == vprocore.VegetationProjectFieldsScope) {
		return LongVegetationReport{}, errors.New("project report source kind does not match original tuple provenance")
	}
	seen := make([]bool, len(inputs))
	unitCodes := map[string]bool{}
	for _, unit := range source.Units {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		if unit.NumPlots <= 0 || len(unit.MembershipIDs) != 0 || len(unit.ProjectUnitIndexes) != unit.NumPlots {
			return LongVegetationReport{}, errors.New("project unit denominator requires original tuple indexes, not physical SU membership IDs")
		}
		if _, err := vprocore.VegetationReportIdentity(unit.Code); err != nil {
			return LongVegetationReport{}, err
		}
		key := reportcore.VegetationTextKey(unit.Code)
		if unitCodes[key] {
			return LongVegetationReport{}, errors.New("project report repeats an original unit label")
		}
		unitCodes[key] = true
		for _, index := range unit.ProjectUnitIndexes {
			if err := ctx.Err(); err != nil {
				return LongVegetationReport{}, err
			}
			if index < 0 || index >= len(inputs) || seen[index] || !reflect.DeepEqual(inputs[index].SiteUnit, unit.Code) {
				return LongVegetationReport{}, errors.New("project unit index has missing, repeated or mismatched original tuple ownership")
			}
			seen[index] = true
		}
	}
	excluded := map[string]int{}
	for _, diagnostic := range source.Diagnostics {
		if diagnostic.Code != "excluded_unit_without_vegetation" {
			continue
		}
		if diagnostic.Count <= 0 || excluded[diagnostic.Identity] != 0 || unitCodes[diagnostic.Identity] {
			return LongVegetationReport{}, errors.New("project report has inconsistent excluded-unit diagnostics")
		}
		excluded[diagnostic.Identity] = diagnostic.Count
	}
	omitted := map[string]int{}
	excludedIndexes := []int{}
	for i, owned := range seen {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		if !owned {
			key := reportcore.VegetationTextKey(inputs[i].SiteUnit)
			omitted[key]++
			excludedIndexes = append(excludedIndexes, i)
		}
	}
	if !reflect.DeepEqual(excluded, omitted) {
		return LongVegetationReport{}, errors.New("project report omitted original tuple ownership without exact excluded-unit evidence")
	}
	result, err := cloneLongVegetationTransport(ctx, source)
	if err != nil {
		return LongVegetationReport{}, err
	}
	result.SourceKind = sourceKind
	if len(excludedIndexes) > 0 {
		result.ExcludedProjectUnitIndexes = excludedIndexes
	}
	result.ProjectUnits = make([]vprocore.LongVegetationProjectUnit, len(inputs))
	for i, row := range inputs {
		if err := ctx.Err(); err != nil {
			return LongVegetationReport{}, err
		}
		result.ProjectUnits[i] = vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: row.AdminRowID, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit}
	}
	return result, nil
}
