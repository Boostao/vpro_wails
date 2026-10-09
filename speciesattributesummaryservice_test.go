package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"
)

func enabledAttributeSummary(t *testing.T, contexts *ContextService) *SpeciesAttributeSummaryService {
	t.Helper()
	service, err := NewSpeciesAttributeSummaryService(contexts, func(name string) (string, bool) {
		if name != speciesAttributeSummaryFeatureEnvironment {
			t.Fatal("attribute gate borrowed another workflow", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func TestSpeciesAttributeSummaryServiceGateAndOwnedZeroWrites(t *testing.T) {
	for _, gate := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSpeciesAttributeSummaryService(nil, func(string) (string, bool) { return gate, gate != "" })
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
		service := enabledAttributeSummary(t, contexts)
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		preview, err := service.Preview(context.Background(), state.ContextID)
		if err != nil || preview.ContextID != state.ContextID || preview.ProjectPath != state.ProjectPath ||
			preview.SUPath != state.SUPath || len(preview.Report.Units) != 3 || len(preview.Report.Definitions) != 6 {
			t.Fatal("owned raw-attribute preview differs", preview, err)
		}
		again, err := service.Preview(context.Background(), state.ContextID)
		if err != nil || !reflect.DeepEqual(preview, again) {
			t.Fatal("repeated snapshot differs", err)
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("read changed retained settings", err)
		}
	}
}

func TestSpeciesAttributeSummaryServiceCancellationOwnershipAndSnapshotRefusal(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	service := enabledAttributeSummary(t, contexts)
	owner := contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	preview, err := service.Preview(ctx, state.ContextID)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(preview, SpeciesAttributeSummaryPreview{}) {
		t.Fatal("cancelled lease published", preview, err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID); err != nil {
		t.Fatal("cancelled lease blocked retry", err)
	}
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("read completion failed") }
	if preview, err := service.Preview(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(preview, SpeciesAttributeSummaryPreview{}) {
		t.Fatal("failed completion published", preview, err)
	}
	service.snapshot.commitRead = nil
	service.snapshot.rollbackRead = func(*sql.Tx) error { return errors.New("read cleanup failed") }
	if preview, err := service.Preview(context.Background(), state.ContextID); err == nil ||
		!reflect.DeepEqual(preview, SpeciesAttributeSummaryPreview{}) {
		t.Fatal("failed cleanup published", preview, err)
	}
	service.snapshot.rollbackRead = nil
	late, cancelLate := context.WithCancel(context.Background())
	service.snapshot.commitRead = func(*sql.Tx) error { cancelLate(); return nil }
	if preview, err := service.Preview(late, state.ContextID); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(preview, SpeciesAttributeSummaryPreview{}) {
		t.Fatal("late cancellation published", preview, err)
	}
	service.snapshot.commitRead = nil
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	if _, err := service.Preview(context.Background(), state.ContextID); err == nil {
		t.Fatal("changed ownership published")
	}

	owner.attachmentInfo["su"] = original
	next, err := contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID); err == nil {
		t.Fatal("stale context published")
	}
	if _, err := service.Preview(context.Background(), next.ContextID); err != nil {
		t.Fatal("fresh context retry failed", err)
	}
	if _, err := service.Preview(nil, next.ContextID); err == nil {
		t.Fatal("nil context published")
	}
}

func TestSpeciesAttributeSummaryServicePhysicalSchemaAndBorrowedLifetime(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	service := enabledAttributeSummary(t, contexts)
	mutate := func(role, statement string) {
		t.Helper()
		db, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments[role], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = db.Exec(statement)
		if err = errors.Join(err, db.Close()); err != nil {
			t.Fatal(err)
		}
	}
	mutate("VPro64", `DROP TABLE LayerCode; DROP TABLE LifeformCodes`)
	if _, err := service.Preview(context.Background(), state.ContextID); err != nil {
		t.Fatal("attribute query borrowed cover/Lifeform dependencies", err)
	}
	mutate("VLists", `ALTER TABLE USysSppAttributes RENAME COLUMN Wetland_Ind TO UnavailableField`)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	preview, err := service.Preview(context.Background(), state.ContextID)
	if err == nil || !reflect.DeepEqual(preview, SpeciesAttributeSummaryPreview{}) {
		t.Fatal("incomplete source table published", preview, err)
	}
	assertProfileSUFiles(t, contexts, before)
	mutate("VLists", `ALTER TABLE USysSppAttributes RENAME COLUMN UnavailableField TO Wetland_Ind`)
	if _, err := service.Preview(context.Background(), state.ContextID); err != nil {
		t.Fatal("restored schema could not retry", err)
	}
	if err := contexts.projects.sqlite.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID); err == nil {
		t.Fatal("closed borrowed owner published")
	}
}
