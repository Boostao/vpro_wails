package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"testing"
)

func TestSIVICoverPlanningOriginalControlMembership(t *testing.T) {
	for _, extended := range []bool{false, true} {
		aForm := "SubVegA-SIVI_BC"
		if extended {
			aForm = "SubVegA-SIVI"
		}
		for _, target := range []struct {
			form, row string
			columns   []string
		}{
			{aForm, "1", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"}},
			{"SubVegC-SIVI", "4", []string{"Cover6"}},
			{"SubVegD-SIVI", "6", []string{"Cover7", "Cover8", "Cover9"}},
		} {
			if extended && target.form == aForm {
				target.columns = append(target.columns, "Cover5a", "Cover5b", "Cover5c")
			}
			for _, column := range target.columns {
				t.Run(target.form+"/"+column, func(t *testing.T) {
					expected := ProjectMetadataCell{Storage: "null"}
					if column == "Cover1" || column == "Cover6" || column == "Cover9" {
						expected = siviReal(0)
					}
					edit := siviHeightEdit{target.row, target.form, column, expected, siviReal(-3.25)}
					source := siviProjectionFixture()
					got, err := planSIVICoverEdits(context.Background(), "P", extended, source, []siviHeightEdit{edit})
					if err != nil || len(got) != 1 || got[0].RowID != target.row ||
						got[0].Column != column || got[0].Value != float64(-3.25) ||
						!reflect.DeepEqual(got[0].Before, expected) || !reflect.DeepEqual(got[0].After, edit.Value) {
						t.Fatal("source field or typed literal changed", got, err)
					}
					if !reflect.DeepEqual(source, siviProjectionFixture()) {
						t.Fatal("cover planning modified physical input")
					}
					if height, err := planSIVIHeightEdits(context.Background(), "P", extended, source, []siviHeightEdit{edit}); err == nil || height != nil {
						t.Fatal("cover authorization leaked into existing height service", height, err)
					}
				})
			}
		}
	}
}

func TestSIVICoverPlanningSingleBoundsNULLAndHistoricalNoop(t *testing.T) {
	for _, value := range []ProjectMetadataCell{
		siviReal(-math.MaxFloat32), siviReal(-1), siviReal(math.Nextafter(100, math.Inf(-1))),
		{Storage: "null"},
	} {
		got, err := planSIVICoverEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), value}})
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].After, value) {
			t.Fatal("valid SINGLE or NULL was constrained/coerced", got, err)
		}
	}
	for _, value := range []ProjectMetadataCell{
		siviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
		siviReal(math.MaxFloat32), siviReal(100), siviReal(125),
		siviReal(math.NaN()), siviReal(math.Inf(1)), metadataInteger("1"), metadataText("1"),
	} {
		got, err := planSIVICoverEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), value}})
		if err == nil || got != nil {
			t.Fatal("malformed/new non-SINGLE value returned a plan", got, err)
		}
	}
	blob := "ff"
	for _, value := range []ProjectMetadataCell{
		metadataText("historical"), metadataInteger("1"), siviReal(math.MaxFloat64), siviReal(100), siviReal(125),
		{Storage: "blob", BlobHex: &blob}, {Storage: "null"},
	} {
		source := siviProjectionFixture()
		source.Rows[0].Cells[3] = value
		source.Rows[0].Cells[4] = siviReal(0)
		got, err := planSIVICoverEdits(context.Background(), "P", false, source,
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", value, value}})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged historical value was rewritten or rejected", got, err)
		}
	}
}

