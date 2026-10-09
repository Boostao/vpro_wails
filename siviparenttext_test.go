package main

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
			env, admin, edit := siviScalarFixture(t, field.column, metadataText("original"))
			exactAstral := strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("a", field.maximum%2)
			for _, value := range []ProjectMetadataCell{
				metadataText(strings.Repeat("a", field.maximum)), metadataText(exactAstral),
				metadataText("  Mixed é \t "), metadataText(" "), {Storage: "null"},
			} {
				edit.Value = value
				got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
				var execution any
				if value.Text != nil {
					execution = *value.Text
				}
				want := []siviParentScalarAssignment{{
					ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: field.column,
					Before: metadataText("original"), After: value, Value: execution,
				}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("literal TEXT/NULL/UTF-16 storage changed", got, want, err)
				}
			}
			for _, value := range []ProjectMetadataCell{
				metadataText(""), metadataText(strings.Repeat("a", field.maximum+1)),
				metadataText(exactAstral + "a"), metadataText(string([]byte{0xff})),
				metadataInteger("1"), siviReal(1), {Storage: "text"}, {Storage: "blob"},
			} {
				edit.Value = value
				if got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("new empty/overlength/malformed/nontext value returned a plan", got, err)
				}
			}
		})
	}
}

func TestSIVIParentTextHistoricalOmissionAndClone(t *testing.T) {
	blob := "00ff"
	for _, column := range []string{"SV_PolygonNumber", "SV_CanopyComposition"} {
		for _, history := range []ProjectMetadataCell{
			metadataText(""), metadataText(strings.Repeat("\U0001f600", 51)),
			metadataInteger("2"), siviReal(99), {Storage: "blob", BlobHex: &blob}, {Storage: "null"},
		} {
			env, admin, edit := siviScalarFixture(t, column, history)
			edit.Value = cloneSiteUnitCell(history)
			got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged historical storage became an assignment", column, got, err)
			}
			edit.Value = metadataText("  New literal  ")
			got, err = planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].Before, history) ||
				!reflect.DeepEqual(got[0].After, metadataText("  New literal  ")) || got[0].Value != "  New literal  " {
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
	env, admin, edit := siviScalarFixture(t, "SV_PolygonNumber", metadataText("original"))
	edit.Value = metadataText("new")
	for index, column := range env.Columns {
		if column.Name == "SV_CanopyComposition" {
			env.Rows[0].Cells[index] = metadataText("original-canopy")
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
				tail.Expected = metadataText("stale")
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
				tail.Value = metadataText("")
			}
			if got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit, tail}); err == nil || got != nil {
				t.Fatal("invalid tail/foreign/domain target returned partial output", kind, got, err)
			}
		})
	}
	if got, err := planSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ordinary TEXT widened scalar planning", got, err)
	}
	if got, err := planSIVIParentOptions(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ordinary TEXT widened option planning", got, err)
	}
	for _, empty := range []string{"context", "project", "edits"} {
		id, project, edits := "owned", "Sample", []siviParentScalarEdit{edit}
		switch empty {
		case "context":
			id = ""
		case "project":
			project = ""
		case "edits":
			edits = nil
		}
		if got, err := planSIVIParentText(context.Background(), id, project, "108050", env, admin, edits); err == nil || got != nil {
			t.Fatal("anonymous/implicit TEXT plan returned success", got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-8"
	if got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ambiguous physical pair returned a TEXT plan", got, err)
	}
}

func TestSIVIParentTextCombinedFieldsAndInFlightCancellation(t *testing.T) {
	env, admin, polygon := siviScalarFixture(t, "SV_PolygonNumber", metadataText("original"))
	polygon.Value = metadataText("new")
	canopy := siviParentScalarEdit{
		ContextID: "owned", Table: "Sample_Env", RowID: polygon.RowID, Column: "SV_CanopyComposition",
		Expected: metadataText("original canopy"), Value: metadataText("literal canopy"),
	}
	for index, column := range env.Columns {
		if column.Name == canopy.Column {
			env.Rows[0].Cells[index] = canopy.Expected
		}
	}
	got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{polygon, canopy})
	want := []siviParentScalarAssignment{
		{"owned", "Sample_Env", polygon.RowID, "SV_PolygonNumber", metadataText("original"), metadataText("new"), "new"},
		{"owned", "Sample_Env", polygon.RowID, "SV_CanopyComposition", metadataText("original canopy"), metadataText("literal canopy"), "literal canopy"},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("combined literal field payload changed", got, want, err)
	}
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 10000}
	if _, err := projectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vegetationCancelContext{Context: context.Background(), remaining: 10000 - probe.remaining + 2}
	if got, err := planSIVIParentText(ctx, "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{polygon, canopy}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight TEXT cancellation returned a partial plan", got, err)
	}
}

func TestSIVIParentTextCancellationAndRetry(t *testing.T) {
	env, admin, edit := siviScalarFixture(t, "SV_PolygonNumber", metadataText("original"))
	edit.Value = metadataText("new")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSIVIParentText(ctx, "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled TEXT plan returned assignments", got, err)
	}
	got, err := planSIVIParentText(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
	if err != nil || len(got) != 1 || got[0].Value != "new" {
		t.Fatal("rejected TEXT plan changed original or retry", got, err)
	}
}
