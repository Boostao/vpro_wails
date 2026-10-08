package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func googleEarthReviewRequestJSON(t *testing.T, request GoogleEarthReviewRequest) string {
	t.Helper()
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestGoogleEarthReviewFeatureLiteralIndependentGate(t *testing.T) {
	for _, test := range []struct {
		value                     string
		present, enabled, invalid bool
	}{
		{"", false, false, false}, {"true", true, true, false}, {"false", true, false, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true},
		{" true ", true, false, true}, {"true\n", true, false, true},
	} {
		enabled, err := googleEarthReviewFeature(func(name string) (string, bool) {
			if name != "VPRO_GOOGLE_EARTH_REVIEW" {
				t.Fatal("unrelated feature lookup", name)
			}
			return test.value, test.present
		})
		if enabled != test.enabled || (err != nil) != test.invalid {
			t.Fatal(test, enabled, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, service := range []*GoogleEarthReviewService{nil, {}, NewGoogleEarthReviewService(nil, false), NewGoogleEarthReviewService(nil, true)} {
		if got, err := service.GetGoogleEarthReview(context.Background(), "id", googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"Zone", 0, 1})); got != nil || err == nil {
			t.Fatal("unavailable service returned review", got, err)
		}
		if got, err := service.GetGoogleEarthDescriptionFields(context.Background(), "id"); got != nil || err == nil {
			t.Fatal("unavailable service returned fields", got, err)
		}
		if got, err := service.GetGoogleEarthReview(ctx, "id", "{}"); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatal("disabled cancellation precedence lost", got, err)
		}
	}
}

func TestGoogleEarthReviewFacadeDirectEnvDynamicLiteralFieldsPagingAndDetachedDTO(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	mutateContextFixture(t, state.ProjectPath, `DELETE FROM Sample_Admin;
ALTER TABLE Sample_Env ADD COLUMN "Custom desc ] ; O'Brien" BLOB;
UPDATE Sample_Env SET "Custom desc ] ; O'Brien"=X'00ff' WHERE rowid%3=0;
UPDATE Sample_Env SET "Custom desc ] ; O'Brien"='' WHERE rowid%3=1;
UPDATE Sample_Env SET Longitude=-181,Latitude=91 WHERE rowid=(SELECT MIN(rowid) FROM Sample_Env WHERE Longitude IS NOT NULL AND Latitude IS NOT NULL)`)
	check := assertPlotLocationFacadeReadOnly(t, contexts)
	defer check()
	service := NewGoogleEarthReviewService(contexts, true)
	field := "Custom desc ] ; O'Brien"
	choices, err := service.GetGoogleEarthDescriptionFields(context.Background(), state.ContextID)
	if err != nil || choices == nil || choices.ContextID != state.ContextID || choices.Project != "Sample" ||
		choices.ProjectPath != state.ProjectPath || choices.SU != "None" || choices.SUPath != "" ||
		choices.EnvTable != "Sample_Env" || choices.Fields[len(choices.Fields)-1] != (ProjectMetadataColumn{field, "BLOB"}) {
		t.Fatal("dynamic physical fields changed", choices, err)
	}
	choices.Fields[len(choices.Fields)-1].Name = "caller"
	again, err := service.GetGoogleEarthDescriptionFields(context.Background(), state.ContextID)
	if err != nil || again.Fields[len(again.Fields)-1].Name != field {
		t.Fatal("field choices alias another snapshot", err)
	}
	want, err := contexts.readGoogleEarthLocations(context.Background(), state.ContextID, field)
	if err != nil || len(want.Report.Rows) < 2 {
		t.Fatal("accepted reader fixture unavailable", err)
	}
	for offset := 0; offset <= len(want.Report.Rows); offset++ {
		got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{field, offset, 1}))
		if err != nil || got == nil || got.TotalRows != len(want.Report.Rows) || got.Offset != offset || got.Limit != 1 ||
			got.ContextID != want.ContextID || got.ProjectPath != want.ProjectPath || got.SUPath != want.SUPath ||
			got.Project != want.Report.Project || got.SU != want.Report.SU || got.DescriptionField != field {
			t.Fatal("paging or snapshot provenance differs", got, err)
		}
		expectedFields := []EnvironmentReportField{
			{"Env", "PlotNumber", "Plot Number", false}, {"Env", "Longitude", "Longitude", false},
			{"Env", "Latitude", "Latitude", false}, {"Env", field, field, false},
		}
		if !reflect.DeepEqual(got.Fields, expectedFields) {
			t.Fatal("exact field labels or keys changed", got.Fields)
		}
		if offset == len(want.Report.Rows) {
			if got.Rows == nil || len(got.Rows) != 0 {
				t.Fatal("end page is not an explicit empty array", got.Rows)
			}
			continue
		}
		row := want.Report.Rows[offset]
		expected := GoogleEarthReviewRow{row.EnvRowID, row.MembershipRowID, row.PlotNumber,
			row.StoredLongitude, row.Longitude, row.Latitude, row.Description}
		if len(got.Rows) != 1 || !reflect.DeepEqual(got.Rows[0], expected) {
			t.Fatal("raw physical cells changed", got.Rows, expected)
		}
		raw, err := json.Marshal(got)
		var roundTrip GoogleEarthReview
		if err != nil || json.Unmarshal(raw, &roundTrip) != nil || !reflect.DeepEqual(*got, roundTrip) {
			t.Fatal("public tagged transport roundtrip differs", err)
		}
	}
	got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"PlotNumber", 0, 500}))
	if err != nil || got.Fields[3].Key != "PlotNumber" || got.Fields[0].Key != "PlotNumber" {
		t.Fatal("exact duplicate projection field lost", err)
	}
	*got.Rows[0].Description.Text = "caller"
	if *got.Rows[0].PlotNumber.Text == "caller" {
		t.Fatal("name and description share mutable storage")
	}
	next, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"PlotNumber", 0, 500}))
	if err != nil || *next.Rows[0].Description.Text == "caller" {
		t.Fatal("detached review changed future read", err)
	}
	for _, offset := range []int{len(want.Report.Rows) + 1, math.MaxInt} {
		got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{field, offset, 500}))
		if err != nil || len(got.Rows) != 0 {
			t.Fatal("beyond-end or extreme paging overflow", got, err)
		}
	}
	disabled := NewGoogleEarthReviewService(contexts, false)
	contexts.plotLocationReviewEnabled, contexts.tableCSVReviewEnabled, contexts.siviParentReviewEnabled = true, true, true
	if got, err := disabled.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{field, 0, 1})); got != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("other flags enabled Earth review", got, err)
	}
}

