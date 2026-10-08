package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func twoPageEntryEdits(t *testing.T, parent *siviParentProjection) []siviParentScalarEdit {
	t.Helper()
	xl, err := twoPageXLFields(parent.Form)
	if err != nil {
		t.Fatal(err)
	}
	common := twoPageCommonValues()
	extra := map[string]ProjectMetadataCell{}
	for column, field := range twoPageParentExtraFields {
		switch field.domain {
		case "single":
			extra[column] = siviReal(1)
		case "integer":
			extra[column] = metadataInteger("1")
		case "text":
			extra[column] = metadataText("X")
		default:
			t.Fatal("unknown source extra policy", column)
		}
	}
	seen := map[string]bool{}
	edits := []siviParentScalarEdit{}
	for _, binding := range parent.Bindings {
		if seen[binding.Binding] || binding.Binding == "PlotNumber" {
			continue
		}
		seen[binding.Binding] = true
		value, found := common[binding.Binding]
		if !found {
			value, found = extra[binding.Binding]
		}
		if !found {
			policy, exists := xl[binding.Binding]
			if !exists {
				t.Fatal("source field has no complete-entry test owner", binding.Binding)
			}
			value = twoPageXLValue(policy)
		}
		edits = append(edits, siviParentEdit(t, parent, binding.Binding, value))
	}
	return edits
}

func TestTwoPageEntryCompleteMixedScopePhysicalPlan(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		parent := twoPageXLOriginal(t, form)
		edits := twoPageEntryEdits(t, parent)
		before, err := json.Marshal(parent)
		if err != nil {
			t.Fatal(err)
		}
		got, err := planTwoPageEntryProjection(context.Background(), parent, edits, true)
		want := 117
		if strings.HasSuffix(form, "-CHARS") {
			want = 119
		}
		if err != nil || len(edits) != want || len(got) != want {
			t.Fatal("complete source mixed-scope plan", form, len(edits), len(got), err)
		}
		for _, assignment := range got {
			found := false
			for _, edit := range edits {
				if edit.Table == assignment.Table && edit.Column == assignment.Column {
					found = true
					if !reflect.DeepEqual(assignment.Before, edit.Expected) || !reflect.DeepEqual(assignment.After, edit.Value) {
						t.Fatal("mixed plan changed exact source values", assignment, edit)
					}
				}
			}
			if !found {
				t.Fatal("mixed plan invented an assignment", assignment)
			}
		}
		after, err := json.Marshal(parent)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("mixed planning mutated its reviewed source")
		}
	}
}

func TestTwoPageEntryInvalidScopeTailsNeverLeakPartialPlan(t *testing.T) {
	for _, column := range []string{"FieldNumber", "PlotType", "GIS_BGC_VER"} {
		parent := twoPageXLOriginal(t, "FS882-8x6XL")
		edits := twoPageEntryEdits(t, parent)
		for i := range edits {
			if edits[i].Column == column {
				edits[i].Value = ProjectMetadataCell{Storage: "blob", BlobHex: metadataText("00").Text}
			}
		}
		if got, err := planTwoPageEntryProjection(context.Background(), parent, edits, true); err == nil || got != nil {
			t.Fatal("invalid mixed-scope tail leaked a partial plan", column, got, err)
		}
	}
	parent := twoPageXLOriginal(t, "FS882-8x6XL")
	edits := twoPageEntryEdits(t, parent)
	for _, tail := range []siviParentScalarEdit{
		siviParentEdit(t, parent, "PlotNumber", metadataText("Rename")),
		edits[0],
		{ContextID: parent.ContextID, Column: "Unknown", Value: metadataText("X")},
	} {
		if got, err := planTwoPageEntryProjection(context.Background(), parent, append(edits, tail), true); err == nil || got != nil {
			t.Fatal("foreign or repeated mixed-scope tail leaked assignments", tail, got, err)
		}
	}
	if got, err := planTwoPageEntryProjection(context.Background(), parent, edits, false); err == nil || got != nil {
		t.Fatal("mixed plan inherited Master authority", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planTwoPageEntryProjection(ctx, parent, edits, true); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("mixed cancellation lost", got, err)
	}
}
