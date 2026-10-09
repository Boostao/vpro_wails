package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageVegetationReferencesStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageVegetationReferenceWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected twoPageVegetationSpeciesReferences
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		t.Fatal(err)
	}
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			contexts, state, _ := twoPageVegetationReadFixture(t, external, "TEXT")
			mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `DELETE FROM USysAllSpecs;
				INSERT INTO USysAllSpecs(rowid,Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode) VALUES
				(-9,'DUP',NULL,-2147483648,'','U','OLD '),
				(7,'DUP','',2147483647,NULL,'x',''),
				(8,NULL,'historical',32768,'',NULL,NULL),
				(9,'LONG NULL','Unicode `+"\U0001FAB4"+`',NULL,'case  ','historical codetype',NULL)`)
			mutateContextFixture(t, contexts.projects.sqlite.attachments["VUser"], `DELETE FROM USysUserSpp;
				INSERT INTO USysUserSpp(rowid,Code,ScientificName,LifeForm,EnglishName,Codetype) VALUES
				(1,'MIN',NULL,-32768,'','s'), (2,'MAX','',32767,NULL,'s'), (3,'P NULL',NULL,NULL,'',NULL)`)
			service, err := NewTwoPageVegetationService(contexts, func(key string) (string, bool) {
				return "true", key == twoPageVegetationReviewEnvironment
			})
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
				actual, err := service.GetSpeciesReferences(context.Background(), state.ContextID, "108050", form)
				if err != nil {
					t.Fatal(err)
				}
				want := expected
				want.ContextID, want.Form = state.ContextID, form
				if !reflect.DeepEqual(actual, &want) {
					t.Fatalf("independently seeded actual facade wire differs from shared client fixture: got %+v", actual)
				}
				encoded, err := json.Marshal(actual)
				if err != nil {
					t.Fatal(err)
				}
				var reopened twoPageVegetationSpeciesReferences
				decoder := json.NewDecoder(bytes.NewReader(encoded))
				decoder.DisallowUnknownFields()
				if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(actual, &reopened) {
					t.Fatal("strict production wire roundtrip differs", err)
				}
			}
			if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("readonly wire acceptance changed source storage")
			}
		})
	}
}
