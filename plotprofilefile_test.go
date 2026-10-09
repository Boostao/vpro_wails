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
	"testing"
)

func profileFileFixture(t *testing.T) (*ContextService, ProjectState, PlotProfileFileCreation) {
	t.Helper()
	service, state, _ := profileRunFixture(t)
	review, err := service.ReviewPlotProfileFileCreation(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	return service, state, PlotProfileFileCreation{Review: review, Name: "Blank_Profile", Path: filepath.Join(t.TempDir(), "new profile O'Brien #.db"), Confirmed: true}
}

func TestProfileFileCreationPreservesOriginalsAndIsUsableWithoutDescriptions(t *testing.T) {
	service, state, request := profileFileFixture(t)
	ctx := context.Background()
	owner := service.projects.sqlite
	files := databaseBytes(t, owner.attachments)
	config, err := os.ReadFile(service.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CreatePlotProfileFile(ctx, state.ContextID, request)
	if err != nil || result.Source.Name != request.Name || result.Table != request.Name+"_Profile" || result.RuleCount != 0 {
		t.Fatal("reviewed original empty profile not published", result, err)
	}
	if service.projects.sqlite != owner || service.projects.contextID != state.ContextID {
		t.Fatal("new-file publication implicitly selected/authorized the new profile")
	}
	currentConfig, err := os.ReadFile(service.projects.preferences.path)
	if err != nil || !bytes.Equal(config, currentConfig) {
		t.Fatal("new-file publication altered persistent YAML", err)
	}
	for role, original := range files {
		current, err := os.ReadFile(owner.attachments[role])
		if err != nil || !bytes.Equal(original, current) {
			t.Fatal("new-file publication wrote original project/support data", role, err)
		}
	}
	found, err := service.ListPlotProfileSources(ctx, state.ContextID, result.Source.Path)
	if err != nil || len(found) != 1 || !found[0].Available || found[0].Writable || found[0].Source != result.Source {
		t.Fatal("published empty profile cannot be independently discovered", found, err)
	}
	selected, err := service.SelectPlotProfile(state.ContextID, result.Source)
	if err != nil || selected.PlotProfile.Writable {
		t.Fatal("published file cannot be explicitly selected without implied grants", selected, err)
	}
	review, err := service.ReviewProjectPlotProfile(ctx, selected.ContextID)
	if err != nil || !reflect.DeepEqual(review.Rules, request.Review.Template) || len(review.Descriptions.Columns) != 0 {
		t.Fatal("published template/absent metadata cannot be reviewed", review, err)
	}
	granted := authorizeProfileFixture(t, service, selected, true)
	created, err := service.CreateProjectPlotProfileRule(ctx, granted.ContextID, blankProfileCreation(review.Rules))
	if err != nil || created.RowID != "1" {
		t.Fatal("new profile cannot begin the owned rule workflow", created, err)
	}
	var raw string
	c := service.projects.sqlite
	if err := c.conn.QueryRowContext(ctx, `SELECT Proposal FROM profile.__VPRO_ProfileCreationHistory WHERE ID=1`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved PlotProfileFileCreation
	if err := json.Unmarshal([]byte(raw), &saved); err != nil || !reflect.DeepEqual(saved, request) {
		t.Fatal("file provenance omitted the reviewed complete proposal", err)
	}
}

func TestProfileFileCreationStrictTemplateTransportDestinationAndRetry(t *testing.T) {
	service, state, request := profileFileFixture(t)
	for _, alter := range []func(*PlotProfileFileCreation){
		func(value *PlotProfileFileCreation) { value.Confirmed = false },
		func(value *PlotProfileFileCreation) { value.Name = "Sample" },
		func(value *PlotProfileFileCreation) { value.Name = strings.Repeat("A", 32) },
		func(value *PlotProfileFileCreation) { value.Path = "relative.db" },
		func(value *PlotProfileFileCreation) { value.Review.MetadataAbsent = false },
		func(value *PlotProfileFileCreation) { value.Review.Template.Rows = nil },
	} {
		invalid := request
		alter(&invalid)
		if _, err := service.CreatePlotProfileFile(context.Background(), state.ContextID, invalid); err == nil {
			t.Fatal("invalid profile creation published")
		}
	}
	if _, err := service.CreatePlotProfileFile(context.Background(), "stale", request); err == nil {
		t.Fatal("stale editor created a profile file")
	}
	for _, raw := range []string{
		`{}`, `{"review":null,"name":"Good","path":"C:\\new.db","confirmed":true}`,
		`{"review":{"template":{},"metadataAbsent":null},"name":"Good","path":"C:\\new.db","confirmed":true}`,
		`{"review":{"template":{}},"name":"Good","path":"C:\\new.db","confirmed":true}`,
		`{"review":{},"name":"\ud800","path":"C:\\new.db","confirmed":true}`,
		`{"review":{},"name":"Good","path":"C:\\new.db","confirmed":true,"overwrite":true}`,
	} {
		var decoded PlotProfileFileCreation
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("incomplete/repaired profile creation transport accepted", raw)
		}
	}
	foreign := []byte("Existing destination is not ours")
	if err := os.WriteFile(request.Path, foreign, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreatePlotProfileFile(context.Background(), state.ContextID, request); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	current, err := os.ReadFile(request.Path)
	if err != nil || !bytes.Equal(current, foreign) {
		t.Fatal("foreign destination changed", err)
	}
	request.Path = filepath.Join(filepath.Dir(request.Path), "retry.db")
	if _, err := service.CreatePlotProfileFile(context.Background(), state.ContextID, request); err != nil {
		t.Fatal("retained valid proposal cannot retry", err)
	}
}

func TestProfileFileCreationRejectsChangedPhysicalTemplateAndPresentMetadata(t *testing.T) {
	for _, change := range []string{
		`ALTER TABLE USysProfileTable ADD COLUMN Extra TEXT`,
		`INSERT INTO USysProfileTable("Order") VALUES(1)`,
		`CREATE INDEX ChangedTemplateIndex ON USysProfileTable("Order")`,
		`CREATE TRIGGER ChangedTemplateTrigger AFTER INSERT ON USysProfileTable BEGIN SELECT 1; END`,
		`CREATE TABLE _table_metadata(table_name TEXT,description TEXT)`,
		`ALTER TABLE USysProfileTable RENAME TO OriginalProfileTemplate; CREATE VIEW USysProfileTable AS SELECT * FROM OriginalProfileTemplate`,
	} {
		t.Run(change, func(t *testing.T) {
			service, state, request := profileFileFixture(t)
			path := service.projects.sqlite.attachments["VPro64"]
			db, err := sql.Open("sqlite3", sqliteFileURI(path, "rw"))
			if err != nil {
				t.Fatal(err)
			}
			_, err = db.Exec(change)
			closeErr := db.Close()
			if err != nil || closeErr != nil {
				t.Fatal(err, closeErr)
			}
			if _, err := service.ReviewPlotProfileFileCreation(context.Background(), state.ContextID); err == nil {
				t.Fatal("changed physical template was normalized or silently dropped")
			}
			if _, err := service.CreatePlotProfileFile(context.Background(), state.ContextID, request); err == nil {
				t.Fatal("stale template review published a new profile")
			}
		})
	}
}

func TestProfileFileCreationLateCancellationCollisionCleanupAndRetry(t *testing.T) {
	for _, cancelLate := range []bool{true, false} {
		t.Run(map[bool]string{true: "cancellation", false: "collision"}[cancelLate], func(t *testing.T) {
			service, state, request := profileFileFixture(t)
			files := databaseBytes(t, service.projects.sqlite.attachments)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			foreign := []byte("Independent late destination")
			var actionErr error
			hooked := &profileSUStagedContext{Context: ctx, directory: filepath.Dir(request.Path), prefix: ".vpro-profile-file-"}
			hooked.action = func() {
				if cancelLate {
					cancel()
				} else {
					actionErr = os.WriteFile(request.Path, foreign, 0600)
				}
			}
			if _, err := service.CreatePlotProfileFile(hooked, state.ContextID, request); err == nil || actionErr != nil ||
				cancelLate && !errors.Is(err, context.Canceled) {
				t.Fatal("late cancellation/collision published or lost its cause", err, actionErr)
			}
			remaining, err := os.ReadDir(filepath.Dir(request.Path))
			expected := 0
			if !cancelLate {
				expected = 1
				current, err := os.ReadFile(request.Path)
				if err != nil || !bytes.Equal(current, foreign) {
					t.Fatal("late collision replaced foreign bytes", err)
				}
			}
			if err != nil || len(remaining) != expected {
				t.Fatal("profile staging/journal cleanup incomplete", remaining, err)
			}
			assertProfileSUFiles(t, service, files)
			request.Path = filepath.Join(filepath.Dir(request.Path), "retry.db")
			if _, err := service.CreatePlotProfileFile(context.Background(), state.ContextID, request); err != nil {
				t.Fatal("retained creation cannot retry", err)
			}
		})
	}
}
