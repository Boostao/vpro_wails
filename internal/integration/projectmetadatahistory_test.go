package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProjectMetadataTypedHistoryOriginalStorageAuditsRepeatedAndNoop(t *testing.T) {
	service, state, request := vprotest.MetadataEditFixture(t)
	db := vprotest.MetadataHistoryDatabase(t, service)
	if _, err := db.Exec(`UPDATE Sample_Metadata SET Notes=X'00FF',ProjectTitle=? WHERE ID=-200`,
		strings.Repeat("historical ", 40)); err != nil {
		t.Fatal(err)
	}
	request = vprotest.ReloadMetadataEdit(t, service, state, request)
	original := request.Original
	request.Changes = []vprocore.ProjectMetadataChange{{Column: "FieldLeader", Value: vprotest.MetadataText("  First \U0001f600 literal  ")}}
	before := vprotest.MetadataFileBytes(t, service)
	for _, text := range []string{"  First \U0001f600 literal  ", ""} {
		request.Changes[0].Value = vprotest.MetadataText(text)
		if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
			t.Fatal(err)
		}
		request = vprotest.ReloadMetadataEdit(t, service, state, request)
	}
	vprotest.AssertMetadataFileBytes(t, service, before, "project")
	rows, err := db.Query(`SELECT ID,Proposal FROM __VPRO_MetadataEditHistory ORDER BY ID`)
	if err != nil {
		t.Fatal(err)
	}
	var events []vprocore.ProjectMetadataEditHistory
	for rows.Next() {
		var id int64
		var raw string
		if err := rows.Scan(&id, &raw); err != nil {
			t.Fatal(err)
		}
		var event vprocore.ProjectMetadataEditHistory
		if err := json.Unmarshal([]byte(raw), &event); err != nil {
			t.Fatal(err)
		}
		if id != int64(len(events)+1) || event.Project != "Sample" || event.PlotNumber != "META1" ||
			event.ID != -200 || !reflect.DeepEqual(event.ProjectID, request.ProjectID) ||
			!reflect.DeepEqual(event.Columns, request.Columns) || event.Original.RowID != "-200" ||
			event.Committed.RowID != "-200" || len(event.Audits) == 0 || len(event.Audits) != len(event.Records) {
			t.Fatalf("incomplete typed metadata event: %+v", event)
		}
		for i, record := range event.Records {
			if record.RowID != event.Audits[i].RowID || record.EditField != event.Audits[i].Column ||
				record.Table != "_Metadata" || record.ID == nil || *record.ID != event.ID {
				t.Fatal("typed event did not bind its exact audit identities")
			}
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || !reflect.DeepEqual(events[0].Original, original) ||
		!reflect.DeepEqual(events[1].Original, events[0].Committed) ||
		!reflect.DeepEqual(events[1].Committed, request.Original) {
		t.Fatal("typed history lost the complete original or reverse-edit chain")
	}
	noChange := vprotest.MetadataFileBytes(t, service)
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	vprotest.AssertMetadataFileBytes(t, service, noChange, "")
}

func TestProjectMetadataTypedHistoryRejectedProvenanceAndAuditRollback(t *testing.T) {
	for name, sqlText := range map[string]string{
		"schema":  `CREATE TABLE __VPRO_MetadataEditHistory(Protected TEXT)`,
		"view":    `CREATE VIEW __VPRO_MetadataEditHistory AS SELECT 1 AS Protected`,
		"index":   vprocore.ProjectMetadataEditHistorySQL + `; CREATE INDEX metadata_history_index ON __VPRO_MetadataEditHistory(Created)`,
		"trigger": vprocore.ProjectMetadataEditHistorySQL + `; CREATE TRIGGER metadata_history_trigger AFTER INSERT ON __VPRO_MetadataEditHistory BEGIN SELECT 1; END`,
		"audit": `CREATE TRIGGER metadata_audit_trigger AFTER INSERT ON Sample_Audit
			WHEN NEW."Table"='_Metadata' BEGIN UPDATE Sample_Audit SET AfterEdit='Unreviewed trigger' WHERE rowid=NEW.rowid; END`,
	} {
		t.Run(name, func(t *testing.T) {
			service, state, request := vprotest.MetadataEditFixture(t)
			db := vprotest.MetadataHistoryDatabase(t, service)
			if _, err := db.Exec(sqlText); err != nil {
				t.Fatal(err)
			}
			before := vprotest.MetadataFileBytes(t, service)
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("unreviewed provenance/audit was accepted")
			}
			vprotest.AssertMetadataFileBytes(t, service, before, "")
			if name == "view" {
				if _, err := db.Exec(`DROP VIEW __VPRO_MetadataEditHistory`); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := db.Exec(`DROP TABLE IF EXISTS __VPRO_MetadataEditHistory`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec(`DROP TRIGGER IF EXISTS metadata_audit_trigger`); err != nil {
				t.Fatal(err)
			}
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
				t.Fatal("retained exact metadata proposal did not retry:", err)
			}
			var edits int
			if err := db.QueryRow(`SELECT COUNT(*) FROM __VPRO_MetadataEditHistory`).Scan(&edits); err != nil || edits != 1 {
				t.Fatal("rejected history committed or retry appended more than one event", edits, err)
			}
			vprotest.AssertMetadataFileBytes(t, service, before, "project")
		})
	}
}

func TestProjectMetadataTypedHistoryRespectsAuditStrength(t *testing.T) {
	service, state, request := vprotest.MetadataEditFixture(t)
	db := vprotest.MetadataHistoryDatabase(t, service)
	if err := service.Plots.SetAuditStrength(0); err != nil {
		t.Fatal(err)
	}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='__VPRO_MetadataEditHistory'`).
		Scan(&count); err != nil || count != 0 {
		t.Fatal("disabled audit invented a restoration event", count, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE "Table"='_Metadata'`).
		Scan(&count); err != nil || count != 0 {
		t.Fatal("disabled audit inserted metadata history", count, err)
	}
}
