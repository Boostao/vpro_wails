package sivi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type SIVIParentActionInput struct {
	ContextID string                       `json:"contextId"`
	ControlID string                       `json:"controlId"`
	Table     string                       `json:"table"`
	RowID     string                       `json:"rowId"`
	Expected  vprocore.ProjectMetadataCell `json:"expected"`
	Option    *int                         `json:"option"`
}

type SIVIParentActionWrite struct {
	Original *sourceform.SIVIParentProjection `json:"original"`
	Actions  []SIVIParentActionInput          `json:"actions"`
}

type SIVIParentActionSaveResult struct {
	ChangedCells          int
	HistoryID             string
	SourceRefreshRequired bool
}

func (input *SIVIParentActionInput) UnmarshalJSON(data []byte) error {
	type plain SIVIParentActionInput
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "contextId", "controlId", "table", "rowId", "expected"); err != nil {
		return fmt.Errorf("SIVI parent action transport: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if _, present := properties["option"]; !present {
		return errors.New("SIVI parent action transport requires an explicit option, including NULL")
	}
	*input = SIVIParentActionInput(decoded)
	return nil
}

func (request *SIVIParentActionWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentActionWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "actions"); err != nil {
		return fmt.Errorf("SIVI parent action-write transport: %w", err)
	}
	*request = SIVIParentActionWrite(decoded)
	return nil
}

func RequireSIVIParentActionEditing(s *vprocore.ContextService, ctx context.Context) error {
	return sourceform.RequireSIVIParentEditing(s, ctx)
}

func GetSIVIParentActionOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentProjection, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIParent(s, ctx, contextID, plot)
}

func SaveSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot string, request SIVIParentActionWrite) (*SIVIParentActionSaveResult, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	actions := make([]sourceform.SiviParentActionEdit, len(request.Actions))
	for index, input := range request.Actions {
		actions[index] = sourceform.SiviParentActionEdit{ContextID: input.ContextID, ControlID: input.ControlID, Table: input.Table, RowID: input.RowID, Expected: input.Expected, Option: input.Option}
	}
	result, err := WriteSIVIParentActions(s, ctx, contextID, plot, request.Original, actions)
	if err != nil {
		return nil, err
	}
	return &SIVIParentActionSaveResult{result.ChangedCells, result.HistoryID, result.SourceRefreshRequired}, nil
}

func RestoreSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := RequireSIVIParentActionEditing(s, ctx); err != nil {
		return nil, err
	}
	return CtxRestoreSIVIParentActions(s, ctx, contextID, plot, historyID, action)
}

