package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func summaryUnitTables(raw []ProjectMetadataCell, members [][2]ProjectMetadataCell) (ProjectMetadataTable, ProjectMetadataTable) {
	veg := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}}}
	su := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}}}
	for i, plot := range raw {
		veg.Rows = append(veg.Rows, ProjectMetadataRow{RowID: fmt.Sprint(i + 1), Cells: []ProjectMetadataCell{plot}})
	}
	for i, member := range members {
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: fmt.Sprint(i + 1), Cells: []ProjectMetadataCell{member[0], member[1]}})
	}
	return veg, su
}

func TestSummaryVegetationUnitsRawAdmissionAndOriginalCounts(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	veg, su := summaryUnitTables(
		[]ProjectMetadataCell{metadataText("P1"), metadataText("P1"), metadataText("P3"), null},
		[][2]ProjectMetadataCell{
			{metadataText("P1"), metadataText("A")}, {metadataText("P1"), metadataText("A")},
			{metadataText("P2"), metadataText("A")}, {null, metadataText("A")},
			{metadataText("P3"), metadataText("B")}, {metadataText("ABSENT"), metadataText("Orphan")},
			{metadataText("P1"), null}, {null, metadataText("NullPlot")},
		})
	result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
	expected := []summaryVegetationUnit{
		{Code: "A", NumPlots: 3, MembershipIDs: []string{"1", "2", "3", "4"}, RawVegRowIDs: []string{"1", "2"}},
		{Code: "B", NumPlots: 1, MembershipIDs: []string{"5"}, RawVegRowIDs: []string{"3"}},
	}
	if err != nil || !reflect.DeepEqual(result, expected) {
		t.Fatal("SV1 counts PlotNumber, raw rows admit but never multiply denominator", result, err)
	}
	result[0].MembershipIDs[0], result[0].RawVegRowIDs[0] = "changed", "changed"
	*su.Rows[0].Cells[1].Text = "different"
	if *veg.Rows[0].Cells[0].Text != "P1" || result[0].Code != "A" || su.Rows[0].RowID != "1" {
		t.Fatal("unit results alias physical source", result, su, veg)
	}
}

func TestSummaryVegetationUnitsEmptyAndLiteralReadModel(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	for _, members := range [][][2]ProjectMetadataCell{
		nil,
		{{metadataText("P"), null}},
		{{metadataText("Absent"), metadataText("A")}},
		{{null, metadataText("A")}},
	} {
		veg, su := summaryUnitTables([]ProjectMetadataCell{metadataText("P")}, members)
		result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
		if err != nil || result == nil || len(result) != 0 {
			t.Fatal("no admitted unit must be an explicit empty read model, not a workbook", result, err)
		}
	}
	veg, su := summaryUnitTables([]ProjectMetadataCell{metadataText("")},
		[][2]ProjectMetadataCell{{metadataText(""), metadataText("")}, {metadataText("p"), metadataText("literal")}})
	result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
	if err != nil || !reflect.DeepEqual(result, []summaryVegetationUnit{
		{Code: "", NumPlots: 1, MembershipIDs: []string{"1"}, RawVegRowIDs: []string{"1"}},
	}) {
		t.Fatal("literal empty identities are not NULL or case-repaired", result, err)
	}
}

