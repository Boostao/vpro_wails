package twopage

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func twoPageSpeciesListFixture() *twoPageVegetationSpeciesReferences {
	return &twoPageVegetationSpeciesReferences{
		ContextID: "owned", Project: "Project", Plot: "00001", Form: "SubVegAXL_BC", Source: twoPageVegetationSpeciesReferenceSource,
		Master: vprocore.ProjectMetadataTable{
			Columns: []vprocore.ProjectMetadataColumn{{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "Lifeform", DeclaredType: "INTEGER"},
				{Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Codetype", DeclaredType: "TEXT"}, {Name: "OldCode", DeclaredType: "TEXT"}},
			Rows: []vprocore.ProjectMetadataRow{
				{RowID: "-9", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), {Storage: "null"}, vprotest.MetadataInteger("1"), vprotest.MetadataText(""), vprotest.MetadataText("U"), vprotest.MetadataText("OLD")}},
				{RowID: "-8", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataText(""), vprotest.MetadataInteger("4"), {Storage: "null"}, vprotest.MetadataText("x"), {Storage: "null"}}},
			},
		},
		Personal: vprocore.ProjectMetadataTable{
			Columns: []vprocore.ProjectMetadataColumn{{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "LifeForm", DeclaredType: "INTEGER"},
				{Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Codetype", DeclaredType: "TEXT"}},
			Rows: []vprocore.ProjectMetadataRow{
				{RowID: "-9", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("A"), vprotest.MetadataText("personal"), vprotest.MetadataInteger("2"), {Storage: "null"}, vprotest.MetadataText("u")}},
			},
		},
	}
}

func TestTwoPageSpeciesListLiteralDefinitionsAndDetachment(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
		references := twoPageSpeciesListFixture()
		references.Form = form
		beforeMaster, beforePersonal := vprotest.CloneLifeformReferenceFixture(references.Master), vprotest.CloneLifeformReferenceFixture(references.Personal)
		got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), references, "A")
		expected := []twoPageVegetationSpeciesListMatch{
			{"master", beforeMaster.Rows[0]}, {"master", beforeMaster.Rows[1]}, {"personal", beforePersonal.Rows[0]},
		}
		if err != nil || !reflect.DeepEqual(got, expected) ||
			!reflect.DeepEqual(references.Master, beforeMaster) || !reflect.DeepEqual(references.Personal, beforePersonal) {
			t.Fatal("physical duplicates, NULL/empty metadata or role-local row identities collapsed", got, err)
		}
		*got[0].Definition.Cells[0].Text = "changed result"
		if *references.Master.Rows[0].Cells[0].Text != "A" {
			t.Fatal("qualified result aliases source metadata")
		}
	}
	for _, code := range []string{"a", "A ", " A", "OLD", "UNKNOWN", "", "123456789", "\xff", "A\x00"} {
		got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), twoPageSpeciesListFixture(), code)
		if err == nil || got != nil {
			t.Fatal("literal list selection entered an unavailable/manual/alias route", code, got, err)
		}
	}
}

func TestTwoPageSpeciesListQualifiedLifeformAndCodetype(t *testing.T) {
	for _, personal := range []bool{false, true} {
		for _, lifeform := range []string{"-2147483648", "-32768", "0", "1", "2", "3", "4", "5", "12", "32767", "2147483647"} {
			for _, codetype := range []string{"u", "U", "x", "X", "s", "S", "", "U ", "\uff35"} {
				t.Run(fmt.Sprintf("%t/%s/%s", personal, lifeform, codetype), func(t *testing.T) {
					references := twoPageSpeciesListFixture()
					references.Master.Rows = references.Master.Rows[:1]
					references.Personal.Rows = references.Personal.Rows[:1]
					references.Master.Rows[0].Cells[0] = vprotest.MetadataText("OTHER")
					references.Personal.Rows[0].Cells[0] = vprotest.MetadataText("OTHER")
					target := &references.Master.Rows[0]
					if personal {
						target = &references.Personal.Rows[0]
					}
					target.Cells[0], target.Cells[2], target.Cells[4] = vprotest.MetadataText("A"), vprotest.MetadataInteger(lifeform), vprotest.MetadataText(codetype)
					eligible := (lifeform == "1" || lifeform == "2" || lifeform == "3" || lifeform == "4") &&
						(codetype == "u" || codetype == "U" || codetype == "x" || codetype == "X")
					got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), references, "A")
					if (err == nil) != eligible || eligible && len(got) != 1 || !eligible && got != nil {
						t.Fatal("qualified source list policy broadened or narrowed", got, err)
					}
				})
			}
		}
	}
}

func TestTwoPageSpeciesListRefusesIncompleteReferencesAndOwners(t *testing.T) {
	for _, role := range []string{"master", "personal"} {
		for column := 0; column < 5; column++ {
			references := twoPageSpeciesListFixture()
			target := &references.Master.Rows[0]
			if role == "personal" {
				target = &references.Personal.Rows[0]
			}
			target.Cells[column] = vprotest.MetadataText("\xff")
			got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), references, "A")
			if err == nil || got != nil {
				t.Fatal("malformed physical source was ignored after another match", role, column, got, err)
			}
		}
	}
	for _, form := range []string{"SubVegCXL", "SubVegDXL", "", "SubVegAXL "} {
		references := twoPageSpeciesListFixture()
		references.Form = form
		if got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), references, "A"); err == nil || got != nil {
			t.Fatal("unqualified source form accepted", form, got, err)
		}
	}
	references := twoPageSpeciesListFixture()
	references.ContextID = ""
	if got, err := qualifyTwoPageVegetationSpeciesListSelection(context.Background(), references, "A"); err == nil || got != nil {
		t.Fatal("unowned references accepted", got, err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := qualifyTwoPageVegetationSpeciesListSelection(canceled, twoPageSpeciesListFixture(), "A"); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled qualification returned a partial selection", got, err)
	}
	if got, err := qualifyTwoPageVegetationSpeciesListSelection(nil, twoPageSpeciesListFixture(), "A"); err == nil || got != nil {
		t.Fatal("nil context accepted", got, err)
	}
}
