package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLongVegetationUnitPreferencesSnapshotCleanupPersistenceCollisionAndCommittedCancel(t *testing.T) {
	contexts, state := reportServiceFixture(t, true)
	contexts.longVegetationNoneUnitsEnabled = true
	service := NewLongVegetationUnitPreferencesService(contexts, true)
	store := contexts.projects.preferences
	request := vegetationUnitRequest(t, 1, 3)
	before, err := os.ReadFile(store.path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []publicationReadSnapshotHooks{
		{commitRead: func(*sql.Tx) error { return errors.New("unit read commit failure") }},
		{rollbackRead: func(*sql.Tx) error { return errors.New("unit cleanup failure") }},
	} {
		service.snapshot = hooks
		if result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, request); result.Committed || result.Changed || result.ErrorMessage == "" {
			t.Fatal("read snapshot failure authorized unit mutation", result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = publicationReadSnapshotHooks{commitRead: func(*sql.Tx) error { cancel(); return nil }}
	if result := service.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request); result.Committed || result.Changed || result.ErrorMessage == "" {
		t.Fatal("cancellation after owned snapshot authorized unit mutation", result)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if value, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID); value == nil || err != nil {
		t.Fatal("snapshot cancellation/cleanup lost pinned source views or owned lease", value, err)
	}
	replace := store.replace
	store.replace = func(string, string) error { return errors.New("unit target collision") }
	result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, request)
	store.replace = replace
	after, err := os.ReadFile(store.path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.path), ".config-*.yml"))
	if result.Committed || result.Changed || !strings.Contains(result.ErrorMessage, "collision") ||
		err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed configuration or leaked staging", result, err, stageErr, stages)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result = service.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	store.replace = replace
	if !result.Committed || !result.Changed || !strings.Contains(result.ErrorMessage, "do not replay") {
		t.Fatal("irreversible source3 persistence receipt was erased by cleanup/cancellation", result)
	}
	current, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID)
	if err != nil || current.Values.UnitGroups != 3 {
		t.Fatal("owned observation lost committed None preference", current, err)
	}
	result = service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, vegetationUnitRequest(t, 3, 3))
	if result.Committed || result.Changed || result.ErrorMessage != "" {
		t.Fatal("explicit committed retry was not an unchanged no-op", result)
	}
}
