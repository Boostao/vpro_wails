package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageVegetationSpeciesReferencesMasterLONGConsumer(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprint("external-", external), func(t *testing.T) {
			contexts, state, _ := twoPageVegetationReadFixture(t, external, "TEXT")
			mutateContextFixture(t, contexts.projects.sqlite.attachments["VLists"], `DELETE FROM USysAllSpecs;
				INSERT INTO USysAllSpecs(rowid,Code,ScientificName,Lifeform,EnglishName,Codetype,OldCode) VALUES
				(1,'DUP',NULL,-2147483648,'','U','old'),
				(2,'DUP','',-32769,NULL,'U','old'),
				(3,'DUP',NULL,-32768,'','U','old'),
				(4,'DUP','',0,NULL,'U','old'),
				(5,'DUP',NULL,32767,'','U','old'),
				(6,'DUP','',32768,NULL,'U','old'),
				(7,'DUP',NULL,2147483647,'','U','old'),
				(8,NULL,'historical',NULL,'',NULL,NULL)`)
			mutateContextFixture(t, contexts.projects.sqlite.attachments["VUser"], `DELETE FROM USysUserSpp;
				INSERT INTO USysUserSpp(rowid,Code,ScientificName,LifeForm,EnglishName,Codetype) VALUES
				(1,'MIN',NULL,-32768,'','s'),
				(2,'MAX','',32767,NULL,'s'),
				(3,'NULL',NULL,NULL,'','s')`)
			service, err := NewTwoPageVegetationService(contexts, func(key string) (string, bool) {
				if key == twoPageVegetationReviewEnvironment {
					return "true", true
				}

				return "", false
			})
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			for _, form := range []string{"SubVegAXL_BC", "SubVegAXL", "SubVegCXL", "SubVegDXL"} {
				references, err := service.GetSpeciesReferences(context.Background(), state.ContextID, "108050", form)
				if err != nil || references == nil {
					t.Fatal("source-valid master LONG definitions were refused", references, err)
				}
				if len(references.Master.Rows) != 8 || len(references.Personal.Rows) != 3 ||
					references.Master.Columns[2].DeclaredType != "INTEGER" ||
					references.Personal.Columns[2].DeclaredType != "INTEGER" {
					t.Fatal("physical rows/declared types changed", references)
				}
				for i, expected := range []string{"-2147483648", "-32769", "-32768", "0", "32767", "32768", "2147483647"} {
					row := references.Master.Rows[i]
					if row.RowID != fmt.Sprint(i+1) || !reflect.DeepEqual(row.Cells[2], metadataInteger(expected)) ||
						!reflect.DeepEqual(row.Cells[0], metadataText("DUP")) {
						t.Fatal("master LONG scalar, duplicate or physical identity changed", row)
					}
				}
				if references.Master.Rows[0].Cells[1].Storage != "null" ||
					!reflect.DeepEqual(references.Master.Rows[1].Cells[1], metadataText("")) ||
					references.Master.Rows[7].Cells[0].Storage != "null" ||
					references.Master.Rows[7].Cells[2].Storage != "null" ||
					!reflect.DeepEqual(references.Personal.Rows[0].Cells[2], metadataInteger("-32768")) ||
					!reflect.DeepEqual(references.Personal.Rows[1].Cells[2], metadataInteger("32767")) ||
					references.Personal.Rows[2].Cells[2].Storage != "null" {
					t.Fatal("NULL/empty or source-specific personal INTEGER values changed", references)
				}
			}
			if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
				t.Fatal("source width reference reads changed an owned database")
			}
			for _, source := range []struct {
				role, table, overflow string
			}{
				{"VLists", "USysAllSpecs", "-2147483649"},
				{"VLists", "USysAllSpecs", "2147483648"},
				{"VUser", "USysUserSpp", "-32769"},
				{"VUser", "USysUserSpp", "32768"},
			} {
				path := contexts.projects.sqlite.attachments[source.role]
				mutateContextFixture(t, path, `UPDATE `+source.table+` SET Lifeform=`+source.overflow+` WHERE rowid=1`)
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				references, err := service.GetSpeciesReferences(context.Background(), state.ContextID, "108050", "SubVegAXL")
				if references != nil || err == nil || !strings.Contains(err.Error(), "range") ||
					!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
					t.Fatal("source range refusal returned partial definitions or changed storage", source, references, err)
				}
				mutateContextFixture(t, path, `UPDATE `+source.table+` SET Lifeform=NULL WHERE rowid=1`)
				if references, err := service.GetSpeciesReferences(context.Background(), state.ContextID, "108050", "SubVegAXL"); err != nil ||
					references == nil || references.Master.Rows[0].Cells[2].Storage != "null" ||
					(source.role == "VUser" && references.Personal.Rows[0].Cells[2].Storage != "null") {
					t.Fatal("corrected source definitions could not be retried", references, err)
				}
			}
		})
	}
}
