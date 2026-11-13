package lifecycle

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boostao/vpro-wails/internal/vprocore"
)

func TestStartupRecoverySampleFallbackRelocationAndPortableReanchor(t *testing.T) {
	// 1. Portable dataDir re-anchoring across moved profiles/OSes
	dataA := filepath.Join(t.TempDir(), "machine-a", "vpro")
	configDir := filepath.Join(t.TempDir(), "config")
	prefs, err := vprocore.OpenDesktopConfig(dataA, configDir)
	if err != nil {
		t.Fatal(err)
	}
	svcA, err := vprocore.NewSQLiteProjectService(dataA, configDir, prefs)
	if err != nil {
		t.Fatal(err)
	}
	_ = svcA.CloseSQLiteContext()

	// Simulate copying config.yml with Windows/Machine-A managed paths onto Machine-B dataDir
	dataB := filepath.Join(t.TempDir(), "machine-b", "vpro")
	if err := prefs.Update("Current", map[string]any{
		"ProjectPath":   `C:\Users\FieldCrew\AppData\Local\vpro\projects\Sample.db`,
		"HierarchyPath": `C:\Users\FieldCrew\AppData\Local\vpro\projects\Sample.db`,
	}); err != nil {
		t.Fatal(err)
	}
	if err := prefs.Update("Desktop", map[string]any{
		"DatabasePaths": map[string]any{
			"VUser": `C:\Users\FieldCrew\AppData\Local\vpro\database-family\VUser.db`,
		},
	}); err != nil {
		t.Fatal(err)
	}
	// Remove Machine-A folder completely so old paths cannot exist anywhere
	_ = os.RemoveAll(dataA)

	svcB, err := vprocore.NewSQLiteProjectService(dataB, configDir, prefs)
	if err != nil {
		t.Fatalf("expected portable dataDir re-anchoring to succeed, got %v", err)
	}
	stateB, err := svcB.GetState(context.Background())
	_ = svcB.CloseSQLiteContext()
	if err != nil || stateB.ProjectPath != filepath.Join(dataB, "projects", "Sample.db") {
		t.Fatalf("unexpected re-anchored project path: %+v (%v)", stateB, err)
	}

	// 2. StartupService ResetToSample backs up broken config.yml to config.backup-<ts>.yml and activates Sample
	recData := t.TempDir()
	recConfig := t.TempDir()
	configFile := filepath.Join(recConfig, "config.yml")
	// Seed retained legacy JSON files (from an earlier VPro migration) to verify ResetToSample
	// stamps Desktop.LegacyImports and never triggers a legacy JSON conflict on activation.
	if err := os.WriteFile(filepath.Join(recConfig, "coordinate-settings.json"), []byte(`{"mode":"dms"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	brokenYAML := []byte("Current:\n  CurrProject: \"MissingUSB\"\n  ProjectPath: \"/unplugged/usb/MissingUSB.db\"\n  User: \"FieldLead\"\n")
	if err := os.WriteFile(configFile, brokenYAML, 0o600); err != nil {
		t.Fatal(err)
	}

	fixedClock := time.Date(2026, 11, 12, 18, 30, 45, 0, time.UTC)
	activateRuntime := func() (StartupState, error) {
		p, err := vprocore.OpenDesktopConfig(recData, recConfig)
		if err != nil {
			return StartupState{}, err
		}
		ps, err := vprocore.NewSQLiteProjectService(recData, recConfig, p)
		if err != nil {
			return StartupState{}, err
		}
		defer ps.CloseSQLiteContext()
		st := StartupState{Ready: true, ConfigPath: p.Path, ReleaseChannel: vprocore.ReleaseChannelStable}
		PopulateRecoveryHints(&st)
		return st, nil
	}

	startup := &StartupService{
		DataDir: recData,
		Now:     func() time.Time { return fixedClock },
		State: StartupState{
			Ready:      false,
			Message:    "missing USB project",
			ConfigPath: configFile,
		},
		Activate: activateRuntime,
	}
	PopulateRecoveryHints(&startup.State)
	if startup.State.ConfiguredProject != "MissingUSB" || startup.State.ProjectPath != "/unplugged/usb/MissingUSB.db" {
		t.Fatalf("PopulateRecoveryHints missed fields: %+v", startup.State)
	}

	recovered, err := startup.ResetToSample(context.Background())
	if err != nil || !recovered.Ready {
		t.Fatalf("ResetToSample failed: %+v (%v)", recovered, err)
	}
	wantBackup := filepath.Join(recConfig, "config.backup-20261112-183045.yml")
	if recovered.BackupPath != wantBackup {
		t.Fatalf("BackupPath = %q, want %q", recovered.BackupPath, wantBackup)
	}
	backupBytes, err := os.ReadFile(wantBackup)
	if err != nil || !bytes.Equal(backupBytes, brokenYAML) {
		t.Fatalf("backup did not preserve exact prior config.yml bytes: %v", err)
	}

	// 3. StartupService RecoverWithPaths relocates missing external .db, preserves User, and backs up without colliding
	relocatedDir := t.TempDir()
	relocatedDB := filepath.Join(relocatedDir, "RelocatedSample.db")
	sampleRaw, err := os.ReadFile(filepath.Join(recData, "projects", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(relocatedDB, sampleRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configFile, []byte(
		"Desktop:\n  SchemaVersion: 1\nAudit:\n  AuditStrength: 2\nCurrent:\n  CoordMethod: 1\n  AssignedSuSource: 1\n  CurrProject: \"Sample\"\n  ProjectPath: \"/gone/drive/Sample.db\"\n  CurrPlotlist: \"MissingSU\"\n  SUPath: \"/gone/drive/MissingSU.db\"\n  CurrHierarchy: \"MissingHier\"\n  HierarchyPath: \"/gone/drive/MissingHier.db\"\n  User: \"EcologistJane\"\n",
	), 0o600); err != nil {
		t.Fatal(err)
	}

	startup2 := &StartupService{
		DataDir:  recData,
		Now:      func() time.Time { return fixedClock }, // same timestamp tests -1 collision suffix!
		Chooser:  func(context.Context) (string, error) { return relocatedDB, nil },
		State:    StartupState{Ready: false, Message: "missing drive", ConfigPath: configFile},
		Activate: activateRuntime,
	}
	chosen, err := startup2.ChooseDatabaseFile(context.Background())
	if err != nil || chosen != relocatedDB {
		t.Fatalf("ChooseDatabaseFile = %q (%v), want %q", chosen, err, relocatedDB)
	}
	relocState, err := startup2.RecoverWithPaths(context.Background(), StartupRecoveryRequest{
		ProjectPath:    chosen,
		ClearSU:        true,
		ClearHierarchy: true,
	})
	if err != nil || !relocState.Ready {
		t.Fatalf("RecoverWithPaths failed: %+v (%v)", relocState, err)
	}
	if !strings.HasSuffix(relocState.BackupPath, "config.backup-20261112-183045-1.yml") {
		t.Fatalf("expected non-overwriting -1 backup suffix, got %q", relocState.BackupPath)
	}
	reopenedPrefs, err := vprocore.OpenDesktopConfig(recData, recConfig)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := reopenedPrefs.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if u, _ := vprocore.ConfigString(snap, "Current", "User"); u != "EcologistJane" {
		t.Fatalf("RecoverWithPaths lost custom Current.User: got %q", u)
	}
}
