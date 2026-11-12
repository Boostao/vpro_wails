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

func TestSIVIParentCategoricalExactBoundsAndStorage(t *testing.T) {
	for _, field := range []struct {
		column  string
		maximum int
	}{{"SnowCoverregime", 1}, {"SV_RootZoneTexture", 100}, {"SV_AhorizonType", 5}} {
		t.Run(field.column, func(t *testing.T) {
			env, admin, edit := sftest.SiviScalarFixture(t, field.column, vprotest.MetadataText("old"))
			exactAstral := strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("a", field.maximum%2)
			for _, value := range []vprocore.ProjectMetadataCell{
				vprotest.MetadataText(""), {Storage: "null"}, vprotest.MetadataText(" "),
				vprotest.MetadataText(strings.Repeat("é", field.maximum)), vprotest.MetadataText(exactAstral),
				vprotest.MetadataText("Z"),
			} {
				edit.Value = value
				got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
				var execution any
				if value.Text != nil {
					execution = *value.Text
				}
				want := []sourceform.SiviParentScalarAssignment{{
					ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: field.column,
					Before: vprotest.MetadataText("old"), After: value, Value: execution,
				}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatal("categorical NULL/empty/unlisted/UTF-16 payload changed", got, want, err)
				}
			}
			for _, value := range []vprocore.ProjectMetadataCell{
				vprotest.MetadataText(strings.Repeat("a", field.maximum+1)), vprotest.MetadataText(exactAstral + "a"),
				vprotest.MetadataText(string([]byte{0xff})), vprotest.MetadataInteger("1"), vprotest.SiviReal(1),
				{Storage: "text"}, {Storage: "blob"}, {Storage: "null", Text: vprotest.MetadataText("").Text},
				{Storage: "text", Text: vprotest.MetadataText("").Text, Integer: vprotest.MetadataInteger("1").Integer},
			} {
				edit.Value = value
				if got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("new overlength/malformed/nontext categorical value returned a plan", got, err)
				}
			}
		})
	}
}

func TestSIVIParentCategoricalHistoryAndClone(t *testing.T) {
	blob := "00ff"
	for _, column := range []string{"SnowCoverregime", "SV_RootZoneTexture", "SV_AhorizonType"} {
		for _, history := range []vprocore.ProjectMetadataCell{
			vprotest.MetadataText(""), {Storage: "null"}, vprotest.MetadataText(strings.Repeat("\U0001f600", 101)),
			vprotest.MetadataInteger("-1"), vprotest.SiviReal(99), {Storage: "blob", BlobHex: &blob},
		} {
			env, admin, edit := sftest.SiviScalarFixture(t, column, history)
			edit.Value = vprocore.CloneSiteUnitCell(history)
			got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged categorical history became an assignment", column, got, err)
			}
			edit.Value = vprotest.MetadataText("z")
			got, err = sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit})
			want := []sourceform.SiviParentScalarAssignment{{ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: column, Before: history, After: vprotest.MetadataText("z"), Value: "z"}}
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
	env, admin, snow := sftest.SiviScalarFixture(t, "SnowCoverregime", vprotest.MetadataText("S"))
	snow.Value = vprotest.MetadataText("Z")
	texture := sourceform.SiviParentScalarEdit{"owned", "Sample_Env", snow.RowID, "SV_RootZoneTexture", vprotest.MetadataText("original"), vprotest.MetadataText("  MiXeD \t ")}
	ah := sourceform.SiviParentScalarEdit{"owned", "Sample_Env", snow.RowID, "SV_AhorizonType", vprotest.MetadataText("Ah"), vprotest.MetadataText("")}
	for index, column := range env.Columns {
		switch column.Name {
		case texture.Column:
			env.Rows[0].Cells[index] = texture.Expected
		case ah.Column:
			env.Rows[0].Cells[index] = ah.Expected
		}
	}
	edits := []sourceform.SiviParentScalarEdit{snow, texture, ah}
	want := []sourceform.SiviParentScalarAssignment{
		{ContextID: "owned", Table: "Sample_Env", RowID: snow.RowID, Column: "SnowCoverregime", Before: vprotest.MetadataText("S"), After: vprotest.MetadataText("Z"), Value: "Z"},
		{ContextID: "owned", Table: "Sample_Env", RowID: snow.RowID, Column: "SV_RootZoneTexture", Before: vprotest.MetadataText("original"), After: vprotest.MetadataText("  MiXeD \t "), Value: "  MiXeD \t "},
		{ContextID: "owned", Table: "Sample_Env", RowID: snow.RowID, Column: "SV_AhorizonType", Before: vprotest.MetadataText("Ah"), After: vprotest.MetadataText(""), Value: ""},
	}
	got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits)
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
				tail.Expected = vprotest.MetadataText("stale")
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
				tail.Value = vprotest.MetadataText("123456")
			}
			if got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{snow, tail}); err == nil || got != nil {
				t.Fatal("invalid categorical tail returned partial output", kind, got, err)
			}
		})
	}
	for _, planner := range []func(context.Context, string, string, string, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, []sourceform.SiviParentScalarEdit) ([]sourceform.SiviParentScalarAssignment, error){
		sourceform.PlanSIVIParentScalars, sourceform.PlanSIVIParentOptions, sourceform.PlanSIVIParentText,
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
		if got, err := sourceform.PlanSIVIParentCategorical(context.Background(), id, project, "108050", env, admin, proposed); err == nil || got != nil {
			t.Fatal("anonymous/implicit categorical plan returned success", got, err)
		}
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-8"
	if got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits); err == nil || got != nil {
		t.Fatal("ambiguous categorical parent returned a plan", got, err)
	}
}

func TestSIVIParentCategoricalCancellationAndRetry(t *testing.T) {
	env, admin, edit := sftest.SiviScalarFixture(t, "SnowCoverregime", vprotest.MetadataText("S"))
	edit.Value = vprotest.MetadataText("Z")
	ah := sourceform.SiviParentScalarEdit{"owned", "Sample_Env", edit.RowID, "SV_AhorizonType", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("")}
	for index, column := range env.Columns {
		if column.Name == ah.Column {
			env.Rows[0].Cells[index] = ah.Expected
		}
	}
	edits := []sourceform.SiviParentScalarEdit{edit, ah}
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000}
	if _, err := sourceform.ProjectSIVIParent(probe, "owned", "Sample", "108050", env, admin); err != nil {
		t.Fatal(err)
	}
	ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000 - probe.Remaining + 2}
	if got, err := sourceform.PlanSIVIParentCategorical(ctx, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("in-flight categorical cancellation returned partial output", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := sourceform.PlanSIVIParentCategorical(canceled, "owned", "Sample", "108050", env, admin, edits); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("pre-canceled categorical planning returned success", got, err)
	}
	got, err := sourceform.PlanSIVIParentCategorical(context.Background(), "owned", "Sample", "108050", env, admin, edits)
	want := []sourceform.SiviParentScalarAssignment{
		{ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: "SnowCoverregime", Before: vprotest.MetadataText("S"), After: vprotest.MetadataText("Z"), Value: "Z"},
		{ContextID: "owned", Table: "Sample_Env", RowID: edit.RowID, Column: "SV_AhorizonType", Before: vprocore.ProjectMetadataCell{Storage: "null"}, After: vprotest.MetadataText(""), Value: ""},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("categorical retry changed exact payload", got, want, err)
	}
}
