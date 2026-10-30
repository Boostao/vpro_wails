package main

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
	service, state := contextServiceFixture(t)
	inspection, err := service.InspectExternalSQLiteFile(context.Background(), state.ContextID, state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !sameDesktopPath(inspection.SelectedPath, state.ProjectPath) {
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
	service, state := contextServiceFixture(t)
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
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\VPro64.accdb`,
			sourceProject: "Sample",
			targetProject: "ImportedVP08",
			wantVersion:   "VP08",
			wantPlots:     52,
			activate:      true,
		},
		{
			name:          "VP08_Haida_Gwaii_2019",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\Haida_Gwaii_2019.accdb`,
			sourceProject: "Haida_Gwaii_VPro19_2019",
			targetProject: "Haida2019",
			wantVersion:   "VP08",
			wantPlots:     21,
			activate:      true,
		},
		{
			name:          "VP08_Manning_TEM",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS_1\VPro64\Manning_TEM_Vpro19.accdb`,
			sourceProject: "Manning_TEM_Vpro19",
			targetProject: "ManningTEM",
			wantVersion:   "VP08",
			wantPlots:     61,
			activate:      true,
		},
		{
			name:          "VP07_TemplateVPro15",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro15.accdb`,
			sourceProject: "VPro15",
			targetProject: "ConvertedVP07",
			wantVersion:   "VP07",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP06_TemplateVPro13",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro13.accdb`,
			sourceProject: "VPro13",
			targetProject: "ConvertedVP06",
			wantVersion:   "VP06",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP05_TemplateVPro03",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVPro03.mdb`,
			sourceProject: "VPro03",
			targetProject: "ConvertedVP05",
			wantVersion:   "VP05",
			wantPlots:     0,
			activate:      false,
		},
		{
			name:          "VP05_TemplateVProXP",
			sourcePath:    `C:\Users\BrunoTremblay\Work\VPRO_ACCESS\VPro64\Templates\TemplateVProXP.mdb`,
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
			inspection, err := service.InspectAccessDatabase(context.Background(), currentContextID, tc.sourcePath)
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
			res, err := service.ImportAccessProject(context.Background(), currentContextID, AccessProjectImportRequest{
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
				page, err := service.projects.ListPlots(context.Background(), 0, 10)
				if err != nil || page.Total != tc.wantPlots {
					t.Fatalf("GetPlots after activation: total=%d want=%d err=%v", page.Total, tc.wantPlots, err)
				}
			}
		})
	}
}