func TestSummaryVegetationUnitsOriginalGroupLimit(t *testing.T) {
	for _, count := range []int{249, 250, 251} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			members := [][2]ProjectMetadataCell{}
			for i := 0; i < count-1; i++ {
				members = append(members, [2]ProjectMetadataCell{metadataText("P"), metadataText(fmt.Sprintf("U%03d", i))})
			}
			members = append(members, [2]ProjectMetadataCell{{Storage: "null"}, {Storage: "null"}})
			veg, su := summaryUnitTables([]ProjectMetadataCell{metadataText("P")}, members)
			result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
			if count <= 250 {
				if err != nil || len(result) != count-1 {
					t.Fatal("source limit includes the NULL group exactly once", count, result, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), ">250") || result != nil {
				t.Fatal("251 original groups must refuse before partial output", result, err)
			}
			veg.Rows = nil
			result, err = prepareSummaryVegetationUnits(context.Background(), veg, su)
			if count == 251 && (err == nil || result != nil) {
				t.Fatal("orphan groups still count toward original source preflight", result, err)
			}
		})
	}
	members := [][2]ProjectMetadataCell{}
	for i := 0; i < 251; i++ {
		members = append(members, [2]ProjectMetadataCell{metadataText("P"), metadataText("A")})
	}
	veg, su := summaryUnitTables([]ProjectMetadataCell{metadataText("P")}, members)
	result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
	if err != nil || len(result) != 1 || result[0].NumPlots != 251 || len(result[0].MembershipIDs) != 251 {
		t.Fatal("250 is a unit-column limit, not a plot/member limit", result, err)
	}
}

func TestSummaryVegetationUnitsStrictInputAndCancellation(t *testing.T) {
	valid := metadataText("P")
	for _, bad := range []ProjectMetadataCell{
		metadataInteger("1"), metadataText(string([]byte{0xff})),
		{Storage: "null", Text: new(string)}, {Storage: "text"},
	} {
		for _, position := range []string{"raw", "plot", "unit"} {
			veg, su := summaryUnitTables([]ProjectMetadataCell{valid}, [][2]ProjectMetadataCell{{valid, metadataText("A")}})
			switch position {
			case "raw":
				veg.Rows[0].Cells[0] = bad
			case "plot":
				su.Rows[0].Cells[0] = bad
			case "unit":
				su.Rows[0].Cells[1] = bad
			}
			result, err := prepareSummaryVegetationUnits(context.Background(), veg, su)
			if err == nil || result != nil {
				t.Fatal("malformed identity cannot produce successful partial units", position, bad, result, err)
			}
		}
	}
	veg, su := summaryUnitTables([]ProjectMetadataCell{valid}, [][2]ProjectMetadataCell{{valid, metadataText("A")}})
	for _, bad := range []ProjectMetadataTable{
		{Rows: veg.Rows}, {Columns: veg.Columns, Rows: append(veg.Rows, veg.Rows[0])},
		{Columns: veg.Columns, Rows: []ProjectMetadataRow{{RowID: "01", Cells: veg.Rows[0].Cells}}},
		{Columns: veg.Columns, Rows: []ProjectMetadataRow{{RowID: "1"}}},
	} {
		if result, err := prepareSummaryVegetationUnits(context.Background(), bad, su); err == nil || result != nil {
			t.Fatal("strict raw schema/physical identities lost", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := prepareSummaryVegetationUnits(ctx, veg, su)
	if !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled request returned units", result, err)
	}
	if result, err := prepareSummaryVegetationUnits(nil, veg, su); err == nil || result != nil {
		t.Fatal("nil context accepted", result, err)
	}
}

func TestSummaryVegetationUnitsCancellationEveryObservation(t *testing.T) {
	veg, su := summaryUnitTables([]ProjectMetadataCell{metadataText("P"), metadataText("P")},
		[][2]ProjectMetadataCell{{metadataText("P"), metadataText("A")}, {metadataText("P"), metadataText("A")},
			{metadataText("Absent"), metadataText("B")}})
	probe := &projectUnitCancellationContext{Context: context.Background()}
	if result, err := prepareSummaryVegetationUnits(probe, veg, su); err != nil || len(result) != 1 || probe.calls < 2 {
		t.Fatal("cancellation probe did not observe complete admission", result, err, probe.calls)
	}
	for point := 1; point <= probe.calls; point++ {
		ctx := &projectUnitCancellationContext{Context: context.Background(), cancel: point}
		if result, err := prepareSummaryVegetationUnits(ctx, veg, su); !errors.Is(err, context.Canceled) || result != nil {
			t.Fatal("in-flight cancellation leaked partial units", point, result, err)
		}
	}
}