func TestGoogleEarthReviewFacadeExternalSUFanoutNullEmptyAndBlob(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	mutateContextFixture(t, state.ProjectPath, `UPDATE Sample_Env SET Latitude=NULL;
UPDATE Sample_Env SET Latitude=99,Longitude=-200 WHERE rowid IN (SELECT rowid FROM Sample_Env ORDER BY rowid LIMIT 3);
ALTER TABLE Sample_Env ADD COLUMN "Earth description";
UPDATE Sample_Env SET "Earth description"='' WHERE rowid=(SELECT rowid FROM Sample_Env ORDER BY rowid LIMIT 1 OFFSET 1);
UPDATE Sample_Env SET "Earth description"=X'ff00' WHERE rowid=(SELECT rowid FROM Sample_Env ORDER BY rowid LIMIT 1 OFFSET 2)`)
	path := filepath.Join(filepath.Dir(state.ProjectPath), "Earth O'Brien #.db")
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ATTACH DATABASE ? AS source; CREATE TABLE Earth_SU(PlotNumber VARCHAR,SiteUnit VARCHAR);
INSERT INTO Earth_SU SELECT PlotNumber,NULL FROM source.Sample_Env WHERE Latitude IS NOT NULL;
INSERT INTO Earth_SU SELECT PlotNumber,'' FROM source.Sample_Env WHERE Latitude IS NOT NULL`, state.ProjectPath)
	if err := errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	next, err := contexts.SwitchContext(state.ContextID, ContextSelection{Project: state.ActiveProject, ProjectPath: state.ProjectPath,
		SU: "Earth", SUPath: path, Hierarchy: "None"})
	if err != nil {
		t.Fatal(err)
	}
	check := assertPlotLocationFacadeReadOnly(t, contexts)
	defer check()
	service := NewGoogleEarthReviewService(contexts, true)
	got, err := service.GetGoogleEarthReview(context.Background(), next.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"Earth description", 0, 500}))
	if err != nil || got == nil || len(got.Rows) != 6 || got.SU != "Earth" || got.SUPath != next.SUPath {
		t.Fatal("external SU fanout or provenance lost", got, err)
	}
	storages := map[string]int{}
	for _, row := range got.Rows {
		storages[row.Description.Storage]++
		longitude, longitudeErr := metadataCellValue(row.Longitude)
		stored, storedErr := metadataCellValue(row.StoredLongitude)
		latitude, latitudeErr := metadataCellValue(row.Latitude)
		if row.MembershipRowID == "" || row.EnvRowID == "" || longitudeErr != nil || storedErr != nil ||
			latitudeErr != nil || longitude != float64(200) || stored != float64(-200) || latitude != float64(99) {
			t.Fatal("historical numeric coordinates or physical IDs changed", row)
		}
	}
	if !reflect.DeepEqual(storages, map[string]int{"null": 2, "text": 2, "blob": 2}) {
		t.Fatal("raw description storage collapsed", storages)
	}
	choices, err := service.GetGoogleEarthDescriptionFields(context.Background(), next.ContextID)
	if err != nil || choices.SU != "Earth" || choices.SUPath != next.SUPath {
		t.Fatal("field snapshot lost SU selection", err)
	}
}

func TestGoogleEarthReviewFacadeRejectsInvalidRequestsAndTransport(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	service := NewGoogleEarthReviewService(contexts, true)
	check := assertPlotLocationFacadeReadOnly(t, contexts)
	defer check()
	for _, request := range []GoogleEarthReviewRequest{
		{"Zone", -1, 1}, {"Zone", 0, 0}, {"Zone", 0, 501}, {"", 0, 1},
		{"zone", 0, 1}, {"Zone;DROP TABLE Sample_Env", 0, 1}, {"Zone\x00", 0, 1}, {string([]byte{255}), 0, 1},
	} {
		if got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, request)); got != nil || err == nil {
			t.Fatal("invalid request returned partial review", request, got, err)
		}
	}
	for _, raw := range []string{
		`{"descriptionField":"\ud800","offset":0,"limit":1}`,
		"{\"descriptionField\":\"\xff\",\"offset\":0,\"limit\":1}",
		`{"descriptionField":"Zone","offset":0}`, `{"descriptionField":null,"offset":0,"limit":1}`,
		`{"descriptionField":"Zone","offset":0,"limit":1,"title":"not a field"}`,
		`{"descriptionField":"Zone","offset":0,"limit":1.5}`,
	} {
		var request GoogleEarthReviewRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("malformed raw transport repaired", raw)
		}
		if got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, raw); got != nil || err == nil {
			t.Fatal("public string-JSON boundary repaired malformed transport", got, err)
		}
	}
	var valid GoogleEarthReviewRequest
	if err := json.Unmarshal([]byte(`{"descriptionField":"Zone","offset":0,"limit":1}`), &valid); err != nil {
		t.Fatal(err)
	}
	source, err := contexts.readGoogleEarthLocations(context.Background(), state.ContextID, "Zone")
	if err != nil {
		t.Fatal(err)
	}
	source.ProjectPath = string([]byte{255})
	if got, err := googleEarthReviewTransport(context.Background(), source, valid); got != nil || err == nil {
		t.Fatal("DTO path repaired during transport", got, err)
	}
	source.ProjectPath = state.ProjectPath
	bad := string([]byte{255})
	source.Report.Rows[len(source.Report.Rows)-1].Description = ProjectMetadataCell{Storage: "text", Text: &bad}
	if got, err := googleEarthReviewTransport(context.Background(), source, valid); got != nil || err == nil {
		t.Fatal("malformed off-page cell returned partial DTO", got, err)
	}
}

func TestGoogleEarthReviewFacadeStaleOwnershipPhysicalScopeCancellationRetry(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	service := NewGoogleEarthReviewService(contexts, true)
	methods := []func(context.Context, string) (bool, error){
		func(ctx context.Context, id string) (bool, error) {
			value, err := service.GetGoogleEarthReview(ctx, id, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"Zone", 0, 1}))
			return value != nil, err
		},
		func(ctx context.Context, id string) (bool, error) {
			value, err := service.GetGoogleEarthDescriptionFields(ctx, id)
			return value != nil, err
		},
	}
	check := assertPlotLocationFacadeReadOnly(t, contexts)
	defer check()
	owner := contexts.projects.sqlite
	for _, method := range methods {
		if got, err := method(context.Background(), "stale"); got || err == nil {
			t.Fatal("stale context returned DTO", got, err)
		}
		prior := owner.attachmentInfo["project"]
		owner.attachmentInfo["project"] = owner.attachmentInfo["VPro64"]
		got, err := method(context.Background(), state.ContextID)
		owner.attachmentInfo["project"] = prior
		if got || err == nil {
			t.Fatal("unowned physical source returned DTO", got, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := method(ctx, state.ContextID); got || !errors.Is(err, context.Canceled) {
			t.Fatal("initial cancellation lost", got, err)
		}
		ctx, cancel = context.WithCancel(context.Background())
		queued := &tableCSVLeaseContext{Context: ctx, queued: make(chan struct{})}
		owner.mu.Lock()
		done := make(chan error, 1)
		go func() {
			got, err := method(queued, state.ContextID)
			if got {
				err = errors.Join(err, errors.New("cancelled read returned partial result"))
			}
			done <- err
		}()
		select {
		case <-queued.queued:
		case <-time.After(5 * time.Second):
			owner.mu.Unlock()
			cancel()
			t.Fatal("request did not reach snapshot lease")
		}
		cancel()
		select {
		case err := <-done:
			owner.mu.Unlock()
			if !errors.Is(err, context.Canceled) {
				t.Fatal("queued cancellation lost", err)
			}
		case <-time.After(5 * time.Second):
			owner.mu.Unlock()
			t.Fatal("cancelled request retained lease")
		}
		if got, err := method(context.Background(), state.ContextID); !got || err != nil {
			t.Fatal("retry lost pinned context", got, err)
		}
	}
}

func TestGoogleEarthDescriptionFieldsDoesNotNestOperationLease(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	service := NewGoogleEarthReviewService(contexts, true)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	queued := &tableCSVLeaseContext{Context: ctx, queued: make(chan struct{})}
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	done := make(chan error, 1)
	go func() {
		_, err := service.GetGoogleEarthDescriptionFields(queued, state.ContextID)
		done <- err
	}()
	select {
	case <-queued.queued:
	case <-ctx.Done():
		owner.mu.Unlock()
		t.Fatal("field request did not acquire operation lease")
	}
	writer := make(chan struct{})
	go func() {
		contexts.projects.operationMu.Lock()
		close(writer)
		contexts.projects.operationMu.Unlock()
	}()
	// A writer queued behind the field request must not prevent its snapshot completing.
	time.Sleep(20 * time.Millisecond)
	owner.mu.Unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal("field choices nested lease blocked behind writer", err)
		}
	case <-ctx.Done():
		t.Fatal("field choices deadlocked with queued context writer")
	}
	select {
	case <-writer:
	case <-ctx.Done():
		t.Fatal("field choices did not release operation lease")
	}
}

func TestGoogleEarthReviewFacadeRejectsPhysicalViewAndMalformedHistoricalText(t *testing.T) {
	for _, mutation := range []string{
		`ALTER TABLE Sample_Env RENAME TO Hidden_Env; CREATE VIEW Sample_Env AS SELECT * FROM Hidden_Env`,
		`UPDATE Sample_Env SET Zone=CAST(X'ff' AS TEXT) WHERE rowid=(SELECT MIN(rowid) FROM Sample_Env)`,
	} {
		contexts, state := contextServiceFixture(t)
		mutateContextFixture(t, state.ProjectPath, mutation)
		check := assertPlotLocationFacadeReadOnly(t, contexts)
		service := NewGoogleEarthReviewService(contexts, true)
		if got, err := service.GetGoogleEarthReview(context.Background(), state.ContextID, googleEarthReviewRequestJSON(t, GoogleEarthReviewRequest{"Zone", 0, 1})); got != nil || err == nil {
			t.Fatal("physical impostor/malformed text returned review", got, err)
		}
		if got, err := service.GetGoogleEarthDescriptionFields(context.Background(), state.ContextID); got != nil || err == nil {
			t.Fatal("physical impostor/malformed snapshot returned fields", got, err)
		}
		check()
	}
}
