package integration

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
	"strings"
	"testing"
)

func TestSIVIParentTextPhysicalBoundsAndLiteralStorage(t *testing.T) {
	for _, field := range []struct {
		column  string
		maximum int
	}{{"SV_PolygonNumber", 25}, {"SV_CanopyComposition", 50}} {
		t.Run(field.column, func(t *testing.T) {
			env, admin, edit := sftest.SiviScalarFixture(t, field.column, vprotest.MetadataText("original"))
			exactAstral := strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("a", field.maximum%2)
			for _, value := range []vprocore.ProjectMetadataCell{
				vprotest.MetadataText(strings.Repeat("a", field.maximum)), vprotest.MetadataText(exactAstral),
				vprotest.MetadataText("  Mixed é \t "), vprotest.MetadataText(" "), {Storage: "null"},
			} {
				edit.Value = value
				got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
				var execution any
				if value.Text != nil {
					execution = *value.Text
				}
				want := []sourceform.SiviParentScalarAssignment{{
					ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: field.column,
					Before: vprotest.MetadataText("original"), After: value, Value: execution,
				}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("literal TEXT/NULL/UTF-16 storage changed", got, want, err)
				}
			}
			for _, value := range []vprocore.ProjectMetadataCell{
				vprotest.MetadataText(""), vprotest.MetadataText(strings.Repeat("a", field.maximum+1)),
				vprotest.MetadataText(exactAstral + "a"), vprotest.MetadataText(string([]byte{0xff})),
				vprotest.MetadataInteger("1"), vprotest.SiviReal(1), {Storage: "text"}, {Storage: "blob"},
			} {
				edit.Value = value
				if got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("new empty/overlength/malformed/nontext value returned a plan", got, err)
				}
			}
		})
	}
}

func TestSIVIParentTextHistoricalOmissionAndClone(t *testing.T) {
	blob := "00ff"
	for _, column := range []string{"SV_PolygonNumber", "SV_CanopyComposition"} {
		for _, history := range []vprocore.ProjectMetadataCell{
			vprotest.MetadataText(""), vprotest.MetadataText(strings.Repeat("\U0001f600", 51)),
			vprotest.MetadataInteger("2"), vprotest.SiviReal(99), {Storage: "blob", BlobHex: &blob}, {Storage: "null"},
		} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, history)
			edit.Value = vprocore.CloneSiteUnitCell(history)
			got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged historical storage became an assignment", column, got, err)
			}
			edit.Value = vprotest.MetadataText("  New literal  ")
			got, err = sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].Before, history) ||
				!reflect.DeepEqual(got[0].After, vprotest.MetadataText("  New literal  ")) || got[0].Value != "  New literal  " {
				t.Fatal("correction changed history or new literal", got, err)
			}
			*got[0].After.Text = "caller"
			if *edit.Value.Text != "  New literal  " {
				t.Fatal("TEXT assignment shares proposed ownership")
			}
			if got[0].Before.Text != nil {
				*got[0].Before.Text = "caller"
				if *history.Text == "caller" || *edit.Expected.Text == "caller" {
					t.Fatal("TEXT assignment shares historical ownership")
				}
			}
		}
	}
}

func TestSIVIParentTextInvalidTailOwnershipAndDomainIsolation(t *testing.T) {
	env, admin, edit := sftest.SiviScalarFixture(t, "SV_PolygonNumber", vprotest.MetadataText("original"))
	edit.Value = vprotest.MetadataText("new")
	for index, column := range env.Columns {
		if column.Name == "SV_CanopyComposition" {
			env.Rows[0].Cells[index] = vprotest.MetadataText("original-canopy")
		}
	}
	for _, kind := range []string{"context", "table", "row", "expected", "duplicate", "scalar", "option", "categorical", "implicit", "empty-tail"} {
		t.Run(kind, func(t *testing.T) {
			tail := edit
			switch kind {
			case "context":
				tail.ContextID = "stale"
			case "table":
				tail.Table = "Sample_Admin"
			case "row":
				tail.RowID = "-999"
			case "expected":
				tail.Expected = vprotest.MetadataText("stale")
			case "scalar":
				tail.Column = "SV_StandHeight"
			case "option":
				tail.Column = "SV_StandAgeEstMeas"
			case "categorical":
				tail.Column = "SV_AhorizonType"
			case "implicit":
				tail.Column = "SpeciesListComplete"
			case "empty-tail":
				tail.Column = "SV_CanopyComposition"
				for index, column := range env.Columns {
					if column.Name == tail.Column {
						tail.Expected = env.Rows[0].Cells[index]
					}
				}
				tail.Value = vprotest.MetadataText("")
			}
			if got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit, tail}); err == nil || got != nil {
				t.Fatal("invalid tail/foreign/domain target returned partial output", kind, got, err)
			}
		})
	}
	if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ordinary TEXT widened scalar planning", got, err)
	}
	if got, err := sourceform.PlanSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ordinary TEXT widened option planning", got, err)
	}
	for _, empty := range []string{"context", "project", "edits"} {
		id, project, edits := "owned", "Sample", []sourceform.SiviParentScalarEdit{edit}
		switch empty {
		case "context":
			id = ""
		case "project":
			project = ""
		case "edits":
			edits = nil
		}
		if got, err := sourceform.PlanSIVIParentText(context.Background(), id, project, "108050", env, admin, edits); err == nil || got != nil {
			t.Fatal("anonymous/implicit TEXT plan returned success", got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-8"
	if got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ambiguous physical pair returned a TEXT plan", got, err)
	}
}

func TestSIVIParentTextCombinedFieldsAndInFlightCancellation(t *testing.T) {
	env, admin, polygon := sftest.SiviScalarFixture(t, "SV_PolygonNumber", vprotest.MetadataText("original"))
	polygon.Value = vprotest.MetadataText("new")
	canopy := sourceform.SiviParentScalarEdit{
		ContextID: "owned", Table: "Sample_Env", RowID: polygon.RowID, Column: "SV_CanopyComposition",
		Expected: vprotest.MetadataText("original canopy"), Value: vprotest.MetadataText("literal canopy"),
	}
	for index, column := range env.Columns {
		if column.Name == canopy.Column {
			env.Rows[0].Cells[index] = canopy.Expected
		}
	}
	got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{polygon, canopy})
	want := []sourceform.SiviParentScalarAssignment{
		{ContextID: "owned", Table: "Sample_Env", RowID: polygon.RowID, Column: "SV_PolygonNumber", Before: vprotest.MetadataText("original"), After: vprotest.MetadataText("new"), Value: "new"},
		{ContextID: "owned", Table: "Sample_Env", RowID: polygon.RowID, Column: "SV_CanopyComposition", Before: vprotest.MetadataText("original canopy"), After: vprotest.MetadataText("literal canopy"), Value: "literal canopy"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("combined literal field payload changed", got, want, err)
	}
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000}
	if _, err := sourceform.ProjectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000 - probe.Remaining + 2}
	if got, err := sourceform.PlanSIVIParentText(ctx, "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{polygon, canopy}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight TEXT cancellation returned a partial plan", got, err)
	}
}

func TestSIVIParentTextCancellationAndRetry(t *testing.T) {
	env, admin, edit := sftest.SiviScalarFixture(t, "SV_PolygonNumber", vprotest.MetadataText("original"))
	edit.Value = vprotest.MetadataText("new")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.PlanSIVIParentText(ctx, "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled TEXT plan returned assignments", got, err)
	}
	got, err := sourceform.PlanSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
	if err != nil || len(got) != 1 || got[0].Value != "new" {
		t.Fatal("rejected TEXT plan changed original or retry", got, err)
	}
}
