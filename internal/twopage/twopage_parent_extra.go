package twopage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/boostao/vpro-wails/internal/fs882layout"
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
)

var twoPageParentLayoutJSON = resources.FS882TwoPageLayoutJSON

var twoPageCHARSParentLayoutJSON = resources.FS882TwoPageCHARSLayoutJSON

var twoPageParentSources = sync.OnceValues(func() (map[string]*fs882layout.Layout, error) {
	result := make(map[string]*fs882layout.Layout, 2)
	for _, source := range []struct {
		form string
		data []byte
	}{{"FS882-8x6XL", twoPageParentLayoutJSON}, {"FS882-8x6XL-CHARS", twoPageCHARSParentLayoutJSON}} {
		var layout fs882layout.Layout
		if err := json.Unmarshal(source.data, &layout); err != nil {
			return nil, fmt.Errorf("two-page parent metadata: %w", err)
		}
		if layout.Root != source.form || len(layout.Forms) != 7 || layout.Forms[0].Name != source.form ||
			layout.Forms[0].RecordSource != "USysEnv" || len(layout.Forms[0].Pages) != 2 {
			return nil, errors.New("two-page parent requires its exact exported variant and source closure")
		}
		result[source.form] = &layout
	}
	return result, nil
})

func twoPageParentBindings(form string) ([]sourceform.SiviParentBinding, error) {
	sources, err := twoPageParentSources()
	if err != nil {
		return nil, err
	}
	layout, exists := sources[form]
	if !exists {
		return nil, errors.New("two-page parent requires an exact normal or CHARS source variant")
	}
	result := []sourceform.SiviParentBinding{}
	controls, columns := map[string]bool{}, map[string]bool{}
	for _, field := range layout.Forms[0].Fields {
		if field.Binding == "" {
			continue
		}
		if field.ControlID == "" || controls[field.ControlID] || !field.ReadOnly || field.Implementation != "unmapped" {
			return nil, errors.New("two-page parent requires distinct unmapped source control identities")
		}
		controls[field.ControlID] = true
		columns[strings.ToLower(field.Binding)] = true
		result = append(result, sourceform.SiviParentBinding{ControlID: field.ControlID, Binding: field.Binding})
	}
	expectedColumns, expectedInstances := 118, 121
	if form == "FS882-8x6XL-CHARS" {
		expectedColumns, expectedInstances = 120, 122
	}
	if len(columns) != expectedColumns || len(result) != expectedInstances {
		return nil, errors.New("two-page parent source instance/column counts changed")
	}
	return result, nil
}

func ProjectTwoPageParent(ctx context.Context, contextID, project, plot, form string, env, admin vprocore.ProjectMetadataTable) (*sourceform.SiviParentProjection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if contextID == "" || project == "" {
		return nil, errors.New("two-page parent requires owned context/project identities")
	}
	bindings, err := twoPageParentBindings(form)
	if err != nil {
		return nil, err
	}
	parent, err := sourceform.ProjectSourceParent(ctx, contextID, project, plot, form, bindings, env, admin)
	if err != nil {
		return nil, fmt.Errorf("two-page parent projection: %w", err)
	}
	return parent, nil
}

func planTwoPageParentExtraFields(ctx context.Context, contextID, project, plot, form string, env, admin vprocore.ProjectMetadataTable, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	parent, err := ProjectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string, len(sourceform.TwoPageParentExtraFields))
	for column, field := range sourceform.TwoPageParentExtraFields {
		owners[column] = field.Owner
	}
	if len(edits) == 0 {
		return nil, fmt.Errorf("two-page additional-field planning requires explicit edits")
	}
	result, err := sourceform.PlanProjectedParentCells(ctx, contextID, project, parent, edits, owners, nil, validateTwoPageParentExtraField)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field planning: %w", err)
	}
	return result, nil
}

func validateTwoPageParentExtraField(column string, value vprocore.ProjectMetadataCell) error {
	field, exists := sourceform.TwoPageParentExtraFields[column]
	if !exists {
		return fmt.Errorf("two-page field %q is outside additional-field source scope", column)
	}
	if value.Storage == "null" {
		return nil
	}
	switch field.Domain {
	case "single":
		if value.Storage != "real" || value.Real == nil {
			return fmt.Errorf("two-page %s requires nullable SINGLE real storage", column)
		}
		return vprocore.ValidateSingleRangeChange(column, nil, value.Real)
	case "integer":
		return sourceform.ValidateSIVIParentSharedNumber(column, "integer", value)
	case "text":
		if value.Storage != "text" || value.Text == nil {
			return fmt.Errorf("two-page %s requires nullable text storage", column)
		}
		return sqlutil.ValidateSiteCodeText(column, value.Text, field.Maximum)
	default:
		return fmt.Errorf("two-page %s has an unavailable source domain", column)
	}
}

