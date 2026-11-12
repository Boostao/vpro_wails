package classification

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func metadataTemplateFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, ProjectMetadataTemplateCreate) {
	t.Helper()
	service, state, blank := metadataCreateFixture(t)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
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
		value := vprotest.MetadataTestCell(t, review.MasterTemplates, selected.RowID, name)
		switch name {
		case "StartDate":
			value = vprotest.MetadataInteger("2024")
		case "GeoRefMethod":
			value = vprotest.MetadataText("Literal chosen method")
		}
		request.Values = append(request.Values, vprocore.ProjectMetadataChange{Column: name, Value: value})
	}
	return service, state, request
}

func TestProjectMetadataTemplateMappingMatchesSourceInsertAndSelect(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("testdata", "projectmetadata", "template-copy.vba"))
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
	before := vprotest.MetadataFileBytes(t, service)
	created, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request)
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
		if got := vprotest.MetadataTestCell(t, review.ProjectRecords, created.RowID, change.Column); !reflect.DeepEqual(got, change.Value) {
			t.Fatal("explicit template value was converted or inferred:", change.Column, got)
		}
	}
	for _, name := range []string{"AllSpecs", "TableOfLists", "DateLastEdited", "ProjectType", "CollectedWildlifeHabitatAssessment", "BAPID"} {
		if vprotest.MetadataTestCell(t, review.ProjectRecords, created.RowID, name).Storage != "null" {
			t.Fatal("template copy invented unmapped values/stamps:", name)
		}
	}
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	var count int
	var snapshot string
	if err := db.QueryRow(`SELECT COUNT(*),AfterEdit FROM Sample_Audit WHERE EditField='CreateRecord' AND "Table"='_Metadata'`).Scan(&count, &snapshot); err != nil || count != 1 {
		t.Fatal("template creation needs one complete audit", count, err)
	}
	var stored vprocore.ProjectMetadataTable
	if err := json.Unmarshal([]byte(snapshot), &stored); err != nil || !reflect.DeepEqual(stored, review.ProjectRecords) {
		t.Fatal("template creation audit lost complete types/schema/values", err)
	}
	vprotest.AssertMetadataFileBytes(t, service, before, "project")
	stable := vprotest.MetadataFileBytes(t, service)
	if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("completed template creation replayed")
	}
	vprotest.AssertMetadataFileBytes(t, service, stable, "")
}

func TestProjectMetadataTemplateRejectsMissingChoicesCoercionIdentityAndCancelledRequests(t *testing.T) {
	service, state, original := metadataTemplateFixture(t)
	before := vprotest.MetadataFileBytes(t, service)
	for _, mutate := range []func(*ProjectMetadataTemplateCreate){
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "01" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "-0" },
		func(r *ProjectMetadataTemplateCreate) { r.RowID = "999999" },
		func(r *ProjectMetadataTemplateCreate) { r.Values = nil },
		func(r *ProjectMetadataTemplateCreate) { r.Values = r.Values[:31] },
		func(r *ProjectMetadataTemplateCreate) { r.Values[0].Column = "ProjectID" },
		func(r *ProjectMetadataTemplateCreate) {
			r.Values[0].Value = vprotest.MetadataText(strings.Repeat("x", 256))
		},
		func(r *ProjectMetadataTemplateCreate) { r.Master.Rows = r.Master.Rows[:0] },
		func(r *ProjectMetadataTemplateCreate) { r.Master.Columns = r.Master.Columns[:41] },
		func(r *ProjectMetadataTemplateCreate) {
			for i := range r.Values {
				if r.Values[i].Column == "StartDate" {
					r.Values[i].Value = vprotest.MetadataText("2002-03-04 05:06:07")
				}
			}
		},
		func(r *ProjectMetadataTemplateCreate) {
			for i := range r.Values {
				if r.Values[i].Column == "GeoRefMethod" {
					r.Values[i].Value = vprotest.MetadataInteger("9")
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
		if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request); err == nil {
			t.Fatal("invalid/stale template proposal accepted")
		}
		vprotest.AssertMetadataFileBytes(t, service, before, "")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := CreateProjectMetadataFromTemplate(service, ctx, state.ContextID, original); err == nil {
		t.Fatal("cancelled template creation committed")
	}
	if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), "stale", original); err == nil {
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
	vprotest.AssertMetadataFileBytes(t, service, before, "")
}

func TestProjectMetadataTemplateLegacyDuplicateCandidatesRemainExplicit(t *testing.T) {
	service, state, request := metadataTemplateFixture(t)
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
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
			request.Values[i].Value = vprotest.MetadataText("Second physical candidate")
		}
	}
	created, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request)
	if err != nil {
		t.Fatal(err)
	}
	final, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.Blank.PlotNumber)
	if err != nil || len(final.ProjectRecords.Rows) != 1 ||
		!reflect.DeepEqual(final.MasterTemplates, request.Master) ||
		!reflect.DeepEqual(vprotest.MetadataTestCell(t, final.ProjectRecords, created.RowID, "ProjectTitle"), vprotest.MetadataText("Second physical candidate")) {
		t.Fatal("legacy duplicate selection copied multiple rows or changed master storage:", err)
	}
}

func TestProjectMetadataTemplateStaleMasterAndParentDriftRollbackRetainRetry(t *testing.T) {
	service, state, request := metadataTemplateFixture(t)
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if _, err := master.Exec(`UPDATE ProjectMetaData SET Notes='hidden source drift' WHERE rowid=?`, request.RowID); err != nil {
		t.Fatal(err)
	}
	before := vprotest.MetadataFileBytes(t, service)
	if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("stale complete master review accepted")
	}
	vprotest.AssertMetadataFileBytes(t, service, before, "")
	if _, err := master.Exec(`UPDATE ProjectMetaData SET Notes='' WHERE rowid=?`, request.RowID); err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`CREATE TRIGGER fail_template_audit AFTER INSERT ON Sample_Audit WHEN NEW.EditField='CreateRecord'
		BEGIN UPDATE Sample_Admin SET OfficeNotes='hidden parent drift' WHERE Plot='META1'; END`); err != nil {
		t.Fatal(err)
	}
	before = vprotest.MetadataFileBytes(t, service)
	if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request); err == nil {
		t.Fatal("audit-trigger parent drift committed")
	}
	vprotest.AssertMetadataFileBytes(t, service, before, "")
	if _, err := db.Exec(`DROP TRIGGER fail_template_audit`); err != nil {
		t.Fatal(err)
	}
	if _, err := CreateProjectMetadataFromTemplate(service, t.Context(), state.ContextID, request); err != nil {
		t.Fatal("template retry failed:", err)
	}
}
