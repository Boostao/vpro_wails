package main

import (
	"context"
	"encoding/json"
	"errors"
)

type SummaryVegetationFieldListAssignment struct {
	Before ProjectMetadataRow `json:"before"`
	After  ProjectMetadataRow `json:"after"`
}

type SummaryVegetationFieldListCommittedPlan struct {
	Original    ProjectMetadataTable                   `json:"original"`
	Proposed    ProjectMetadataTable                   `json:"proposed"`
	Assignments []SummaryVegetationFieldListAssignment `json:"assignments"`
}

type SummaryVegetationFieldListOutcome struct {
	ContextID    string                                   `json:"contextId"`
	RequestID    string                                   `json:"requestId"`
	Changed      bool                                     `json:"changed"`
	Committed    bool                                     `json:"committed"`
	Replayed     bool                                     `json:"replayed"`
	Verified     bool                                     `json:"verified"`
	Plan         *SummaryVegetationFieldListCommittedPlan `json:"plan"`
	ErrorMessage string                                   `json:"errorMessage"`
}

func (s *SummaryVegetationFieldListService) SaveSummaryVegetationFieldList(ctx context.Context, contextID, requestJSON string) *SummaryVegetationFieldListOutcome {
	return s.saveSummaryVegetationFieldList(ctx, contextID, requestJSON, summaryFieldListWriterHooks{})
}

func (s *SummaryVegetationFieldListService) saveSummaryVegetationFieldList(ctx context.Context, contextID, requestJSON string,
	hooks summaryFieldListWriterHooks) *SummaryVegetationFieldListOutcome {
	outcome := &SummaryVegetationFieldListOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SummaryVegetationFieldListRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		outcome.RequestID = request.RequestID
		if request.Review.ContextID != contextID {
			err = errors.New("Summary field-list Save request does not match the requested context")
		}
	}
	if err == nil {
		result, commitErr := commitSummaryVegetationFieldList(ctx, s, requestJSON, hooks)
		outcome.Changed, outcome.Committed, outcome.Replayed, outcome.Verified =
			result.Changed, result.Committed, result.Replayed, result.Verified
		if result.Plan != nil {
			plan := &SummaryVegetationFieldListCommittedPlan{
				Original:    cloneSummaryVegetationFieldListTable(result.Plan.Original),
				Proposed:    cloneSummaryVegetationFieldListTable(result.Plan.Proposed),
				Assignments: []SummaryVegetationFieldListAssignment{},
			}
			for _, assignment := range result.Plan.Assignments {
				plan.Assignments = append(plan.Assignments, SummaryVegetationFieldListAssignment{
					Before: cloneSummaryVegetationFieldListRow(assignment.Before),
					After:  cloneSummaryVegetationFieldListRow(assignment.After),
				})
			}
			outcome.Plan = plan
		}
		err = commitErr
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}
