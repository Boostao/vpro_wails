package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

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
	if err := vprocore.DecodeStrictRequiredJSON(data, &decoded, "project report read", "revision", "approvalHash"); err != nil {
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

func decodeProjectReportOptions(config vprocore.ConfigValues, entry vprocore.VegetationProjectScopeEntry) (longVegetationOptions, LongVegetationSettings, error) {
	desired := 2
	if entry.SourceKind == vprocore.VegetationProjectAllScope {
		desired = 3
	} else if entry.SourceKind != vprocore.VegetationProjectFieldsScope {
		return longVegetationOptions{}, LongVegetationSettings{}, errors.New("project options require one explicit original project scope")
	}
	group, err := vprocore.ConfigInt(config, "ReportOptions", "LVUnitGroups", 1, 3)
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
		settings.UnitFields = append([]string{}, entry.Fields...)
	} else {
		settings.UnitGrouping = "all-plots"
	}
	return options, settings, nil
}

func (s *LongVegetationReportScopeService) readProjectOptions(ctx context.Context, plots *vprocore.PlotService,
	tx *sql.Tx, contextID string, request vegetationProjectReadRequest) (LongVegetationProjectOptions, longVegetationOptions, error) {
	fail := func(err error) (LongVegetationProjectOptions, longVegetationOptions, error) {
		return LongVegetationProjectOptions{}, longVegetationOptions{}, err
	}
	values, err := plots.Projects.Preferences.Snapshot()
	if err != nil {
		return fail(err)
	}
	return s.readProjectOptionsForConfig(ctx, plots, tx, contextID, request, values)
}

func (s *LongVegetationReportScopeService) readProjectOptionsForConfig(ctx context.Context, plots *vprocore.PlotService,
	tx *sql.Tx, contextID string, request vegetationProjectReadRequest, values vprocore.ConfigValues) (LongVegetationProjectOptions, longVegetationOptions, error) {
	fail := func(err error) (LongVegetationProjectOptions, longVegetationOptions, error) {
		return LongVegetationProjectOptions{}, longVegetationOptions{}, err
	}
	owner := plots.Projects.Sqlite
	entry, err := ApprovedProjectScope(s.contexts, ctx, contextID, owner, tx, values)
	if err != nil {
		return fail(err)
	}
	if request.Revision != projectScopeDTO(contextID, owner, entry).Revision || request.ApprovalHash != entry.Build.ApprovalHash {
		return fail(errors.New("project report read scope/revision/Build changed; reload before output"))
	}
	options, settings, err := decodeProjectReportOptions(values, entry)
	if err != nil {
		return fail(err)
	}
	if err := CheckLongVegetationGrouping(s.contexts, options); err != nil {
		return fail(err)
	}
	scope := projectScopeDTO(contextID, owner, entry)
	scope.BuildCurrent = true
	return LongVegetationProjectOptions{Scope: *scope, Options: LongVegetationOptions{
		ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
		SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, Settings: settings}}, options, nil
}

func (s *LongVegetationReportScopeService) GetLongVegetationProjectOptions(ctx context.Context, contextID, requestJSON string) (LongVegetationProjectOptions, error) {
	if err := s.require(ctx, contextID); err != nil {
		return LongVegetationProjectOptions{}, err
	}
	var request vegetationProjectReadRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return LongVegetationProjectOptions{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (LongVegetationProjectOptions, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(_ *vprocore.SqliteContext, tx *sql.Tx) (LongVegetationProjectOptions, error) {
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
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (LongVegetationProjectPreview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (LongVegetationProjectPreview, error) {
			review, options, err := s.readProjectOptions(ctx, plots, tx, contextID, request)
			if err != nil {
				return LongVegetationProjectPreview{}, err
			}
			fields := review.Scope.Fields
			if review.Scope.SourceKind == vprocore.VegetationProjectAllScope {
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
				ContextID: contextID, ProjectPath: owner.Selection.ProjectPath, SUPath: owner.Selection.SUPath,
				Settings: review.Options.Settings, Report: report}}, nil
		})
	})
}
