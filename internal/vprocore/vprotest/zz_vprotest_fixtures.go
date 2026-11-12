package vprotest

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"

	"database/sql"

	"encoding/json"

	"errors"
	"fmt"

	"math"

	"os"
	"path/filepath"
	"reflect"

	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"testing"
)

func BecTestDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}

func BecFixture(t *testing.T) *references.BECService {
	t.Helper()
	s, err := references.NewBECService(BecTestDir(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func BecString(value string) *string { return &value }

func ContextServiceFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	projects, _, _, _ := SqliteServiceFixture(t)
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	service, err := vprocore.NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	state, err := projects.GetState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func ContextSelection(state vprocore.ProjectState) vprocore.ContextSelection {
	return vprocore.ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: state.ActiveSU, SUPath: state.SUPath, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath}
}

func ConfigFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	data, config := filepath.Join(root, "data"), filepath.Join(root, "config")
	for _, dir := range []string{data, config} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return data, config
}

func ReportServiceFixture(t *testing.T, external bool) (*vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	service, state := ContextServiceFixture(t)
	path := state.ProjectPath
	if external {
		path = filepath.Join(t.TempDir(), "external.db")
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rwc"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Report_SU(PlotNumber TEXT,SiteUnit TEXT);
		INSERT INTO Report_SU VALUES('108050','  Unit ''quoted''  '),('missing','Orphan'),('108050x',NULL)`)
	err = errors.Join(err, db.Close())
	if err != nil {
		t.Fatal(err)
	}
	selection := ContextSelection(state)
	selection.SU, selection.SUPath = "Report", path
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	return service, next
}

func WorkbookRequest(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func PrepareKMLContextFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	service, state := ContextServiceFixture(t)
	MutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Longitude=123.25,Latitude=54.5,Zone=NULL
WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL`)
	return service, state
}

func HeightFloat(value float64) *float64 { return &value }

func SeedHeight(t *testing.T, db *sql.DB, plot string, id any) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO Sample_Veg
		(PlotNumber,ID,Species,Cover1,Height1,Cover2,Height2,TotalA,TotalB,Collected)
		VALUES (?,?,'PSEUMEN',20,1.25,30,4.75,45,55,'C')`, plot, id); err != nil {
		t.Fatal(err)
	}
}

func StoredHeight(t *testing.T, db *sql.DB, plot string, id int, field string) *float64 {
	t.Helper()
	var value sql.NullFloat64
	if err := db.QueryRow(`SELECT `+vprocore.QuoteHeaderIdentifier(field)+` FROM Sample_Veg WHERE PlotNumber=? AND ID=?`, plot, id).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if !value.Valid {
		return nil
	}
	return &value.Float64
}

func HeightTableSnapshot(t *testing.T, db *sql.DB, table string) string {
	t.Helper()
	rows, err := db.Query(`SELECT * FROM ` + vprocore.QuoteHeaderIdentifier(table) + ` ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var records [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		destinations := make([]any, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			t.Fatal(err)
		}
		for i, value := range values {
			if data, ok := value.([]byte); ok {
				values[i] = append([]byte(nil), data...)
			}
		}
		records = append(records, values)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(records)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func ReadonlyCatalogueFixture(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sqlutil.OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func ChildFixture(t *testing.T) (*vprocore.PlotService, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	projects, err := vprocore.NewProjectServiceWithConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := vprocore.NewPlotService(projects)
	service.SetCurrentUser("ChildTester")
	db, _, release, err := service.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(release)
	for _, plot := range []string{"CHILD1", "CHILD2"} {
		if err := service.CreatePlot(vprocore.FS882Header{PlotNumber: plot}); err != nil {
			t.Fatal(err)
		}
	}
	return service, db
}

func HeaderFixture(t *testing.T) (*vprocore.PlotService, *sql.DB) {
	t.Helper()
	root := t.TempDir()
	projects, err := vprocore.NewProjectServiceWithConfig(root, root)
	if err != nil {
		t.Fatal(err)
	}
	service := vprocore.NewPlotService(projects)
	service.SetAuditStrength(3)
	service.SetCurrentUser("Will MacKenzie")
	db, _, release, err := service.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(release)
	return service, db
}

func AuditCount(t *testing.T, db *sql.DB, plot string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM Sample_Audit WHERE PlotNumber = ?`, plot).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func LocationTestTables() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, field := range vprocore.PlotLocationFields() {
		env.Columns = append(env.Columns, vprocore.ProjectMetadataColumn{Name: field.Key, DeclaredType: "VARCHAR"})
	}
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	env.Rows = []vprocore.ProjectMetadataRow{
		{RowID: "-9007199254740993", Cells: []vprocore.ProjectMetadataCell{MetadataText("000001"), MetadataText("  Zone  "), MetadataText(""), MetadataText("01"), null, SiviReal(49.25), SiviReal(123.75), MetadataInteger("9007199254740993")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{MetadataText("000002"), null, null, null, MetadataInteger("0"), SiviReal(math.Copysign(0, -1)), SiviReal(-123.75), null}},
		{RowID: "3", Cells: []vprocore.ProjectMetadataCell{MetadataText("000003"), null, null, null, null, null, SiviReal(130), null}},
		{RowID: "4", Cells: []vprocore.ProjectMetadataCell{MetadataText("000004"), null, null, null, null, SiviReal(50), null, null}},
		{RowID: "5", Cells: []vprocore.ProjectMetadataCell{MetadataText("NOADMIN"), null, null, null, null, SiviReal(50), SiviReal(130), null}},
	}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "VARCHAR"}}, Rows: []vprocore.ProjectMetadataRow{}}
	for i := 0; i < 4; i++ {
		admin.Rows = append(admin.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: []vprocore.ProjectMetadataCell{vprocore.CloneSiteUnitCell(env.Rows[i].Cells[0])}})
	}
	su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "VARCHAR"}, {Name: "SiteUnit", DeclaredType: "VARCHAR"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{MetadataText("000001"), null}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{MetadataText("000001"), MetadataText("")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{MetadataText("000001"), MetadataText("different classification")}},
			{RowID: "4", Cells: []vprocore.ProjectMetadataCell{null, MetadataText("unused")}},
		}}
	return env, admin, su
}

func AssertPlotLocationFacadeReadOnly(t *testing.T, service *vprocore.ContextService) func() {
	t.Helper()
	before := DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	contextID := service.Projects.ContextID
	return func() {
		t.Helper()
		AssertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.Projects.Preferences.Path)
		if err != nil || !reflect.DeepEqual(config, after) || service.Projects.ContextID != contextID {
			t.Fatal("location facade changed configuration/context:", err)
		}
	}
}

func PoolFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, *vprocore.FS882Header) {
	t.Helper()
	service, state := ContextServiceFixture(t)
	page, err := service.Projects.ListPlots(context.Background(), 0, 1)
	if err != nil || len(page.Plots) != 1 {
		t.Fatalf("plot fixture: %+v %v", page, err)
	}
	header, err := service.GetPlot(context.Background(), state.ContextID, page.Plots[0].PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, header
}

func AssertNoWALSidecars(t *testing.T, dbPath string) {
	t.Helper()
	for _, suffix := range []string{"-wal", "-shm"} {
		if _, err := os.Stat(dbPath + suffix); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected SQLite sidecar %s%s after read-only workload: %v", dbPath, suffix, err)
		}
	}
}

func ProjectMetadataFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState) {
	t.Helper()
	service, state := ContextServiceFixture(t)
	projectID := "ZMETATEST"
	if err := service.CreatePlot(state.ContextID, vprocore.FS882Header{PlotNumber: "META1", ProjectID: &projectID}); err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.Plots.GetActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := db.Exec(`INSERT INTO Sample_Metadata(ID,ProjectID,StartDate,ProjectTitle,CollectedSite,Notes)
		VALUES (-200,'ZMETATEST',2000,NULL,-1,'  Literal metadata  '),(-201,'ZMETATEST',NULL,'',0,NULL)`); err != nil {
		t.Fatal(err)
	}
	master, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.SupportPaths["VMetaData"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	if _, err := master.Exec(`INSERT INTO ProjectMetaData(ProjectID,StartDate,GeoRefMethod,ProjectTitle)
		VALUES ('ZMETATEST','2000-01-02 03:04:05',7,NULL)`); err != nil {
		t.Fatal(err)
	}
	return service, state
}

func MetadataTestCell(t *testing.T, table vprocore.ProjectMetadataTable, rowID, column string) vprocore.ProjectMetadataCell {
	t.Helper()
	for _, row := range table.Rows {
		if row.RowID == rowID {
			for i, field := range table.Columns {
				if field.Name == column {
					return row.Cells[i]
				}
			}
		}
	}
	t.Fatal("metadata row/column missing:", rowID, column)
	return vprocore.ProjectMetadataCell{}
}

func MetadataEditFixture(t *testing.T) (*vprocore.ContextService, vprocore.ProjectState, vprocore.ProjectMetadataEdit) {
	t.Helper()
	service, state := ProjectMetadataFixture(t)
	if err := service.Plots.SetAuditStrength(3); err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, "META1")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range review.ProjectRecords.Rows {
		if row.RowID == "-200" {
			return service, state, vprocore.ProjectMetadataEdit{PlotNumber: "META1", ProjectID: review.ProjectID,
				ID: -200, Columns: review.ProjectRecords.Columns, Original: row,
				Changes: []vprocore.ProjectMetadataChange{{Column: "Notes", Value: MetadataText("  Edited literal metadata  ")}}}
		}
	}
	t.Fatal("metadata fixture lacks its explicit physical record")
	return nil, vprocore.ProjectState{}, vprocore.ProjectMetadataEdit{}
}

func MetadataFileBytes(t *testing.T, service *vprocore.ContextService) map[string][]byte {
	t.Helper()
	return DatabaseBytes(t, service.Projects.Sqlite.Attachments)
}

func AssertMetadataFileBytes(t *testing.T, service *vprocore.ContextService, before map[string][]byte, except string) {
	t.Helper()
	for role, original := range before {
		if role == except || except != "" && os.SameFile(service.Projects.Sqlite.AttachmentInfo[role], service.Projects.Sqlite.AttachmentInfo[except]) {
			continue
		}
		current, err := os.ReadFile(service.Projects.Sqlite.Attachments[role])
		if err != nil || !bytes.Equal(current, original) {
			t.Fatal("metadata operation changed an unexpected file:", role, err)
		}
	}
}

func MetadataText(value string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
}

func MetadataInteger(value string) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func MetadataHistoryDatabase(t *testing.T, service *vprocore.ContextService) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.Sqlite.Attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func ReloadMetadataEdit(t *testing.T, service *vprocore.ContextService, state vprocore.ProjectState, request vprocore.ProjectMetadataEdit) vprocore.ProjectMetadataEdit {
	t.Helper()
	review, err := service.ReviewProjectMetadata(t.Context(), state.ContextID, request.PlotNumber)
	if err != nil {
		t.Fatal(err)
	}
	request.Columns = review.ProjectRecords.Columns
	for _, row := range review.ProjectRecords.Rows {
		if row.RowID == request.Original.RowID {
			request.Original = row
			return request
		}
	}
	t.Fatal("reviewed metadata physical row disappeared")
	return vprocore.ProjectMetadataEdit{}
}

func AssertProfileSUFiles(t *testing.T, service *vprocore.ContextService, files map[string][]byte) {
	t.Helper()
	for role, original := range files {
		data, err := os.ReadFile(service.Projects.Sqlite.Attachments[role])
		if err != nil || !bytes.Equal(data, original) {
			t.Fatal("SU operation changed an original database", role, err)
		}
	}
}

func QualitySet(h *vprocore.FS882Header, member string, value *string) {
	reflect.ValueOf(h).Elem().FieldByName(member).Set(reflect.ValueOf(value))
}

func QualityString(value string) *string { return &value }

func SiteCodeRestoreFixture(t *testing.T, member string, before, after *string) (*vprocore.PlotService, *sql.DB, string, string) {
	t.Helper()
	s, db := HeaderFixture(t)
	h := vprocore.FS882Header{PlotNumber: "CODERESTORE", FieldNotes: QualityString("Env after")}
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE Sample_Env SET `+vprocore.QuoteHeaderIdentifier(member)+`=? WHERE PlotNumber=?`, after, h.PlotNumber); err != nil {
		t.Fatal(err)
	}
	result, err := db.Exec(`INSERT INTO Sample_Audit
 ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag")
 VALUES ('Sample','RestoreTester',?,'_Env',?,'2026-09-30 12:00:00',?,?,0,0)`, h.PlotNumber, member, before, after)
	if err != nil {
		t.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	return s, db, h.PlotNumber, strconv.FormatInt(id, 10)
}

type SiviCreationJournalCancellation struct {
	context.Context
	Journal string             `json:"-" yaml:"-"`
	Cancel  context.CancelFunc `json:"-" yaml:"-"`
	once    sync.Once
	Staged  bool `json:"-" yaml:"-"`
}

func (ctx *SiviCreationJournalCancellation) Err() error {
	if info, err := os.Stat(ctx.Journal); err == nil && info.Size() > 0 {
		ctx.once.Do(func() {
			ctx.Staged = true
			ctx.Cancel()
		})
	}
	return ctx.Context.Err()
}

func SiviParentTables(t *testing.T) (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("resources", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "ro"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	plot := "108050"
	env, err := vprocore.ReadSQLiteStorageRows(context.Background(), db, "main", "Sample_Env", "PlotNumber", &plot, "")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := vprocore.ReadSQLiteStorageRows(context.Background(), db, "main", "Sample_Admin", "Plot", &plot, "")
	if err != nil || len(env.Rows) != 1 || len(admin.Rows) != 1 {
		t.Fatal("source fixture requires one physical Env/Admin pair", err)
	}
	return env, admin
}

func SiviSharedTestPointer[T any](value T) *T { return &value }

func SiviProjectionFixture() vprocore.ProjectMetadataTable {
	real := func(value float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	names := []string{"ID", "PlotNumber", "Species", "Cover1", "Cover2", "Cover3", "TotalA", "HeightA",
		"Cover4", "Cover5", "Cover5a", "Cover5b", "Cover5c", "TotalB", "HeightB", "Collected",
		"Cover6", "Height6", "Cover7", "Cover8", "Cover9"}
	table := vprocore.ProjectMetadataTable{}
	for _, name := range names {
		table.Columns = append(table.Columns, vprocore.ProjectMetadataColumn{Name: name})
	}
	for _, values := range []map[string]vprocore.ProjectMetadataCell{
		{"Cover1": real(0), "HeightA": real(2), "HeightB": MetadataText("")},
		{"Cover5a": real(-1), "HeightB": MetadataText("  historical words  ")},
		{"HeightA": real(5), "HeightB": MetadataText("height-only")},
		{"Cover6": real(0), "Height6": real(-3)},
		{"Height6": real(3)},
		{"Cover9": real(0)},
		{"Cover5c": real(120), "Cover6": real(2), "Cover7": real(-1), "HeightB": MetadataInteger("1")},
		{"PlotNumber": MetadataText("Other"), "Cover1": real(4)},
		{"PlotNumber": {Storage: "null"}, "Cover1": real(4)},
	} {
		row := vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(table.Rows) + 1)}
		for _, name := range names {
			cell := vprocore.ProjectMetadataCell{Storage: "null"}
			switch name {
			case "ID":
				cell = MetadataInteger("0")
			case "PlotNumber":
				cell = MetadataText("P")
			case "Species":
				cell = MetadataText("S")
			}
			if value, present := values[name]; present {
				cell = value
			}
			row.Cells = append(row.Cells, cell)
		}
		table.Rows = append(table.Rows, row)
	}
	return table
}

func SiviReal(value float64) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
}

func MutateContextFixture(t *testing.T, path, query string) {
	t.Helper()
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(query); err != nil {
		t.Fatal(err)
	}
}

func DatabaseBytes(t *testing.T, paths map[string]string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	for name, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		result[name] = data
	}
	return result
}

func SqliteServiceFixture(t *testing.T) (*vprocore.ProjectService, *vprocore.DesktopConfig, string, string) {
	t.Helper()
	data, config := ConfigFixture(t)
	preferences, err := vprocore.OpenDesktopConfig(data, config)
	if err != nil {
		t.Fatal(err)
	}

	service, err := vprocore.NewSQLiteProjectService(data, config, preferences)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.CloseSQLiteContext(); err != nil {
			t.Error(err)
		}
	})
	return service, preferences, data, config
}

