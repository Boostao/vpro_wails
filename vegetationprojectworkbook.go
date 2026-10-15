package main

import (
	"context"
	"encoding/hex"
	"errors"
	"reflect"
)

type vegetationProjectWorkbook struct {
	Preview  LongVegetationProjectPreview
	Workbook vegetationWorkbook
}

type vegetationProjectWorkbookEvidence struct {
	Scope   LongVegetationReportScope
	Summary *vegetationProjectWorkbookSummary
}

func prepareLongVegetationProjectWorkbook(ctx context.Context, source vegetationLayerReport,
	scope LongVegetationReportScope, settings LongVegetationSettings, layout vegetationWorkbookLayout) (vegetationProjectWorkbook, error) {
	return prepareLongVegetationProjectWorkbookWithSummary(ctx, source, scope, settings, layout, nil)
}

func prepareLongVegetationProjectWorkbookWithSummary(ctx context.Context, source vegetationLayerReport,
	scope LongVegetationReportScope, settings LongVegetationSettings, layout vegetationWorkbookLayout,
	summary *vegetationProjectWorkbookSummary) (vegetationProjectWorkbook, error) {
	fail := func(err error) (vegetationProjectWorkbook, error) { return vegetationProjectWorkbook{}, err }
	if ctx == nil {
		return fail(errors.New("project workbook requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if scope.ContextID == "" || scope.Project == "" || scope.ProjectPath == "" || scope.SU == "" ||
		(scope.SU == "None") != (scope.SUPath == "") ||
		source.Project != scope.Project || source.SU != scope.SU || !scope.BuildCurrent || scope.Build == nil {
		return fail(errors.New("project workbook requires complete actual owner paths and current original Build authority"))
	}
	for _, value := range []string{scope.ContextID, scope.Project, scope.ProjectPath, scope.SU, scope.SUPath} {
		if err := environmentWorkbookText(value); err != nil {
			return fail(err)
		}
	}
	if _, err := selectedUnitRevision(scope.Revision); err != nil {
		return fail(err)
	}
	hash, err := hex.DecodeString(scope.Build.ApprovalHash)
	if err != nil || len(hash) != 32 || hex.EncodeToString(hash) != scope.Build.ApprovalHash {
		return fail(errors.New("project workbook requires its exact lowercase SHA256 Build approval"))
	}
	switch scope.SourceKind {
	case vegetationProjectFieldsScope:
		if len(scope.Fields) == 0 || settings.UnitGrouping != "selected-fields" || !reflect.DeepEqual(settings.UnitFields, scope.Fields) {
			return fail(errors.New("project workbook settings lost their explicit selected field order"))
		}
	case vegetationProjectAllScope:
		if scope.SU != "None" || settings.UnitGrouping != "all-plots" || settings.UnitFields != nil {
			return fail(errors.New("project All plots workbook requires actual None ownership without active field settings"))
		}
	default:
		return fail(errors.New("project workbook requires an explicit original project source"))
	}
	if layout.Summary != nil || layout.ReportSummary != (summary != nil) {
		return fail(errors.New("project workbook summary requires explicit distinct original project evidence; selected-list membership summaries are unavailable"))
	}
	report, err := longVegetationProjectTransport(ctx, source, scope.SourceKind)
	if err != nil {
		return fail(err)
	}
	if (settings.Quality != nil) != (report.Quality != nil) {
		return fail(errors.New("project workbook settings and original-SU quality provenance disagree"))
	}
	inputs := make([]vegetationProjectUnitInput, len(scope.Build.ProjectTuples))
	for i, row := range scope.Build.ProjectTuples {
		inputs[i] = vegetationProjectUnitInput{row.EnvRowID, row.AdminRowID, row.PlotNumber, row.SiteUnit}
	}
	validated, err := validateProjectUnitInputs(ctx, inputs)
	if err != nil || len(validated) == 0 {
		return fail(errors.Join(errors.New("project workbook requires complete original Build tuples"), err))
	}
	if (validated[0].AdminRowID != nil) != (scope.SourceKind == vegetationProjectFieldsScope) {
		return fail(errors.New("project workbook Build tuples contradict original source kind"))
	}
	plots := map[string]bool{}
	if report.Quality != nil {
		for _, occurrence := range report.Quality.Occurrences {
			plots[vegetationTextKey(occurrence.PlotNumber)] = true
		}
	}
	expected := make([]LongVegetationProjectUnit, 0, len(validated))
	full := make([]LongVegetationProjectUnit, len(validated))
	for i, row := range validated {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		full[i] = LongVegetationProjectUnit{row.EnvRowID, row.AdminRowID, row.PlotNumber, row.SiteUnit}
		if report.Quality == nil || plots[vegetationTextKey(row.PlotNumber)] {
			expected = append(expected, full[i])
		}
	}
	if !reflect.DeepEqual(expected, report.ProjectUnits) {
		return fail(errors.New("project workbook output differs from complete Build tuples or exact original-SU qualified-plot subset"))
	}
	if summary != nil && (summary.SourceKind != scope.SourceKind || !reflect.DeepEqual(summary.ProjectTuples, report.ProjectUnits)) {
		return fail(errors.New("project workbook summary differs from its current original report tuples"))
	}
	scope.Fields = append([]string{}, scope.Fields...)
	scope.Build = &LongVegetationProjectBuild{ApprovalHash: scope.Build.ApprovalHash, ProjectTuples: full}
	settings.UnitFields = append([]string(nil), settings.UnitFields...)
	settings.Quality = cloneCatalogueValue(settings.Quality)
	preview := LongVegetationPreview{ContextID: scope.ContextID, ProjectPath: scope.ProjectPath, SUPath: scope.SUPath,
		Settings: settings, Report: report}
	workbook, err := renderLongVegetationWorkbook(ctx, preview, layout, &vegetationProjectWorkbookEvidence{Scope: scope, Summary: summary})
	if err != nil {
		return fail(err)
	}
	return vegetationProjectWorkbook{Preview: LongVegetationProjectPreview{Scope: scope, Preview: preview}, Workbook: workbook}, nil
}
