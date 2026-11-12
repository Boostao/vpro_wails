package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func twoPageXLOriginal(t *testing.T, form string) *sourceform.SiviParentProjection {
	t.Helper()
	env, admin := vprotest.SiviParentTables(t)
	parent, err := ProjectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []*vprocore.ProjectMetadataRow{&parent.Rows[0].Env, &parent.Rows[0].Admin} {
		for i := 1; i < len(row.Cells); i++ {
			row.Cells[i] = vprocore.ProjectMetadataCell{Storage: "null"}
		}
	}
	return parent
}

func twoPageXLValue(field twoPageXLField) vprocore.ProjectMetadataCell {
	switch field.kind {
	case "text":
		return vprotest.MetadataText("X")
	case "date":
		return vprotest.MetadataText("2026-10-06 12:34:56")
	case "integer":
		return vprotest.MetadataInteger("1")
	case "single", "latitude", "longitude":
		return vprotest.SiviReal(1)
	case "boolean":
		return vprotest.MetadataInteger("-1")
	default:
		panic("unknown test policy")
	}
}

func TestTwoPageXLExactSourceScopeAndCompletePhysicalPlan(t *testing.T) {
	for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
		parent := twoPageXLOriginal(t, form)
		fields, err := twoPageXLFields(form)
		want := 95
		if strings.HasSuffix(form, "-CHARS") {
			want = 97
		}
		if err != nil || len(fields) != want {
			t.Fatal(form, len(fields), err)
		}
		edits := []sourceform.SiviParentScalarEdit{}
		for column, field := range fields {
			edit := sftest.SiviParentEdit(t, parent, column, twoPageXLValue(field))
			if edit.Table != "Sample_"+field.owner {
				t.Fatal("source storage differs from accepted XL owner", column, edit.Table)
			}
			edits = append(edits, edit)
		}
		got, err := planTwoPageXLProjection(context.Background(), parent, edits, true)
		if err != nil || len(got) != want {
			t.Fatal("complete source plan", form, len(got), err)
		}
		for _, assignment := range got {
			if !reflect.DeepEqual(assignment.After, twoPageXLValue(fields[assignment.Column])) {
				t.Fatal("planned value changed", assignment)
			}
		}
		for _, column := range []string{"PlotNumber", "SV_CanopyComposition", "GIS_BGC", "PlotType"} {
			edit := sftest.SiviParentEdit(t, parent, column, vprotest.MetadataText("X"))
			if partial, err := planTwoPageXLProjection(context.Background(), parent, append(edits, edit), true); err == nil || partial != nil {
				t.Fatal("foreign tail leaked a partial plan", column, partial, err)
			}
		}
		for _, column := range []string{"EnteredBy", "UpdatedFromCards"} {
			_, found := fields[column]
			if found != strings.HasSuffix(form, "-CHARS") {
				t.Fatal("normal inherited an unbound CHARS-only field", form, column)
			}
		}
	}
}

func TestTwoPageXLPhysicalThresholdsAndRawStorage(t *testing.T) {
	fields, err := twoPageXLFields("FS882-8x6XL-CHARS")
	if err != nil {
		t.Fatal(err)
	}
	for column, field := range fields {
		t.Run(column, func(t *testing.T) {
			parent := twoPageXLOriginal(t, "FS882-8x6XL-CHARS")
			valid := []vprocore.ProjectMetadataCell{{Storage: "null"}, twoPageXLValue(field)}
			invalid := []vprocore.ProjectMetadataCell{{Storage: "blob", BlobHex: vprotest.MetadataText("00").Text}}
			switch field.kind {
			case "text":
				valid = append(valid, vprotest.MetadataText(" "))
				invalid = append(invalid, vprotest.MetadataText(""), vprotest.MetadataText("X\x00Y"), vprotest.MetadataText(string([]byte{0xff})), vprotest.MetadataInteger("1"))
				if field.maximum > 0 {
					valid = append(valid, vprotest.MetadataText(strings.Repeat("x", field.maximum)),
						vprotest.MetadataText(strings.Repeat("\U0001f600", field.maximum/2)+strings.Repeat("x", field.maximum%2)))
					invalid = append(invalid, vprotest.MetadataText(strings.Repeat("x", field.maximum+1)),
						vprotest.MetadataText(strings.Repeat("\U0001f600", field.maximum/2+1)))
				}
			case "integer":
				valid = append(valid, vprotest.MetadataInteger("-32768"), vprotest.MetadataInteger("32767"))
				invalid = append(invalid, vprotest.MetadataInteger("-32769"), vprotest.MetadataInteger("32768"), vprotest.MetadataInteger("01"), vprotest.SiviReal(1), vprotest.MetadataText("1"))
			case "single":
				valid = append(valid, vprotest.SiviReal(-math.MaxFloat32), vprotest.SiviReal(math.MaxFloat32), vprotest.SiviReal(0.1))
				invalid = append(invalid, vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))), vprotest.SiviReal(math.Inf(1)), vprotest.SiviReal(math.NaN()), vprotest.MetadataInteger("1"))
			case "latitude", "longitude":
				bound := float64(90)
				if field.kind == "longitude" {
					bound = 180
				}
				valid = append(valid, vprotest.SiviReal(-bound), vprotest.SiviReal(bound))
				invalid = append(invalid, vprotest.SiviReal(math.Nextafter(bound, math.Inf(1))), vprotest.SiviReal(math.NaN()), vprotest.MetadataText("1"))
			case "boolean":
				valid = append(valid, vprotest.MetadataInteger("0"), vprotest.MetadataInteger("-1"))
				invalid = append(invalid, vprotest.MetadataInteger("1"), vprotest.MetadataInteger("-2"), vprotest.MetadataText("-1"), vprotest.SiviReal(-1))
			case "date":
				invalid = append(invalid, vprotest.MetadataText("2026-02-30"), vprotest.MetadataText("2026-10-06T12:34:56Z"), vprotest.MetadataText(""))
			}
			for _, value := range valid {
				edit := sftest.SiviParentEdit(t, parent, column, value)
				got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit}, true)
				want := 1
				if value.Storage == "null" {
					want = 0
				}
				if err != nil || len(got) != want {
					t.Fatal("valid threshold rejected", value, got, err)
				}
				if want == 1 && !reflect.DeepEqual(got[0].After, value) {
					t.Fatal("literal storage changed", value, got)
				}
			}
			for _, value := range invalid {
				edit := sftest.SiviParentEdit(t, parent, column, value)
				if got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit}, true); err == nil || got != nil {
					t.Fatal("invalid threshold accepted", value, got, err)
				}
			}
		})
	}
}

