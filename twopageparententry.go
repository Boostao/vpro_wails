package main

import (
	"context"
	"errors"
	"fmt"
)

func planTwoPageEntryProjection(ctx context.Context, original *siviParentProjection, edits []siviParentScalarEdit, masterAllowed bool) ([]siviParentScalarAssignment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original == nil || len(edits) == 0 {
		return nil, errors.New("two-page entry planning requires a reviewed source original and explicit edits")
	}
	xl, err := twoPageXLFields(original.Form)
	if err != nil {
		return nil, err
	}
	var xlEdits, commonEdits, extraEdits []siviParentScalarEdit
	for _, edit := range edits {
		_, inXL := xl[edit.Column]
		_, inCommon := twoPageParentCommonFields[edit.Column]
		_, inExtra := twoPageParentExtraFields[edit.Column]
		owners := 0
		for _, belongs := range []bool{inXL, inCommon, inExtra} {
			if belongs {
				owners++
			}
		}
		if owners != 1 {
			return nil, fmt.Errorf("two-page entry %s requires exactly one accepted field scope", edit.Column)
		}
		switch {
		case inXL:
			xlEdits = append(xlEdits, edit)
		case inCommon:
			commonEdits = append(commonEdits, edit)
		case inExtra:
			extraEdits = append(extraEdits, edit)
		}
	}
	assignments := []siviParentScalarAssignment{}
	for _, scope := range []struct {
		edits []siviParentScalarEdit
		plan  func(context.Context, *siviParentProjection, []siviParentScalarEdit) ([]siviParentScalarAssignment, error)
	}{
		{xlEdits, func(ctx context.Context, parent *siviParentProjection, edits []siviParentScalarEdit) ([]siviParentScalarAssignment, error) {
			return planTwoPageXLProjection(ctx, parent, edits, masterAllowed)
		}},
		{commonEdits, planTwoPageParentCommonProjection},
		{extraEdits, planTwoPageParentExtraProjection},
	} {
		if len(scope.edits) == 0 {
			continue
		}
		planned, err := scope.plan(ctx, original, scope.edits)
		if err != nil {
			return nil, fmt.Errorf("two-page complete entry planning: %w", err)
		}
		assignments = append(assignments, planned...)
	}
	return assignments, nil
}
