package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func metadataTemplateFixture(t *testing.T) (*ContextService, ProjectState, ProjectMetadataTemplateCreate) {
	t.Helper()
	service, state, blank := metadataCreateFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO ProjectMetaData(ProjectID,ProjectTitle,StartDate,GeoRefMethod,CollectedSite,Notes)
		VALUES ('OtherTemplate','Unselected template','2001-02-03 04:05:06',7,1,NULL),
		('BlankMetadata','  Selected template  ','2002-03-04 05:06:07',9,2,'')`); err != nil {
		db.Close()
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, blank.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := metadataTemplateMapping()
	if err != nil {
		t.Fatal(err)
	}
	selected := review.MasterTemplates.Rows[0]
	request := ProjectMetadataTemplateCreate{Blank: blank, Master: review.MasterTemplates, RowID: selected.RowID}
	for _, name := range mapping[1:] {
		value := metadataTestCell(t, review.MasterTemplates, selected.RowID, name)
		switch name {
		case "StartDate":
			value = metadataInteger("2024")
		case "GeoRefMethod":
			value = metadataText("Literal chosen method")
		}
		request.Values = append(request.Values, ProjectMetadataChange{Column: name, Value: value})
	}
	return service, state, request
}

func TestProjectMetadataTemplateMappingMatchesSourceInsertAndSelect(t *testing.T) {
	source, err := os.ReadFile(`testdata\projectmetadata\template-copy.vba`)
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := metadataTemplateMapping()
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range []string{`_Metadata \( (.+) \)"`, `" SELECT (.+)"`} {
		match := regexp.MustCompile(pattern).FindStringSubmatch(string(source))
		if len(match) != 2 || !reflect.DeepEqual(strings.Split(match[1], ", "), mapping) {
			t.Fatal("template mapping does not preserve all33 literal source destinations/selections")
		}
	}
	if len(mapping) != 33 {
		t.Fatal("source mapping changed")
	}
}

func TestProjectMetadataTemplateCreationSelectsOneRowAndAuditsExplicitValues(t *testing.T) {
	service, state, request := metadataTemplateFixture(t)
	before := metadataFileBytes(t, service)
	created, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.Blank.PlotNumber)
	if err != nil || len(review.ProjectRecords.Rows) != 1 || !reflect.DeepEqual(created, review.ProjectRecords.Rows[0]) {
		t.Fatal("template must create one exact physical row, never all duplicate candidates:", err)
	}
	if !reflect.DeepEqual(review.MasterTemplates, request.Master) {
		t.Fatal("readonly master rows/schema changed")
	}
	for _, change := range request.Values {
		if got := metadataTestCell(t, review.ProjectRecords, created.RowID, change.Column); !reflect.DeepEqual(got, change.Value) {
			t.Fatal("explicit template value was converted or inferred:", change.Column, got)
		}
	}
	for _, name := range []string{"AllSpecs", "TableOfLists", "DateLastEdited", "ProjectType", "CollectedWildlifeHabitatAssessment", "BAPID"} {
		if metadataTestCell(t, review.ProjectRecords, created.RowID, name).Storage != "null" {
			t.Fatal("template copy invented unmapped values/stamps:", name)
		}
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var count int
	var snapshot string
	if err := db.QueryRow(`SELECT COUNT(*),AfterEdit FROM Sample_Audit WHERE EditField='CreateRecord' AND "Table"='_Metadata'`).Scan(&count, &snapshot); err != nil || count != 1 {
		t.Fatal("template creation needs one complete audit", count, err)
	}
	var stored ProjectMetadataTable
	if err := json.Unmarshal([]byte(snapshot), &stored); err != nil || !reflect.DeepEqual(stored, review.ProjectRecords) {
		t.Fatal("template creation audit lost complete types/schema/values", err)
	}
	assertMetadataFileBytes(t, service, before, "project")
	stable := metadataFileBytes(t, service)
	if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed template creation replayed")
	}
	assertMetadataFileBytes(t, service, stable, "")
}

