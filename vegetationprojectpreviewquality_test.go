package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestProjectOwnedPreviewQualityRetainsOriginalSUAndExistencePruning(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state, scope := projectPreviewFixture(t, external, false, false)
		ctx := context.Background()
		owner := service.contexts.projects.sqlite
		project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = project.Exec(`DELETE FROM Sample_Env WHERE rowid=2; DELETE FROM Sample_Admin WHERE rowid=2`)
		if err = errors.Join(err, project.Close()); err != nil {
			t.Fatal(err)
		}
		lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName='DataQuality';
			INSERT INTO USysTableOfLists(ListName,Item,ItemOrder)
			VALUES('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),('DataQuality','Good',3)`)
		if err = errors.Join(err, lists.Close()); err != nil {
			t.Fatal(err)
		}
		if err := service.contexts.projects.preferences.update("ReportOptions", map[string]any{
			"DataQualityFilterEnforceLV": true,
			"DataQualityFilterSiteLV":    "Good", "DataQualityFilterVegLV": "Good", "DataQualityFilterSoilLV": "Good",
		}); err != nil {
			t.Fatal(err)
		}
		scope, err = service.BuildLongVegetationProjectScope(ctx, state.ContextID, scope.Revision)
		if err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, owner.attachments)
		preview, err := service.PreviewLongVegetationProject(ctx, state.ContextID, projectReadJSON(t, scope))
		if err != nil || preview.Preview.Report.Quality == nil ||
			len(preview.Preview.Report.Quality.Occurrences) != 8 || len(preview.Preview.Report.ProjectUnits) != 1 ||
			len(preview.Preview.Report.Units) != 1 || preview.Preview.Report.Units[0].NumPlots != 1 ||
			len(preview.Scope.Build.ProjectTuples) != 2 {
			t.Fatal("original full project Build must stay separate from QCLV existence-pruned output", preview, err)
		}
		if *preview.Preview.Report.ProjectUnits[0].PlotNumber.Text != "108050" ||
			len(preview.Preview.Report.Units[0].MembershipIDs) != 0 {
			t.Fatal("quality fanout changed project unit identity or denominator", preview)
		}
		assertProfileSUFiles(t, service.contexts, before)
	}
}

func TestProjectOwnedPreviewStaleScopeAndRetiredContextRemainUnavailable(t *testing.T) {
	service, state, scope := projectPreviewFixture(t, true, true, false)
	ctx := context.Background()
	request := projectReadJSON(t, scope)
	if _, err := service.SetLongVegetationReportScope(ctx, state.ContextID,
		projectScopeRequestJSON(t, scope.Revision, vegetationProjectAllScope)); err != nil {
		t.Fatal(err)
	}
	if preview, err := service.PreviewLongVegetationProject(ctx, state.ContextID, request); err == nil || !reflect.DeepEqual(preview, LongVegetationProjectPreview{}) {
		t.Fatal("old project source2 request passed under source3 intent", preview, err)
	}
	if _, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state)); err != nil {
		t.Fatal(err)
	}
	if preview, err := service.PreviewLongVegetationProject(ctx, state.ContextID, request); err == nil || !reflect.DeepEqual(preview, LongVegetationProjectPreview{}) {
		t.Fatal("retired owner supplied project report under old Build", preview, err)
	}
}
