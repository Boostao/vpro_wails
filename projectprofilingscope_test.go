package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectProfilingExternalProjectAndSelectedSUScopePreserveFiles(t *testing.T) {
	service, old, _ := profileRunFixture(t)
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE Preview_SU(PlotNumber TEXT,SiteUnit TEXT);
		INSERT INTO Preview_SU VALUES('108050','A'),('9003104','A'),('off-project','A')`)
	release()
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(old.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(t.TempDir(), "external ' # \u00e9.db")
	if err := os.WriteFile(external, original, 0600); err != nil {
		t.Fatal(err)
	}
	selection := contextSelection(old)
	selection.ProjectPath, selection.SU, selection.SUPath = external, "Preview", external
	state, err := service.SwitchContext(old.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	review, err := service.ReviewProjectPlotProfile(ctx, state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	lump, err := service.ReviewProjectPlotProfileLump(ctx, state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	request := ProjectPlotProfileRunRequest{OriginalRules: review.Rules, ProjectLump: &lump}
	for _, combine := range []bool{false, true} {
		request.Subvarieties = combine
		result, err := service.RunProjectPlotProfile(ctx, state.ContextID, request)
		if err != nil || result.TotalPlots != 2 || result.SU != "Preview" ||
			!reflect.DeepEqual(result.PlotNumbers, []string{"108050", "9003104"}) {
			t.Fatal("external owned context/SU scope was broadened or changed", combine, result, err)
		}
	}
	for role, before := range files {
		after, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("scoped preview changed attached data", role, err)
		}
	}
	after, err := os.ReadFile(old.ProjectPath)
	if err != nil || !bytes.Equal(original, after) {
		t.Fatal("external preview changed the previously selected managed project", err)
	}
}
