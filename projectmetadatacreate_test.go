package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func metadataCreateFixture(t *testing.T) (*ContextService, ProjectState, ProjectMetadataCreate) {
	t.Helper()
	service, state := projectMetadataFixture(t)
	if err := service.plots.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET ProjectID='BlankMetadata' WHERE PlotNumber='META1'`); err != nil {
		release()
		t.Fatal(err)
	}
	release()
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	return service, state, ProjectMetadataCreate{PlotNumber: "META1", ProjectID: review.ProjectID, Original: review.ProjectRecords}
}

func TestProjectMetadataBlankCreationPreservesSourceNullsAndAuditsFullSnapshot(t *testing.T) {
	service, state, request := metadataCreateFixture(t)
	before := metadataFileBytes(t, service)
	row, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	if len(review.ProjectRecords.Rows) != 1 || !reflect.DeepEqual(row, review.ProjectRecords.Rows[0]) {
		t.Fatal("created row differs from independent selected-context review")
	}
	var id string
	for i, column := range request.Original.Columns {
		cell := row.Cells[i]
		switch column.Name {
		case "ID":
			if cell.Integer == nil || cell.Storage != "integer" {
				t.Fatal("blank identity missing")
			}
			id = *cell.Integer
		case "ProjectID":
			if cell.Text == nil || *cell.Text != *request.ProjectID {
				t.Fatal("parent identity changed")
			}
		default:
			if cell.Storage != "null" {
				t.Fatal("blank creation invented a source field/stamp:", column.Name, cell)
			}
		}
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var audits, reserved int
	var snapshot string
	if err := db.QueryRow(`SELECT COUNT(*),AfterEdit FROM Sample_Audit WHERE "Table"='_Metadata' AND EditField='CreateRecord' AND ID=?`, id).Scan(&audits, &snapshot); err != nil || audits != 1 {
		t.Fatal("blank creation must audit one complete snapshot:", audits, err)
	}
	var observed ProjectMetadataTable
	if err := json.Unmarshal([]byte(snapshot), &observed); err != nil || !reflect.DeepEqual(observed, review.ProjectRecords) {
		t.Fatal("blank audit lost typed NULLs/schema/identity:", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE ChildTable='"Sample_Metadata"' AND ID=?`, id).Scan(&reserved); err != nil || reserved != 1 {
		t.Fatal("created identity was not reserved", err)
	}
	assertMetadataFileBytes(t, service, before, "project")
	stable := metadataFileBytes(t, service)
	if _, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed creation replayed from stale empty review")
	}
	assertMetadataFileBytes(t, service, stable, "")
}

func TestProjectMetadataBlankCreationReservesDeletedAuditedIDs(t *testing.T) {
	service, state, request := metadataCreateFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`INSERT INTO Sample_Audit("Table",ID,EditField) VALUES
		('_Metadata',1,'DeleteRecord'),('Sample_Metadata',2,'DeleteRecord');
		CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity"(ChildTable TEXT NOT NULL,ID INTEGER NOT NULL,PRIMARY KEY(ChildTable,ID));
		INSERT INTO "__VPRO_ChildIdentity" VALUES ('"Sample_Metadata"',3)`); err != nil {
		t.Fatal(err)
	}
	row, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	for i, column := range request.Original.Columns {
		if column.Name == "ID" && (row.Cells[i].Integer == nil || *row.Cells[i].Integer != "4") {
			t.Fatal("deleted/audited/reserved identities were reused")
		}
	}
}

func TestProjectMetadataBlankCreationRollbackRetainsEmptyReviewForRetry(t *testing.T) {
	for _, trigger := range []string{
		`CREATE TRIGGER fail_create_audit BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN SELECT RAISE(ABORT,'audit rejected'); END`,
		`CREATE TRIGGER fail_create_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN UPDATE Sample_Metadata SET Notes='audit drift' WHERE ID=NEW.ID; END`,
		`CREATE TRIGGER fail_create_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN UPDATE Sample_Audit SET AfterEdit='lost snapshot' WHERE rowid=NEW.rowid; END`,
		`CREATE TRIGGER fail_create_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN DELETE FROM "__VPRO_ChildIdentity" WHERE ID=NEW.ID AND ChildTable='"Sample_Metadata"'; END`,
		`CREATE TRIGGER fail_create_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN UPDATE Sample_Env SET ProjectID='ChangedParent' WHERE PlotNumber=NEW.PlotNumber; END`,
		`CREATE TRIGGER fail_create_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord' BEGIN UPDATE Sample_Admin SET OfficeNotes='Unplanned parent change' WHERE Plot=NEW.PlotNumber; END`,
	} {
		t.Run(trigger, func(t *testing.T) {
			service, state, request := metadataCreateFixture(t)
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(trigger); err != nil {
				t.Fatal(err)
			}
			before := metadataFileBytes(t, service)
			if _, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request); err == nil {
				t.Fatal("creation committed after audit/snapshot/reservation failure")
			}
			assertMetadataFileBytes(t, service, before, "")
			if _, err := db.Exec("DROP TRIGGER fail_create_audit"); err != nil {
				t.Fatal(err)
			}
			if _, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request); err != nil {
				t.Fatal("unchanged empty review could not retry rolled-back creation:", err)
			}
		})
	}
}

func TestProjectMetadataBlankCreationRejectsStaleIdentitySchemaUnicodeAndCancelledRequests(t *testing.T) {
	service, state, original := metadataCreateFixture(t)
	before := metadataFileBytes(t, service)
	for _, mutate := range []func(*ProjectMetadataCreate){
		func(r *ProjectMetadataCreate) { r.ProjectID = nil },
		func(r *ProjectMetadataCreate) { value := ""; r.ProjectID = &value },
		func(r *ProjectMetadataCreate) { value := strings.Repeat("x", 21); r.ProjectID = &value },
		func(r *ProjectMetadataCreate) { value := "blankmetadata"; r.ProjectID = &value },
		func(r *ProjectMetadataCreate) { r.Original.Rows = nil },
		func(r *ProjectMetadataCreate) { r.Original.Columns = r.Original.Columns[:74] },
		func(r *ProjectMetadataCreate) { r.PlotNumber = "META1' OR 1=1" },
	} {
		request := original
		mutate(&request)
		if _, err := service.CreateBlankProjectMetadata(t.Context(), state.ContextID, request); err == nil {
			t.Fatal("invalid/stale blank proposal accepted")
		}
		assertMetadataFileBytes(t, service, before, "")
	}
	if _, err := service.CreateBlankProjectMetadata(t.Context(), "stale", original); err == nil {
		t.Fatal("stale context created metadata")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.CreateBlankProjectMetadata(ctx, state.ContextID, original); err == nil {
		t.Fatal("cancelled creation committed")
	}
	for _, raw := range []string{
		`{"plotNumber":"META1","projectId":"BlankMetadata","original":{"columns":[{"name":"\ud800","declaredType":"TEXT"}],"rows":[]}}`,
		`{"plotNumber":"META1","projectId":"BlankMetadata"}`,
		`{"plotNumber":"META1","projectId":"BlankMetadata","original":{},"values":[]}`,
	} {
		var request ProjectMetadataCreate
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed/unsupported blank JSON accepted")
		}
	}
	assertMetadataFileBytes(t, service, before, "")
}
