package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
					expected := vprocore.ProjectMetadataCell{Storage: "null"}
					if column == "Cover1" || column == "Cover6" || column == "Cover9" {
						expected = vprotest.SiviReal(0)
					}
					edit := SiviHeightEdit{target.row, target.form, column, expected, vprotest.SiviReal(-3.25)}
					source := vprotest.SiviProjectionFixture()
					got, err := PlanSIVICoverEdits(context.Background(), "P", extended, source, []SiviHeightEdit{edit})
					if err != nil || len(got) != 1 || got[0].RowID != target.row ||
						got[0].Column != column || got[0].Value != float64(-3.25) ||
						!reflect.DeepEqual(got[0].Before, expected) || !reflect.DeepEqual(got[0].After, edit.Value) {
						t.Fatal("source field or typed literal changed", got, err)
					}
					if !reflect.DeepEqual(source, vprotest.SiviProjectionFixture()) {
						t.Fatal("cover planning modified physical input")
					}
					if height, err := PlanSIVIHeightEdits(context.Background(), "P", extended, source, []SiviHeightEdit{edit}); err == nil || height != nil {
						t.Fatal("cover authorization leaked into existing height service", height, err)
					}
				})
			}
		}
	}
}

func TestSIVICoverPlanningSingleBoundsNULLAndHistoricalNoop(t *testing.T) {
	for _, value := range []vprocore.ProjectMetadataCell{
		vprotest.SiviReal(-math.MaxFloat32), vprotest.SiviReal(-1), vprotest.SiviReal(math.Nextafter(100, math.Inf(-1))),
		{Storage: "null"},
	} {
		got, err := PlanSIVICoverEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), value}})
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].After, value) {
			t.Fatal("valid SINGLE or NULL was constrained/coerced", got, err)
		}
	}
	for _, value := range []vprocore.ProjectMetadataCell{
		vprotest.SiviReal(math.Nextafter(float64(math.MaxFloat32), math.Inf(1))),
		vprotest.SiviReal(math.MaxFloat32), vprotest.SiviReal(100), vprotest.SiviReal(125),
		vprotest.SiviReal(math.NaN()), vprotest.SiviReal(math.Inf(1)), vprotest.MetadataInteger("1"), vprotest.MetadataText("1"),
	} {
		got, err := PlanSIVICoverEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), value}})
		if err == nil || got != nil {
			t.Fatal("malformed/new non-SINGLE value returned a plan", got, err)
		}
	}
	blob := "ff"
	for _, value := range []vprocore.ProjectMetadataCell{
		vprotest.MetadataText("historical"), vprotest.MetadataInteger("1"), vprotest.SiviReal(math.MaxFloat64), vprotest.SiviReal(100), vprotest.SiviReal(125),
		{Storage: "blob", BlobHex: &blob}, {Storage: "null"},
	} {
		source := vprotest.SiviProjectionFixture()
		source.Rows[0].Cells[3] = value
		source.Rows[0].Cells[4] = vprotest.SiviReal(0)
		got, err := PlanSIVICoverEdits(context.Background(), "P", false, source,
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "Cover1", value, value}})
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
			expected := vprocore.ProjectMetadataCell{Storage: "null"}
			if column == "Cover1" || column == "Cover6" || column == "Cover9" {
				expected = vprotest.SiviReal(0)
			}
			for _, value := range []float64{math.Nextafter(100, math.Inf(-1)), 100, math.Nextafter(100, math.Inf(1))} {
				edit := SiviHeightEdit{target.row, target.form, column, expected, vprotest.SiviReal(value)}
				got, err := PlanSIVICoverEdits(context.Background(), "P", target.extended, vprotest.SiviProjectionFixture(), []SiviHeightEdit{edit})
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
	valid := SiviHeightEdit{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)}
	for _, invalid := range []SiviHeightEdit{
		{"1", "SubVegA-SIVI", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
		{"2", "SubVegA-SIVI_BC", "Cover5a", vprotest.SiviReal(-1), vprotest.SiviReal(3)},
		{"3", "SubVegA-SIVI_BC", "Cover1", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		{"5", "SubVegC-SIVI", "Cover6", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		{"8", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(4), vprotest.SiviReal(3)},
		{"9", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(4), vprotest.SiviReal(3)},
		{"01", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
		{"1", "SubVegC-SIVI", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
		{"6", "SubVegD-SIVI", "Cover6", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.SiviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(4), vprotest.SiviReal(3)},
		{"1", "SubVegA-SIVI_BC", "HeightA", vprotest.SiviReal(2), vprotest.SiviReal(3)},
		{"1", "SubVegA-SIVI_BC", "Species", vprotest.MetadataText("S"), vprotest.MetadataText("NEW")},
		{"1", "SubVegA-SIVI_BC", "Collected", vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("C")},
		{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")},
		valid,
	} {
		got, err := PlanSIVICoverEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), []SiviHeightEdit{valid, invalid})
		if err == nil || got != nil {
			t.Fatal("invalid tail leaked a partial cover plan", got, err)
		}
	}
	if got, err := PlanSIVICoverEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), nil); err == nil || got != nil {
		t.Fatal("empty edits accepted", got, err)
	}
}

func TestSIVICoverPlanningExtendedOnlyMembershipAndClones(t *testing.T) {
	edit := SiviHeightEdit{"2", "SubVegA-SIVI", "Cover5a", vprotest.SiviReal(-1), vprotest.SiviReal(0)}
	source := vprotest.SiviProjectionFixture()
	got, err := PlanSIVICoverEdits(context.Background(), "P", true, source, []SiviHeightEdit{edit})
	if err != nil || len(got) != 1 || got[0].RowID != "2" {
		t.Fatal("extended-only physical membership was lost", got, err)
	}
	*got[0].Before.Real, *got[0].After.Real = 80, 90
	if !reflect.DeepEqual(source, vprotest.SiviProjectionFixture()) || *edit.Expected.Real != -1 || *edit.Value.Real != 0 {
		t.Fatal("cover output aliases input")
	}
	edit.Form = "SubVegA-SIVI_BC"
	edit.Column, edit.Expected = "Cover2", vprocore.ProjectMetadataCell{Storage: "null"}
	if got, err := PlanSIVICoverEdits(context.Background(), "P", false, source, []SiviHeightEdit{edit}); err != nil || len(got) != 1 {
		t.Fatal("normal query's extended-only row must remain visible for its normal bound controls", got, err)
	}
}

func TestSIVICoverPlanningCancellationDiscardsAllAssignments(t *testing.T) {
	edits := []SiviHeightEdit{
		{"1", "SubVegA-SIVI_BC", "Cover1", vprotest.SiviReal(0), vprotest.SiviReal(3)},
		{"4", "SubVegC-SIVI", "Cover6", vprotest.SiviReal(0), vprotest.SiviReal(2)},
	}
	complete := false
	for remaining := 0; remaining < 100; remaining++ {
		ctx := &vprotest.VegetationCancelContext{Context: context.Background(), Remaining: remaining}
		got, err := PlanSIVICoverEdits(ctx, "P", false, vprotest.SiviProjectionFixture(), edits)
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