func SubstrateSnapshot(t *testing.T, db *sql.DB) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	for _, table := range []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other", "Metadata", "Audit"} {
		snapshot[table] = HeightTableSnapshot(t, db, "Sample_"+table)
	}
	return snapshot
}

func AssertSubstrateSnapshot(t *testing.T, db *sql.DB, before map[string]string) {
	t.Helper()
	if !reflect.DeepEqual(before, SubstrateSnapshot(t, db)) {
		t.Fatal("refused mutation changed parent/children/metadata/history")
	}
}

func TableCSVPublicationOnly(t *testing.T, directory string, names ...string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	actual := make(map[string]bool)
	for _, entry := range entries {
		actual[entry.Name()] = true
	}
	expected := make(map[string]bool)
	for _, name := range names {
		expected[name] = true
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("directory contents = %v, want %v", actual, expected)
	}
}

func TableCSVPublicationRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type TableCSVLeaseContext struct {
	context.Context
	doneCalls atomic.Int32
	Queued    chan struct{} `json:"-" yaml:"-"`
}

func (ctx *TableCSVLeaseContext) Done() <-chan struct{} {

	if ctx.doneCalls.Add(1) == 5 {
		close(ctx.Queued)
	}
	return ctx.Context.Done()
}

func TwoPageCommonValues() map[string]vprocore.ProjectMetadataCell {
	return map[string]vprocore.ProjectMetadataCell{
		"SV_StandHeight": SiviReal(-3.25), "SV_AhorizonDepth": SiviReal(0.1),
		"SV_GleyingMottlingCM": SiviReal(25), "SV_PercentCoarseFrags": SiviReal(101),
		"SV_SoilDepth": SiviReal(0), "StrataCoverTotal": SiviReal(125),
		"SV_FloodPlain": MetadataInteger("-1"), "SV_StandAgeEstMeas": MetadataText("2"),
		"SV_StandHeightEstMeas": MetadataText("2"), "SV_PolygonNumber": MetadataText("  MiXeD  "),
		"SV_CanopyComposition": MetadataText("e\u0301 Canopy"), "SV_RootZoneTexture": MetadataText(""),
		"SV_AhorizonType": MetadataText(" Ah "), "PlotType": MetadataText("  Custom  "),
	}
}