func TestSIVICoverPlanningStrictSourceCeilingAllControls(t *testing.T) {
	for _, target := range []struct {
		form, row string
		extended  bool
		columns   []string
	}{
		{"SubVegA-SIVI_BC", "1", false, []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"}},
		{"SubVegA-SIVI", "1", true, []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB"}},
		{"SubVegC-SIVI", "4", false, []string{"Cover6"}},
		{"SubVegD-SIVI", "6", false, []string{"Cover7", "Cover8", "Cover9"}},
	} {
		for _, column := range target.columns {
			expected := ProjectMetadataCell{Storage: "null"}
			if column == "Cover1" || column == "Cover6" || column == "Cover9" {
				expected = siviReal(0)
			}
			for _, value := range []float64{math.Nextafter(100, math.Inf(-1)), 100, math.Nextafter(100, math.Inf(1))} {
				edit := siviHeightEdit{target.row, target.form, column, expected, siviReal(value)}
				got, err := planSIVICoverEdits(context.Background(), "P", target.extended, siviProjectionFixture(), []siviHeightEdit{edit})
				if value < 100 {
					if err != nil || len(got) != 1 || got[0].Value != value {
						t.Fatal("strict ceiling changed precision or excluded a below-ceiling value", target.form, column, got, err)
					}
				} else if err == nil || got != nil {
					t.Fatal("source <100 ceiling was not enforced", target.form, column, value, got, err)
				}
			}
		}
	}
}

func TestSIVICoverPlanningRejectsUnavailableTargetsWithoutPartialPlan(t *testing.T) {
	valid := siviHeightEdit{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)}
	for _, invalid := range []siviHeightEdit{
		{"1", "SubVegA-SIVI", "Cover1", siviReal(0), siviReal(3)},
		{"2", "SubVegA-SIVI_BC", "Cover5a", siviReal(-1), siviReal(3)},
		{"3", "SubVegA-SIVI_BC", "Cover1", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"5", "SubVegC-SIVI", "Cover6", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"8", "SubVegA-SIVI_BC", "Cover1", siviReal(4), siviReal(3)},
		{"9", "SubVegA-SIVI_BC", "Cover1", siviReal(4), siviReal(3)},
		{"01", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)},
		{"1", "SubVegC-SIVI", "Cover1", siviReal(0), siviReal(3)},
		{"6", "SubVegD-SIVI", "Cover6", ProjectMetadataCell{Storage: "null"}, siviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(4), siviReal(3)},
		{"1", "SubVegA-SIVI_BC", "HeightA", siviReal(2), siviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Species", metadataText("S"), metadataText("NEW")},
		{"1", "SubVegA-SIVI_BC", "Collected", ProjectMetadataCell{Storage: "null"}, metadataText("C")},
		{"1", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")},
		valid,
	} {
		got, err := planSIVICoverEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{valid, invalid})
		if err == nil || got != nil {
			t.Fatal("invalid tail leaked a partial cover plan", got, err)
		}
	}
	if got, err := planSIVICoverEdits(context.Background(), "P", false, siviProjectionFixture(), nil); err == nil || got != nil {
		t.Fatal("empty edits accepted", got, err)
	}
}

func TestSIVICoverPlanningExtendedOnlyMembershipAndClones(t *testing.T) {
	edit := siviHeightEdit{"2", "SubVegA-SIVI", "Cover5a", siviReal(-1), siviReal(0)}
	source := siviProjectionFixture()
	got, err := planSIVICoverEdits(context.Background(), "P", true, source, []siviHeightEdit{edit})
	if err != nil || len(got) != 1 || got[0].RowID != "2" {
		t.Fatal("extended-only physical membership was lost", got, err)
	}
	*got[0].Before.Real, *got[0].After.Real = 80, 90
	if !reflect.DeepEqual(source, siviProjectionFixture()) || *edit.Expected.Real != -1 || *edit.Value.Real != 0 {
		t.Fatal("cover output aliases input")
	}
	edit.Form = "SubVegA-SIVI_BC"
	edit.Column, edit.Expected = "Cover2", ProjectMetadataCell{Storage: "null"}
	if got, err := planSIVICoverEdits(context.Background(), "P", false, source, []siviHeightEdit{edit}); err != nil || len(got) != 1 {
		t.Fatal("normal query's extended-only row must remain visible for its normal bound controls", got, err)
	}
}

func TestSIVICoverPlanningCancellationDiscardsAllAssignments(t *testing.T) {
	edits := []siviHeightEdit{
		{"1", "SubVegA-SIVI_BC", "Cover1", siviReal(0), siviReal(3)},
		{"4", "SubVegC-SIVI", "Cover6", siviReal(0), siviReal(2)},
	}
	complete := false
	for remaining := 0; remaining < 100; remaining++ {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		got, err := planSIVICoverEdits(ctx, "P", false, siviProjectionFixture(), edits)
		if err == nil {
			if len(got) != 2 {
				t.Fatal("incomplete successful cover plan", got)
			}
			complete = true
			break
		}
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancellation leaked assignments or lost cause", got, err)
		}
	}
	if !complete {
		t.Fatal("cover planner never completed with adequate cancellation budget")
	}
}
