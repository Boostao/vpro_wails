package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func metadataEditFixture(t *testing.T) (*ContextService, ProjectState, ProjectMetadataEdit) {
	t.Helper()
	service, state := projectMetadataFixture(t)
	if err := service.plots.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range review.ProjectRecords.Rows {
		if row.RowID == "-200" {
			return service, state, ProjectMetadataEdit{PlotNumber: "META1", ProjectID: review.ProjectID,
				ID: -200, Columns: review.ProjectRecords.Columns, Original: row,
				Changes: []ProjectMetadataChange{{Column: "Notes", Value: metadataText("  Edited literal metadata  ")}}}
		}
	}
	t.Fatal("metadata fixture lacks its explicit physical record")
	return nil, ProjectState{}, ProjectMetadataEdit{}
}

func metadataFileBytes(t *testing.T, service *ContextService) map[string][]byte {
	t.Helper()
	return databaseBytes(t, service.projects.sqlite.attachments)
}

func assertMetadataFileBytes(t *testing.T, service *ContextService, before map[string][]byte, except string) {
	t.Helper()
	for role, original := range before {
		if role == except || except != "" && os.SameFile(service.projects.sqlite.attachmentInfo[role], service.projects.sqlite.attachmentInfo[except]) {
			continue
		}
		current, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("metadata operation changed an unexpected file:", role, err)
		}
	}
}

