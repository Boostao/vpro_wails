package twopage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type TwoPageParentExtraWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Edits    []sourceform.SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentExtraWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentExtraWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page additional-field transport: %w", err)
	}
	*request = TwoPageParentExtraWrite(decoded)
	return nil
}

type TwoPageParentExtraService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageParentExtraService(contexts *vprocore.ContextService) (*TwoPageParentExtraService, error) {
	if contexts == nil {
		return nil, errors.New("two-page parent service requires owned contexts")
	}
	return &TwoPageParentExtraService{contexts: contexts}, nil
}

func (s *TwoPageParentExtraService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page parent service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page parent service requires owned contexts")
	}
	return nil
}

func (s *TwoPageParentExtraService) GetOriginal(ctx context.Context, contextID, plot, form string) (*sourceform.SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageParent(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageParentExtraService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentExtraWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return WriteTwoPageParentExtra(s.contexts, ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentExtraService) Restore(ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return RestoreTwoPageParentExtra(s.contexts, ctx, contextID, plot, form, historyID, action)
}

type twoPageParentCommonField struct {
	Owner, Domain string `json:"-" yaml:"-"`
}

var TwoPageParentCommonFields = map[string]twoPageParentCommonField{
	"SV_StandHeight":        {"Env", "scalar"},
	"SV_AhorizonDepth":      {"Env", "scalar"},
	"SV_GleyingMottlingCM":  {"Env", "scalar"},
	"SV_PercentCoarseFrags": {"Env", "scalar"},
	"SV_SoilDepth":          {"Env", "scalar"},
	"StrataCoverTotal":      {"Admin", "scalar"},
	"SV_FloodPlain":         {"Env", "scalar"},
	"SV_StandAgeEstMeas":    {"Env", "option"},
	"SV_StandHeightEstMeas": {"Env", "option"},
	"SV_PolygonNumber":      {"Env", "text"},
	"SV_CanopyComposition":  {"Env", "text"},
	"SV_RootZoneTexture":    {"Env", "categorical"},
	"SV_AhorizonType":       {"Env", "categorical"},
	"PlotType":              {"Admin", "plot-type"},
}

func validateTwoPageParentCommonField(column string, value vprocore.ProjectMetadataCell) error {
	field, exists := TwoPageParentCommonFields[column]
	if !exists {
		return fmt.Errorf("two-page common field %q is outside its source scope", column)
	}
	switch field.Domain {
	case "scalar":
		return sourceform.ValidateSIVIParentScalar(column, value)
	case "option":
		return sourceform.ValidateSIVIParentOption(column, value)
	case "text":
		return sourceform.ValidateSIVIParentText(column, value)
	case "categorical":
		return sourceform.ValidateSIVIParentCategorical(column, value)
	case "plot-type":
		if value.Storage != "null" && value.Storage != "text" {
			return errors.New("two-page PlotType requires nullable TEXT storage")
		}
		return sqlutil.ValidateSiteCodeText(column, value.Text, 10)
	default:
		return fmt.Errorf("two-page %s has an unavailable common-field domain", column)
	}
}

func PlanTwoPageParentCommonProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("two-page common-field editing requires one owned physical pair and explicit edits")
	}
	parent, err := ProjectTwoPageParent(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}})
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(TwoPageParentCommonFields))
	for column, field := range TwoPageParentCommonFields {
		owners[column] = field.Owner
	}
	result, err := sourceform.PlanProjectedParentCells(ctx, original.ContextID, original.Project, parent, edits, owners, nil, validateTwoPageParentCommonField)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field planning: %w", err)
	}
	return result, nil
}

func TwoPageParentCommonHistory(form string) (sourceform.SiviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Common", "two-page common fields", PlanTwoPageParentCommonProjection)
}

func WriteTwoPageParentCommon(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) (*sourceform.SiviParentWriteResult, error) {
	history, err := TwoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, history,
		func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
			assignments, err := PlanTwoPageParentCommonProjection(ctx, observed, edits)
			if err != nil {
				return nil, err
			}
			if err := sourceform.ValidateSourceParentBooleanAssignments(assignments); err != nil {
				return nil, err
			}
			return assignments, nil
		}, sourceform.SiviParentWriteHooks{Read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page common-field writing: %w", err)
	}
	return result, nil
}

func RestoreTwoPageParentCommon(s *vprocore.ContextService, ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	history, err := TwoPageParentCommonHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.RestoreSourceParentHistory(s, ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page common-field restoration: %w", err)
	}
	return result, nil
}

type TwoPageParentCommonWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Edits    []sourceform.SIVIParentCellEdit  `json:"edits"`
}

func (request *TwoPageParentCommonWrite) UnmarshalJSON(data []byte) error {
	type plain TwoPageParentCommonWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("two-page common-field transport: %w", err)
	}
	*request = TwoPageParentCommonWrite(decoded)
	return nil
}

type TwoPageParentCommonService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageParentCommonService(contexts *vprocore.ContextService) (*TwoPageParentCommonService, error) {
	if contexts == nil {
		return nil, errors.New("two-page common-field service requires owned contexts")
	}
	return &TwoPageParentCommonService{contexts: contexts}, nil
}

func (s *TwoPageParentCommonService) require(ctx context.Context, _ bool) error {
	if ctx == nil {
		return errors.New("two-page common-field service requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("two-page common-field service requires owned contexts")
	}
	return nil
}

func (s *TwoPageParentCommonService) GetOriginal(ctx context.Context, contextID, plot, form string) (*sourceform.SIVIParentProjection, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ReadTwoPageParent(s.contexts, ctx, contextID, plot, form)
}

func (s *TwoPageParentCommonService) Save(ctx context.Context, contextID, plot, form string, request TwoPageParentCommonWrite) (*sourceform.SIVIParentWriteResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return WriteTwoPageParentCommon(s.contexts, ctx, contextID, plot, form, request.Original, request.Edits)
}

func (s *TwoPageParentCommonService) Restore(ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx, true); err != nil {
		return nil, err
	}
	return RestoreTwoPageParentCommon(s.contexts, ctx, contextID, plot, form, historyID, action)
}

type TwoPageSourceAuditOriginal struct {
	ContextID    string                        `json:"contextId"`
	Project      string                        `json:"project"`
	Plot         string                        `json:"plot"`
	Form         string                        `json:"form"`
	RecordSource string                        `json:"recordSource"`
	Table        string                        `json:"table"`
	Records      vprocore.ProjectMetadataTable `json:"records"`
}

type TwoPageSourceAuditService struct {
	contexts *vprocore.ContextService
}

func NewTwoPageSourceAuditService(contexts *vprocore.ContextService) (*TwoPageSourceAuditService, error) {
	if contexts == nil {
		return nil, errors.New("two-page source audit requires owned contexts")
	}
	return &TwoPageSourceAuditService{contexts: contexts}, nil
}

func (s *TwoPageSourceAuditService) GetOriginal(ctx context.Context, contextID, plot, form string) (*TwoPageSourceAuditOriginal, error) {
	if ctx == nil {
		return nil, errors.New("two-page source audit requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil {
		return nil, errors.New("two-page source audit requires owned contexts")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page source audit requires an exact valid literal parent")
	}
	if form != "FS882-8x6XL" && form != "FS882-8x6XL-CHARS" {
		return nil, fmt.Errorf("two-page source audit does not support source form %q", form)
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*TwoPageSourceAuditOriginal, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*TwoPageSourceAuditOriginal, error) {
			project := owner.Selection.Project
			table := project + "_Audit"
			if err := sourceform.RequireTwoPageChildReadScope(ctx, tx, project, table, plot, "audit"); err != nil {
				return nil, err
			}
			if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "project", table, "two-page source audit"); err != nil {
				return nil, err
			}
			columns, err := vprocore.ReadSQLiteStorageColumns(ctx, tx, "project", table)
			if err != nil {
				return nil, err
			}
			for _, name := range []string{"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID"} {
				found := false
				for _, column := range columns {
					found = found || column.Name == name
				}
				if !found {
					return nil, fmt.Errorf("two-page source audit requires exact source column %q; no binding was repaired", name)
				}
			}
			records, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "project", table, "PlotNumber", plot, "EditWhen")
			if err != nil {
				return nil, fmt.Errorf("two-page source audit storage: %w", err)
			}
			for _, row := range records.Rows {
				for index, column := range records.Columns {
					if column.Name == "Project" && (row.Cells[index].Storage != "text" || row.Cells[index].Text == nil || *row.Cells[index].Text != project) {
						return nil, fmt.Errorf("two-page source audit row %s has foreign or nonliteral project ownership", row.RowID)
					}
				}
			}
			return &TwoPageSourceAuditOriginal{ContextID: contextID, Project: project, Plot: plot, Form: form,
				RecordSource: "UsysAuditTrail", Table: table, Records: records}, nil
		})
	})
}
