package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func lifeformReferenceFixture(rows ...[]ProjectMetadataCell) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
	for _, name := range vegetationLifeformReferenceColumns {
		result.Columns = append(result.Columns, ProjectMetadataColumn{Name: name})
	}
	for i, cells := range rows {
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: siteUnitNumericString(float64(i + 1)), Cells: cells})
	}
	return result
}

func lifeformReferenceRow(code, scientific string, form ProjectMetadataCell, english, kind ProjectMetadataCell) []ProjectMetadataCell {
	return []ProjectMetadataCell{metadataText(code), metadataText(scientific), form, english, kind}
}

func TestVegetationLifeformReferencesWholeRowUnionAndOrigins(t *testing.T) {
	null := ProjectMetadataCell{Storage: "null"}
	row := lifeformReferenceRow(" A ", "Exact", metadataInteger("2"), null, metadataText("U"))
	master := lifeformReferenceFixture(row, row,
		lifeformReferenceRow(" A ", "Exact", metadataInteger("2"), metadataText(""), metadataText("U")),
		lifeformReferenceRow("A", "Excluded null", null, null, null),
		lifeformReferenceRow("A", "Excluded synonym", null, null, metadataText("s")))
	personal := lifeformReferenceFixture(row,
		lifeformReferenceRow(" A ", "Exact", metadataInteger("3"), null, metadataText("U")),
		lifeformReferenceRow("a", "literal", null, metadataText(""), metadataText("")),
		lifeformReferenceRow("A", "Excluded upper", null, null, metadataText("S")))
	result, err := prepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil || len(result.Table.Rows) != 4 || len(result.Origins["1"]) != 3 {
		t.Fatal("whole-row UNION, NULL predicate, or physical origins lost", result, err)
	}
	if result.Table.Rows[0].Cells[3].Storage != "null" || result.Table.Rows[1].Cells[3].Text == nil ||
		*result.Table.Rows[1].Cells[3].Text != "" || *result.Table.Rows[0].Cells[0].Text != " A " {
		t.Fatal("NULL/empty or literal identity repaired", result)
	}
	*result.Table.Rows[0].Cells[0].Text = "changed"
	if *master.Rows[0].Cells[0].Text != " A " {
		t.Fatal("UNION aliases input cells")
	}
}

func TestVegetationLifeformReferencesStrictTypedAndCancellation(t *testing.T) {
	valid := lifeformReferenceFixture(lifeformReferenceRow("A", "A", metadataInteger("1"), metadataText(""), metadataText("U")))
	for _, cell := range []ProjectMetadataCell{metadataText("1"), strataReal(1), metadataInteger("32768"),
		{Storage: "integer", Integer: nil}, {Storage: "null", Text: new(string)}} {
		bad := lifeformReferenceFixture(lifeformReferenceRow("A", "A", cell, metadataText(""), metadataText("U")))
		result, err := prepareVegetationLifeformReferences(context.Background(), valid, bad)
		if err == nil || !reflect.DeepEqual(result, vegetationLifeformReferences{}) {
			t.Fatal("invalid typed lifeform returned partial UNION", cell, result, err)
		}

	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := prepareVegetationLifeformReferences(ctx, valid, valid)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, vegetationLifeformReferences{}) {
		t.Fatal(result, err)
	}
}

func TestVegetationLifeformReferencesExplicitPersonalLifeFormProjection(t *testing.T) {
	row := lifeformReferenceRow("A", "Exact", metadataInteger("2"), ProjectMetadataCell{Storage: "null"}, metadataText("U"))
	master, personal := lifeformReferenceFixture(row), lifeformReferenceFixture(row)
	personal.Columns[2].Name = "LifeForm"
	personalBefore := cloneLifeformReferenceFixture(personal)
	result, err := prepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil || len(result.Table.Rows) != 1 || len(result.Origins["1"]) != 2 ||
		result.Table.Columns[2].Name != "Lifeform" || *result.Table.Rows[0].Cells[2].Integer != "2" ||
		!reflect.DeepEqual(personalBefore, personal) {
		t.Fatal("source-spelled personal projection did not preserve whole-row UNION or original schema", result, personal, err)
	}
	for _, spelling := range []string{"LIFEFORM", "lifeform", "LifeForm "} {
		bad := cloneLifeformReferenceFixture(personal)
		bad.Columns[2].Name = spelling
		result, err := prepareVegetationLifeformReferences(context.Background(), master, bad)
		if err == nil || !reflect.DeepEqual(result, vegetationLifeformReferences{}) {
			t.Fatal("unlisted personal spelling silently repaired", spelling, result, err)
		}
	}
	ambiguous := cloneLifeformReferenceFixture(personal)
	ambiguous.Columns = append(ambiguous.Columns, ProjectMetadataColumn{Name: "Lifeform"})
	ambiguous.Rows[0].Cells = append(ambiguous.Rows[0].Cells, metadataInteger("3"))
	result, err = prepareVegetationLifeformReferences(context.Background(), master, ambiguous)
	if err == nil || !reflect.DeepEqual(result, vegetationLifeformReferences{}) {
		t.Fatal("ambiguous personal aliases accepted", result, err)
	}
	master.Columns[2].Name = "LifeForm"
	result, err = prepareVegetationLifeformReferences(context.Background(), master, personal)
	if err == nil || !reflect.DeepEqual(result, vegetationLifeformReferences{}) {
		t.Fatal("personal source alias leaked into master", result, err)
	}
}

func cloneLifeformReferenceFixture(table ProjectMetadataTable) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn{}, table.Columns...), Rows: []ProjectMetadataRow{}}
	for _, row := range table.Rows {
		cells := []ProjectMetadataCell{}
		for _, cell := range row.Cells {
			cells = append(cells, cloneSiteUnitCell(cell))
		}
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: cells})
	}
	return result
}

func TestVegetationLifeformReferencesActualImportedMixedCaseSchemas(t *testing.T) {
	service, _ := reportServiceFixture(t, false)
	owner := service.projects.sqlite
	before := databaseBytes(t, owner.attachments)
	owner.mu.Lock()
	defer owner.mu.Unlock()
	tx, err := owner.beginReadSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	master, err := readSQLiteStorageRows(context.Background(), tx, "VLists", "USysAllSpecs", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	personal, err := readSQLiteStorageRows(context.Background(), tx, "VUser", "USysUserSpp", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	masterColumns, err := siteUnitTransferColumns(master, "Lifeform")
	if err != nil {
		t.Fatal("imported master spelling changed", err)
	}
	personalColumns, err := siteUnitTransferColumns(personal, "LifeForm")
	if err != nil {
		t.Fatal("imported personal spelling changed", err)
	}
	if master.Columns[masterColumns["Lifeform"]].Name != "Lifeform" ||
		personal.Columns[personalColumns["LifeForm"]].Name != "LifeForm" {
		t.Fatal("fixture did not exercise original mixed-case source schemas")
	}
	original := cloneLifeformReferenceFixture(personal)
	result, err := prepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil || len(result.Table.Rows) == 0 || result.Table.Columns[2].Name != "Lifeform" ||
		!reflect.DeepEqual(original, personal) {
		t.Fatal("actual imported source UNION projection failed or mutated the source", result, err)
	}
	for _, row := range result.Table.Rows {
		if _, err := vegetationLifeformInteger(row.Cells[2]); err != nil {
			t.Fatal("imported UNION lost typed nullable lifeform", row, err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertProfileSUFiles(t, service, before)
}
