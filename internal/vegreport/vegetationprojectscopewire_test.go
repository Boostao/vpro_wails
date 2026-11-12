package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type projectScopeWireCase struct {
	Scope           LongVegetationReportScope             `json:"scope"`
	Catalogue       LongVegetationProjectFieldDefinitions `json:"catalogue"`
	UnitPreferences LongVegetationProjectUnitPreferences  `json:"unitPreferences"`
	Outcome         LongVegetationProjectUnitOutcome      `json:"outcome"`
}

func TestProjectUnitRequestStrictClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationProjectUnitRequestWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var requests []vegetationProjectUnitRequest
	if err := json.Unmarshal(raw, &requests); err != nil || len(requests) != 2 {
		t.Fatal("project unit fixture requires two strict production-Go Save requests", err)
	}
	for i, request := range requests {
		expected := vegetationProjectUnitRequest{Revision: "3", ApprovalHash: strings.Repeat(string(rune('a'+i)), 64),
			Expected: LongVegetationUnitPreferenceValues{UnitGroups: 1}, Proposed: LongVegetationUnitPreferenceValues{UnitGroups: i + 2}}
		if !reflect.DeepEqual(request, expected) {
			t.Fatal("frontend project Unit request differs from strict production boundary", request, expected)
		}
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		var reopened vegetationProjectUnitRequest
		if err := json.Unmarshal(encoded, &reopened); err != nil || !reflect.DeepEqual(reopened, expected) {
			t.Fatal("strict project unit request changed after production-Go serialization", err)
		}
	}
}

func TestProjectScopeStrictClientWireMatchesPhysicalProjectionAndProductionDTOs(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationProjectScopeWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected []projectScopeWireCase
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil || len(expected) != 3 {
		t.Fatal("project scope fixture must contain three strict production DTO cases", err)
	}
	env := vprocore.ProjectMetadataTable{
		Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT"}, {Name: "Zone", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Alpha")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Beta")}},
		},
	}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Plot", DeclaredType: "TEXT"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "21", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
			{RowID: "22", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1")}},
		}}
	for i, source := range []string{vprocore.VegetationProjectFieldsScope, vprocore.VegetationProjectAllScope, vprocore.VegetationProjectFieldsScope} {
		fields, su, suPath := []string{"Zone"}, "None", ""
		tuples := []vprocore.LongVegetationProjectUnit{}
		if i == 2 {
			env.Rows[1] = vprocore.ProjectMetadataRow{RowID: "13", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2"), vprotest.MetadataText("Orphan")}}
			admin.Rows[1] = vprocore.ProjectMetadataRow{RowID: "23", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2")}}
			su, suPath = "Selected", `C:\external.db`
		}
		if source == vprocore.VegetationProjectFieldsScope {
			rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, fields)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				id := row.AdminRowID
				tuples = append(tuples, vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: &id, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit})
			}
		} else {
			rows, err := planLongVegetationProjectNoneUnits(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				tuples = append(tuples, vprocore.LongVegetationProjectUnit{EnvRowID: row.EnvRowID, AdminRowID: nil, PlotNumber: row.PlotNumber, SiteUnit: row.SiteUnit})
			}
			fields = []string{"MissingField"}
		}
		owner := &vprocore.SqliteContext{Selection: vprocore.DesktopSelection{
			Project: "Sample", ProjectPath: `C:\project.db`, SU: su, SUPath: suPath,
		}}
		// Literal digests test the wire; complete real hash authority is covered by owned scope tests.
		build := &vprocore.LongVegetationProjectBuild{ApprovalHash: strings.Repeat(string(rune('a'+i)), 64), ProjectTuples: tuples}
		scope := projectScopeDTO("owned", owner, vprocore.VegetationProjectScopeEntry{Revision: 3, SourceKind: source, Fields: fields, Build: build})
		scope.BuildCurrent = true
		preferences := LongVegetationUnitPreferencesReview{
			ContextID: "owned", Project: "Sample", ProjectPath: `C:\project.db`, SU: su, SUPath: suPath,
			Values:       LongVegetationUnitPreferenceValues{UnitGroups: 1},
			Availability: LongVegetationUnitAvailability{SelectedFields: source == vprocore.VegetationProjectFieldsScope, None: source == vprocore.VegetationProjectAllScope},
		}
		actual := projectScopeWireCase{
			Scope: *scope,
			Catalogue: LongVegetationProjectFieldDefinitions{
				ContextID: "owned", Project: "Sample", ProjectPath: `C:\project.db`, SU: su, SUPath: suPath,
				SourceKind: source, Revision: "3", Definitions: reportUnitFieldDefinitions(env, admin),
			},
			UnitPreferences: LongVegetationProjectUnitPreferences{Scope: *scope, Preferences: preferences},
			Outcome: LongVegetationProjectUnitOutcome{SourceKind: source, Revision: "3",
				Outcome: LongVegetationUnitPreferencesOutcome{ContextID: "owned", Changed: true, Committed: true}},
		}
		if !reflect.DeepEqual(actual, expected[i]) {
			t.Fatal("independent scope wire differs from original physical projection or production DTO", i, actual, expected[i])
		}
		encoded, err := json.Marshal(actual)
		if err != nil {
			t.Fatal(err)
		}
		var reopened projectScopeWireCase
		if err := vprocore.DecodeProfileLifecycleJSON(encoded, &reopened, "scope", "catalogue", "unitPreferences", "outcome"); err != nil ||
			!reflect.DeepEqual(reopened, expected[i]) {
			t.Fatal("strict production scope serialization changed complete wrapper ownership", i, err)
		}
	}
}
