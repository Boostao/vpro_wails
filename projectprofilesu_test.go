package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func profileSUFixture(t *testing.T) (*ContextService, ProjectState, ProfileSUCreation) {
	t.Helper()
	service, state, input := profileRunFixture(t)
	result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, input)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfileSU(context.Background(), state.ContextID, ProjectPlotProfileFilterRequest{input, result})
	if err != nil {
		t.Fatal(err)
	}
	return service, state, ProfileSUCreation{review, "ReviewedSU", filepath.Join(t.TempDir(), "SU O'Brien #.db"), true}
}

func assertProfileSUFiles(t *testing.T, service *ContextService, files map[string][]byte) {
	t.Helper()
	for role, original := range files {
		data, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(data, original) {
			t.Fatal("SU operation changed an original database", role, err)
		}
	}
}

func TestProfileSUCreatesExactSeparateFileWithoutContextOrOriginalWrites(t *testing.T) {
	service, state, request := profileSUFixture(t)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	if request.Review.Descriptions != nil || request.Review.SourceSU != nil || len(request.Review.Plots) != 11 {
		t.Fatal("absent descriptions/SU inferred or canonical11 rows lost", request.Review)
	}
	transport, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProfileSUCreation
	if err := json.Unmarshal(transport, &decoded); err != nil {
		t.Fatal("actual explicit-null transport failed", err)
	}
	created, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, decoded)
	resolved, resolveErr := existingDatabasePath(request.Path)
	if err != nil || resolveErr != nil || created.Name != request.Name || created.Path != resolved || created.PlotCount != 11 {
		t.Fatal("explicit fresh-file creation failed", created, err)
	}
	assertProfileSUFiles(t, service, files)
	after, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, after) || service.projects.contextID != state.ContextID {
		t.Fatal("SU creation implicitly changed configuration/context")
	}
	db, err := openReadOnlyContext(context.Background(), created.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	table, err := readSQLiteStorageRows(context.Background(), db, "main", "ReviewedSU_SU", "", nil, "PlotNumber")
	if err != nil || len(table.Rows) != 11 {
		t.Fatal("stored SU row count", table, err)
	}
	for index, row := range table.Rows {
		if row.Cells[0].Text == nil || *row.Cells[0].Text != request.Review.Plots[index].PlotNumber || row.Cells[1].Storage != "null" {
			t.Fatal("literal membership/NULL SiteUnit changed", row)
		}
	}
	var metadata, history int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='_table_metadata'`).Scan(&metadata); err != nil || metadata != 0 {
		t.Fatal("missing native descriptions synthesized", metadata, err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM __VPRO_ProfileSUHistory`).Scan(&history); err != nil || history != 1 {
		t.Fatal("technical history missing", history, err)
	}
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, decoded); err == nil {
		t.Fatal("completed creation replayed over existing output")
	}
	next, err := service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: created.Name, SUPath: created.Path, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil || next.ActiveSU != created.Name {
		t.Fatal("explicitly saved original SU cannot be attached by existing coordinator", next, err)
	}
	entries, err := service.projects.ListPlots(context.Background(), 0, 100)
	if err != nil || len(entries.Plots) != 11 {
		t.Fatal("new SU failed exact stored project membership", entries, err)
	}
}

