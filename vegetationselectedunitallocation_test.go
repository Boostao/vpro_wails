package main

import (
	"context"
	"runtime"
	"strconv"
	"testing"
)

func TestSelectedUnitMultiplicityDiagnosticsHaveBoundedAllocation(t *testing.T) {
	veg, su, layers, master, env, admin := selectedUnitReportFixture(t)
	env.Rows = env.Rows[:1]
	su.Rows = nil
	for i := 1; i <= 2000; i++ {
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i),
			Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("original")}})
	}
	prepared, _, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	prepared.Observations = prepared.Observations[:1]
	prepared.ReducedRows = prepared.ReducedRows[:1]
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	result, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
	runtime.ReadMemStats(&after)
	if err != nil || len(result.Units) != 1 || result.Units[0].NumPlots != 4000 ||
		len(result.Units[0].MembershipIDs) != 2000 {
		t.Fatal("independent2000 original memberships/two derived joins", len(result.Units), err)
	}
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "selected_unit_join_multiplicity" {
			count++
			if diagnostic.Count != 2 {
				t.Fatal("completed derived count was not used", diagnostic)
			}
		}
	}
	if count != 2000 {
		t.Fatal("each original membership requires exactly one multiplicity diagnostic", count)
	}
	allocated := after.TotalAlloc - before.TotalAlloc
	if allocated > 32<<20 {
		t.Fatalf("kernel allocated %d bytes for4000 derived joins; limit32MiB refuses quadratic diagnostic accumulation", allocated)
	}
	t.Logf("kernel allocated %d bytes for4000 joins and2000 diagnostics", allocated)
}
