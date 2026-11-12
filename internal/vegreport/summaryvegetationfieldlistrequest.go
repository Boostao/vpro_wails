package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type SummaryVegetationFieldListAction struct {
	Kind     string `json:"kind"`
	RowID    string `json:"rowId"`
	Selected bool   `json:"selected"`
}

type SummaryVegetationFieldListRequest struct {
	RequestID string                             `json:"requestId"`
	Review    SummaryVegetationFieldListReview   `json:"review"`
	Actions   []SummaryVegetationFieldListAction `json:"actions"`
}

func (request SummaryVegetationFieldListRequest) plannedActions() []summaryVegetationFieldListAction {
	if request.Actions == nil {
		return nil
	}
	actions := make([]summaryVegetationFieldListAction, len(request.Actions))
	for i, action := range request.Actions {
		actions[i] = summaryVegetationFieldListAction{Kind: action.Kind, RowID: action.RowID, Selected: action.Selected}
	}
	return actions
}

func (request *SummaryVegetationFieldListRequest) UnmarshalJSON(data []byte) error {
	const operation = "Summary field-list selection request"
	properties, err := sourceform.SourceChildJSONObject(data, operation, []string{"requestId", "review", "actions"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, operation, "requestId", "review", "actions"); err != nil {
		return err
	}
	reviewKeys := []string{"contextId", "project", "projectPath", "su", "suPath", "sourcePath", "source", "physical"}
	review, err := sourceform.SourceChildJSONObject(properties["review"], operation+" review", reviewKeys, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(review, operation+" review", reviewKeys...); err != nil {
		return err
	}
	physical, err := sourceform.SourceChildJSONObject(review["physical"], operation+" physical", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(physical, operation+" physical", "columns", "rows"); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(physical["columns"], func(raw json.RawMessage) error {
		column, err := sourceform.SourceChildJSONObject(raw, operation+" column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceform.SourceChildJSONNonNull(column, operation+" column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceform.SourceChildRowsJSON(physical["rows"], operation); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["actions"], func(raw json.RawMessage) error {
		action, err := sourceform.SourceChildJSONObject(raw, operation+" action", []string{"kind", "rowId", "selected"}, nil)
		if err != nil {
			return err
		}
		return sourceform.SourceChildJSONNonNull(action, operation+" action", "kind", "rowId", "selected")
	}); err != nil {
		return err
	}
	type plain SummaryVegetationFieldListRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !vprocore.SiviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("Summary field-list selection request requires a stable lowercase UUIDv4")
	}
	authority := decoded.Review
	if err := vprocore.ValidateGoogleEarthReviewStrings(authority.ContextID, authority.Project, authority.ProjectPath,
		authority.SU, authority.SUPath, authority.SourcePath, authority.Source); err != nil {
		return err
	}
	for _, value := range []string{authority.ContextID, authority.Project, authority.ProjectPath,
		authority.SU, authority.SUPath, authority.SourcePath, authority.Source} {
		if strings.ContainsRune(value, 0) {
			return errors.New("Summary field-list owned provenance must not contain NUL")
		}
	}
	if authority.ContextID == "" || authority.Project == "" || authority.ProjectPath == "" || authority.SU == "" ||
		authority.SU != "None" && authority.SUPath == "" || authority.SourcePath == "" || authority.Source != summaryVegetationFieldListSource {
		return errors.New("Summary field-list selection request requires complete literal owned provenance")
	}
	result := SummaryVegetationFieldListRequest(decoded)
	if _, err := planSummaryVegetationFieldList(context.Background(), authority.Physical, authority.Physical, result.plannedActions()); err != nil {
		return err
	}
	*request = result
	return nil
}