func TestProfileSUSelectedSiteUnitsPreserveNullEmptyAndLiteralValues(t *testing.T) {
	service, state, input := profileRunFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Picked_SU(PlotNumber TEXT,SiteUnit TEXT);
		INSERT INTO Picked_SU VALUES('108050',NULL),('108050x',''),('8229723',' U'' # ')`)
	release()
	if err != nil {
		t.Fatal(err)
	}
	state, err = service.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Picked", SUPath: state.ProjectPath, Hierarchy: state.ActiveHierarchy, HierarchyPath: state.HierarchyPath})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, input)
	if err != nil || len(result.PlotNumbers) != 3 {
		t.Fatal(result, err)
	}
	review, err := service.ReviewProjectPlotProfileSU(context.Background(), state.ContextID, ProjectPlotProfileFilterRequest{input, result})
	if err != nil || review.SourceSU == nil || len(review.Plots) != 3 || review.Plots[0].SiteUnit != nil ||
		review.Plots[1].SiteUnit == nil || *review.Plots[1].SiteUnit != "" ||
		review.Plots[2].SiteUnit == nil || *review.Plots[2].SiteUnit != " U' # " {
		t.Fatal("source SU SiteUnit semantics were repaired", review, err)
	}
	request := ProfileSUCreation{review, "SelectedCopy", filepath.Join(t.TempDir(), "selected.db"), true}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, service, files)
	writer, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.Exec(`UPDATE Picked_SU SET SiteUnit='changed' WHERE PlotNumber='108050x'`)
	release()
	if err != nil {
		t.Fatal(err)
	}
	request.Path = filepath.Join(t.TempDir(), "stale-siteunit.db")
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, request); err == nil ||
		!strings.Contains(err.Error(), "SiteUnit") {
		t.Fatal("changed selected SU escaped independently reviewed snapshot", err)
	}
	if _, err := os.Stat(request.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("rejected snapshot published a file")
	}
}

func TestProfileSURejectsMalformedUnconfirmedStaleAndCollidingCreation(t *testing.T) {
	service, state, request := profileSUFixture(t)
	files := databaseBytes(t, service.projects.sqlite.attachments)
	for _, name := range []string{"", "None", "Sample", "MasterSet", "1Bad", " bad ", strings.Repeat("A", 32)} {
		bad := request
		bad.Name = name
		if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, bad); err == nil {
			t.Fatal("reserved/incompatible name accepted", name)
		}
	}
	bad := request
	bad.Confirmed = false
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, bad); err == nil {
		t.Fatal("unconfirmed creation allocated")
	}
	for _, path := range []string{"relative.db", state.ProjectPath, filepath.Join(t.TempDir(), "missing", "out.db")} {
		bad := request
		bad.Path = path
		if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, bad); err == nil {
			t.Fatal("unowned/colliding destination accepted", path)
		}
	}
	bad = request
	bad.Review.Plots = nil
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, bad); err == nil {
		t.Fatal("forged destination rows accepted")
	}
	for _, id := range []string{"", "stale"} {
		if _, err := service.SaveProjectPlotProfileSU(context.Background(), id, request); err == nil {
			t.Fatal("foreign editor allocated SU")
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.SaveProjectPlotProfileSU(cancelled, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled creation published", err)
	}
	assertProfileSUFiles(t, service, files)
	paths, err := os.ReadDir(filepath.Dir(request.Path))
	if err != nil || len(paths) != 0 {
		t.Fatal("rejected creation left temporary or published files", paths, err)
	}
}

func TestProfileSUStrictTransportAndUtf16Bounds(t *testing.T) {
	for _, raw := range []string{
		`{}`, `{"review":null,"name":"SU","path":"C:\\x.db","confirmed":true}`,
		`{"review":{},"name":"SU","path":"C:\\x.db","confirmed":null}`,
		`{"review":{"filter":{},"template":{},"plots":[]},"name":"SU","path":"C:\\x.db","confirmed":true}`,
		`{"review":{},"name":"\ud800","path":"C:\\x.db","confirmed":true}`,
		`{"review":{},"name":"SU","path":"C:\\x.db","confirmed":true,"unknown":1}`,
	} {
		var request ProfileSUCreation
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("incomplete/repaired transport accepted", raw)
		}
	}
	for _, limit := range []int{7, 255} {
		if err := validateProfileSUText(strings.Repeat("A", limit), limit); err != nil {
			t.Fatal(err)
		}
		for _, raw := range []string{strings.Repeat("A", limit+1), strings.Repeat("\U0001f332", (limit+1)/2), "x\x00", string([]byte{255})} {
			if err := validateProfileSUText(raw, limit); err == nil {
				t.Fatal("new overlength/malformed text repaired", limit, raw)
			}
		}
	}
}

func TestProfileSUDescriptionNullEmptyDuplicatesAndExtraStoragePreserved(t *testing.T) {
	service, state, input := profileRunFixture(t)
	db, err := sql.Open("sqlite3", sqliteFileURI(service.projects.sqlite.attachments["VPro64"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE _table_metadata(table_name TEXT,description TEXT,extra BLOB);
		INSERT INTO _table_metadata VALUES('USysSuTable',NULL,x'00'),('USysSuTable','',x'ff'),('USysSuTable','VP05-2',NULL)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	result, err := service.RunProjectPlotProfile(context.Background(), state.ContextID, input)
	if err != nil {
		t.Fatal(err)
	}
	review, err := service.ReviewProjectPlotProfileSU(context.Background(), state.ContextID, ProjectPlotProfileFilterRequest{input, result})
	if err != nil || review.Descriptions == nil || len(review.Descriptions.Rows) != 3 {
		t.Fatal(review, err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	request := ProfileSUCreation{review, "Descriptions", filepath.Join(t.TempDir(), "metadata.db"), true}
	if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, request); err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, service, files)
	output, err := openReadOnlyContext(context.Background(), request.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	descriptions, err := readSQLiteStorageRows(context.Background(), output, "main", "_table_metadata", "", nil, "")
	if err != nil || len(descriptions.Rows) != 3 {
		t.Fatal(descriptions, err)
	}
	for index, row := range descriptions.Rows {
		if row.Cells[0].Text == nil || *row.Cells[0].Text != "Descriptions_SU" ||
			!reflect.DeepEqual(row.Cells[1:], review.Descriptions.Rows[index].Cells[1:]) {
			t.Fatal("description storage normalized or extra metadata lost", row)
		}
	}
}

type profileSUStagedContext struct {
	context.Context
	directory string
	once      sync.Once
	action    func()
}

func (ctx *profileSUStagedContext) Err() error {
	files, err := os.ReadDir(ctx.directory)
	if err == nil {
		for _, file := range files {
			if !strings.HasPrefix(file.Name(), ".vpro-profile-su-") {
				continue
			}
			info, err := file.Info()
			if err == nil && info.Size() > 0 {
				ctx.once.Do(ctx.action)
			}
		}
	}
	return ctx.Context.Err()
}

func TestProfileSULateCancellationAndPublicationCollisionRemoveStagedFiles(t *testing.T) {
	for _, cancelLate := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancellation", false: "collision"}[cancelLate], func(t *testing.T) {
			service, state, request := profileSUFixture(t)
			files := databaseBytes(t, service.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			foreign := []byte("Independently created destination must remain untouched")
			var actionErr error
			hooked := &profileSUStagedContext{Context: ctx, directory: filepath.Dir(request.Path)}
			hooked.action = func() {
				if cancelLate {
					cancel()
				} else {
					actionErr = os.WriteFile(request.Path, foreign, 0600)
				}
			}
			_, err := service.SaveProjectPlotProfileSU(hooked, state.ContextID, request)
			if err == nil || actionErr != nil {
				t.Fatal("late cancellation/collision published success", err, actionErr)
			}
			if cancelLate && !errors.Is(err, context.Canceled) {
				t.Fatal("late cancellation lost its cause", err)
			}
			remaining, readErr := os.ReadDir(filepath.Dir(request.Path))
			expectedCount := 0
			if !cancelLate {
				expectedCount = 1
				data, err := os.ReadFile(request.Path)
				if err != nil || !bytes.Equal(data, foreign) {
					t.Fatal("publication replaced independent colliding bytes", err)
				}
			}
			if readErr != nil || len(remaining) != expectedCount {
				t.Fatal("staging/rollback journal artifacts remained", remaining, readErr)
			}
			assertProfileSUFiles(t, service, files)
			retry := filepath.Join(filepath.Dir(request.Path), "retry.db")
			request.Path = retry
			if _, err := service.SaveProjectPlotProfileSU(context.Background(), state.ContextID, request); err != nil {
				t.Fatal("retained proposal could not retry", err)
			}
		})
	}
}
