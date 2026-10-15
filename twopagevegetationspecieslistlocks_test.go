package main

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestTwoPageSpeciesListReferenceReservationWALRefusalAndRetry(t *testing.T) {
	for _, role := range []string{"VLists", "VUser"} {
		for _, wal := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/wal-%t", role, wal), func(t *testing.T) {
				contexts, state, db, request := twoPageSpeciesListWriteFixture(t, true, "SubVegAXL_BC", 3)
				path := contexts.projects.sqlite.attachments[role]
				competing, err := sql.Open("sqlite3", sqliteFileURI(path, "rw")+"&_busy_timeout=20")
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := competing.Close(); err != nil {
						t.Error(err)
					}
				}()
				table := "USysAllSpecs"
				if role == "VUser" {
					table = "USysUserSpp"
				}
				if wal {
					var mode string
					if err := competing.QueryRow(`PRAGMA journal_mode=WAL`).Scan(&mode); err != nil || mode != "wal" {
						t.Fatal("reference WAL fixture setup failed", mode, err)
					}
					before := databaseBytes(t, contexts.projects.sqlite.attachments)
					priorAudits := soilWriteCount(t, db, "Sample_Audit")
					result, err := contexts.writeTwoPageVegetation(context.Background(), state.ContextID, request)
					if result != nil || err == nil ||
						!strings.Contains(err.Error(), "reference definitions require rollback-journal") ||
						soilWriteCount(t, db, "Sample_Audit") != priorAudits ||
						!reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
						t.Fatal("WAL reference observation granted writes or left history", result, err)
					}
					if err := competing.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil || mode != "delete" {
						t.Fatal("reference rollback-journal correction failed", mode, err)
					}
				}
				before := databaseBytes(t, contexts.projects.sqlite.attachments)
				blocked := false
				result, err := contexts.writeTwoPageVegetationObserved(context.Background(), state.ContextID, request, func(*sql.Tx) error {
					_, err := competing.Exec(`UPDATE ` + table + ` SET ScientificName='competing' WHERE Code='A'`)
					if err == nil || !strings.Contains(strings.ToLower(err.Error()), "locked") {
						return fmt.Errorf("reference definition was not reserved through commit: %w", err)
					}
					blocked = true
					return nil
				}, nil)
				after := databaseBytes(t, contexts.projects.sqlite.attachments)
				if err != nil || result == nil || !result.DidCommit || !blocked ||
					!reflect.DeepEqual(before[role], after[role]) {
					t.Fatal("reference reservation or corrected retry failed", result, err)
				}
				if _, err := competing.Exec(`UPDATE ` + table + ` SET ScientificName='after commit' WHERE Code='A'`); err != nil {
					t.Fatal("completed selection did not release the reference reservation", err)
				}
			})
		}
	}
}
