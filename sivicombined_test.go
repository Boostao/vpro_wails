package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSIVICombinedPlanningPresentationsGroupsAndAssignmentOrder(t *testing.T) {
	for _, extended := range []bool{false, true} {
		form := "SubVegA-SIVI_BC"
		if extended {
			form = "SubVegA-SIVI"
		}
		edits := []siviHeightEdit{
			{"1", form, "HeightA", siviReal(2), siviReal(100)},
			{"6", "SubVegD-SIVI", "Cover9", siviReal(0), siviReal(9)},
			{"1", form, "HeightB", metadataText(""), ProjectMetadataCell{Storage: "null"}},
			{"4", "SubVegC-SIVI", "Cover6", siviReal(0), siviReal(6)},
			{"4", "SubVegC-SIVI", "Height6", siviReal(-3), siviReal(100)},
			{"1", form, "Cover1", siviReal(0), siviReal(1)},
		}
		for _, column := range []string{"Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"} {
			edits = append(edits, siviHeightEdit{"1", form, column, ProjectMetadataCell{Storage: "null"}, siviReal(2)})
		}
		for _, column := range []string{"Cover7", "Cover8"} {
			edits = append(edits, siviHeightEdit{"6", "SubVegD-SIVI", column, ProjectMetadataCell{Storage: "null"}, siviReal(2)})
		}
		if extended {
			edits = append(edits, siviHeightEdit{"2", form, "Cover5a", siviReal(-1), siviReal(5)})
			for _, column := range []string{"Cover5b", "Cover5c"} {
				edits = append(edits, siviHeightEdit{"1", form, column, ProjectMetadataCell{Storage: "null"}, siviReal(2)})
			}
		}
		source := siviProjectionFixture()
		got, err := planSIVICombinedEdits(context.Background(), "P", extended, source, edits)
		if err != nil || len(got) != len(edits) {
			t.Fatal(extended, got, err)
		}
		for i, assignment := range got {
			if assignment.RowID != edits[i].RowID || assignment.Column != edits[i].Column ||
				!reflect.DeepEqual(assignment.Before, edits[i].Expected) || !reflect.DeepEqual(assignment.After, edits[i].Value) {
				t.Fatal("domain merge changed reviewed assignment order", i, assignment, edits[i])
			}
		}
		*got[0].Before.Real, *got[0].After.Real = 77, 88
		if !reflect.DeepEqual(source, siviProjectionFixture()) || *edits[0].Value.Real != 100 {
			t.Fatal("combined plan aliases caller/source")
		}
	}
	want := append(append([]string{}, siviCoverWritePolicy().Columns...), siviHeightWritePolicy().Columns...)
	if !reflect.DeepEqual(siviCombinedWritePolicy().Columns, want) {
		t.Fatal("combined restoration columns differ from existing domain policies")
	}
}

func TestSIVICombinedPlanningRejectsWrongFormsRowsDomainsAndDuplicates(t *testing.T) {
	valid := siviHeightEdit{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(100)}
	cover := siviHeightEdit{"1", valid.Form, "Cover1", siviReal(0), siviReal(3)}
	for _, invalid := range []siviHeightEdit{
		{"1", "SubVegA-SIVI", "HeightA", siviReal(2), siviReal(3)},
		{"1", "SubVegA-SIVI", "Cover1", siviReal(0), siviReal(3)},
		{"3", valid.Form, "HeightA", siviReal(5), siviReal(3)},
		{"5", "SubVegC-SIVI", "Height6", siviReal(3), siviReal(4)},
		{"8", valid.Form, "Cover1", siviReal(4), siviReal(3)},
		{"9", valid.Form, "Cover1", siviReal(4), siviReal(3)},
		{"01", valid.Form, "HeightA", siviReal(2), siviReal(3)},
		{"1", "SubVegC-SIVI", "Cover1", siviReal(0), siviReal(3)},
		{"6", "SubVegD-SIVI", "Height6", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"1", valid.Form, "Cover5a", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"1", valid.Form, "Species", metadataText("S"), metadataText("NEW")},
		{"1", valid.Form, "Collected", ProjectMetadataCell{Storage: "null"}, metadataText("C")},
		{"1", valid.Form, "ID", metadataInteger("0"), metadataInteger("1")},
		{"1", valid.Form, "PlotNumber", metadataText("P"), metadataText("Q")},
		{"1", valid.Form, "HeightA", siviReal(4), siviReal(3)},
		{"1", valid.Form, "Cover1", siviReal(0), siviReal(100)},
		valid, cover,
	} {
		got, err := planSIVICombinedEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{valid, cover, invalid})
		if err == nil || got != nil {
			t.Fatal("invalid combined tail leaked a partial plan", invalid, got, err)
		}
	}
	if got, err := planSIVICombinedEdits(context.Background(), "P", false, siviProjectionFixture(), nil); err == nil || got != nil {
		t.Fatal("empty combined plan accepted", got, err)
	}
}

