package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIParentCategoricalExactBoundsAndStorage(t *testing.T) {
	for _, field := range []struct {
		column  string
		maximum int
	}{{"SnowCoverregime", 1}, {"SV_RootZoneTexture", 100}, {"SV_AhorizonType", 5}} {
		t.Run(field.column, func(t *testing.T) {
			env, admin, edit := siviScalarFixture(t, field.column, metadataText("old"))
			exactAstral := strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("a", field.maximum%2)
			for _, value := range []ProjectMetadataCell{
				metadataText(""), {Storage: "null"}, metadataText(" "),
				metadataText(strings.Repeat("é", field.maximum)), metadataText(exactAstral),
				metadataText("Z"),
			} {
				edit.Value = value
				got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
				var execution any
				if value.Text != nil {
					execution = *value.Text
				}
				want := []siviParentScalarAssignment{{
					ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: field.column,
					Before: metadataText("old"), After: value, Value: execution,
				}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("categorical NULL/empty/unlisted/UTF-16 payload changed", got, want, err)
				}
			}
			for _, value := range []ProjectMetadataCell{
				metadataText(strings.Repeat("a", field.maximum+1)), metadataText(exactAstral + "a"),
				metadataText(string([]byte{0xff})), metadataInteger("1"), siviReal(1),
				{Storage: "text"}, {Storage: "blob"}, {Storage: "null", Text: metadataText("").Text},
				{Storage: "text", Text: metadataText("").Text, Integer: metadataInteger("1").Integer},
			} {
				edit.Value = value
				if got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("new overlength/malformed/nontext categorical value returned a plan", got, err)
				}
			}
		})
	}
}

func TestSIVIParentCategoricalHistoryAndClone(t *testing.T) {
	blob := "00ff"
	for _, column := range []string{"SnowCoverregime", "SV_RootZoneTexture", "SV_AhorizonType"} {
		for _, history := range []ProjectMetadataCell{
			metadataText(""), {Storage: "null"}, metadataText(strings.Repeat("\U0001f600", 101)),
			metadataInteger("-1"), siviReal(99), {Storage: "blob", BlobHex: &blob},
		} {
			env, admin, edit := siviScalarFixture(t, column, history)
			edit.Value = cloneSiteUnitCell(history)
			got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged categorical history became an assignment", column, got, err)
			}
			edit.Value = metadataText("z")
			got, err = planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit})
			want := []siviParentScalarAssignment{{"owned", "Sample_Env", edit.RowID, column, history, metadataText("z"), "z"}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatal("categorical correction changed exact history/new storage", got, want, err)
			}
			*got[0].After.Text = "caller"
			if *edit.Value.Text != "z" {
				t.Fatal("categorical assignment shares proposed ownership")
			}
			if got[0].Before.Text != nil {
				*got[0].Before.Text = "caller"
				if *history.Text == "caller" || *edit.Expected.Text == "caller" {
					t.Fatal("categorical assignment shares historical ownership")
				}
			}
			if got[0].Before.BlobHex != nil {
				*got[0].Before.BlobHex = "ffff"
				if *history.BlobHex != "00ff" {
					t.Fatal("categorical assignment shares historical BLOB ownership")
				}
			}
		}
	}
}

