package main

import (
	"context"
	"reflect"
	"testing"
)

func TestTwoPageSpeciesListPlanPreservesLegacyDenialAndPhysicalIdentity(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
		original := twoPageVegetationPlanFixture(t, form)
		references := twoPageSpeciesListFixture()
		references.ContextID, references.Project, references.Plot, references.Form =
			original.ContextID, original.Project, original.Plot, original.Form
		edits := []twoPageVegetationEdit{
			vegetationPlanEdit(original, "Species", metadataText("A")),
			vegetationPlanEdit(original, "Cover1", siviReal(2.5)),
		}
		before := cloneVegetationPlanOriginal(t, original)
		got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original, edits, references)
		if err != nil || len(got) != 2 || got[0].ID != -2147483648 || got[0].RowID != "-7" ||
			got[0].Column != "Species" || got[0].Value != "A" || !reflect.DeepEqual(got[0].After, metadataText("A")) ||
			got[1].Column != "Cover1" || got[1].Value != 2.5 || !reflect.DeepEqual(original, before) {
			t.Fatal("qualified planning lost exact original or canonical coupled assignment", got, err)
		}
		for _, planner := range []func(context.Context, *twoPageVegetationOriginal, []twoPageVegetationEdit) ([]twoPageVegetationAssignment, error){
			planTwoPageVegetationEdits, planTwoPageVegetationHistoricalEdits,
		} {
			if plan, err := planner(context.Background(), original, edits); err == nil || plan != nil {
				t.Fatal("private list policy broadened legacy request/history admission", plan, err)
			}
		}
		*vegetationPlanCell(original, "-7", "Species") = metadataText("historical overlength")
		unchanged := vegetationPlanEdit(original, "Species", metadataText("historical overlength"))
		if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original, []twoPageVegetationEdit{unchanged}, references); err != nil || len(got) != 0 {
			t.Fatal("unchanged invalid historical Species was revalidated or repaired", got, err)
		}
	}
}

func TestTwoPageSpeciesListPlanOwnerAndWarningRefusals(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegAXL")
	references := twoPageSpeciesListFixture()
	references.ContextID, references.Project, references.Plot, references.Form =
		original.ContextID, original.Project, original.Plot, original.Form
	for _, change := range []func(*twoPageVegetationSpeciesReferences){
		func(r *twoPageVegetationSpeciesReferences) { r.ContextID += " " },
		func(r *twoPageVegetationSpeciesReferences) { r.Project += " " },
		func(r *twoPageVegetationSpeciesReferences) { r.Plot += " " },
		func(r *twoPageVegetationSpeciesReferences) { r.Form = "SubVegAXL_BC" },
	} {
		copy := *references
		change(&copy)
		if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original,
			[]twoPageVegetationEdit{vegetationPlanEdit(original, "Species", metadataText("A"))}, &copy); err == nil || got != nil {
			t.Fatal("reference ownership changed", got, err)
		}
	}
	for _, rowID := range []string{"8", "9"} {
		edit := vegetationPlanEdit(original, "Species", metadataText("A"))
		edit.RowID, edit.Expected = rowID, cloneSiteUnitCell(*vegetationPlanCell(original, rowID, "Species"))
		if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original, []twoPageVegetationEdit{edit}, references); err == nil || got != nil {
			t.Fatal("NULL/text application ID acquired an invented numeric identity", got, err)
		}
	}
	clear := []twoPageVegetationEdit{vegetationPlanEdit(original, "Species", metadataText("A"))}
	for _, cover := range []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"} {
		clear = append(clear, vegetationPlanEdit(original, cover, ProjectMetadataCell{Storage: "null"}))
	}
	if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original, clear, references); err == nil || got != nil {
		t.Fatal("coupled cover clearing entered unavailable warning/advance route", got, err)
	}
	for _, cover := range []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"} {
		*vegetationPlanCell(original, "-7", cover) = ProjectMetadataCell{Storage: "null"}
	}
	if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original,
		[]twoPageVegetationEdit{vegetationPlanEdit(original, "Species", metadataText("A"))}, references); err == nil || got != nil {
		t.Fatal("CHARS-only cover admitted common-cover-empty original", got, err)
	}
	if got, err := planTwoPageVegetationSpeciesListEdits(context.Background(), original,
		[]twoPageVegetationEdit{vegetationPlanEdit(original, "Species", metadataText("A"))}, nil); err == nil || got != nil {
		t.Fatal("missing references admitted Species", got, err)
	}
}
