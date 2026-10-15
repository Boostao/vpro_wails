package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func summaryFieldListPlanFixture(t *testing.T) ProjectMetadataTable {
	t.Helper()
	db, _ := summaryFieldListFixture(t)
	if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES
		(1,'Env','duplicate','A',0,'sum','F1'),
		(1,'Env','duplicate','A',0,'sum','F1'),
		(-1,'Admin','hidden','','-1',NULL,'H'),
		(NULL,NULL,'NULL order',NULL,-1,NULL,NULL),
		(2,'Env',NULL,NULL,NULL,NULL,'F2')`); err != nil {
		t.Fatal(err)
	}
	table, err := summaryFieldListRead(t, db, t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return table
}

func TestSummaryVegetationFieldListPlanSourceGlobalAndPhysicalDuplicateActions(t *testing.T) {
	table := summaryFieldListPlanFixture(t)
	for _, test := range []struct {
		name        string
		actions     []summaryVegetationFieldListAction
		reports     []any
		assignments []string
	}{
		{"none", []summaryVegetationFieldListAction{{Kind: "none"}}, []any{int64(0), int64(0), int64(0), int64(0), int64(0)}, []string{"3", "4", "5"}},
		{"all", []summaryVegetationFieldListAction{{Kind: "all", Selected: true}}, []any{int64(-1), int64(-1), int64(-1), int64(-1), int64(-1)}, []string{"1", "2", "5"}},
		{"one duplicate", []summaryVegetationFieldListAction{{Kind: "row", RowID: "2", Selected: true}}, []any{int64(0), int64(-1), int64(-1), int64(-1), nil}, []string{"2"}},
		{"all then row", []summaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "row", RowID: "1"}}, []any{int64(0), int64(-1), int64(-1), int64(-1), int64(-1)}, []string{"2", "5"}},
		{"all then none", []summaryVegetationFieldListAction{{Kind: "all", Selected: true}, {Kind: "none"}}, []any{int64(0), int64(0), int64(0), int64(0), int64(0)}, []string{"3", "4", "5"}},
		{"row away back", []summaryVegetationFieldListAction{{Kind: "row", RowID: "1", Selected: true}, {Kind: "row", RowID: "1"}}, []any{int64(0), int64(0), int64(-1), int64(-1), nil}, []string{}},
		{"empty", []summaryVegetationFieldListAction{}, []any{int64(0), int64(0), int64(-1), int64(-1), nil}, []string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := planSummaryVegetationFieldList(t.Context(), table, table, test.actions)
			if err != nil || !reflect.DeepEqual(plan.Original, table) || !reflect.DeepEqual(plan.Proposed.Columns, table.Columns) {
				t.Fatal("source plan failed or altered schema/original", plan, err)
			}
			ids := []string{}
			for _, assignment := range plan.Assignments {
				ids = append(ids, assignment.Before.RowID)
				if len(assignment.Before.Cells) != 7 || len(assignment.After.Cells) != 7 || assignment.Before.RowID != assignment.After.RowID {
					t.Fatal("assignment lacks complete immutable physical row witness", assignment)
				}
			}
			if !reflect.DeepEqual(ids, test.assignments) {
				t.Fatal("source plan assignments collapsed physical duplicates or lost hidden rows", ids, test.assignments)
			}
			for i, row := range table.Rows {
				want := append([]ProjectMetadataCell(nil), row.Cells...)
				want[4] = summaryFieldListExpected(t, []any{test.reports[i]})[0]
				if plan.Proposed.Rows[i].RowID != row.RowID || !reflect.DeepEqual(plan.Proposed.Rows[i].Cells, want) {
					t.Fatal("independent full proposed row differs", i, plan.Proposed.Rows[i], want)
				}
			}
		})
	}
}

func TestSummaryVegetationFieldListPlanStaleFullWitnessAndActualRowidReuse(t *testing.T) {
	db, _ := summaryFieldListFixture(t)
	if _, err := db.Exec(`INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES(1,'Env','original','A',0,'sum','F1')`); err != nil {
		t.Fatal(err)
	}
	expected, err := summaryFieldListRead(t, db, t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM VPro64.USysShortVegSummaryFieldList;
		INSERT INTO VPro64.USysShortVegSummaryFieldList VALUES(1,'Admin','replacement','A',0,'sum','F1')`); err != nil {
		t.Fatal(err)
	}
	current, err := summaryFieldListRead(t, db, t.Context())
	if err != nil || current.Rows[0].RowID != expected.Rows[0].RowID {
		t.Fatal("fixture did not exercise actual automatic physical rowid reuse", current, err)
	}
	if got, err := planSummaryVegetationFieldList(t.Context(), current, expected,
		[]summaryVegetationFieldListAction{{Kind: "row", RowID: "1", Selected: true}}); got != nil || err == nil ||
		!strings.Contains(err.Error(), "changed since review") {
		t.Fatal("same-rowid replacement passed reduced witness", got, err)
	}
	table := summaryFieldListPlanFixture(t)
	for _, mutate := range []func(*ProjectMetadataTable){
		func(p *ProjectMetadataTable) { *p.Rows[3].Cells[2].Text = "changed hidden peer" },
		func(p *ProjectMetadataTable) { p.Rows = p.Rows[:4] },
		func(p *ProjectMetadataTable) { p.Columns[1].DeclaredType = "NUMERIC" },
	} {
		stale := cloneSummaryVegetationFieldListTable(table)
		mutate(&stale)
		if got, err := planSummaryVegetationFieldList(t.Context(), table, stale,
			[]summaryVegetationFieldListAction{{Kind: "none"}}); got != nil || err == nil {
			t.Fatal("full peer/schema witness change accepted", got, err)
		}
	}
}

