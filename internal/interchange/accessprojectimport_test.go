package interchange

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func firstExistingPath(paths ...string) string {
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return paths[0]
}

func TestConvertLongLatToDeg(t *testing.T) {
	if got := convertLongLatToDeg(nil, int64(30), float64(0)); got != nil {
		t.Fatalf("expected nil when degrees is nil, got %v", got)
	}
	got, ok := convertLongLatToDeg(float64(123), float64(30), float64(0)).(float64)
	if !ok || math.Abs(got-123.5) > 1e-9 {
		t.Fatalf("expected 123.5, got %v", got)
	}
	got2, ok := convertLongLatToDeg(float64(49), nil, nil).(float64)
	if !ok || math.Abs(got2-49.0) > 1e-9 {
		t.Fatalf("expected 49.0, got %v", got2)
	}
}

func TestInspectExternalSQLiteFile(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	inspection, err := service.InspectExternalSQLiteFile(context.Background(), state.ContextID, state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !vprocore.SameDesktopPath(inspection.SelectedPath, state.ProjectPath) {
		t.Fatalf("selectedPath = %q, want %q", inspection.SelectedPath, state.ProjectPath)
	}
	if len(inspection.Projects) != 1 || inspection.Projects[0].Name != "Sample" || !inspection.Projects[0].Compatible {
		t.Fatalf("unexpected projects: %+v", inspection.Projects)
	}
	if len(inspection.SUs) == 0 || inspection.SUs[0].Name != "Sample" {
		t.Fatalf("unexpected SUs: %+v", inspection.SUs)
	}
}

func TestInspectAndImportAccessProjectsAllVersions(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	destDir := t.TempDir()

	cases := []struct {
		name          string
		sourcePath    string
		sourceProject string
		targetProject string
		wantVersion   string
		wantPlots     int
		activate      bool
	}{
		{
			name:          "VP08_VPro64_Sample",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/VPro64.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\VPro64.accdb`),
			sourceProject: "Sample",
			targetProject: "ImportedVP08",
			wantVersion:   "VP08",
			wantPlots:     52,
			activate:      true,
		},
		{
			name:          "VP08_Haida_Gwaii_2019",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS_1/VPro64/Haida_Gwaii_2019.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\Haida_Gwaii_2019.accdb`),
			sourceProject: "Haida_Gwaii_VPro19_2019",
			targetProject: "Haida2019",
			wantVersion:   "VP08",
			wantPlots:     21,
			activate:      true,
		},
		{
			name:          "VP08_Manning_TEM",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS_1/VPro64/Manning_TEM_Vpro19.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\Manning_TEM_Vpro19.accdb`),
			sourceProject: "Manning_TEM_Vpro19",
			targetProject: "ManningTEM",
			wantVersion:   "VP08",
			wantPlots:     61,
			activate:      true,
		},
		{
			name:          "VP07_TemplateVPro15",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/Templates/TemplateVPro15.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro15.accdb`),
			sourceProject: "VPro15",
			targetProject: "ConvertedVP07",
			wantVersion:   "VP07",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP06_TemplateVPro13",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/Templates/TemplateVPro13.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro13.accdb`),
			sourceProject: "VPro13",
			targetProject: "ConvertedVP06",
			wantVersion:   "VP06",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP05_TemplateVPro03",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/Templates/TemplateVPro03.mdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro03.mdb`),
			sourceProject: "VPro03",
			targetProject: "ConvertedVP05",
			wantVersion:   "VP05",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP05_TemplateVProXP",
			sourcePath:    firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/Templates/TemplateVProXP.mdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVProXP.mdb`),
			sourceProject: "VProXP",
			targetProject: "ConvertedXP",
			wantVersion:   "VP05",
			wantPlots:     0,
			activate:      false,
		},
	}

	currentContextID := state.ContextID
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.sourcePath); err != nil {
				t.Skipf("fixture %s not present: %v", tc.sourcePath, err)
			}
			inspection, err := InspectAccessDatabase(service, context.Background(), currentContextID, tc.sourcePath)
			if err != nil {
				t.Fatalf("InspectAccessDatabase: %v", err)
			}
			var found *AccessProjectCandidate
			for i := range inspection.Projects {
				if strings.EqualFold(inspection.Projects[i].SourceProject, tc.sourceProject) {
					found = &inspection.Projects[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("project %s not found in %+v", tc.sourceProject, inspection.Projects)
			}
			if !found.Compatible || found.SourceVersion != tc.wantVersion || found.PlotCount != tc.wantPlots {
				t.Fatalf("unexpected candidate metadata: %+v", *found)
			}

			destFile := filepath.Join(destDir, tc.targetProject+".db")
			res, err := ImportAccessProject(service, context.Background(), currentContextID, AccessProjectImportRequest{
				SourcePath:        tc.sourcePath,
				SourceProject:     tc.sourceProject,
				TargetProject:     tc.targetProject,
				DestinationPath:   destFile,
				ActivateOnSuccess: tc.activate,
			})
			if err != nil {
				t.Fatalf("ImportAccessProject: %v", err)
			}
			if res.TargetProject != tc.targetProject || res.PlotCount != tc.wantPlots || res.SourceVersion != tc.wantVersion {
				t.Fatalf("unexpected import result: %+v", res)
			}
			if tc.activate {
				if !res.Activated || res.State == nil || res.State.ActiveProject != tc.targetProject {
					t.Fatalf("expected activated state for %s, got %+v", tc.targetProject, res.State)
				}
				currentContextID = res.State.ContextID
				page, err := service.Projects.ListPlots(context.Background(), 0, 10)
				if err != nil || page.Total != tc.wantPlots {
					t.Fatalf("GetPlots after activation: total=%d want=%d err=%v", page.Total, tc.wantPlots, err)
				}
			}
		})
	}
}

func TestImportAccessProjectAuditOwnershipChildIdentityAndCompanions(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)
	srcPath := firstExistingPath(`/home/bruno/Work/VPRO_ACCESS/VPro64/VPro64.accdb`, `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\VPro64.accdb`)
	if _, err := os.Stat(srcPath); err != nil {
		t.Skipf("fixture %s not present: %v", srcPath, err)
	}

	destFile := filepath.Join(t.TempDir(), "RenamedTarget.db")
	res, err := ImportAccessProject(service, ctx, state.ContextID, AccessProjectImportRequest{
		SourcePath:        srcPath,
		SourceProject:     "Sample",
		TargetProject:     "RenamedTarget",
		DestinationPath:   destFile,
		ActivateOnSuccess: true,
	})
	if err != nil {
		t.Fatalf("ImportAccessProject(Sample -> RenamedTarget): %v", err)
	}
	activeCtxID := res.State.ContextID

	// 1. Audit Project ownership normalized to "RenamedTarget" so ListAuditEntries & ListAuditReadEntries succeed!
	entries, err := service.ListAuditEntries(ctx, activeCtxID, "00337")
	if err != nil || len(entries) == 0 {
		t.Fatalf("ListAuditEntries('00337') failed on renamed Access import: entries=%d err=%v", len(entries), err)
	}
	readEntries, err := service.ListAuditReadEntries(ctx, activeCtxID, "00337")
	if err != nil || len(readEntries) == 0 {
		t.Fatalf("ListAuditReadEntries('00337') failed on renamed Access import: entries=%d err=%v", len(readEntries), err)
	}

	db, err := sqlutil.OpenReadOnly(destFile)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var foreignAuditCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM "RenamedTarget_Audit" WHERE "Project" IS NULL OR "Project" <> 'RenamedTarget'`).Scan(&foreignAuditCount); err != nil || foreignAuditCount != 0 {
		t.Fatalf("expected 0 foreign Project rows in RenamedTarget_Audit, got %d (err=%v)", foreignAuditCount, err)
	}

	// 2. Binary PlotNumber index on <Target>_Audit must exist!
	var hasAuditIdx bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='index' AND name='idx_vpro_bin_RenamedTarget_Audit_PlotNumber')`).Scan(&hasAuditIdx); err != nil || !hasAuditIdx {
		t.Fatalf("missing idx_vpro_bin_RenamedTarget_Audit_PlotNumber on RenamedTarget.db (err=%v)", err)
	}

	// 3. __VPRO_ChildIdentity must exist and seed positive IDs from _Veg, _Humus, _Mineral, _Other, _Metadata, and _Audit!
	for _, suffix := range []string{"Veg", "Humus", "Mineral"} {
		qChild := vprocore.QuoteHeaderIdentifier("RenamedTarget_" + suffix)
		var cnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ChildTable" = ? AND "ID" > 0`, qChild).Scan(&cnt); err != nil || cnt == 0 {
			t.Fatalf("expected seeded __VPRO_ChildIdentity entries for %s, got %d (err=%v)", qChild, cnt, err)
		}
	}

	// 4. Companion tables *_Hierarchy (VP04), *_Lump (VP05), and *_Profile (VP05-2 / VP05) must be discovered and imported!
	for _, comp := range []struct {
		table      string
		descPrefix string
	}{
		{"Sample_Hierarchy", "VP04"},
		{"Sample_Lump", "VP05"},
		{"Sample_Profile", "VP05"},
	} {
		var rowCnt int
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + vprocore.QuoteHeaderIdentifier(comp.table)).Scan(&rowCnt); err != nil || rowCnt == 0 {
			t.Fatalf("expected imported companion table %s with rows > 0, got %d (err=%v)", comp.table, rowCnt, err)
		}
		var desc string
		if err := db.QueryRow(`SELECT "description" FROM "_table_metadata" WHERE "table_name" = ?`, comp.table).Scan(&desc); err != nil || !strings.HasPrefix(desc, comp.descPrefix) {
			t.Fatalf("companion table %s _table_metadata description = %q (err=%v), want prefix %q", comp.table, desc, err, comp.descPrefix)
		}
	}
}