func ReadTwoPageParent(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string) (*sourceform.SiviParentProjection, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SiviParentProjection, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*sourceform.SiviParentProjection, error) {
			tables, err := sourceform.ReadSourceParentTables(ctx, owner, tx, plot)
			if err != nil {
				return nil, err
			}
			return ProjectTwoPageParent(ctx, contextID, owner.Selection.Project, plot, form, tables[0], tables[1])
		})
	})
}

func TwoPageParentProjector(form string) (sourceform.SourceParentProjector, error) {
	if _, err := twoPageParentBindings(form); err != nil {
		return nil, err
	}
	return func(ctx context.Context, contextID, project, plot string, env, admin vprocore.ProjectMetadataTable) (*sourceform.SiviParentProjection, error) {
		return ProjectTwoPageParent(ctx, contextID, project, plot, form, env, admin)
	}, nil
}

func TwoPageParentExtraHistory(form string) (sourceform.SiviParentHistoryDomain, error) {
	return twoPageParentHistory(form, "Extra", "two-page additional fields", PlanTwoPageParentExtraProjection)
}

func twoPageParentHistory(form, scope, label string, plan func(context.Context, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error)) (sourceform.SiviParentHistoryDomain, error) {
	projectParent, err := TwoPageParentProjector(form)
	if err != nil {
		return sourceform.SiviParentHistoryDomain{}, err
	}
	table := "__VPRO_TwoPageParent" + scope + "History"
	if form == "FS882-8x6XL-CHARS" {
		table = "__VPRO_TwoPageCHARSParent" + scope + "History"
	}
	return sourceform.SiviParentHistoryDomain{
		Table: table,
		Schema: `CREATE TABLE ` + vprocore.QuoteHeaderIdentifier(table) +
			`(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`,
		Label: label,
		Assignments: func(ctx context.Context, event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
			if event.ProjectAssignment != nil || event.EntryPlan != nil || event.Committed == nil || event.Committed.Form != form {
				return nil, fmt.Errorf("%s history belongs to another source domain", label)
			}
			original, err := sourceform.ValidateSourceParentHistoryOriginal(ctx, event, project, plot, projectParent)
			if err != nil {
				return nil, err
			}
			edits := make([]sourceform.SiviParentScalarEdit, 0, len(event.Changes))
			for _, change := range event.Changes {
				edits = append(edits, sourceform.SiviParentScalarEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
			}
			assignments, err := plan(ctx, original, edits)
			if err != nil || len(assignments) != len(event.Changes) {
				return nil, errors.Join(err, errors.New("two-page typed history contains repeated or unchanged assignments"))
			}
			return assignments, nil
		},
	}, nil
}

func PlanTwoPageParentExtraProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("two-page additional-field editing requires one owned physical pair")
	}
	return planTwoPageParentExtraFields(ctx, original.ContextID, original.Project, original.Plot, original.Form,
		vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}},
		vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}, edits)
}

func TwoPageParentWriteReader(form string) (func(context.Context, *sql.Tx, *vprocore.SqliteContext, string, string, string) (*sourceform.SiviParentProjection, error), error) {
	projectParent, err := TwoPageParentProjector(form)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, suAlias, contextID, plot string) (*sourceform.SiviParentProjection, error) {
		return sourceform.ReadSourceParentForWrite(ctx, tx, owner, suAlias, contextID, plot, projectParent)
	}, nil
}

func WriteTwoPageParentExtra(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) (*sourceform.SiviParentWriteResult, error) {
	history, err := TwoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.WriteSIVIParentPlannedWithHooks(s, ctx, contextID, plot, original, history,
		func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
			return PlanTwoPageParentExtraProjection(ctx, observed, edits)
		}, sourceform.SiviParentWriteHooks{Read: read})
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field writing: %w", err)
	}
	return result, nil
}

func RestoreTwoPageParentExtra(s *vprocore.ContextService, ctx context.Context, contextID, plot, form, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	history, err := TwoPageParentExtraHistory(form)
	if err != nil {
		return nil, err
	}
	read, err := TwoPageParentWriteReader(form)
	if err != nil {
		return nil, err
	}
	result, err := sourceform.RestoreSourceParentHistory(s, ctx, contextID, plot, historyID, action, history, read)
	if err != nil {
		return nil, fmt.Errorf("two-page additional-field restoration: %w", err)
	}
	return result, nil
}