func TestTwoPageXLHistoricalOmissionAndMasterAuthorization(t *testing.T) {
	parent := twoPageXLOriginal(t, "FS882-8x6XL-CHARS")
	historical := map[string]vprocore.ProjectMetadataCell{
		"UserSiteUnit": vprotest.MetadataText(strings.Repeat("x", 101)), "BECSiteUnit": vprotest.MetadataText(strings.Repeat("x", 101)),
		"UpdatedFromCards": vprotest.MetadataInteger("1"), "Latitude": vprotest.SiviReal(91), "Date": vprotest.MetadataText("historical"),
		"OfficeNotes": {Storage: "blob", BlobHex: vprotest.MetadataText("00").Text},
	}
	for column, value := range historical {
		binding := sftest.SiviParentEdit(t, parent, column, value)
		row, schema := &parent.Rows[0].Env, parent.EnvColumns
		if binding.Table == parent.AdminTable {
			row, schema = &parent.Rows[0].Admin, parent.AdminColumns
		}
		for i, physical := range schema {
			if physical.Name == column {
				row.Cells[i] = value
			}
		}
		edit := sftest.SiviParentEdit(t, parent, column, value)
		got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit}, false)
		if err != nil || len(got) != 0 {
			t.Fatal("unchanged historical value inherited an assignment or Master authorization", column, got, err)
		}
	}
	edit := sftest.SiviParentEdit(t, parent, "BECSiteUnit", vprotest.MetadataText("New"))
	valid := sftest.SiviParentEdit(t, parent, "FieldNumber", vprotest.MetadataText("Valid"))
	for _, value := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("New"), {Storage: "null"}} {
		edit.Value = value
		if got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{valid, edit}, false); err == nil || got != nil {
			t.Fatal("ordinary copy or clear granted Master writes or leaked an unrelated assignment", got, err)
		}
	}
	edit.Value = vprotest.MetadataText("New")
	if got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit}, true); err != nil || len(got) != 1 {
		t.Fatal("authorized Master physical plan", got, err)
	}
}

func TestTwoPageXLOwnershipCollisionCancellationAndNoPartialPlan(t *testing.T) {
	parent := twoPageXLOriginal(t, "FS882-8x6XL")
	edit := sftest.SiviParentEdit(t, parent, "FieldNumber", vprotest.MetadataText("New"))
	for _, change := range []func(*sourceform.SiviParentScalarEdit){
		func(e *sourceform.SiviParentScalarEdit) { e.ContextID = "foreign" },
		func(e *sourceform.SiviParentScalarEdit) { e.Table = parent.AdminTable },
		func(e *sourceform.SiviParentScalarEdit) { e.RowID = "9007199254740993" },
		func(e *sourceform.SiviParentScalarEdit) { e.Expected = vprotest.MetadataText("stale") },
	} {
		changed := edit
		change(&changed)
		if got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{changed}, false); err == nil || got != nil {
			t.Fatal("foreign or stale ownership accepted", got, err)
		}
	}
	if got, err := planTwoPageXLProjection(context.Background(), parent, []sourceform.SiviParentScalarEdit{edit, edit}, false); err == nil || got != nil {
		t.Fatal("duplicate tail leaked a partial plan", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planTwoPageXLProjection(ctx, parent, []sourceform.SiviParentScalarEdit{edit}, false); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancellation lost", got, err)
	}
	if got, err := planTwoPageXLProjection(context.Background(), parent, nil, false); err == nil || got != nil {
		t.Fatal("empty plan was success-shaped", got, err)
	}
	if got, err := planTwoPageXLProjection(context.Background(), nil, []sourceform.SiviParentScalarEdit{edit}, false); err == nil || got != nil {
		t.Fatal("missing original was success-shaped", got, err)
	}
}
