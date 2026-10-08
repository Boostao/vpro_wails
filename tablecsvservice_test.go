package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestTableCSVReviewFeatureRequiresIndependentLiteralOptIn(t *testing.T) {
	for _, test := range []struct {
		name, value               string
		present, enabled, invalid bool
	}{
		{"unset", "", false, false, false},
		{"true", "true", true, true, false},
		{"false", "false", true, false, false},
		{"empty", "", true, false, true},
		{"uppercase", "TRUE", true, false, true},
		{"number", "1", true, false, true},
		{"padded", " true ", true, false, true},
		{"newline", "true\n", true, false, true},
		{"negative", "False", true, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			enabled, err := tableCSVReviewFeature(func(name string) (string, bool) {
				calls++
				if name != "VPRO_TABLE_CSV_REVIEW" {
					t.Fatal("wrong independent feature lookup:", name)
				}
				return test.value, test.present
			})
			if calls != 1 || enabled != test.enabled || (err != nil) != test.invalid {
				t.Fatal("literal opt-in changed:", test, enabled, err, calls)
			}
			if err != nil && !strings.Contains(err.Error(), "VPRO_TABLE_CSV_REVIEW") {
				t.Fatal("malformed feature error lost its setting identity:", err)
			}
		})
	}
	enabled, err := tableCSVReviewFeature(func(name string) (string, bool) {
		if name == "VPRO_TABLE_CSV_REVIEW" {
			return "", false
		}
		return "true", true
	})
	if enabled || err != nil {
		t.Fatal("unrelated opt-ins enabled table CSV review:", err)
	}
}

func assertTableCSVFacadeReadOnly(t *testing.T, service *ContextService) func() {
	t.Helper()
	before := databaseBytes(t, service.projects.sqlite.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	return func() {
		t.Helper()
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !reflect.DeepEqual(config, after) {
			t.Fatal("table CSV facade changed configuration:", err)
		}
	}
}

func TestTableCSVReviewFacadeDefaultOffDoesNotInheritSIVI(t *testing.T) {
	service, state := contextServiceFixture(t)
	check := assertTableCSVFacadeReadOnly(t, service)
	defer check()
	for _, flag := range []*bool{
		&service.siviHeightEnabled, &service.siviParentReviewEnabled,
		&service.siviParentEditingEnabled, &service.siviParentActionEditingEnabled,
		&service.siviProjectAssignmentEnabled,
	} {
		*flag = true
		if result, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other"); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
			t.Fatal("SIVI opt-in enabled table CSV review:", result, err)
		}
	}
	service.tableCSVReviewEnabled = false
	if result, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other"); result != nil || err == nil {
		t.Fatal("explicit false enabled table CSV review:", result, err)
	}
}