func TwoPageWriteTables(t *testing.T, db *sql.DB) map[string]vprocore.ProjectMetadataTable {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	tables, err := vprocore.EnvironmentSiteUnitTables(context.Background(), tx)
	if err := errors.Join(err, tx.Rollback()); err != nil {
		t.Fatal(err)
	}
	return tables
}

func TwoPageSoilReadFixture(t *testing.T, external bool, affinity string) (*vprocore.ContextService, vprocore.ProjectState, *sql.DB) {
	t.Helper()
	service, state := ReportServiceFixture(t, external)
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.Sqlite.Attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	for _, kind := range []string{"Humus", "Mineral"} {
		if _, err := db.Exec(`DROP TABLE IF EXISTS "Sample_` + kind + `"`); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`CREATE TABLE "Sample_` + kind + `" (
			ID, PlotNumber ` + affinity + `, UpperDepth, Horizon, Comment, Unmapped)`); err != nil {
			t.Fatal(err)
		}
	}
	return service, state, db
}

func LifeformReferenceRow(code, scientific string, form vprocore.ProjectMetadataCell, english, kind vprocore.ProjectMetadataCell) []vprocore.ProjectMetadataCell {
	return []vprocore.ProjectMetadataCell{MetadataText(code), MetadataText(scientific), form, english, kind}
}