func TestProjectMetadataEditAll70FieldsStampsAuditsAndNoop(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	reference, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reference.Exec(`INSERT INTO USysTableOfLists(ListName,Item,Note,ItemOrder)
		VALUES ('PlotQualitySite','ZMETADATA','Literal quality',10000)`); err != nil {
		reference.Close()
		t.Fatal(err)
	}
	if err := reference.Close(); err != nil {
		t.Fatal(err)
	}
	before := metadataFileBytes(t, service)
	request.Changes = nil
	for column, field := range projectMetadataFields {
		cell := metadataText("  Updated literal metadata  ")
		if field.kind == "integer" {
			cell = metadataInteger("7")
			if field.options {
				cell = metadataInteger("3")
			}
		}
		if field.limitToList {
			cell = metadataText("Literal quality")
		}
		request.Changes = append(request.Changes, ProjectMetadataChange{column, cell})
	}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	assertMetadataFileBytes(t, service, before, "project")
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	if len(review.ProjectRecords.Rows) != 2 || len(review.MasterTemplates.Rows) != 1 {
		t.Fatal("metadata edit collapsed duplicate candidates or changed the master template")
	}
	for _, change := range request.Changes {
		actual := metadataTestCell(t, review.ProjectRecords, "-200", change.Column)
		left, _ := json.Marshal(actual)
		right, _ := json.Marshal(change.Value)
		if !bytes.Equal(left, right) {
			t.Fatal("metadata stored field differs from explicit proposal:", change.Column, string(left), string(right))
		}
	}
	for _, column := range []string{"AllSpecs", "TableOfLists", "DateLastEdited"} {
		cell := metadataTestCell(t, review.ProjectRecords, "-200", column)
		if cell.Storage != "text" || cell.Text == nil || *cell.Text == "" {
			t.Fatal("metadata source stamp missing:", column, cell)
		}
	}
	if metadataTestCell(t, review.ProjectRecords, "-201", "ProjectTitle").Text == nil ||
		*metadataTestCell(t, review.ProjectRecords, "-201", "ProjectTitle").Text != "" {
		t.Fatal("another physical candidate was implicitly copied or edited")
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var audits int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE "Table"='_Metadata' AND ID=-200`).Scan(&audits); err != nil || audits != 73 {
		t.Fatal("metadata edits/stamps must share73 exact field audits:", audits, err)
	}
	for _, row := range review.ProjectRecords.Rows {
		if row.RowID == "-200" {
			request.Original = row
		}
	}
	before = metadataFileBytes(t, service)
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	assertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataEditAuditRollbackFinalObservationAndRetainedRetry(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER metadata_fail BEFORE INSERT ON Sample_Audit WHEN NEW."Table"='_Metadata'
		 BEGIN SELECT RAISE(ABORT,'metadata audit rejected'); END`,
		`CREATE TRIGGER metadata_fail AFTER UPDATE ON Sample_Metadata WHEN NEW.ID=-200
		 BEGIN UPDATE Sample_Metadata SET ProjectTitle='unexpected drift' WHERE ID=NEW.ID; END`,
		`CREATE TRIGGER metadata_fail AFTER INSERT ON Sample_Audit WHEN NEW."Table"='_Metadata'
		 BEGIN UPDATE Sample_Metadata SET ProjectTitle='later audit drift' WHERE ID=NEW.ID; END`,
		`CREATE TRIGGER metadata_fail AFTER INSERT ON Sample_Audit WHEN NEW."Table"='_Metadata'
		 BEGIN UPDATE Sample_Env SET ProjectID='OTHER' WHERE PlotNumber='META1'; END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			service, state, request := metadataEditFixture(t)
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			before := metadataFileBytes(t, service)
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("metadata drift/audit failure committed")
			}
			assertMetadataFileBytes(t, service, before, "")
			if _, err := db.Exec(`DROP TRIGGER metadata_fail`); err != nil {
				t.Fatal(err)
			}
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
				t.Fatal("retained original request could not be retried after rollback:", err)
			}
		})
	}
}

func TestProjectMetadataEditOwnershipCollisionAndStaleOriginal(t *testing.T) {
	for _, mutation := range []string{
		`UPDATE Sample_Env SET ProjectID='OTHER' WHERE PlotNumber='META1'`,
		`DELETE FROM Sample_Admin WHERE Plot='META1'`,
		`UPDATE Sample_Metadata SET Notes='concurrent edit' WHERE ID=-200`,
		`ALTER TABLE Sample_Metadata ADD COLUMN Unexpected`,
		`ALTER TABLE Sample_Metadata RENAME TO MetadataBefore;
		 CREATE TABLE Sample_Metadata AS SELECT * FROM MetadataBefore;
		 INSERT INTO Sample_Metadata(ID,ProjectID) VALUES(-200,'UNRELATED'); DROP TABLE MetadataBefore`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state, request := metadataEditFixture(t)
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(mutation); err != nil {
				t.Fatal(err)
			}
			before := metadataFileBytes(t, service)
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("metadata ownership/collision/stale guard accepted")
			}
			assertMetadataFileBytes(t, service, before, "")
		})
	}
	service, state, request := metadataEditFixture(t)
	before := metadataFileBytes(t, service)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := service.SaveProjectMetadata(cancelled, state.ContextID, request); err == nil {
		t.Fatal("cancelled metadata edit accepted")
	}
	if err := service.SaveProjectMetadata(t.Context(), "stale", request); err == nil {
		t.Fatal("stale metadata context accepted")
	}
	request.ID = -201
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("selected candidate ID changed implicitly")
	}
	assertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataEditStrictNoteBindingVersionsAndExplicitStandardDecision(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	before := metadataFileBytes(t, service)
	request.Changes = []ProjectMetadataChange{{Column: "DataQualityVeg", Value: metadataText("unregistered Note")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("strict quality field accepted an unregistered Note")
	}
	assertMetadataFileBytes(t, service, before, "")
	request.Changes = []ProjectMetadataChange{{Column: "EcosysCollectionStandard", Value: metadataText("DEIF literal")}}
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("recognized source standard bypassed the population decision")
	}
	assertMetadataFileBytes(t, service, before, "")
	request.StandardPopulation = "keep"
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal("explicitly declined source-default population rejected:", err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil || metadataTestCell(t, review.ProjectRecords, "-200", "CoordinatingAgency").Storage != "null" {
		t.Fatal("standard edit silently populated source defaults:", err)
	}
}

func TestProjectMetadataEditMissingAmbiguousAndMalformedVersionsNeverFallback(t *testing.T) {
	for _, mutation := range []string{
		`DELETE FROM _table_metadata WHERE table_name='USysTableOfLists'`,
		`UPDATE _table_metadata SET description=NULL WHERE table_name='USysTableOfLists'`,
		`UPDATE _table_metadata SET description=CAST(x'FF' AS TEXT) WHERE table_name='USysTableOfLists'`,
		`ALTER TABLE _table_metadata RENAME TO MetadataBefore;
		 CREATE TABLE _table_metadata AS SELECT * FROM MetadataBefore;
		 INSERT INTO _table_metadata SELECT * FROM MetadataBefore WHERE table_name='USysTableOfLists';
		 DROP TABLE MetadataBefore`,
	} {
		t.Run(mutation, func(t *testing.T) {
			service, state, request := metadataEditFixture(t)
			db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VLists"], "rw"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(mutation); err != nil {
				db.Close()
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			before := metadataFileBytes(t, service)
			if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("missing/ambiguous/malformed table-object description became a success-shaped stamp")
			}
			assertMetadataFileBytes(t, service, before, "")
		})
	}
}

func TestProjectMetadataEditJSONRejectsUnicodeRepairUnknownFieldsAndNullIdentity(t *testing.T) {
	_, _, request := metadataEditFixture(t)
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProjectMetadataEdit
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal("complete typed metadata request failed transport:", err)
	}
	for _, raw := range []string{
		strings.Replace(string(data), `"  Edited literal metadata  "`, `"\ud800"`, 1),
		strings.Replace(string(data), `"  Edited literal metadata  "`, `"\udc00"`, 1),
		strings.Replace(string(data), `"  Edited literal metadata  "`, "\""+string([]byte{0xff})+"\"", 1),
		strings.Replace(string(data), `"column":"Notes"`, `"column":"\ud800","column":"Notes"`, 1),
		strings.Replace(string(data), `"id":-200`, `"id":null`, 1),
		strings.Replace(string(data), `"id":-200`, `"id":-200,"hiddenDefault":true`, 1),
		strings.Replace(string(data), `"id":-200,`, "", 1),
	} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("metadata JSON repair/default/unknown assignment accepted:", raw)
		}
	}

}

func TestProjectMetadataEditSharedSupportOwnershipAndRestoreAliasesFailClosed(t *testing.T) {
	service, state, request := metadataEditFixture(t)
	before := metadataFileBytes(t, service)
	c := service.projects.sqlite
	projectPath, projectInfo := c.attachments["project"], c.attachmentInfo["project"]
	c.attachments["project"], c.attachmentInfo["project"] = c.attachments["VLists"], c.attachmentInfo["VLists"]
	err := service.SaveProjectMetadata(t.Context(), state.ContextID, request)
	c.attachments["project"], c.attachmentInfo["project"] = projectPath, projectInfo
	if err == nil || !strings.Contains(err.Error(), "support-file writes") {
		t.Fatal("shared support/project ownership was accepted:", err)
	}
	assertMetadataFileBytes(t, service, before, "")
	if err := service.SaveProjectMetadata(t.Context(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var auditID string
	if err := db.QueryRow(`SELECT CAST(rowid AS TEXT) FROM Sample_Audit WHERE "Table"='_Metadata' AND ID=-200 LIMIT 1`).Scan(&auditID); err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"_Metadata", "Sample_Metadata"} {
		if _, err := db.Exec(`UPDATE Sample_Audit SET "Table"=? WHERE rowid=?`, alias, auditID); err != nil {
			t.Fatal(err)
		}
		before = metadataFileBytes(t, service)
		if _, err := service.RestoreSelectedAuditRecords(state.ContextID, "META1", []string{auditID}, AuditRestoreRetain); err == nil {
			t.Fatal("unavailable metadata restoration alias was treated as a child/header field:", alias)
		}
		assertMetadataFileBytes(t, service, before, "")
	}
}
