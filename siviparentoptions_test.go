package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSIVIParentTextOptionsRetainNativeCoercionAndUnchangedHistory(t *testing.T) {
	for _, column := range []string{"SV_StandAgeEstMeas", "SV_StandHeightEstMeas"} {
		for _, value := range []ProjectMetadataCell{metadataText("1"), metadataText("2"), {Storage: "null"}} {
			env, admin, edit := siviScalarFixture(t, column, metadataText("??"))
			edit.Value = value
			got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || got[0].Table != "Sample_Env" || got[0].RowID != edit.RowID ||
				!reflect.DeepEqual(got[0].Before, metadataText("??")) || !reflect.DeepEqual(got[0].After, value) {
				t.Fatal("source TEXT option mapping or literal invalid history changed", got, err)
			}
			if value.Storage == "null" && got[0].Value != nil || value.Storage == "text" && got[0].Value != *value.Text {
				t.Fatal("option was stored as numeric/caption or NULL repaired", got)
			}
		}
		blob := "ff"
		for _, old := range []ProjectMetadataCell{
			metadataText(""), metadataText("01"), metadataText("  historical  "),
			metadataInteger("1"), {Storage: "blob", BlobHex: &blob},
		} {
			env, admin, edit := siviScalarFixture(t, column, old)
			edit.Value = old
			got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("historical aliases were normalized or assigned", old, got, err)
			}
		}
		for _, invalid := range []ProjectMetadataCell{
			metadataText(""), metadataText("01"), metadataText(" 1"), metadataText("1 "), metadataText("Est."),
			metadataText("Meas."), metadataText("3"), metadataText("\xff"), metadataInteger("1"), siviReal(2),
			{Storage: "text"},
		} {
			env, admin, edit := siviScalarFixture(t, column, ProjectMetadataCell{Storage: "null"})
			edit.Value = invalid
			if got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
				t.Fatal("new option was trimmed/coerced/guessed", invalid, got, err)
			}
		}
	}
}

func TestSIVIParentTextOptionBatchOwnershipScopeCloneAndCancellation(t *testing.T) {
	env, admin, age := siviScalarFixture(t, "SV_StandAgeEstMeas", metadataText("1"))
	age.Value = metadataText("2")
	var height siviParentScalarEdit
	for i, column := range env.Columns {
		if column.Name == "SV_StandHeightEstMeas" {
			env.Rows[0].Cells[i] = metadataText("2")
			height = siviParentScalarEdit{
				ContextID: "owned", Table: age.Table, RowID: age.RowID, Column: column.Name,
				Expected: metadataText("2"), Value: ProjectMetadataCell{Storage: "null"},
			}
		}
	}
	edits := []siviParentScalarEdit{age, height}
	got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	if err != nil || len(got) != 2 || got[0].Value != "2" || got[1].Value != nil {
		t.Fatal("two source groups collapsed or coerced", got, err)
	}
	*got[0].Before.Text, *got[0].After.Text = "changed", "changed"
	if *age.Expected.Text != "1" || *age.Value.Text != "2" {
		t.Fatal("option assignment aliases caller/source")
	}
	for _, kind := range []string{"scalar", "foreign", "stale", "duplicate", "expected", "other-option"} {
		invalid := height
		switch kind {
		case "scalar":
			invalid.Column = "SV_StandHeight"
		case "foreign":
			invalid.Table = "Sample_Admin"
		case "stale":
			invalid.ContextID = "stale"
		case "duplicate":
			invalid = age
		case "expected":
			invalid.Expected = metadataText("1")
		case "other-option":
			invalid.Column = "PlotType"
		}
		if got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{age, invalid}); err == nil || got != nil {
			t.Fatal("invalid tail/cross-domain target returned partial plan", kind, got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-999"
	if got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
		t.Fatal("duplicate joined parent accepted option changes", got, err)
	}
	admin.Rows = admin.Rows[:1]
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 10000}
	if _, err := projectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vegetationCancelContext{Context: context.Background(), remaining: 10000 - probe.remaining + 2}
	if got, err := planSIVIParentOptions(ctx, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight cancellation returned partial option plan", got, err)
	}
}