func TestSIVIParentCategoricalCompletePayloadAndDomainIsolation(t *testing.T) {
	env, admin, snow := siviScalarFixture(t, "SnowCoverregime", metadataText("S"))
	snow.Value = metadataText("Z")
	texture := siviParentScalarEdit{"owned", "Sample_Env", snow.RowID, "SV_RootZoneTexture", metadataText("original"), metadataText("  MiXeD \t ")}
	ah := siviParentScalarEdit{"owned", "Sample_Env", snow.RowID, "SV_AhorizonType", metadataText("Ah"), metadataText("")}
	for index, column := range env.Columns {
		switch column.Name {
		case texture.Column:
			env.Rows[0].Cells[index] = texture.Expected
		case ah.Column:
			env.Rows[0].Cells[index] = ah.Expected
		}
	}
	edits := []siviParentScalarEdit{snow, texture, ah}
	want := []siviParentScalarAssignment{
		{"owned", "Sample_Env", snow.RowID, "SnowCoverregime", metadataText("S"), metadataText("Z"), "Z"},
		{"owned", "Sample_Env", snow.RowID, "SV_RootZoneTexture", metadataText("original"), metadataText("  MiXeD \t "), "  MiXeD \t "},
		{"owned", "Sample_Env", snow.RowID, "SV_AhorizonType", metadataText("Ah"), metadataText(""), ""},
	}
	got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("combined categorical payload changed", got, want, err)
	}
	for _, kind := range []string{"context", "table", "row", "expected", "duplicate", "text", "scalar", "option", "implicit", "overlength"} {
		t.Run(kind, func(t *testing.T) {
			tail := ah
			switch kind {
			case "context":
				tail.ContextID = "stale"
			case "table":
				tail.Table = "Sample_Admin"
			case "row":
				tail.RowID = "-999"
			case "expected":
				tail.Expected = metadataText("stale")
			case "duplicate":
				tail = snow
			case "text":
				tail.Column = "SV_PolygonNumber"
			case "scalar":
				tail.Column = "SV_StandHeight"
			case "option":
				tail.Column = "SV_StandAgeEstMeas"
			case "implicit":
				tail.Column = "SpeciesListComplete"
			case "overlength":
				tail.Value = metadataText("123456")
			}
			if got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{snow, tail}); err == nil || got != nil {
				t.Fatal("invalid categorical tail returned partial output", kind, got, err)
			}
		})
	}
	for _, planner := range []func(context.Context, string, string, string, ProjectMetadataTable, ProjectMetadataTable, []siviParentScalarEdit) ([]siviParentScalarAssignment, error){
		planSIVIParentScalars, planSIVIParentOptions, planSIVIParentText,
	} {
		if got, err := planner(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
			t.Fatal("categorical edits widened another domain", got, err)
		}
	}
	for _, empty := range []string{"context", "project", "edits"} {
		id, project, proposed := "owned", "Sample", edits
		switch empty {
		case "context":
			id = ""
		case "project":
			project = ""
		case "edits":
			proposed = nil
		}
		if got, err := planSIVIParentCategorical(context.Background(), id, project, "108050", env, admin, proposed); err == nil || got != nil {
			t.Fatal("anonymous/implicit categorical plan returned success", got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-8"
	if got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
		t.Fatal("ambiguous categorical parent returned a plan", got, err)
	}
}

func TestSIVIParentCategoricalCancellationAndRetry(t *testing.T) {
	env, admin, edit := siviScalarFixture(t, "SnowCoverregime", metadataText("S"))
	edit.Value = metadataText("Z")
	ah := siviParentScalarEdit{"owned", "Sample_Env", edit.RowID, "SV_AhorizonType", ProjectMetadataCell{Storage: "null"}, metadataText("")}
	for index, column := range env.Columns {
		if column.Name == ah.Column {
			env.Rows[0].Cells[index] = ah.Expected
		}
	}
	edits := []siviParentScalarEdit{edit, ah}
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 10000}
	if _, err := projectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vegetationCancelContext{Context: context.Background(), remaining: 10000 - probe.remaining + 2}
	if got, err := planSIVIParentCategorical(ctx, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight categorical cancellation returned partial output", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSIVIParentCategorical(canceled, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("pre-canceled categorical planning returned success", got, err)
	}
	got, err := planSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	want := []siviParentScalarAssignment{
		{"owned", "Sample_Env", edit.RowID, "SnowCoverregime", metadataText("S"), metadataText("Z"), "Z"},
		{"owned", "Sample_Env", edit.RowID, "SV_AhorizonType", ProjectMetadataCell{Storage: "null"}, metadataText(""), ""},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("categorical retry changed exact payload", got, want, err)
	}
}
