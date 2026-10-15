package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
)

type vegetationProjectReadRequest struct {
	Revision     string `json:"revision"`
	ApprovalHash string `json:"approvalHash"`
}

func (request *vegetationProjectReadRequest) UnmarshalJSON(data []byte) error {
	type plain vegetationProjectReadRequest
	var decoded plain
	if err := decodeStrictRequiredJSON(data, &decoded, "project report read", "revision", "approvalHash"); err != nil {
		return err
	}
	if err := validateProjectReadRequest(vegetationProjectReadRequest(decoded)); err != nil {
		return err
	}
	*request = vegetationProjectReadRequest(decoded)
	return nil
}

func validateProjectReadRequest(request vegetationProjectReadRequest) error {
	if _, err := selectedUnitRevision(request.Revision); err != nil {
		return err
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != 32 || hex.EncodeToString(hash) != request.ApprovalHash {
		return errors.New("project report read requires its exact reviewed lowercase SHA256 Build hash")
	}
	return nil
}

type LongVegetationProjectOptions struct {
	Scope   LongVegetationReportScope `json:"scope"`
	Options LongVegetationOptions     `json:"options"`
}

type LongVegetationProjectPreview struct {
	Scope   LongVegetationReportScope `json:"scope"`
	Preview LongVegetationPreview     `json:"preview"`
}

func decodeProjectReportOptions(config configValues, entry vegetationProjectScopeEntry) (longVegetationOptions, LongVegetationSettings, error) {
	desired := 2
	if entry.sourceKind == vegetationProjectAllScope {
		desired = 3
	} else if entry.sourceKind != vegetationProjectFieldsScope {
		return longVegetationOptions{}, LongVegetationSettings{}, errors.New("project options require one explicit original project scope")
	}
	group, err := configInt(config, "ReportOptions", "LVUnitGroups", 1, 3)
	if err != nil {
		return longVegetationOptions{}, LongVegetationSettings{}, err
	}
	if group != desired {
		return longVegetationOptions{}, LongVegetationSettings{}, errors.New("saved report unit choice does not match explicit project scope; save the reviewed unit choice before output")
	}
	ordinary, err := selectedUnitConfigForChoice(config, 1)
	if err != nil {
		return longVegetationOptions{}, LongVegetationSettings{}, err
	}
	options, err := decodeLongVegetationOptions(ordinary)
	if err != nil {
		return longVegetationOptions{}, LongVegetationSettings{}, err
	}
	settings := vegetationSettings(options)
	if desired == 2 {
		settings.UnitGrouping = "selected-fields"
		settings.UnitFields = append([]string{}, entry.fields...)
	} else {
		settings.UnitGrouping = "all-plots"
	}
	return options, settings, nil
}

func (s *LongVegetationReportScopeService) readProjectOptions(ctx context.Context, plots *PlotService,
	tx *sql.Tx, contextID string, request vegetationProjectReadRequest) (LongVegetationProjectOptions, longVegetationOptions, error) {
	fail := func(err error) (LongVegetationProjectOptions, longVegetationOptions, error) {
		return LongVegetationProjectOptions{}, longVegetationOptions{}, err
	}
	values, err := plots.projects.preferences.snapshot()
	if err != nil {
		return fail(err)
	}
	return s.readProjectOptionsForConfig(ctx, plots, tx, contextID, request, values)
}

func (s *LongVegetationReportScopeService) readProjectOptionsForConfig(ctx context.Context, plots *PlotService,
	tx *sql.Tx, contextID string, request vegetationProjectReadRequest, values configValues) (LongVegetationProjectOptions, longVegetationOptions, error) {
	fail := func(err error) (LongVegetationProjectOptions, longVegetationOptions, error) {
		return LongVegetationProjectOptions{}, longVegetationOptions{}, err
	}
	owner := plots.projects.sqlite
	entry, err := s.contexts.approvedProjectScope(ctx, contextID, owner, tx, values)
	if err != nil {
		return fail(err)
	}
	if request.Revision != projectScopeDTO(contextID, owner, entry).Revision || request.ApprovalHash != entry.build.ApprovalHash {
		return fail(errors.New("project report read scope/revision/Build changed; reload before output"))
	}
	options, settings, err := decodeProjectReportOptions(values, entry)
	if err != nil {
		return fail(err)
	}
	if err := s.contexts.checkLongVegetationGrouping(options); err != nil {
		return fail(err)
	}
	scope := projectScopeDTO(contextID, owner, entry)
	scope.BuildCurrent = true
	return LongVegetationProjectOptions{Scope: *scope, Options: LongVegetationOptions{
		ContextID: contextID, Project: owner.selection.Project, ProjectPath: owner.selection.ProjectPath,
		SU: owner.selection.SU, SUPath: owner.selection.SUPath, Settings: settings}}, options, nil
}

func (s *LongVegetationReportScopeService) GetLongVegetationProjectOptions(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectOptions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return LongVegetationProjectOptions{}, err
	}
	var request vegetationProjectReadRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return LongVegetationProjectOptions{}, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (LongVegetationProjectOptions, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(_ *sqliteContext, tx *sql.Tx) (LongVegetationProjectOptions, error) {
			result, _, err := s.readProjectOptions(ctx, plots, tx, contextID, request)
			return result, err
		})
	})
}

func (s *LongVegetationReportScopeService) PreviewLongVegetationProject(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectPreview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return LongVegetationProjectPreview{}, err
	}
	var request vegetationProjectReadRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return LongVegetationProjectPreview{}, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (LongVegetationProjectPreview, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *sqliteContext, tx *sql.Tx) (LongVegetationProjectPreview, error) {
			review, options, err := s.readProjectOptions(ctx, plots, tx, contextID, request)
			if err != nil {
				return LongVegetationProjectPreview{}, err
			}
			fields := review.Scope.Fields
			if review.Scope.SourceKind == vegetationProjectAllScope {
				fields = nil
			}
			source, err := readLongVegetationProjectSource(ctx, owner, tx, fields, options)
			if err != nil {
				return LongVegetationProjectPreview{}, err
			}
			report, err := longVegetationProjectTransport(ctx, source.Report, review.Scope.SourceKind)
			if err != nil {
				return LongVegetationProjectPreview{}, err
			}
			return LongVegetationProjectPreview{Scope: review.Scope, Preview: LongVegetationPreview{
				ContextID: contextID, ProjectPath: owner.selection.ProjectPath, SUPath: owner.selection.SUPath,
				Settings: review.Options.Settings, Report: report}}, nil
		})
	})
}
