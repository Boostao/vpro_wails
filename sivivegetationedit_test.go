package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func siviReal(value float64) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "real", Real: &value}
}

func TestSIVIHeightPlanningPreservesTypedValuesAndPhysicalIdentity(t *testing.T) {
	veg := siviProjectionFixture()
	original := siviProjectionFixture()
	edits := []siviHeightEdit{
		{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(-4)},
		{"1", "SubVegA-SIVI_BC", "HeightB", metadataText(""), metadataText("  literal words  ")},
		{"2", "SubVegA-SIVI_BC", "HeightB", metadataText("  historical words  "), ProjectMetadataCell{Storage: "null"}},
		{"4", "SubVegC-SIVI", "Height6", siviReal(-3), siviReal(0)},
		{"7", "SubVegA-SIVI_BC", "HeightB", metadataInteger("1"), metadataText("")},
	}
	got, err := planSIVIHeightEdits(context.Background(), "P", false, veg, edits)
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
	for _, cell := range []ProjectMetadataCell{
		metadataText(strings.Repeat("x", 256)), metadataInteger("1"),
		{Storage: "blob", BlobHex: &blob}, {Storage: "null"},
	} {
		veg := siviProjectionFixture()
		veg.Rows[0].Cells[14] = cell
		got, err := planSIVIHeightEdits(context.Background(), "P", false, veg,
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightB", cell, cell}})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged historical HeightB must not become an assignment", got, err)
		}
	}
	for _, cell := range []ProjectMetadataCell{metadataText("historical"), metadataInteger("1"), siviReal(math.MaxFloat64)} {
		veg := siviProjectionFixture()
		veg.Rows[0].Cells[7] = cell
		got, err := planSIVIHeightEdits(context.Background(), "P", false, veg,
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", cell, cell}})
		if err != nil || len(got) != 0 {
			t.Fatal("unchanged historical numeric storage was assigned", got, err)
		}
	}
}

func TestSIVIHeightPlanningExactPhysicalBoundsAndUnicode(t *testing.T) {
	for _, text := range []string{strings.Repeat("x", 255), strings.Repeat("\U0001f332", 127) + "x", "", "  a  "} {
		got, err := planSIVIHeightEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"2", "SubVegA-SIVI_BC", "HeightB", metadataText("  historical words  "), metadataText(text)}})
		if err != nil || len(got) != 1 || got[0].Value != text {
			t.Fatal("valid literal/255 UTF-16-unit value rejected or repaired", got, err)
		}
	}
	for _, cell := range []ProjectMetadataCell{
		metadataText(strings.Repeat("x", 256)), metadataText(strings.Repeat("\U0001f332", 128)),
		metadataText("\xff"), metadataInteger("1"), siviReal(1),
	} {
		got, err := planSIVIHeightEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightB", metadataText(""), cell}})
		if err == nil || got != nil {
			t.Fatal("invalid new text returned assignments", got, err)
		}
	}
	for _, value := range []float64{math.MaxFloat32, -math.MaxFloat32, 0, -1} {
		got, err := planSIVIHeightEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(value)}})
		if err != nil || len(got) != 1 || got[0].Value != value {
			t.Fatal("finite SINGLE-bound value rejected", got, err)
		}
	}
	for _, cell := range []ProjectMetadataCell{
		siviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
		siviReal(math.NaN()), siviReal(math.Inf(1)), metadataText("1"), metadataInteger("1"),
	} {
		got, err := planSIVIHeightEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), cell}})
		if err == nil || got != nil {
			t.Fatal("invalid new numeric storage returned assignments", got, err)
		}
	}
}

func TestSIVIHeightPlanningRejectsStaleForeignHiddenAndRepeatedEdits(t *testing.T) {
	valid := siviHeightEdit{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(3)}
	for _, invalid := range []siviHeightEdit{
		{"1", "SubVegA-SIVI", "HeightA", siviReal(2), siviReal(3)},
		{"3", "SubVegA-SIVI_BC", "HeightA", siviReal(5), siviReal(3)},
		{"8", "SubVegA-SIVI_BC", "HeightA", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"01", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(3)},
		{"1", "SubVegC-SIVI", "HeightA", siviReal(2), siviReal(3)},
		{"6", "SubVegD-SIVI", "Height6", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)},
		{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(4), siviReal(3)},
		valid,
	} {
		got, err := planSIVIHeightEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{valid, invalid})
		if err == nil || got != nil {
			t.Fatal("invalid tail leaked partial plan", got, err)
		}
	}
	valid.Form = "SubVegA-SIVI"
	got, err := planSIVIHeightEdits(context.Background(), "P", true, siviProjectionFixture(), []siviHeightEdit{valid})
	if err != nil || len(got) != 1 {
		t.Fatal("extended source form did not retain aggregate height", got, err)
	}
	probe := &vegetationCancelContext{Context: context.Background(), remaining: 1000}
	if _, err := projectSIVIVegetation(probe, "P", true, siviProjectionFixture()); err != nil {
		t.Fatal(err)
	}
	for _, remaining := range []int{1, 1000 - probe.remaining + 2} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		edits := make([]siviHeightEdit, 100)
		for i := range edits {
			edits[i] = valid
		}
		got, err := planSIVIHeightEdits(ctx, "P", true, siviProjectionFixture(), edits)
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancelled planner returned output", got, err)
		}
	}
}

func TestSIVIHeightDraftTransportRejectsUnicodeBeforeDecoderRepair(t *testing.T) {
	valid := `{"rowId":"1","form":"SubVegA-SIVI_BC","column":"HeightB","expected":{"storage":"null"},"value":{"storage":"text","text":"  literal  "}}`
	var edit siviHeightEdit
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
