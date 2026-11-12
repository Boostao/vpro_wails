package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
)

import (
	"context"
	"errors"
	"fmt"
)

func PlanTwoPageEntryProjection(ctx context.Context, original *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit, masterAllowed bool) ([]sourceform.SiviParentScalarAssignment, error) {
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
	var xlEdits, commonEdits, extraEdits []sourceform.SiviParentScalarEdit
	for _, edit := range edits {
		_, inXL := xl[edit.Column]
		_, inCommon := TwoPageParentCommonFields[edit.Column]
		_, inExtra := sourceform.TwoPageParentExtraFields[edit.Column]
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
	assignments := []sourceform.SiviParentScalarAssignment{}
	for _, scope := range []struct {
		edits []sourceform.SiviParentScalarEdit
		plan  func(context.Context, *sourceform.SiviParentProjection, []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error)
	}{
		{xlEdits, func(ctx context.Context, parent *sourceform.SiviParentProjection, edits []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error) {
			return planTwoPageXLProjection(ctx, parent, edits, masterAllowed)
		}},
		{commonEdits, PlanTwoPageParentCommonProjection},
		{extraEdits, PlanTwoPageParentExtraProjection},
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