func TestProjectMetadataTemplateRejectsMissingChoicesCoercionIdentityAndCancelledRequests(t *testing.T) {
	service, state, original := metadataTemplateFixture(t)
	before := metadataFileBytes(t, service)
	for _, mutate := range []func(*ProjectMetadataTemplateCreate){
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "01" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "-0" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "999999" },
		func(r *ProjectMetadataTemplateCreate) { r.Values = nil },
		func(r *ProjectMetadataTemplateCreate) { r.Values = r.Values[:31] },
		func(r *ProjectMetadataTemplateCreate) { r.Values[0].Column = "ProjectID" },
		func(r *ProjectMetadataTemplateCreate) { r.Values[0].Value = metadataText(strings.Repeat("x", 256)) },
		func(r *ProjectMetadataTemplateCreate) { r.Master.Rows = r.Master.Rows[:0] },
		func(r *ProjectMetadataTemplateCreate) { r.Master.Columns = r.Master.Columns[:41] },
		func(r *ProjectMetadataTemplateCreate) {
			for i := range r.Values {
				if r.Values[i].Column == "StartDate" {
					r.Values[i].Value = metadataText("2002-03-04 05:06:07")
				}
			}
		},
		func(r *ProjectMetadataTemplateCreate) {
			for i := range r.Values {
				if r.Values[i].Column == "GeoRefMethod" {
					r.Values[i].Value = metadataInteger("9")
				}
			}
		},
	} {
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		var request ProjectMetadataTemplateCreate
		if err := json.Unmarshal(data, &request); err != nil {
			t.Fatal(err)
		}
		mutate(&request)
		if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request); err == nil {
			t.Fatal("invalid/stale template proposal accepted")
		}
		assertMetadataFileBytes(t, service, before, "")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := service.CreateProjectMetadataFromTemplate(ctx, state.ContextID, original); err == nil {
		t.Fatal("cancelled template creation committed")
	}
	if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), "stale", original); err == nil {
		t.Fatal("stale template context accepted")
	}
	for _, raw := range []string{
		`{}`, `{"blank":null,"master":{},"rowId":"1","values":[]}`,
		`{"blank":{"plotNumber":"META1","projectId":"\ud800","original":{"columns":[],"rows":[]}},"master":{},"rowId":"1","values":[]}`,
	} {
		var request ProjectMetadataTemplateCreate
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("missing/null/malformed raw template JSON accepted")
		}
	}
	assertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataTemplateLegacyDuplicateCandidatesRemainExplicit(t *testing.T) {
	service, state, request := metadataTemplateFixture(t)
	master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if _, err := master.Exec(`ALTER TABLE ProjectMetaData RENAME TO OriginalMaster;
		CREATE TABLE ProjectMetaData AS SELECT * FROM OriginalMaster;
		DROP TABLE OriginalMaster;
		INSERT INTO ProjectMetaData SELECT * FROM ProjectMetaData WHERE ProjectID='BlankMetadata';
		UPDATE ProjectMetaData SET ProjectTitle='Second physical candidate' WHERE rowid=(SELECT MAX(rowid) FROM ProjectMetaData)`); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.Blank.PlotNumber)
	if err != nil || len(review.MasterTemplates.Rows) != 2 {
		t.Fatal("legacy duplicate candidates unavailable:", err)
	}
	request.Master = review.MasterTemplates
	request.RowID = review.MasterTemplates.Rows[1].RowID
	for i := range request.Values {
		if request.Values[i].Column == "ProjectTitle" {
			request.Values[i].Value = metadataText("Second physical candidate")
		}
	}
	created, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	final, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.Blank.PlotNumber)
	if err != nil || len(final.ProjectRecords.Rows) != 1 ||
		!reflect.DeepEqual(final.MasterTemplates, request.Master) ||
		!reflect.DeepEqual(metadataTestCell(t, final.ProjectRecords, created.RowID, "ProjectTitle"), metadataText("Second physical candidate")) {
		t.Fatal("legacy duplicate selection copied multiple rows or changed master storage:", err)
	}
}

func TestProjectMetadataTemplateStaleMasterAndParentDriftRollbackRetainRetry(t *testing.T) {
	service, state, request := metadataTemplateFixture(t)
	master, err := sql.Open("sqlite3", sqliteFileURI(service.projects.supportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if _, err := master.Exec(`UPDATE ProjectMetaData SET Notes='hidden source drift' WHERE rowid=?`, request.RowID); err != nil {
		t.Fatal(err)
	}
	before := metadataFileBytes(t, service)
	if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("stale complete master review accepted")
	}
	assertMetadataFileBytes(t, service, before, "")
	if _, err := master.Exec(`UPDATE ProjectMetaData SET Notes='' WHERE rowid=?`, request.RowID); err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`CREATE TRIGGER fail_template_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord'
		BEGIN UPDATE Sample_Admin SET OfficeNotes='hidden parent drift' WHERE Plot='META1'; END`); err != nil {
		t.Fatal(err)
	}
	before = metadataFileBytes(t, service)
	if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request); err == nil {
		t.Fatal("audit-trigger parent drift committed")
	}
	assertMetadataFileBytes(t, service, before, "")
	if _, err := db.Exec(`DROP TRIGGER fail_template_audit`); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateProjectMetadataFromTemplate(t.Context(), state.ContextID, request); err != nil {
		t.Fatal("template retry failed:", err)
	}
}
