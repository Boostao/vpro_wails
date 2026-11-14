package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	contexts, state := vprotest.ReportServiceFixture(t, true)
	service := NewLongVegetationUnitPreferencesService(contexts)
	store := contexts.Projects.Preferences
	request := vegetationUnitRequest(t, 1, 3)
	before, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, hooks := range []vprocore.PublicationReadSnapshotHooks{
		{CommitRead: func(*sql.Tx) error { return errors.New("unit read commit failure") }},
		{RollbackRead: func(*sql.Tx) error { return errors.New("unit cleanup failure") }},
	} {
		service.snapshot = hooks
		if result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, request); result.Committed || result.Changed || result.ErrorMessage == "" {
			t.Fatal("read snapshot failure authorized unit mutation", result)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	service.snapshot = vprocore.PublicationReadSnapshotHooks{CommitRead: func(*sql.Tx) error { cancel(); return nil }}
	if result := service.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request); result.Committed || result.Changed || result.ErrorMessage == "" {
		t.Fatal("cancellation after owned snapshot authorized unit mutation", result)
	}
	service.snapshot = vprocore.PublicationReadSnapshotHooks{}
	if value, err := service.GetLongVegetationUnitPreferences(context.Background(), state.ContextID); value == nil || err != nil {
		t.Fatal("snapshot cancellation/cleanup lost pinned source views or owned lease", value, err)
	}
	replace := store.Replace
	store.Replace = func(string, string) error { return errors.New("unit target collision") }
	result := service.SaveLongVegetationUnitPreferences(context.Background(), state.ContextID, request)
	store.Replace = replace
	after, err := os.ReadFile(store.Path)
	stages, stageErr := filepath.Glob(filepath.Join(filepath.Dir(store.Path), ".config-*.yml"))
	if result.Committed || result.Changed || !strings.Contains(result.ErrorMessage, "collision") ||
		err != nil || !bytes.Equal(before, after) || stageErr != nil || len(stages) != 0 {
		t.Fatal("failed persistence changed configuration or leaked staging", result, err, stageErr, stages)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	store.Replace = func(from, to string) error {
		if err := replace(from, to); err != nil {
			return err
		}
		cancel()
		return nil
	}
	result = service.SaveLongVegetationUnitPreferences(ctx, state.ContextID, request)
	store.Replace = replace
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
