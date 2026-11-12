package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIHeightPlanningPreservesTypedValuesAndPhysicalIdentity(t *testing.T) {
	veg := vprotest.SiviProjectionFixture()
	original := vprotest.SiviProjectionFixture()
	edits := []SiviHeightEdit{
		{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(-4)},
		{"1", "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText(""), vprotest.MetadataText("  literal words  ")},
		{"2", "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText("  historical words  "), vprocore.ProjectMetadataCell{Storage: "null"}},
		{"4", "SubVegC-SIVI", "Height6", vprotest.SiviReal(-3), vprotest.SiviReal(0)},
		{"7", "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataInteger("1"), vprotest.MetadataText("")},
	}
	got, err := PlanSIVIHeightEdits(context.Background(), "P", false, veg, edits)
	if err != nil || len(got) != 5 {
		t.Fatal(got, err)
	}
	if got[0].RowID != "1" || got[0].Value != float64(-4) ||
		got[1].Value != "  literal words  " || got[2].Value != nil ||
		got[3].Value != float64(0) || got[4].RowID != "7" || got[4].Value != "" {
		t.Fatal("NULL/empty/literal/numeric values or physical identities changed", got)
	}
	*got[0].Before.Real = 90
	*got[0].After.Real = 91
	*got[1].After.Text = "changed"
	if !reflect.DeepEqual(veg, original) || *edits[0].Expected.Real != 2 ||
		*edits[0].Value.Real != -4 || *edits[1].Value.Text != "  literal words  " {
		t.Fatal("planner output aliases caller/source cells")
	}
}

func TestSIVIHeightPlanningOmitsUnchangedHistoricalInvalidStorage(t *testing.T) {
	blob := "ff"
	for _, cell := range []vprocore.ProjectMetadataCell{
		vprotest.MetadataText(strings.Repeat("x", 256)), vprotest.MetadataInteger("1"),
		{Storage: "blob", BlobHex: &blob}, {Storage: "null"},
	} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[0].Cells[14] = cell
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, veg,
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightB", cell, cell}})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged historical HeightB must not become an assignment", got, err)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("historical"), vprotest.MetadataInteger("1"), vprotest.SiviReal(math.MaxFloat64)} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[0].Cells[7] = cell
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, veg,
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", cell, cell}})
		if err != nil || len(got) != 0 {
			t.Fatal("unchanged historical numeric storage was assigned", got, err)
		}
	}
}

func TestSIVIHeightPlanningExactPhysicalBoundsAndUnicode(t *testing.T) {
	for _, text := range []string{strings.Repeat("x", 255), strings.Repeat("\U0001f332", 127) + "x", "", "  a  "} {
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"2", "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText("  historical words  "), vprotest.MetadataText(text)}})
		if err != nil || len(got) != 1 || got[0].Value != text {
			t.Fatal("valid literal/255 UTF-16-unit value rejected or repaired", got, err)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{
		vprotest.MetadataText(strings.Repeat("x", 256)), vprotest.MetadataText(strings.Repeat("\U0001f332", 128)),
		vprotest.MetadataText("\xff"), vprotest.MetadataInteger("1"), vprotest.SiviReal(1),
	} {
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightB", vprotest.MetadataText(""), cell}})
		if err == nil || got != nil {
			t.Fatal("invalid new text returned assignments", got, err)
		}
	}
	for _, value := range []float64{math.MaxFloat32, -math.MaxFloat32, 0, -1} {
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(value)}})
		if err != nil || len(got) != 1 || got[0].Value != value {
			t.Fatal("finite SINGLE-bound value rejected", got, err)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{
		vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
		vprotest.SiviReal(math.NaN()), vprotest.SiviReal(math.Inf(1)), vprotest.MetadataText("1"), vprotest.MetadataInteger("1"),
	} {
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), cell}})
		if err == nil || got != nil {
			t.Fatal("invalid new numeric storage returned assignments", got, err)
		}
	}
}

func TestSIVIHeightPlanningRejectsStaleForeignHiddenAndRepeatedEdits(t *testing.T) {
	valid := SiviHeightEdit{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)}
	for _, invalid := range []SiviHeightEdit{
		{"1", "SubVegA-SIVI", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)},
		{"3", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(5), vprotest.SiviReal(3)},
		{"8", "SubVegA-SIVI_BC", "HeightA", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		{"01", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)},
		{"1", "SubVegC-SIVI", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)},
		{"6", "SubVegD-SIVI", "Height6", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
		{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(4), vprotest.SiviReal(3)},
		valid,
	} {
		got, err := PlanSIVIHeightEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), []SiviHeightEdit{valid, invalid})
		if err == nil || got != nil {
			t.Fatal("invalid tail leaked partial plan", got, err)
		}
	}
	valid.Form = "SubVegA-SIVI"
	got, err := PlanSIVIHeightEdits(context.Background(), "P", true, vprotest.SiviProjectionFixture(), []SiviHeightEdit{valid})
	if err != nil || len(got) != 1 {
		t.Fatal("extended source form did not retain aggregate height", got, err)
	}
	probe := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: 1000}
	if _, err := ProjectSIVIVegetation(probe, "P", true, vprotest.SiviProjectionFixture()); err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []int{1, 1000 - probe.Remaining + 2} {
		ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: remaining}
		edits := make([]SiviHeightEdit, 100)
		for i := range edits {
			edits[i] = valid
		}
		got, err := PlanSIVIHeightEdits(ctx, "P", true, vprotest.SiviProjectionFixture(), edits)
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled planner returned output", got, err)
		}
	}
}

func TestSIVIHeightDraftTransportRejectsUnicodeBeforeDecoderRepair(t *testing.T) {
	valid := `{"rowId":"1","form":"SubVegA-SIVI_BC","column":"HeightB","expected":{"storage":"null"},"value":{"storage":"text","text":"  literal  "}}`
	var edit SiviHeightEdit
	if err := json.Unmarshal([]byte(valid), &edit); err != nil || *edit.Value.Text != "  literal  " {
		t.Fatal(edit, err)
	}
	for _, data := range []string{
		strings.Replace(valid, "  literal  ", `\ud800`, 1),
		strings.Replace(valid, "  literal  ", "\xff", 1),
		strings.Replace(valid, `"rowId":"1",`, "", 1),
		strings.Replace(valid, `"expected":{"storage":"null"}`, `"expected":null`, 1),
		strings.Replace(valid, `"storage":"text"`, `"storage":"text","extra":true`, 1),
		valid + `{}`,
	} {
		before := edit
		if err := json.Unmarshal([]byte(data), &edit); err == nil || !reflect.DeepEqual(before, edit) {
			t.Fatal("malformed transport repaired Unicode or replaced request", data, edit, err)
		}
	}
}