const siviParentActionHistoryTable = "__VPRO_SIVIParentActionHistory"
const siviParentActionHistorySQL = `CREATE TABLE "__VPRO_SIVIParentActionHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviParentActionHistory = sourceform.SiviParentHistoryDomain{
	Table: siviParentActionHistoryTable, Schema: siviParentActionHistorySQL, Label: "SIVI parent action", Assignments: siviParentActionHistoryAssignments,
}

type siviParentActionWriteResult struct {
	sourceform.SiviParentWriteResult
	SourceRefreshRequired bool
}

func planSIVIParentActionEdits(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentActionEdit) ([]sourceform.SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 || len(edits) == 0 {
		return nil, errors.New("SIVI parent action writing requires one physical pair and explicit source actions")
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	return sourceform.PlanSIVIParentActions(ctx, original.ContextID, original.Project, original.Plot, env, admin, edits)
}

func WriteSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot string, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentActionEdit) (*siviParentActionWriteResult, error) {
	refresh := false
	result, err := sourceform.WriteSIVIParentPlanned(s, ctx, contextID, plot, original, siviParentActionHistory, func(observed *sourceform.SiviParentProjection) ([]sourceform.SiviParentScalarAssignment, error) {
		assignments, err := planSIVIParentActionEdits(ctx, observed, edits)
		if err != nil {
			return nil, err
		}
		if err := sourceform.ValidateSourceParentBooleanAssignments(assignments); err != nil {
			return nil, fmt.Errorf("SIVI source action: %w", err)
		}
		sources, err := sourceform.SiviParentActionSources()
		if err != nil {
			return nil, err
		}
		for _, edit := range edits {
			refresh = refresh || sources[edit.ControlID].Binding == "PlotType"
		}
		return assignments, nil
	})
	if err != nil {
		return nil, err
	}
	return &siviParentActionWriteResult{*result, refresh}, nil
}

func siviParentActionHistoryAssignments(ctx context.Context, event sourceform.SiviParentHistory, project, plot string) ([]sourceform.SiviParentScalarAssignment, error) {
	original, err := sourceform.ValidateSIVIParentHistoryOriginal(ctx, event, project, plot)
	if err != nil {
		return nil, err
	}
	sources, err := sourceform.SiviParentActionSources()
	if err != nil {
		return nil, err
	}
	edits := make([]sourceform.SiviParentActionEdit, 0, len(event.Changes))
	for _, change := range event.Changes {
		control := ""
		binding := ""
		for id, source := range sources {
			if source.Binding == change.Column {
				control, binding = id, source.Binding
			}
		}
		if control == "" {
			return nil, errors.New("typed SIVI action history contains an unavailable direct or identity target")
		}
		options := []*int{}
		if binding == "SpeciesListComplete" {
			options = append(options, nil)
		}
		max := 5
		if binding == "SpeciesListComplete" {
			max = 2
		}
		for value := 1; value <= max; value++ {
			option := value
			options = append(options, &option)
		}
		matched := false
		for _, option := range options {
			edit := sourceform.SiviParentActionEdit{ContextID: original.ContextID, ControlID: control, Table: change.Table, RowID: change.RowID, Expected: change.Before, Option: option}
			planned, err := planSIVIParentActionEdits(ctx, original, []sourceform.SiviParentActionEdit{edit})
			if err != nil {
				return nil, err
			}
			if len(planned) == 1 && reflect.DeepEqual(planned[0].After, change.After) {
				edits = append(edits, edit)
				matched = true
				break
			}
		}
		if !matched {
			return nil, errors.New("typed SIVI action history contains an unchanged or unverified source value")
		}
	}
	assignments, err := planSIVIParentActionEdits(ctx, original, edits)
	if err != nil || len(assignments) != len(event.Changes) {
		return nil, errors.Join(err, errors.New("typed SIVI action history contains repeated or unchanged assignments"))
	}
	return assignments, nil
}

func CtxRestoreSIVIParentActions(s *vprocore.ContextService, ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	return sourceform.RestoreSIVIParentHistory(s, ctx, contextID, plot, historyID, action, siviParentActionHistory)
}

func requireSIVIParentReview(s *vprocore.ContextService, ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI parent review requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil {
		return errors.New("SIVI parent review requires owned contexts")
	}
	return nil
}

func GetSIVIParentOriginal(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentProjection, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIParent(s, ctx, contextID, plot)
}

func GetSIVIParentJoinReview(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (*sourceform.SIVIParentJoinReview, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SIVIParentJoinReview, error) {
		return sourceform.ReadOwnedSIVIParentJoin(ctx, plots, contextID, plot)
	})
}

func GetSIVIProjectIDChoices(s *vprocore.ContextService, ctx context.Context, contextID string) (*sourceform.SIVIProjectChoices, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return sourceform.ReadSIVIProjectChoices(s, ctx, contextID)
}

func SetSIVIProjectIDSource(s *vprocore.ContextService, ctx context.Context, contextID string, expected, source int) (*sourceform.SIVIProjectChoices, error) {
	if err := requireSIVIParentReview(s, ctx); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*sourceform.SIVIProjectChoices, error) {
		choices, err := sourceform.ReadOwnedSIVIProjectChoicesAtSource(ctx, plots, contextID, source)
		if err != nil {
			return nil, err
		}
		if err := plots.Projects.Preferences.CompareAndSetProjectIDSource(ctx, expected, source); err != nil {
			return nil, err
		}
		return choices, nil
	})
}