func TestSeedImportedChildIdentitiesBlocksDeletedAuditIDReuse(t *testing.T) {
	ctx := context.Background()
	service, state := vprotest.ContextServiceFixture(t)

	// Simulate an imported project where Sample_Veg only has ID=2, while Sample_Audit records deleted ID=1!
	vprotest.MutateContextFixture(t, state.ProjectPath, `
		DELETE FROM Sample_Veg;
		DROP TABLE IF EXISTS "__VPRO_ChildIdentity";
		INSERT INTO Sample_Veg (PlotNumber, Species, ID, Cover1, Flag) VALUES ('00337', 'PSEUMEN', 2, 25.0, 0);
		INSERT INTO Sample_Audit ("Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "Restore", "Flag", "ID")
		VALUES ('Sample', 'VPro', '00337', '_Veg', 'Delete record', '2024-01-01 00:00:00', 0, 0, 1);
	`)

	rwDB, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := rwDB.BeginTx(ctx, nil)
	if err != nil {
		rwDB.Close()
		t.Fatal(err)
	}
	if err := seedImportedChildIdentities(ctx, tx, "OldSource", "Sample"); err != nil {
		_ = tx.Rollback()
		rwDB.Close()
		t.Fatalf("seedImportedChildIdentities: %v", err)
	}
	if err := tx.Commit(); err != nil {
		rwDB.Close()
		t.Fatal(err)
	}
	rwDB.Close()

	// Refresh context file profile after direct test setup mutation, then create a new Veg row (ID=0 -> auto-allocate):
	// AllocateChildID MUST skip deleted historical ID=1 and existing ID=2, assigning ID=3!
	state, err = service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	cov := 12.0
	if err := service.Edit(state.ContextID, func(plots *vprocore.PlotService) error {
		return plots.SaveVegRecord(vprocore.VegRecord{
			PlotNumber: "00337",
			Species:    "TSUGHET",
			Cover1:     &cov,
		})
	}); err != nil {
		t.Fatalf("SaveVegRecord: %v", err)
	}

	roDB, err := sqlutil.OpenReadOnly(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	defer roDB.Close()
	var newID int64
	if err := roDB.QueryRow(`SELECT "ID" FROM Sample_Veg WHERE PlotNumber='00337' AND Species='TSUGHET'`).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	if newID == 1 || newID != 3 {
		t.Fatalf("deleted historical child ID=1 was reused or unexpected ID allocated: got %d, want 3", newID)
	}
}