func TestTableCSVReviewFacadeParityAndIndependentTypedDTO(t *testing.T) {
	service, state := contextServiceFixture(t)
	tableCSVProjectWriter(t, service, `DROP TABLE _table_metadata;
		CREATE TABLE _table_metadata(table_name TEXT,description TEXT);
		INSERT INTO _table_metadata VALUES
		('Sample_Other',NULL),('Sample_Other',''),('Sample_Other',''),
		('Sample_Other','  literal café description  ');`)
	check := assertTableCSVFacadeReadOnly(t, service)
	defer check()
	service.tableCSVReviewEnabled = true
	private, err := service.readProjectTableCSV(context.Background(), state.ContextID, "Sample_Other")
	if err != nil {
		t.Fatal(err)
	}
	want := &ProjectTableCSVReview{
		ContextID: private.ContextID, Project: private.Project, ProjectPath: private.ProjectPath,
		DescriptionMetadataPresent: private.DescriptionMetadataPresent,
		Manifest:                   private.Document.Manifest, CSV: string(private.Document.Data),
	}
	got, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other")
	if err != nil || !reflect.DeepEqual(got, want) || !utf8.ValidString(got.CSV) {
		t.Fatal("public facade changed the private physical review:", err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	if !reflect.DeepEqual(keys, []string{"contextId", "csv", "descriptionMetadataPresent", "manifest", "project", "projectPath"}) {
		t.Fatal("public JSON contract changed:", keys)
	}
	decoded, err := decodeTableCSV(context.Background(), tableCSVDocument{Manifest: got.Manifest, Data: []byte(got.CSV)})
	privateDecoded, privateErr := decodeTableCSV(context.Background(), private.Document)
	if err != nil || privateErr != nil || !reflect.DeepEqual(decoded, privateDecoded) {
		t.Fatal("public CSV failed exact private codec parity:", err, privateErr)
	}
	got.Manifest.Columns[0].Name = "caller column"
	got.Manifest.RowIDs[0] = "caller row"
	got.Manifest.Storage[0][0] = "caller storage"
	*got.Manifest.Descriptions[3].Value.Text = "caller description"
	got.Manifest.Descriptions[0].RowID = "caller identity"
	got.CSV = "caller CSV"
	again, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other")
	if err != nil || !reflect.DeepEqual(again, want) {
		t.Fatal("public review shared mutable source or caller ownership:", err)
	}
	if result, err := service.GetSIVIParentOriginal(context.Background(), state.ContextID, "108050"); result != nil || err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("table CSV opt-in enabled the SIVI review:", err)
	}
}

func TestTableCSVReviewFacadeRejectsStaleForeignDerivedAndCancelledWithNil(t *testing.T) {
	service, state := contextServiceFixture(t)
	service.tableCSVReviewEnabled = true
	check := assertTableCSVFacadeReadOnly(t, service)
	for _, test := range []struct{ contextID, table string }{
		{"stale", "Sample_Other"}, {"", "Sample_Other"},
		{state.ContextID, ""}, {state.ContextID, "sample_Other"},
		{state.ContextID, "OtherProject_Other"}, {state.ContextID, "_table_metadata"},
		{state.ContextID, "Sample_Profile"}, {state.ContextID, "VLists.USysAllSpecs"},
		{state.ContextID, "Sample_Other; DELETE FROM Sample_Env"},
	} {
		if result, err := service.GetProjectTableCSVReview(context.Background(), test.contextID, test.table); result != nil || err == nil {
			t.Fatal("unowned request returned a public document:", test, result, err)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.GetProjectTableCSVReview(cancelled, state.ContextID, "Sample_Other"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request returned data or lost context error:", result, err)
	}
	owner := service.projects.sqlite
	owner.mu.Lock()
	waiting, cancelWaiting := context.WithCancel(context.Background())
	defer cancelWaiting()
	observed := &tableCSVLeaseContext{Context: waiting, queued: make(chan struct{})}
	done := make(chan error, 1)
	go func() {
		result, err := service.GetProjectTableCSVReview(observed, state.ContextID, "Sample_Other")
		if result != nil {
			done <- errors.New("queued cancellation returned a public document")
			return
		}
		done <- err
	}()
	select {
	case <-observed.queued:
		cancelWaiting()
	case <-time.After(3 * time.Second):
		owner.mu.Unlock()
		t.Fatal("facade never reached the held snapshot mutex")
	}
	select {
	case err := <-done:
		owner.mu.Unlock()
		if !errors.Is(err, context.Canceled) {
			t.Fatal("queued public cancellation lost context error:", err)
		}
	case <-time.After(3 * time.Second):
		owner.mu.Unlock()
		t.Fatal("cancelled facade remained blocked at the snapshot mutex")
	}
	if result, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other"); result == nil || err != nil {
		t.Fatal("public retry failed after cancellation:", err)
	}
	check()
	tableCSVProjectWriter(t, service, `ALTER TABLE Sample_Other RENAME TO Archived_Other;
		CREATE VIEW Sample_Other AS SELECT * FROM Archived_Other;`)
	check = assertTableCSVFacadeReadOnly(t, service)
	defer check()
	if result, err := service.GetProjectTableCSVReview(context.Background(), state.ContextID, "Sample_Other"); result != nil || err == nil || !strings.Contains(err.Error(), "physical table") {
		t.Fatal("derived view impersonated a public physical table:", result, err)
	}
}
