package projectops_test

import (
	"context"
	"testing"

	"github.com/boostao/vpro-wails/internal/projectops"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

func TestMergeCellCalibrationStrataCoverAndTextBounds(t *testing.T) {
	svc, state := vprotest.ContextServiceFixture(t)
	ctx := context.Background()

	tgtRes, err := projectops.CreateNewProject(svc, ctx, state.ContextID, projectops.NewProjectRequest{
		ProjectName:       "MrgCalTgt",
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	cid := tgtRes.State.ContextID

	stgRes, err := projectops.CreateNewProject(svc, ctx, cid, projectops.NewProjectRequest{
		ProjectName: "MrgCalStg",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Valid 99.999996 StrataCoverTree and StrataCoverTotal must merge cleanly.
	vprotest.MutateContextFixture(t, stgRes.DestinationPath, `
		INSERT INTO "MrgCalStg_Env" ("PlotNumber","Zone","StrataCoverTree") VALUES ('P001','CWH',99.999996);
		INSERT INTO "MrgCalStg_Admin" ("Plot","PlotType","StrataCoverTotal") VALUES ('P001','FS882',99.999996);
		INSERT INTO "MrgCalStg_Other" ("PlotNumber","ID","DataName","DataItem") VALUES ('P001',1,'NoteName','Valid text');
	`)
	if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:    stgRes.DestinationPath,
		SourceProject: "MrgCalStg",
	}); err != nil {
		t.Fatalf("expected valid 99.999996 StrataCover* merge to succeed, got: %v", err)
	}

	// 99.999999 StrataCoverTree (narrows to float32 100) must reject!
	vprotest.MutateContextFixture(t, stgRes.DestinationPath, `
		UPDATE "MrgCalStg_Env" SET "StrataCoverTree" = 99.999999 WHERE "PlotNumber" = 'P001';
	`)
	if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:                stgRes.DestinationPath,
		SourceProject:             "MrgCalStg",
		DefaultConflictResolution: projectops.MergeResolutionAcceptSource,
	}); err == nil {
		t.Fatalf("expected StrataCoverTree=99.999999 in merge to be rejected")
	}

	// Reset StrataCoverTree, test StrataCoverTotal = -0.0001 -> must reject!
	vprotest.MutateContextFixture(t, stgRes.DestinationPath, `
		UPDATE "MrgCalStg_Env" SET "StrataCoverTree" = 50.0 WHERE "PlotNumber" = 'P001';
		UPDATE "MrgCalStg_Admin" SET "StrataCoverTotal" = -0.0001 WHERE "Plot" = 'P001';
	`)
	if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:                stgRes.DestinationPath,
		SourceProject:             "MrgCalStg",
		DefaultConflictResolution: projectops.MergeResolutionAcceptSource,
	}); err == nil {
		t.Fatalf("expected StrataCoverTotal=-0.0001 in merge to be rejected")
	}

	// Reset StrataCoverTotal, test Env.Zone > 4 UTF-16 units and Other.DataName NUL byte -> must reject!
	vprotest.MutateContextFixture(t, stgRes.DestinationPath, `
		UPDATE "MrgCalStg_Admin" SET "StrataCoverTotal" = 50.0 WHERE "Plot" = 'P001';
		UPDATE "MrgCalStg_Env" SET "Zone" = 'ABCDE' WHERE "PlotNumber" = 'P001';
	`)
	if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:                stgRes.DestinationPath,
		SourceProject:             "MrgCalStg",
		DefaultConflictResolution: projectops.MergeResolutionAcceptSource,
	}); err == nil {
		t.Fatalf("expected Env.Zone='ABCDE' (>4 UTF-16 units) in merge to be rejected")
	}

	vprotest.MutateContextFixture(t, stgRes.DestinationPath, `
		UPDATE "MrgCalStg_Env" SET "Zone" = 'CWH' WHERE "PlotNumber" = 'P001';
		UPDATE "MrgCalStg_Other" SET "DataItem" = 'bad' || char(0) || 'nul' WHERE "PlotNumber" = 'P001';
	`)
	if _, err := projectops.ExecuteThreeWayMerge(svc, ctx, cid, projectops.ThreeWayMergeExecuteRequest{
		SourcePath:                stgRes.DestinationPath,
		SourceProject:             "MrgCalStg",
		DefaultConflictResolution: projectops.MergeResolutionAcceptSource,
	}); err == nil {
		t.Fatalf("expected NUL byte in Other.DataItem during merge to be rejected")
	}
	_ = vprocore.FS882Header{}
}
