package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
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

func twoPageExtraFixture(t *testing.T, column string, before vprocore.ProjectMetadataCell) (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, sourceform.SiviParentScalarEdit) {
	t.Helper()
	env, admin := vprotest.SiviParentTables(t)
	field, exists := sourceform.TwoPageParentExtraFields[column]
	if !exists {
		t.Fatal("unknown additional field", column)
	}
	table := &env
	if field.Owner == "Admin" {
		table = &admin
	}
	for i, physical := range table.Columns {
		if physical.Name == column {
			table.Rows[0].Cells[i] = before
			return env, admin, sourceform.SiviParentScalarEdit{
				ContextID: "owned", Table: "Sample_" + field.Owner, RowID: table.Rows[0].RowID,
				Column: column, Expected: before,
			}
		}
	}
	t.Fatal("missing additional physical field", column)
	return vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataTable{}, sourceform.SiviParentScalarEdit{}
}

func TestTwoPageParentExtraFieldTypedBoundsAndExactValues(t *testing.T) {
	for column, field := range sourceform.TwoPageParentExtraFields {
		t.Run(column, func(t *testing.T) {
			env, admin, edit := twoPageExtraFixture(t, column, vprocore.ProjectMetadataCell{Storage: "null"})
			var valid, invalid []vprocore.ProjectMetadataCell
			switch field.Domain {
			case "single":
				valid = []vprocore.ProjectMetadataCell{vprotest.SiviReal(-math.MaxFloat32), vprotest.SiviReal(math.MaxFloat32), vprotest.SiviReal(-3), vprotest.SiviReal(0.1)}
				invalid = []vprocore.ProjectMetadataCell{vprotest.SiviReal(math.NaN()), vprotest.SiviReal(math.Inf(1)),
					vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
					vprotest.SiviReal(math.Nextafter(-float64(math.MaxFloat32), math.Inf(-1))),
					vprotest.MetadataInteger("3"), vprotest.MetadataText("3"), {Storage: "real"}}
			case "integer":
				valid = []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("-32768"), vprotest.MetadataInteger("32767"), vprotest.MetadataInteger("0")}
				invalid = []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("-32769"), vprotest.MetadataInteger("32768"),
					vprotest.MetadataInteger("01"), vprotest.MetadataInteger("+1"), vprotest.MetadataInteger("1.0"),
					vprotest.MetadataInteger("9223372036854775808"), vprotest.SiviReal(1), vprotest.MetadataText("1"), {Storage: "integer"}}
			case "text":
				valid = []vprocore.ProjectMetadataCell{vprotest.MetadataText("  e\u0301 MixEd  "), vprotest.MetadataText(strings.Repeat("x", field.Maximum)),
					vprotest.MetadataText(strings.Repeat("\U0001f600", field.Maximum/2) + strings.Repeat("x", field.Maximum%2))}
				invalid = []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText(strings.Repeat("x", field.Maximum+1)),
					vprotest.MetadataText(strings.Repeat("\U0001f600", field.Maximum/2+1)),
					vprotest.MetadataText(string([]byte{0xff})), vprotest.SiviReal(1), vprotest.MetadataInteger("1"), {Storage: "text"}}
			}
			for _, cell := range valid {
				edit.Value = cell
				for _, form := range []string{"FS882-8x6XL", "FS882-8x6XL-CHARS"} {
					got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", form, env, admin, []sourceform.SiviParentScalarEdit{edit})
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
					value, err := vprocore.MetadataCellValue(cell)
					if err != nil || !reflect.DeepEqual(got[0].Value, value) {
						t.Fatal("planned SQL value differs from proposed storage", got, err)
					}
				}
			}
			for _, cell := range invalid {
				edit.Value = cell
				if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
					t.Fatal("invalid new field returned a plan", cell, got, err)
				}
			}
			edit.Value = vprocore.ProjectMetadataCell{Storage: "null"}
			got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged NULL returned an assignment", got, err)
			}
		})
	}
}

func TestTwoPageParentExtraFieldHistoricalOmissionAndNullableCorrection(t *testing.T) {
	blob := "00ff"
	for column := range sourceform.TwoPageParentExtraFields {
		for _, historical := range []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText(strings.Repeat("x", 256)),
			vprotest.MetadataInteger("40000"), vprotest.SiviReal(math.MaxFloat64), {Storage: "blob", BlobHex: &blob}} {
			env, admin, edit := twoPageExtraFixture(t, column, historical)
			edit.Value = historical
			got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || got == nil || len(got) != 0 {
				t.Fatal("unchanged historical invalid storage became an assignment", column, got, err)
			}
			edit.Value = vprocore.ProjectMetadataCell{Storage: "null"}
			got, err = planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit})
			if err != nil || len(got) != 1 || got[0].Value != nil || !reflect.DeepEqual(got[0].Before, historical) {
				t.Fatal("explicit NULL correction rewrote historical before value", column, got, err)
			}
		}
	}
}

func TestTwoPageParentExtraFieldRejectsStaleRepeatedAndForeignTargets(t *testing.T) {
	env, admin, edit := twoPageExtraFixture(t, "PlotSize", vprotest.SiviReal(2))
	edit.Value = vprotest.SiviReal(3)
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
			invalid.Expected = vprotest.SiviReal(4)
		case "other-source":
			invalid.Column = "HumusThickness"
		}
		if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit, invalid}); err == nil || got != nil {
			t.Fatal("invalid tail leaked a partial plan", kind, got, err)
		}
	}
	if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, nil); err == nil || got != nil {
		t.Fatal("empty request became an authorized plan", got, err)
	}
	if got, err := sourceform.PlanSIVIParentScalars(context.Background(), "owned", "Sample", "108050", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("existing SIVI scope widened to accept two-page fields", got, err)
	}
	admin.Rows = append(admin.Rows, admin.Rows[0])
	admin.Rows[1].RowID = "-999"
	if got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit}); err == nil || got != nil {
		t.Fatal("ambiguous physical pair became writable", got, err)
	}
}

func TestTwoPageParentExtraFieldCloneSafetyAndLateCancellation(t *testing.T) {
	env, admin, edit := twoPageExtraFixture(t, "GIS_BGC", vprotest.MetadataText("original"))
	edit.Value = vprotest.MetadataText("  NewCase  ")
	got, err := planTwoPageParentExtraFields(context.Background(), "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit})
	if err != nil || len(got) != 1 {
		t.Fatal(got, err)
	}
	*got[0].Before.Text, *got[0].After.Text = "mutated before", "mutated after"
	if *edit.Expected.Text != "original" || *edit.Value.Text != "  NewCase  " {
		t.Fatal("plan aliases original or proposed cells")
	}
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 10000}
	if _, err := ProjectTwoPageParent(probe, "owned", "Sample", "108050", "FS882-8x6XL", env, admin); err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []int{1, 10000 - probe.Remaining + 1, 10000 - probe.Remaining + 2} {
		ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: remaining}
		got, err := planTwoPageParentExtraFields(ctx, "owned", "Sample", "108050", "FS882-8x6XL", env, admin, []sourceform.SiviParentScalarEdit{edit})
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled planning returned a partial or complete plan", got, err)
		}
	}
}
