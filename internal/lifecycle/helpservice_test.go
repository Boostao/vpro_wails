package lifecycle

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/resources"
)

func TestHelpBundleLookupAndOfflineSearch(t *testing.T) {
	dataDir := t.TempDir()
	configDir := t.TempDir()
	prefs, err := vprocore.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewHelpService(prefs)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	bundle, err := svc.GetHelpBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.ProductName != "VPro" ||
		bundle.LMH25Edition != "LMH25 (2023)" ||
		len(bundle.Fields) < 15 ||
		len(bundle.DecisionKeys) != 3 ||
		len(bundle.Guides) != 5 ||
		len(bundle.Tours) != 4 ||
		len(bundle.Shortcuts) < 7 ||
		len(bundle.WhatsNew) < 2 ||
		len(bundle.DataUseAgreement.Clauses) != 9 ||
		!strings.HasPrefix(bundle.LibraryStamps.BECVersion, "BECdb_v13_2026") {
		t.Fatalf("unexpected help bundle contents: product=%q fields=%d keys=%d guides=%d tours=%d clauses=%d",
			bundle.ProductName, len(bundle.Fields), len(bundle.DecisionKeys), len(bundle.Guides), len(bundle.Tours), len(bundle.DataUseAgreement.Clauses))
	}

	// Verify deep-clone isolation on GetHelpBundle
	origFirstTitle := bundle.Fields[0].Title
	bundle.Fields[0].Title = "mutated"
	bundle.Fields[0].Rules[0] = "mutated-rule"
	bundle.Tours[0].Steps[0].Title = "mutated-step"
	bundle.DataUseAgreement.Clauses[0] = "mutated-clause"
	freshBundle, err := svc.GetHelpBundle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if freshBundle.Fields[0].Title != origFirstTitle ||
		freshBundle.Fields[0].Rules[0] == "mutated-rule" ||
		freshBundle.Tours[0].Steps[0].Title == "mutated-step" ||
		freshBundle.DataUseAgreement.Clauses[0] == "mutated-clause" {
		t.Fatal("GetHelpBundle aliased internal slice state")
	}

	// Exact topic lookup with linked decision keys
	rsmrTopic, err := svc.LookupHelpTopic(ctx, "site.moisture_regime")
	if err != nil {
		t.Fatal(err)
	}
	if rsmrTopic.MatchKind != "exact" || rsmrTopic.Field == nil || rsmrTopic.Field.Code != "SITE 20" ||
		rsmrTopic.DecisionKey == nil || rsmrTopic.DecisionKey.ID != "key.rsmr_9_4" || len(rsmrTopic.RelatedGuides) == 0 {
		t.Fatalf("unexpected site.moisture_regime lookup: %+v", rsmrTopic)
	}

	// Alias, prefix, column, and decision-key lookups
	cases := []struct {
		input      string
		wantID     string
		wantKeyID  string
	}{
		{"humus.upper_depth", "humus.upper_lower_depth", ""},
		{"humus.upper", "humus.upper_lower_depth", ""},
		{"veg.cover6", "veg.covers", "key.cover_chart"},
		{"Cover5a", "veg.covers", "key.cover_chart"},
		{"NutrientRegime", "site.nutrient_regime", "key.rsnr_9_6"},
		{"SITE 20", "site.moisture_regime", "key.rsmr_9_4"},
		{"sivi.plot_type", "sivi.plot_type", ""},
		{"sivi.observers", "sivi.plot_type", ""},
		{"key.rsnr_9_6", "key.rsnr_9_6", "key.rsnr_9_6"},
	}
	for _, tc := range cases {
		res, err := svc.LookupHelpTopic(ctx, tc.input)
		if err != nil {
			t.Fatalf("LookupHelpTopic(%q) failed: %v", tc.input, err)
		}
		if res.ResolvedID != tc.wantID {
			t.Fatalf("LookupHelpTopic(%q) resolved %q, want %q", tc.input, res.ResolvedID, tc.wantID)
		}
		gotKey := ""
		if res.DecisionKey != nil {
			gotKey = res.DecisionKey.ID
		}
		if gotKey != tc.wantKeyID {
			t.Fatalf("LookupHelpTopic(%q) decisionKey=%q, want %q", tc.input, gotKey, tc.wantKeyID)
		}
	}

	if _, err := svc.LookupHelpTopic(ctx, ""); err == nil {
		t.Fatal("expected error on empty help topic ID")
	}
	if _, err := svc.LookupHelpTopic(ctx, "nonexistent_domain.foo"); err == nil {
		t.Fatal("expected error on unknown help topic ID")
	}
	if _, err := svc.LookupHelpTopic(ctx, string([]byte{0xff})); err == nil {
		t.Fatal("expected error on malformed UTF-8 topic ID")
	}

	// Ranked offline search across LMH25 codes, soil/humus symbols, decision keys, guides, and shortcuts
	searchChecks := []struct {
		query    string
		wantTop  string
		wantKind string
	}{
		{"SITE 20", "site.moisture_regime", "field"},
		{"SOIL 5", "humus.upper_lower_depth", "field"},
		{"VEG 2", "veg.covers", "field"},
		{"Of", "humus.horizon", "field"},
		{"rSMR", "site.moisture_regime", "field"},
		{"Key 9.6", "key.rsnr_9_6", "key"},
		{"Ctrl+S", "shortcut-5", "shortcut"},
	}
	for _, sc := range searchChecks {
		hits, err := svc.SearchHelp(ctx, sc.query)
		if err != nil {
			t.Fatalf("SearchHelp(%q) failed: %v", sc.query, err)
		}
		if len(hits) == 0 {
			t.Fatalf("SearchHelp(%q) returned 0 hits", sc.query)
		}
		if hits[0].ID != sc.wantTop || hits[0].Kind != sc.wantKind {
			t.Fatalf("SearchHelp(%q) top hit = (%s, %s, score=%d), want (%s, %s)",
				sc.query, hits[0].Kind, hits[0].ID, hits[0].Score, sc.wantKind, sc.wantTop)
		}
	}

	morHits, err := svc.SearchHelp(ctx, "Mor")
	if err != nil || len(morHits) < 2 {
		t.Fatalf("expected multiple Mor search hits across humus/rSNR topics, got %d (%v)", len(morHits), err)
	}

	emptyHits, err := svc.SearchHelp(ctx, "   ")
	if err != nil || emptyHits == nil || len(emptyHits) != 0 {
		t.Fatalf("expected non-nil empty slice for blank search query, got %#v (%v)", emptyHits, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := svc.SearchHelp(cancelled, "SITE 20"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled on SearchHelp, got %v", err)
	}
}

func TestWhatsNewPersistenceInDesktopConfig(t *testing.T) {
	dataDir := t.TempDir()
	configDir := t.TempDir()
	prefs, err := vprocore.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewHelpService(prefs)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	initial, err := svc.GetWhatsNewStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !initial.HasUnread || initial.UnreadCount != 2 || initial.LatestVersion != "2.0.0" || initial.LastSeenVersion != "" {
		t.Fatalf("unexpected initial WhatsNewStatus: %+v", initial)
	}

	afterPreview, err := svc.AcknowledgeWhatsNew(ctx, "0.1.0")
	if err != nil {
		t.Fatal(err)
	}
	if !afterPreview.HasUnread || afterPreview.UnreadCount != 1 || afterPreview.LastSeenVersion != "0.1.0" {
		t.Fatalf("expected 1 unread entry after acknowledging 0.1.0, got %+v", afterPreview)
	}

	if _, err := svc.AcknowledgeWhatsNew(ctx, "9.9.9-unknown"); err == nil {
		t.Fatal("expected error acknowledging unknown release version")
	}

	afterLatest, err := svc.AcknowledgeWhatsNew(ctx, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if afterLatest.HasUnread || afterLatest.UnreadCount != 0 || afterLatest.LastSeenVersion != "2.0.0" {
		t.Fatalf("expected 0 unread entries after acknowledging 2.0.0, got %+v", afterLatest)
	}

	rawYAML, err := os.ReadFile(filepath.Join(configDir, "config.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rawYAML), "last_seen_whats_new_version: 2.0.0") {
		t.Fatalf("expected last_seen_whats_new_version persisted in config.yml, got:\n%s", string(rawYAML))
	}

	// Reopen DesktopConfig + HelpService and verify persistence survives restart
	reopenedPrefs, err := vprocore.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	reopenedSvc, err := NewHelpService(reopenedPrefs)
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := reopenedSvc.GetWhatsNewStatus(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.HasUnread || reloaded.UnreadCount != 0 || reloaded.LastSeenVersion != "2.0.0" {
		t.Fatalf("expected persisted WhatsNew acknowledgement after restart, got %+v", reloaded)
	}
}

func createUserProjectFromSample(t *testing.T, srcSamplePath, dstPath, projectName string) {
	t.Helper()
	raw, err := os.ReadFile(srcSamplePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", dstPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name LIKE 'Sample_%'")
	if err != nil {
		t.Fatal(err)
	}
	var sampleTables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		sampleTables = append(sampleTables, name)
	}
	rows.Close()
	for _, tbl := range sampleTables {
		newTbl := projectName + strings.TrimPrefix(tbl, "Sample")
		if _, err := db.Exec(`ALTER TABLE "` + tbl + `" RENAME TO "` + newTbl + `"`); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`UPDATE _table_metadata SET table_name = REPLACE(table_name, 'Sample_', ? || '_')`, projectName); err != nil {
		t.Fatal(err)
	}
}

func TestResetWorkspaceToSampleRestoresPristineSampleWithoutDeletingUserProjects(t *testing.T) {
	dataDir := t.TempDir()
	configDir := t.TempDir()
	prefs, err := vprocore.OpenDesktopConfig(dataDir, configDir)
	if err != nil {
		t.Fatal(err)
	}
	projects, err := vprocore.NewSQLiteProjectService(dataDir, configDir, prefs)
	if err != nil {
		t.Fatal(err)
	}
	defer projects.CloseSQLiteContext()
	plots, err := vprocore.NewPlotServiceWithPreferences(projects)
	if err != nil {
		t.Fatal(err)
	}
	contexts, err := vprocore.NewContextService(projects, plots)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := NewHelpService(prefs)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetContext(contexts)
	ctx := context.Background()

	samplePath := filepath.Join(dataDir, "projects", "Sample.db")
	userProjectPath := filepath.Join(dataDir, "projects", "KootenayStudy.db")
	createUserProjectFromSample(t, samplePath, userProjectPath, "KootenayStudy")
	userBytesBefore, err := os.ReadFile(userProjectPath)
	if err != nil {
		t.Fatal(err)
	}

	initialState, err := projects.GetState(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate a plot while Sample.db is actively open in WAL mode
	pageBefore, err := projects.ListPlots(ctx, 0, 5)
	if err != nil || len(pageBefore.Plots) == 0 {
		t.Fatalf("expected Sample plots, got %+v (%v)", pageBefore, err)
	}
	firstPlotNum := pageBefore.Plots[0].PlotNumber
	headerBefore, err := contexts.GetPlot(ctx, initialState.ContextID, firstPlotNum)
	if err != nil {
		t.Fatal(err)
	}
	mutatedHeader := *headerBefore
	sentinelLoc := "MUTATED_DURING_TOUR"
	mutatedHeader.GeneralLocation = &sentinelLoc
	if err := contexts.UpdatePlot(initialState.ContextID, mutatedHeader); err != nil {
		t.Fatal(err)
	}
	headerDirty, err := contexts.GetPlot(ctx, initialState.ContextID, firstPlotNum)
	if err != nil || headerDirty.GeneralLocation == nil || *headerDirty.GeneralLocation != sentinelLoc {
		t.Fatalf("expected mutated Sample plot before reset, got %+v (%v)", headerDirty, err)
	}

	// Stale contextID must be rejected without touching Sample.db
	if _, err := svc.ResetWorkspaceToSample(ctx, "stale-0000000000000000000000000000"); err == nil {
		t.Fatal("expected stale contextID rejection on ResetWorkspaceToSample")
	}
	headerStillDirty, err := contexts.GetPlot(ctx, initialState.ContextID, firstPlotNum)
	if err != nil || headerStillDirty.GeneralLocation == nil || *headerStillDirty.GeneralLocation != sentinelLoc {
		t.Fatal("stale ResetWorkspaceToSample should not have altered active Sample database")
	}

	// Case 1: ResetWorkspaceToSample while Sample.db is currently attached in WAL mode
	resetState1, err := svc.ResetWorkspaceToSample(ctx, initialState.ContextID)
	if err != nil {
		t.Fatalf("ResetWorkspaceToSample while Sample attached failed: %v", err)
	}
	if resetState1.ActiveProject != "Sample" || resetState1.ActiveSU != "Sample" || resetState1.ActiveHierarchy != "Sample" {
		t.Fatalf("expected Sample/Sample/Sample active selection, got %+v", resetState1)
	}
	if len(resetState1.ContextID) != 32 || resetState1.ContextID == initialState.ContextID {
		t.Fatalf("expected fresh 128-bit contextID, got %q (prev %q)", resetState1.ContextID, initialState.ContextID)
	}
	headerRestored, err := contexts.GetPlot(ctx, resetState1.ContextID, firstPlotNum)
	if err != nil {
		t.Fatal(err)
	}
	if headerRestored.GeneralLocation != nil && *headerRestored.GeneralLocation == sentinelLoc {
		t.Fatalf("Sample.db was not restored to pristine embedded state; GeneralLocation=%q", *headerRestored.GeneralLocation)
	}

	// Case 2: Switch to KootenayStudy user project, then run ResetWorkspaceToSample
	userState, err := contexts.SwitchContext(resetState1.ContextID, vprocore.ContextSelection{
		Project:       "KootenayStudy",
		ProjectPath:   userProjectPath,
		SU:            "KootenayStudy",
		SUPath:        userProjectPath,
		Hierarchy:     "None",
		HierarchyPath: "",
	})
	if err != nil {
		t.Fatal(err)
	}
	resetState2, err := svc.ResetWorkspaceToSample(ctx, userState.ContextID)
	if err != nil {
		t.Fatalf("ResetWorkspaceToSample from user project failed: %v", err)
	}
	if resetState2.ActiveProject != "Sample" || resetState2.ActiveSU != "Sample" || resetState2.ActiveHierarchy != "Sample" ||
		len(resetState2.ContextID) != 32 || resetState2.ContextID == userState.ContextID {
		t.Fatalf("unexpected state after resetting from user project: %+v", resetState2)
	}

	// User project .db must remain untouched on disk and discoverable in ProjectState
	userBytesAfter, err := os.ReadFile(userProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(userBytesBefore, userBytesAfter) {
		t.Fatal("ResetWorkspaceToSample modified or corrupted user project KootenayStudy.db")
	}
	foundUserProject := false
	for _, p := range resetState2.Projects {
		if p.Name == "KootenayStudy" && p.Compatible {
			foundUserProject = true
			break
		}
	}
	if !foundUserProject {
		t.Fatalf("expected KootenayStudy to remain listed in Projects: %+v", resetState2.Projects)
	}
	sampleBytesOnDisk, err := os.ReadFile(samplePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sampleBytesOnDisk, resources.SampleDB) {
		t.Fatal("Sample.db on disk does not match pristine resources.SampleDB")
	}
	if _, err := os.Stat(filepath.Join(dataDir, ".sample-reset-staging")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected temporary staging directory to be cleaned up, stat err=%v", err)
	}
}
