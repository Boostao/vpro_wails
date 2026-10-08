package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func twoPageXLOriginal(t *testing.T, form string) *siviParentProjection {
	t.Helper()
	env, admin := siviParentTables(t)
	parent, err := projectTwoPageParent(context.Background(), "owned", "Sample", "108050", form, env, admin)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []*ProjectMetadataRow{&parent.Rows[0].Env, &parent.Rows[0].Admin} {
		for i := 1; i < len(row.Cells); i++ {
			row.Cells[i] = ProjectMetadataCell{Storage: "null"}
		}
	}
	return parent
}

func twoPageXLValue(field twoPageXLField) ProjectMetadataCell {
	switch field.kind {
	case "text":
		return metadataText("X")
	case "date":
		return metadataText("2026-10-06 12:34:56")
	case "integer":
		return metadataInteger("1")
	case "single", "latitude", "longitude":
		return siviReal(1)
	case "boolean":
		return metadataInteger("-1")
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
		edits := []siviParentScalarEdit{}
		for column, field := range fields {
			edit := siviParentEdit(t, parent, column, twoPageXLValue(field))
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
			edit := siviParentEdit(t, parent, column, metadataText("X"))
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
			valid := []ProjectMetadataCell{{Storage: "null"}, twoPageXLValue(field)}
			invalid := []ProjectMetadataCell{{Storage: "blob", BlobHex: metadataText("00").Text}}
			switch field.kind {
			case "text":
				valid = append(valid, metadataText(" "))
				invalid = append(invalid, metadataText(""), metadataText("X\x00Y"), metadataText(string([]byte{0xff})), metadataInteger("1"))
				if field.maximum > 0 {
					valid = append(valid, metadataText(strings.Repeat("x", field.maximum)),
						metadataText(strings.Repeat("\U0001f600", field.maximum/2)+strings.Repeat("x", field.maximum%2)))
					invalid = append(invalid, metadataText(strings.Repeat("x", field.maximum+1)),
						metadataText(strings.Repeat("\U0001f600", field.maximum/2+1)))
				}
			case "integer":
				valid = append(valid, metadataInteger("-32768"), metadataInteger("32767"))
				invalid = append(invalid, metadataInteger("-32769"), metadataInteger("32768"), metadataInteger("01"), siviReal(1), metadataText("1"))
			case "single":
				valid = append(valid, siviReal(-math.MaxFloat32), siviReal(math.MaxFloat32), siviReal(0.1))
				invalid = append(invalid, siviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))), siviReal(math.Inf(1)), siviReal(math.NaN()), metadataInteger("1"))
			case "latitude", "longitude":
				bound := float64(90)
				if field.kind == "longitude" {
					bound = 180
				}
				valid = append(valid, siviReal(-bound), siviReal(bound))
				invalid = append(invalid, siviReal(math.Nextafter(bound, math.Inf(1))), siviReal(math.NaN()), metadataText("1"))
			case "boolean":
				valid = append(valid, metadataInteger("0"), metadataInteger("-1"))
				invalid = append(invalid, metadataInteger("1"), metadataInteger("-2"), metadataText("-1"), siviReal(-1))
			case "date":
				invalid = append(invalid, metadataText("2026-02-30"), metadataText("2026-10-06T12:34:56Z"), metadataText(""))
			}
			for _, value := range valid {
				edit := siviParentEdit(t, parent, column, value)
				got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{edit}, true)
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
				edit := siviParentEdit(t, parent, column, value)
				if got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{edit}, true); err == nil || got != nil {
					t.Fatal("invalid threshold accepted", value, got, err)
				}
			}
		})
	}
}

func TestTwoPageXLHistoricalOmissionAndMasterAuthorization(t *testing.T) {
	parent := twoPageXLOriginal(t, "FS882-8x6XL-CHARS")
	historical := map[string]ProjectMetadataCell{
		"UserSiteUnit": metadataText(strings.Repeat("x", 101)), "BECSiteUnit": metadataText(strings.Repeat("x", 101)),
		"UpdatedFromCards": metadataInteger("1"), "Latitude": siviReal(91), "Date": metadataText("historical"),
		"OfficeNotes": {Storage: "blob", BlobHex: metadataText("00").Text},
	}
	for column, value := range historical {
		binding := siviParentEdit(t, parent, column, value)
		row, schema := &parent.Rows[0].Env, parent.EnvColumns
		if binding.Table == parent.AdminTable {
			row, schema = &parent.Rows[0].Admin, parent.AdminColumns
		}
		for i, physical := range schema {
			if physical.Name == column {
				row.Cells[i] = value
			}
		}
		edit := siviParentEdit(t, parent, column, value)
		got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{edit}, false)
		if err != nil || len(got) != 0 {
			t.Fatal("unchanged historical value inherited an assignment or Master authorization", column, got, err)
		}
	}
	edit := siviParentEdit(t, parent, "BECSiteUnit", metadataText("New"))
	valid := siviParentEdit(t, parent, "FieldNumber", metadataText("Valid"))
	for _, value := range []ProjectMetadataCell{metadataText("New"), {Storage: "null"}} {
		edit.Value = value
		if got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{valid, edit}, false); err == nil || got != nil {
			t.Fatal("ordinary copy or clear granted Master writes or leaked an unrelated assignment", got, err)
		}
	}
	edit.Value = metadataText("New")
	if got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{edit}, true); err != nil || len(got) != 1 {
		t.Fatal("authorized Master physical plan", got, err)
	}
}

func TestTwoPageXLOwnershipCollisionCancellationAndNoPartialPlan(t *testing.T) {
	parent := twoPageXLOriginal(t, "FS882-8x6XL")
	edit := siviParentEdit(t, parent, "FieldNumber", metadataText("New"))
	for _, change := range []func(*siviParentScalarEdit){
		func(e *siviParentScalarEdit) { e.ContextID = "foreign" },
		func(e *siviParentScalarEdit) { e.Table = parent.AdminTable },
		func(e *siviParentScalarEdit) { e.RowID = "9007199254740993" },
		func(e *siviParentScalarEdit) { e.Expected = metadataText("stale") },
	} {
		changed := edit
		change(&changed)
		if got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{changed}, false); err == nil || got != nil {
			t.Fatal("foreign or stale ownership accepted", got, err)
		}
	}
	if got, err := planTwoPageXLProjection(context.Background(), parent, []siviParentScalarEdit{edit, edit}, false); err == nil || got != nil {
		t.Fatal("duplicate tail leaked a partial plan", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planTwoPageXLProjection(ctx, parent, []siviParentScalarEdit{edit}, false); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancellation lost", got, err)
	}
	if got, err := planTwoPageXLProjection(context.Background(), parent, nil, false); err == nil || got != nil {
		t.Fatal("empty plan was success-shaped", got, err)
	}
	if got, err := planTwoPageXLProjection(context.Background(), nil, []siviParentScalarEdit{edit}, false); err == nil || got != nil {
		t.Fatal("missing original was success-shaped", got, err)
	}
}
