package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func enabledLifeformService(t *testing.T, contexts *ContextService) *LifeformSummaryService {
	t.Helper()
	service, err := NewLifeformSummaryService(contexts, func(name string) (string, bool) {
		if name != lifeformSummaryFeatureEnvironment {
			t.Fatal("gate borrowed another workflow", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestLifeformSummaryServiceGateAndOwnedZeroWrites(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewLifeformSummaryService(nil, func(string) (string, bool) { return gate, gate != "" })
		if gate == "" || gate == "false" || gate == "true" {
			if err != nil || service.enabled != (gate == "true") {
				t.Fatal(gate, service, err)
			}
			if _, err := service.Preview(context.Background(), "owned"); err == nil {
				t.Fatal("unavailable service published")
			}
		} else if err == nil {
			t.Fatal("nonliteral gate accepted", gate)
		}
	}
	for _, external := range []bool{false, true} {
		contexts, state := reportServiceFixture(t, external)
		service := enabledLifeformService(t, contexts)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := service.Preview(context.Background(), state.ContextID)
		if err != nil || preview.ContextID != state.ContextID || preview.ProjectPath != state.ProjectPath ||
			preview.SUPath != state.SUPath || preview.Report.Project != state.ActiveProject ||
			preview.Report.SU != state.ActiveSU || len(preview.Report.Units) != 3 ||
			len(preview.Report.Catalogue) == 0 {
			t.Fatal("owned preview unavailable or flattened", preview, err)
		}
		again, err := service.Preview(context.Background(), state.ContextID)
		if err != nil || !reflect.DeepEqual(preview, again) {
			t.Fatal("detached repeated read differs", err)
		}
		preview.Report.Catalogue[0].Label = metadataText("detached")
		if reflect.DeepEqual(preview, again) {
			t.Fatal("result did not detach")
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("preview changed config", err)
		}
	}
}

func TestLifeformSummaryServiceCancellationStalenessIdentityAndRetry(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	service := enabledLifeformService(t, contexts)
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	preview, err := service.Preview(ctx, state.ContextID)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(preview, LifeformSummaryPreview{}) {
		t.Fatal("cancelled lease published", preview, err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID); err != nil {
		t.Fatal("cancellation leaked lease", err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	if preview, err := service.Preview(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(preview, LifeformSummaryPreview{}) {
		t.Fatal("changed file identity published", preview, err)
	}
	owner.attachmentInfo["su"] = original
	service.snapshot.commitRead = func(_ *sql.Tx) error { return errors.New("read completion failed") }
	if preview, err := service.Preview(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(preview, LifeformSummaryPreview{}) {
		t.Fatal("failed completion published report", preview, err)
	}
	service.snapshot.commitRead = nil
	service.snapshot.rollbackRead = func(_ *sql.Tx) error { return errors.New("read cleanup failed") }
	if preview, err := service.Preview(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(preview, LifeformSummaryPreview{}) {
		t.Fatal("failed cleanup published report", preview, err)
	}
	service.snapshot.rollbackRead = nil
	cancelled, cancelSnapshot := context.WithCancel(context.Background())
	service.snapshot.commitRead = func(_ *sql.Tx) error { cancelSnapshot(); return nil }
	if preview, err := service.Preview(cancelled, state.ContextID); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(preview, LifeformSummaryPreview{}) {
		t.Fatal("late cancellation published report", preview, err)
	}
	service.snapshot.commitRead = nil
	if _, err := service.Preview(context.Background(), state.ContextID); err != nil {
		t.Fatal("rejected snapshot leaked lease", err)
	}
	next, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID); err == nil {
		t.Fatal("stale context published")
	}
	if _, err := service.Preview(context.Background(), next.ContextID); err != nil {
		t.Fatal("context switch retry failed", err)
	}
	selection := contextSelection(next)
	selection.SU, selection.SUPath = "None", ""
	next, err = contexts.SwitchContext(next.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(context.Background(), next.ContextID); err == nil {
		t.Fatal("missing SU became implicit all-project report")
	}
	if _, err := service.Preview(nil, next.ContextID); err == nil {
		t.Fatal("nil request context accepted")
	}
}

func TestLifeformSummaryServiceSchemaRefusalHierarchyExclusionAndBorrowedLifetime(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	calls, gate := 0, "true"
	service, err := NewLifeformSummaryService(contexts, func(string) (string, bool) {
		calls++
		return gate, true
	})
	if err != nil {
		t.Fatal(err)
	}
	gate = "false"
	owner := contexts.projects.sqlite
	mutate := func(role, statement string) {
		t.Helper()
		db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments[role], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(statement)
		if err = errors.Join(err, db.Close()); err != nil {
			t.Fatal(err)
		}
	}
	mutate("VPro64", `DROP TABLE LayerCode`)
	mutate("project", `CREATE TABLE USysSuTableDynamic_SU(PlotNumber TEXT,SiteUnit BLOB);
		INSERT INTO USysSuTableDynamic_SU VALUES('108050',x'ff')`)
	verifyRead := func(expect string) {
		t.Helper()
		before := databaseBytes(t, owner.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := service.Preview(context.Background(), state.ContextID)
		if expect == "" && err != nil || expect != "" && (err == nil || !strings.Contains(err.Error(), expect) ||
			!reflect.DeepEqual(preview, LifeformSummaryPreview{})) {
			t.Fatal("schema/scope refusal differs", expect, preview, err)
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("read/refusal changed configuration", err)
		}
	}
	verifyRead("")
	if calls != 1 {
		t.Fatal("feature gate was not constructor-lifetime captured", calls)
	}
	owner.selection.SU = "USysSuTableDynamic"
	verifyRead("hierarchy/dynamic-break scope is unavailable")
	owner.selection.SU = state.ActiveSU
	verifyRead("")
	mutate("VPro64", `ALTER TABLE LifeformCodes RENAME COLUMN LifeformTXT TO LegacyLabel`)
	verifyRead("LifeformTXT")
	mutate("VPro64", `ALTER TABLE LifeformCodes RENAME COLUMN LegacyLabel TO LifeformTXT`)
	verifyRead("")
	mutate("project", `DROP TABLE Report_SU;
		CREATE VIEW Report_SU AS SELECT '108050' AS PlotNumber,'U' AS SiteUnit`)
	verifyRead("requires original physical table su.Report_SU")
	mutate("project", `DROP VIEW Report_SU; CREATE TABLE Report_SU(PlotNumber TEXT,SiteUnit TEXT);
		INSERT INTO Report_SU VALUES('108050','U')`)
	verifyRead("")
	if _, err := contexts.projects.GetState(context.Background()); err != nil || owner.conn == nil {
		t.Fatal("report closed its borrowed owner", err)
	}
	disabled, err := NewLifeformSummaryService(contexts, func(string) (string, bool) { return "false", true })
	if err != nil {
		t.Fatal(err)
	}
	before := databaseBytes(t, owner.attachments)
	if _, err := disabled.Preview(context.Background(), state.ContextID); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatal("default-off gate did not refuse preview", err)
	}
	assertProfileSUFiles(t, contexts, before)
}

func TestLifeformSummarySampleRepresentativeCounts(t *testing.T) {
	contexts, state := contextServiceFixture(t)
	selection := contextSelection(state)
	selection.SU, selection.SUPath = "Sample", state.ProjectPath
	state, err := contexts.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := enabledLifeformService(t, contexts).Preview(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Report.Units) != 11 {
		t.Fatal("Sample normal SU scope differs", len(preview.Report.Units))
	}
	expected := map[string][3]int{"BWBSdk 1 /08": {3, 42, 80}, "BWBSmw 2 /05": {9, 106, 288}}
	seen := 0
	for _, unit := range preview.Report.Units {
		if unit.Code.Text == nil {
			continue
		}
		if counts, found := expected[*unit.Code.Text]; found {
			seen++
			if [3]int{unit.NPlots, unit.UniqueSpecies, unit.Occurrences} != counts {
				t.Fatal("Sample representative physical counts changed", unit)
			}
			t.Logf("Sample unit %q: nPlots=%d uniqueSpecies=%d occurrences=%d", *unit.Code.Text,
				unit.NPlots, unit.UniqueSpecies, unit.Occurrences)
		}
	}
	if seen != len(expected) {
		t.Fatal("Sample representative units disappeared", seen)
	}
}
