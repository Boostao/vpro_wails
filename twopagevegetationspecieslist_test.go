package main

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
		Master: ProjectMetadataTable{
			Columns: []ProjectMetadataColumn{{"Code", "TEXT"}, {"ScientificName", "TEXT"}, {"Lifeform", "INTEGER"},
				{"EnglishName", "TEXT"}, {"Codetype", "TEXT"}, {"OldCode", "TEXT"}},
			Rows: []ProjectMetadataRow{
				{RowID: "-9", Cells: []ProjectMetadataCell{metadataText("A"), {Storage: "null"}, metadataInteger("1"), metadataText(""), metadataText("U"), metadataText("OLD")}},
				{RowID: "-8", Cells: []ProjectMetadataCell{metadataText("A"), metadataText(""), metadataInteger("4"), {Storage: "null"}, metadataText("x"), {Storage: "null"}}},
			},
		},
		Personal: ProjectMetadataTable{
			Columns: []ProjectMetadataColumn{{"Code", "TEXT"}, {"ScientificName", "TEXT"}, {"LifeForm", "INTEGER"},
				{"EnglishName", "TEXT"}, {"Codetype", "TEXT"}},
			Rows: []ProjectMetadataRow{
				{RowID: "-9", Cells: []ProjectMetadataCell{metadataText("A"), metadataText("personal"), metadataInteger("2"), {Storage: "null"}, metadataText("u")}},
			},
		},
	}
}

func TestTwoPageSpeciesListLiteralDefinitionsAndDetachment(t *testing.T) {
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
		references := twoPageSpeciesListFixture()
		references.Form = form
		beforeMaster, beforePersonal := cloneLifeformReferenceFixture(references.Master), cloneLifeformReferenceFixture(references.Personal)
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
					references.Master.Rows[0].Cells[0] = metadataText("OTHER")
					references.Personal.Rows[0].Cells[0] = metadataText("OTHER")
					target := &references.Master.Rows[0]
					if personal {
						target = &references.Personal.Rows[0]
					}
					target.Cells[0], target.Cells[2], target.Cells[4] = metadataText("A"), metadataInteger(lifeform), metadataText(codetype)
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
			target.Cells[column] = metadataText("\xff")
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
