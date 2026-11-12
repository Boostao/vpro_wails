package sivi

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestSIVIParentTextOptionsRetainNativeCoercionAndUnchangedHistory(t *testing.T) {
	for _, column := range []string{"SV_StandAgeEstMeas", "SV_StandHeightEstMeas"} {
		for _, value := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.MetadataText("2"), {Storage: "null"}} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, vprotest.MetadataText("??"))
			edit.Value = value
			got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || got[0].Table != "Sample_Env" || got[0].RowID != edit.RowID ||
				!reflect.DeepEqual(got[0].Before, vprotest.MetadataText("??")) || !reflect.DeepEqual(got[0].After, value) {
				t.Fatal("source TEXT option mapping or literal invalid history changed", got, err)
			}
			if value.Storage == "null" && got[0].Value != nil || value.Storage == "text" && got[0].Value != *value.Text {
				t.Fatal("option was stored as numeric/caption or NULL repaired", got)
			}
		}
		blob := "ff"
		for _, old := range []vprocore.ProjectMetadataCell{
			vprotest.MetadataText(""), vprotest.MetadataText("01"), vprotest.MetadataText("  historical  "),
			vprotest.MetadataInteger("1"), {Storage: "blob", BlobHex: &blob},
		} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, old)
			edit.Value = old
			got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("historical aliases were normalized or assigned", old, got, err)
			}
		}
		for _, invalid := range []vprocore.ProjectMetadataCell{
			vprotest.MetadataText(""), vprotest.MetadataText("01"), vprotest.MetadataText(" 1"), vprotest.MetadataText("1 "), vprotest.MetadataText("Est."),
			vprotest.MetadataText("Meas."), vprotest.MetadataText("3"), vprotest.MetadataText("\xff"), vprotest.MetadataInteger("1"), vprotest.SiviReal(2),
			{Storage: "text"},
		} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, vprocore.ProjectMetadataCell{Storage: "null"})
			edit.Value = invalid
			if got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
				t.Fatal("new option was trimmed/coerced/guessed", invalid, got, err)
			}
		}
	}
}

func TestSIVIParentTextOptionBatchOwnershipScopeCloneAndCancellation(t *testing.T) {
	env, admin, age := sftest.SiviScalarFixture(t, "SV_StandAgeEstMeas", vprotest.MetadataText("1"))
	age.Value = vprotest.MetadataText("2")
	var height sourceform.SiviParentScalarEdit
	for i, column := range env.Columns {
		if column.Name == "SV_StandHeightEstMeas" {
			env.Rows[0].Cells[i] = vprotest.MetadataText("2")
			height = sourceform.SiviParentScalarEdit{
				ContextID: "owned", Table: age.Table, RowID: age.RowID, Column: column.Name,
				Expected: vprotest.MetadataText("2"), Value: vprocore.ProjectMetadataCell{Storage: "null"},
			}
		}
	}
	edits := []sourceform.SiviParentScalarEdit{age, height}
	got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, edits)
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
			invalid.Expected = vprotest.MetadataText("1")
		case "other-option":
			invalid.Column = "PlotType"
		}
		if got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{age, invalid}); err == nil || got != nil {
			t.Fatal("invalid tail/cross-domain target returned partial plan", kind, got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-999"
	if got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
		t.Fatal("duplicate joined parent accepted option changes", got, err)
	}
	admin.Rows = admin.Rows[:1]
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000}
	if _, err := sourceform.ProjectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000 - probe.Remaining + 2}
	if got, err := sourceform.PlanSIVIParentOptions(ctx, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight cancellation returned partial option plan", got, err)
	}
}
