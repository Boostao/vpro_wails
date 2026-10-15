package main

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
	properties, err := sourceChildJSONObject(data, operation, []string{"requestId", "review", "actions"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, operation, "requestId", "review", "actions"); err != nil {
		return err
	}
	reviewKeys := []string{"contextId", "project", "projectPath", "su", "suPath", "sourcePath", "source", "physical"}
	review, err := sourceChildJSONObject(properties["review"], operation+" review", reviewKeys, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(review, operation+" review", reviewKeys...); err != nil {
		return err
	}
	physical, err := sourceChildJSONObject(review["physical"], operation+" physical", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(physical, operation+" physical", "columns", "rows"); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(physical["columns"], func(raw json.RawMessage) error {
		column, err := sourceChildJSONObject(raw, operation+" column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(column, operation+" column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceChildRowsJSON(physical["rows"], operation); err != nil {
		return err
	}
	if err := sourceChildArrayJSON(properties["actions"], func(raw json.RawMessage) error {
		action, err := sourceChildJSONObject(raw, operation+" action", []string{"kind", "rowId", "selected"}, nil)
		if err != nil {
			return err
		}
		return sourceChildJSONNonNull(action, operation+" action", "kind", "rowId", "selected")
	}); err != nil {
		return err
	}
	type plain SummaryVegetationFieldListRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !siviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("Summary field-list selection request requires a stable lowercase UUIDv4")
	}
	authority := decoded.Review
	if err := validateGoogleEarthReviewStrings(authority.ContextID, authority.Project, authority.ProjectPath,
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