func TestSIVICombinedPlanningSingleDomainAndNoopOrdering(t *testing.T) {
	height := siviHeightEdit{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(100)}
	cover := siviHeightEdit{"1", height.Form, "Cover1", siviReal(0), siviReal(3)}
	for _, edits := range [][]siviHeightEdit{{height}, {cover}, {
		{"1", height.Form, "HeightB", metadataText(""), metadataText("")}, cover, height,
	}} {
		got, err := planSIVICombinedEdits(context.Background(), "P", false, siviProjectionFixture(), edits)
		if err != nil {
			t.Fatal("empty opposite domain rejected valid edits", got, err)
		}
		want := []siviHeightEdit{}
		for _, edit := range edits {
			if !reflect.DeepEqual(edit.Expected, edit.Value) {
				want = append(want, edit)
			}
		}
		if len(got) != len(want) {
			t.Fatal("combined noop changed assignment count", got)
		}
		for i := range got {
			if got[i].Column != want[i].Column {
				t.Fatal("noop changed reviewed order", got)
			}
		}
	}
}

func TestSIVICombinedPlanningHeightBPhysicalBoundsAndHistoricalNoops(t *testing.T) {
	cover := siviHeightEdit{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	for _, value := range []ProjectMetadataCell{
		{Storage: "null"}, metadataText(""), metadataText("  literal  "),
		metadataText(strings.Repeat("x", 255)), metadataText(strings.Repeat("🌲", 127) + "x"),
	} {
		got, err := planSIVICombinedEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{
			{"2", cover.Form, "HeightB", metadataText("  historical words  "), value}, cover,
		})
		if err != nil || len(got) != 2 || !reflect.DeepEqual(got[0].After, value) {
			t.Fatal("valid HeightB was constrained/coerced", got, err)
		}
	}
	for _, value := range []ProjectMetadataCell{
		metadataText(strings.Repeat("x", 256)), metadataText(strings.Repeat("🌲", 128)),
		metadataText("\xff"), metadataInteger("1"), siviReal(1),
	} {
		got, err := planSIVICombinedEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{
			cover, {"1", cover.Form, "HeightB", metadataText(""), value},
		})
		if err == nil || got != nil {
			t.Fatal("invalid HeightB leaked cover assignment", got, err)
		}
	}
	blob := "ff"
	for _, old := range []ProjectMetadataCell{
		metadataText(strings.Repeat("x", 256)), metadataInteger("1"), siviReal(math.MaxFloat64),
		{Storage: "blob", BlobHex: &blob}, siviReal(100),
	} {
		source := siviProjectionFixture()
		source.Rows[0].Cells[3], source.Rows[0].Cells[7], source.Rows[0].Cells[14] = old, old, old
		source.Rows[0].Cells[4] = siviReal(0)
		edits := []siviHeightEdit{
			{"1", cover.Form, "HeightA", old, old},
			{"1", cover.Form, "Cover2", siviReal(0), siviReal(3)},
			{"1", cover.Form, "Cover1", old, old},
			{"1", cover.Form, "HeightB", old, old},
		}
		got, err := planSIVICombinedEdits(context.Background(), "P", false, source, edits)
		if err != nil || len(got) != 1 || got[0].Column != "Cover2" {
			t.Fatal("unchanged historical invalids were assigned or rejected", got, err)
		}
		edits = append(edits, edits[0])
		if got, err := planSIVICombinedEdits(context.Background(), "P", false, source, edits); err == nil || got != nil {
			t.Fatal("duplicate historical noop escaped validation", got, err)
		}
	}
}

func TestSIVICombinedPlanningCancellationDiscardsMergedPlan(t *testing.T) {
	edits := []siviHeightEdit{
		{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(100)},
		{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)},
	}
	complete := false
	for remaining := 0; remaining < 300; remaining++ {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		got, err := planSIVICombinedEdits(ctx, "P", false, siviProjectionFixture(), edits)
		if err == nil {
			if len(got) != 2 {
				t.Fatal("successful combined plan incomplete", got)
			}
			complete = true
			break
		}
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("canceled combined plan returned partial output", remaining, got, err)
		}
	}
	if !complete {
		t.Fatal("combined cancellation sweep never reached completion")
	}
	if got, err := planSIVICombinedEdits(nil, "P", false, siviProjectionFixture(), edits); err == nil || got != nil {
		t.Fatal("nil context accepted", got, err)
	}
}
