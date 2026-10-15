package main

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
	env := ProjectMetadataTable{
		Columns: []ProjectMetadataColumn{{"PlotNumber", "TEXT"}, {"Zone", "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "11", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Alpha")}},
			{RowID: "12", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Beta")}},
		},
	}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{"Plot", "TEXT"}},
		Rows: []ProjectMetadataRow{
			{RowID: "21", Cells: []ProjectMetadataCell{metadataText("P1")}},
			{RowID: "22", Cells: []ProjectMetadataCell{metadataText("P1")}},
		}}
	for i, source := range []string{vegetationProjectFieldsScope, vegetationProjectAllScope, vegetationProjectFieldsScope} {
		fields, su, suPath := []string{"Zone"}, "None", ""
		tuples := []LongVegetationProjectUnit{}
		if i == 2 {
			env.Rows[1] = ProjectMetadataRow{RowID: "13", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Orphan")}}
			admin.Rows[1] = ProjectMetadataRow{RowID: "23", Cells: []ProjectMetadataCell{metadataText("P2")}}
			su, suPath = "Selected", `C:\external.db`
		}
		if source == vegetationProjectFieldsScope {
			rows, err := planLongVegetationProjectSelectedUnits(context.Background(), env, admin, fields)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				id := row.AdminRowID
				tuples = append(tuples, LongVegetationProjectUnit{row.EnvRowID, &id, row.PlotNumber, row.SiteUnit})
			}
		} else {
			rows, err := planLongVegetationProjectNoneUnits(context.Background(), env)
			if err != nil {
				t.Fatal(err)
			}
			for _, row := range rows {
				tuples = append(tuples, LongVegetationProjectUnit{row.EnvRowID, nil, row.PlotNumber, row.SiteUnit})
			}
			fields = []string{"MissingField"}
		}
		owner := &sqliteContext{selection: desktopSelection{
			Project: "Sample", ProjectPath: `C:\project.db`, SU: su, SUPath: suPath,
		}}
		// Literal digests test the wire; complete real hash authority is covered by owned scope tests.
		build := &LongVegetationProjectBuild{ApprovalHash: strings.Repeat(string(rune('a'+i)), 64), ProjectTuples: tuples}
		scope := projectScopeDTO("owned", owner, vegetationProjectScopeEntry{revision: 3, sourceKind: source, fields: fields, build: build})
		scope.BuildCurrent = true
		preferences := LongVegetationUnitPreferencesReview{
			ContextID: "owned", Project: "Sample", ProjectPath: `C:\project.db`, SU: su, SUPath: suPath,
			Values:       LongVegetationUnitPreferenceValues{UnitGroups: 1},
			Availability: LongVegetationUnitAvailability{SelectedFields: source == vegetationProjectFieldsScope, None: source == vegetationProjectAllScope},
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
		if err := decodeProfileLifecycleJSON(encoded, &reopened, "scope", "catalogue", "unitPreferences", "outcome"); err != nil ||
			!reflect.DeepEqual(reopened, expected[i]) {
			t.Fatal("strict production scope serialization changed complete wrapper ownership", i, err)
		}
	}
}