func TestSummaryVegetationFieldListPlanInvalidInputsAndUnavailableRowCheckbox(t *testing.T) {
	table := summaryFieldListPlanFixture(t)
	for _, actions := range [][]summaryVegetationFieldListAction{
		nil, {{Kind: "label", RowID: "1"}}, {{Kind: "all"}}, {{Kind: "none", Selected: true}},
		{{Kind: "all", Selected: true, RowID: "1"}}, {{Kind: "row", RowID: "3"}},
		{{Kind: "row", RowID: "4"}}, {{Kind: "row", RowID: "5"}}, {{Kind: "row", RowID: "999"}},
		{{Kind: "row", RowID: "01"}}, {{Kind: "row", RowID: "+1"}}, {{Kind: "row", RowID: ""}},
		{{Kind: "all", Selected: true}, {Kind: "row", RowID: "5"}},
	} {
		if got, err := planSummaryVegetationFieldList(t.Context(), table, table, actions); got != nil || err == nil {
			t.Fatal("invalid/unavailable action returned a partial plan", actions, got, err)
		}
	}
	for _, mutate := range []func(*ProjectMetadataTable){
		func(p *ProjectMetadataTable) { p.Rows = nil },
		func(p *ProjectMetadataTable) { p.Rows[1].RowID = "1" },
		func(p *ProjectMetadataTable) { p.Rows[0].RowID = "01" },
		func(p *ProjectMetadataTable) { p.Rows[0].Cells = p.Rows[0].Cells[:6] },
		func(p *ProjectMetadataTable) { p.Rows[0].Cells[4].Text = new(string) },
		func(p *ProjectMetadataTable) { p.Rows[0].Cells[0].Integer = nil },
		func(p *ProjectMetadataTable) { *p.Rows[0].Cells[2].Text = string([]byte{0xff}) },
		func(p *ProjectMetadataTable) { p.Columns[2].Name = "label" },
	} {
		invalid := cloneSummaryVegetationFieldListTable(table)
		mutate(&invalid)
		if got, err := planSummaryVegetationFieldList(t.Context(), invalid, invalid,
			[]summaryVegetationFieldListAction{}); got != nil || err == nil {
			t.Fatal("invalid physical witness accepted", got, err)
		}
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := planSummaryVegetationFieldList(cancelled, table, table,
		[]summaryVegetationFieldListAction{}); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled plan accepted", got, err)
	}
}

func TestSummaryVegetationFieldListPlanDetachedHistoricalCellsAndZeroNegativeIDs(t *testing.T) {
	table := summaryFieldListPlanFixture(t)
	table.Rows = table.Rows[:2]
	table.Rows[0].RowID, table.Rows[1].RowID = "-1", "0"
	*table.Rows[0].Cells[2].Text = strings.Repeat("x", 256) + "\x00"
	plan, err := planSummaryVegetationFieldList(t.Context(), table, table,
		[]summaryVegetationFieldListAction{{Kind: "row", RowID: "0", Selected: true}})
	if err != nil || len(plan.Assignments) != 1 || plan.Assignments[0].Before.RowID != "0" ||
		*plan.Proposed.Rows[0].Cells[2].Text != strings.Repeat("x", 256)+"\x00" {
		t.Fatal("canonical zero/negative identity or unchanged historical cells rejected", plan, err)
	}
	*plan.Assignments[0].Before.Cells[2].Text = "assignment before"
	*plan.Assignments[0].After.Cells[2].Text = "assignment after"
	*plan.Original.Rows[1].Cells[2].Text = "original"
	if *plan.Proposed.Rows[1].Cells[2].Text != "duplicate" || *table.Rows[1].Cells[2].Text != "duplicate" {
		t.Fatal("plan/receipt/source witness aliases")
	}
	*table.Rows[1].Cells[2].Text = "caller"
	if *plan.Proposed.Rows[1].Cells[2].Text != "duplicate" {
		t.Fatal("caller mutation changed owned plan")
	}
}
