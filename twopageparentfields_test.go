package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func twoPageExtraFixture(t *testing.T, column string, before ProjectMetadataCell) (ProjectMetadataTable, ProjectMetadataTable, siviParentScalarEdit) {
	t.Helper()
	env, admin := siviParentTables(t)
	field, exists := twoPageParentExtraFields[column]
	if !exists {
		t.Fatal("unknown additional field", column)
	}
	table := &env
	if field.owner == "Admin" {
		table = &admin
	}
	for i, physical := range table.Columns {
		if physical.Name == column {
			table.Rows[0].Cells[i] = before
			return env, admin, siviParentScalarEdit{
				ContextID: "owned", Table: "Sample_" + field.owner, RowID: table.Rows[0].RowID,
				Column: column, Expected: before,
			}
		}
	}
	t.Fatal("missing additional physical field", column)
	return ProjectMetadataTable{}, ProjectMetadataTable{}, siviParentScalarEdit{}
}

func TestTwoPageParentExtraFieldTypedBoundsAndExactValues(t *testing.T) {
	for column, field := range twoPageParentExtraFields {
		t.Run(column, func(t *testing.T) {
			env, admin, edit := twoPageExtraFixture(t, column, ProjectMetadataCell{Storage: "null"})
			var valid, invalid []ProjectMetadataCell
			switch field.domain {
			case "single":
				valid = []ProjectMetadataCell{siviReal(-math.MaxFloat32), siviReal(math.MaxFloat32), siviReal(-3), siviReal(0.1)}
				invalid = []ProjectMetadataCell{siviReal(math.NaN()), siviReal(math.Inf(1)),
					siviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
					siviReal(math.Nextafter(-float64(math.MaxFloat32), math.Inf(-1))),
					metadataInteger("3"), metadataText("3"), {Storage: "real"}}
			case "integer":
				valid = []ProjectMetadataCell{metadataInteger("-32768"), metadataInteger("32767"), metadataInteger("0")}
				invalid = []ProjectMetadataCell{metadataInteger("-32769"), metadataInteger("32768"),
					metadataInteger("01"), metadataInteger("+1"), metadataInteger("1.0"),
					metadataInteger("9223372036854775808"), siviReal(1), metadataText("1"), {Storage: "integer"}}
			case "text":
				valid = []ProjectMetadataCell{metadataText("  e\u0301 MixEd  "), metadataText(strings.Repeat("x", field.maximum)),
					metadataText(strings.Repeat("\U0001f600", field.maximum/2) + strings.Repeat("x", field.maximum%2))}
				invalid = []ProjectMetadataCell{metadataText(""), metadataText(strings.Repeat("x", field.maximum+1)),
					metadataText(strings.Repeat("\U0001f600", field.maximum/2+1)),
					metadataText(string([]byte{0xff})), siviReal(1), metadataInteger("1"), {Storage: "text"}}
			}
			for _, cell := range valid {
				edit.Value = cell
				for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
					got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", form, env, admin, []siviParentScalarEdit{edit})
					if column == "BEC_Use" && strings.HasSuffix(form, "-CHARS") {
						if err == nil || got != nil {
							t.Fatal("CHARS inherited an unbound normal-only field", got, err)
						}
						continue
					}
					if err != nil || len(got) != 1 || got[0].Table != edit.Table || got[0].RowID != edit.RowID ||
						got[0].ContextID != "owned" || !reflect.DeepEqual(got[0].After, cell) {
						t.Fatal("typed boundary or literal proposed storage changed", cell, got, err)
					}
					value, err := metadataCellValue(cell)
					if err != nil || !reflect.DeepEqual(got[0].Value, value) {
						t.Fatal("planned SQL value differs from proposed storage", got, err)
					}
				}
			}
			for _, cell := range invalid {
				edit.Value = cell
				if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("invalid new field returned a plan", cell, got, err)
				}
			}
			edit.Value = ProjectMetadataCell{Storage: "null"}
			got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged NULL returned an assignment", got, err)
			}
		})
	}
}

func TestTwoPageParentExtraFieldHistoricalOmissionAndNullableCorrection(t *testing.T) {
	blob := "00ff"
	for column := range twoPageParentExtraFields {
		for _, historical := range []ProjectMetadataCell{metadataText(""), metadataText(strings.Repeat("x", 256)),
			metadataInteger("40000"), siviReal(math.MaxFloat64), {Storage: "blob", BlobHex: &blob}} {
			env, admin, edit := twoPageExtraFixture(t, column, historical)
			edit.Value = historical
			got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged historical invalid storage became an assignment", column, got, err)
			}
			edit.Value = ProjectMetadataCell{Storage: "null"}
			got, err = planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || got[0].Value != nil || !reflect.DeepEqual(got[0].Before, historical) {
				t.Fatal("explicit NULL correction rewrote historical before value", column, got, err)
			}
		}
	}
}

func TestTwoPageParentExtraFieldRejectsStaleRepeatedAndForeignTargets(t *testing.T) {
	env, admin, edit := twoPageExtraFixture(t, "PlotSize", siviReal(2))
	edit.Value = siviReal(3)
	for _, kind := range []string{"context", "table", "row", "alias", "expected", "duplicate", "other-source"} {
		invalid := edit
		switch kind {
		case "context":
			invalid.ContextID = "stale"
		case "table":
			invalid.Table = "Sample_Env"
		case "row":
			invalid.RowID = "-777"
		case "alias":
			invalid.RowID = "0" + edit.RowID
		case "expected":
			invalid.Expected = siviReal(4)
		case "other-source":
			invalid.Column = "HumusThickness"
		}
		if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit, invalid}); err == nil || got != nil {
			t.Fatal("invalid tail leaked a partial plan", kind, got, err)
		}
	}
	if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, nil); err == nil || got != nil {
		t.Fatal("empty request became an authorized plan", got, err)
	}
	if got, err := planSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("existing SIVI scope widened to accept two-page fields", got, err)
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-999"
	if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ambiguous physical pair became writable", got, err)
	}
}

func TestTwoPageParentExtraFieldCloneSafetyAndLateCancellation(t *testing.T) {
	env, admin, edit := twoPageExtraFixture(t, "GIS_BGC", metadataText("original"))
	edit.Value = metadataText("  NewCase  ")
	got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit})
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	*got[0].Before.Text, *got[0].After.Text = "mutated before", "mutated after"
	if *edit.Expected.Text != "original" || *edit.Value.Text != "  NewCase  " {
		t.Fatal("plan aliases original or proposed cells")
	}
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 10000}
	if _, err := projectTwoPageParent(probe, "owned", "Sample", "108050", "FS882-8x6XL", env, admin); err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []int{1, 10000 - probe.remaining + 1, 10000 - probe.remaining + 2} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		got, err := planTwoPageParentExtraFields(ctx, "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []siviParentScalarEdit{edit})
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled planning returned a partial or complete plan", got, err)
		}
	}
}