func CloneLifeformReferenceFixture(table vprocore.ProjectMetadataTable) vprocore.ProjectMetadataTable {
	result := vprocore.ProjectMetadataTable{Columns: append([]vprocore.ProjectMetadataColumn{}, table.Columns...), Rows: []vprocore.ProjectMetadataRow{}}
	for _, row := range table.Rows {
		cells := []vprocore.ProjectMetadataCell{}
		for _, cell := range row.Cells {
			cells = append(cells, vprocore.CloneSiteUnitCell(cell))
		}
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result
}

func VegetationReportFixture(t *testing.T) (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	t.Helper()
	source, err := os.ReadFile(filepath.Join("testdata", "long-vegetation-cover-columns.txt"))
	if err != nil {
		t.Fatal(err)
	}
	veg := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Species"}}}
	for _, field := range strings.Fields(string(source)) {
		veg.Columns = append(veg.Columns, vprocore.ProjectMetadataColumn{Name: field})
	}
	add := func(plot, species vprocore.ProjectMetadataCell, values map[string]vprocore.ProjectMetadataCell) {
		row := vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(veg.Rows) + 1), Cells: []vprocore.ProjectMetadataCell{plot, species}}
		for _, column := range veg.Columns[2:] {
			cell := vprocore.ProjectMetadataCell{Storage: "null"}
			if value, present := values[column.Name]; present {
				cell = value
			}
			row.Cells = append(row.Cells, cell)
		}
		veg.Rows = append(veg.Rows, row)
	}
	real := func(value float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	add(MetadataText("P1"), MetadataText("A"), map[string]vprocore.ProjectMetadataCell{
		"Cover1": MetadataInteger("-2"), "Cover2": MetadataInteger("0"), "Cover5a": real(2),
		"Cover8": real(8), "Cover9": real(9), "Cover10": real(10), "TotalA": real(111)})
	add(MetadataText("P1"), MetadataText("A"), map[string]vprocore.ProjectMetadataCell{
		"Cover1": real(-3), "Cover2": real(0), "Cover5a": real(4), "Cover5b": real(0), "Cover5c": real(-1), "TotalB": real(33)})
	add(MetadataText("P2"), MetadataText("A"), map[string]vprocore.ProjectMetadataCell{
		"Cover1": MetadataInteger("9007199254740993"), "Cover5c": real(120)})
	add(MetadataText("P2"), MetadataText("A"), map[string]vprocore.ProjectMetadataCell{
		"Cover1": real(9007199254740992), "Cover5a": real(-2)})
	add(MetadataText("OUT"), MetadataText("A"), map[string]vprocore.ProjectMetadataCell{"Cover1": MetadataText("bad outside scope")})
	add(MetadataText(""), MetadataText(""), map[string]vprocore.ProjectMetadataCell{"Cover7": real(-5)})
	add(MetadataText("P1"), vprocore.ProjectMetadataCell{Storage: "null"}, map[string]vprocore.ProjectMetadataCell{"Cover3": real(0)})
	add(vprocore.ProjectMetadataCell{Storage: "null"}, MetadataText("A"), map[string]vprocore.ProjectMetadataCell{"Cover1": MetadataText("no SQL NULL join")})
	add(MetadataText("P1"), MetadataText(""), map[string]vprocore.ProjectMetadataCell{"Cover3": real(1)})
	su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for _, values := range [][2]vprocore.ProjectMetadataCell{
		{MetadataText("P1"), MetadataText("U")}, {MetadataText("P1"), MetadataText("U")},
		{MetadataText("P1"), MetadataText("V")}, {MetadataText("P2"), {Storage: "null"}},
		{MetadataText("P3"), MetadataText("Orphan")}, {MetadataText(""), MetadataText("")},
		{{Storage: "null"}, MetadataText("Unassigned")}, {MetadataText("p1"), MetadataText("lowercase")},
	} {
		su.Rows = append(su.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(su.Rows) + 1), Cells: values[:]})
	}
	layers := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "LayerText"}}}
	for _, layer := range []string{"1", "2", "3", "4", "5", "5A", "5b", "5c", "6", "7", "8", "9", "10"} {
		layers.Rows = append(layers.Rows, vprocore.ProjectMetadataRow{RowID: fmt.Sprint(len(layers.Rows) + 1), Cells: []vprocore.ProjectMetadataCell{MetadataText(layer)}})
	}
	layers.Rows = append(layers.Rows, vprocore.ProjectMetadataRow{RowID: "14", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}}})
	return veg, su, layers
}

type VegetationCancelContext struct {
	context.Context
	Remaining int `json:"-" yaml:"-"`
}

func (ctx *VegetationCancelContext) Err() error {
	ctx.Remaining--
	if ctx.Remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func StrataReal(value float64) vprocore.ProjectMetadataCell {
	return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
}

func WorkingUnitServiceFixture(t *testing.T) (*vprocore.WorkingUnitService, *vprocore.ProjectService, *sql.DB) {
	t.Helper()
	root := BecTestDir(t)
	config := filepath.Join(root, "config")
	projects, err := vprocore.NewProjectServiceWithConfig(root, config)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", filepath.Join(root, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	service, err := vprocore.NewWorkingUnitService(projects, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := service.Close(); err != nil {
			t.Error(err)
		}
	})
	return service, projects, db
}
