package integration

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestVegetationLifeformReferencesWholeRowUnionAndOrigins(t *testing.T) {
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	row := vprotest.LifeformReferenceRow(" A ", "Exact", vprotest.MetadataInteger("2"), null, vprotest.MetadataText("U"))
	master := sftest.LifeformReferenceFixture(row, row,
		vprotest.LifeformReferenceRow(" A ", "Exact", vprotest.MetadataInteger("2"), vprotest.MetadataText(""), vprotest.MetadataText("U")),
		vprotest.LifeformReferenceRow("A", "Excluded null", null, null, null),
		vprotest.LifeformReferenceRow("A", "Excluded synonym", null, null, vprotest.MetadataText("s")))
	personal := sftest.LifeformReferenceFixture(row,
		vprotest.LifeformReferenceRow(" A ", "Exact", vprotest.MetadataInteger("3"), null, vprotest.MetadataText("U")),
		vprotest.LifeformReferenceRow("a", "literal", null, vprotest.MetadataText(""), vprotest.MetadataText("")),
		vprotest.LifeformReferenceRow("A", "Excluded upper", null, null, vprotest.MetadataText("S")))
	result, err := reportcore.PrepareVegetationLifeformReferences(context.Background(), master, personal)
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
	valid := sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("A", "A", vprotest.MetadataInteger("1"), vprotest.MetadataText(""), vprotest.MetadataText("U")))
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("1"), vprotest.StrataReal(1), vprotest.MetadataInteger("32768"),
		{Storage: "integer", Integer: nil}, {Storage: "null", Text: new(string)}} {
		bad := sftest.LifeformReferenceFixture(vprotest.LifeformReferenceRow("A", "A", cell, vprotest.MetadataText(""), vprotest.MetadataText("U")))
		result, err := reportcore.PrepareVegetationLifeformReferences(context.Background(), valid, bad)
		if err == nil || !reflect.DeepEqual(result, reportcore.VegetationLifeformReferences{}) {
			t.Fatal("invalid typed lifeform returned partial UNION", cell, result, err)
		}

	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := reportcore.PrepareVegetationLifeformReferences(ctx, valid, valid)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, reportcore.VegetationLifeformReferences{}) {
		t.Fatal(result, err)
	}
}

func TestVegetationLifeformReferencesExplicitPersonalLifeFormProjection(t *testing.T) {
	row := vprotest.LifeformReferenceRow("A", "Exact", vprotest.MetadataInteger("2"), vprocore.ProjectMetadataCell{Storage: "null"}, vprotest.MetadataText("U"))
	master, personal := sftest.LifeformReferenceFixture(row), sftest.LifeformReferenceFixture(row)
	personal.Columns[2].Name = "LifeForm"
	personalBefore := vprotest.CloneLifeformReferenceFixture(personal)
	result, err := reportcore.PrepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil || len(result.Table.Rows) != 1 || len(result.Origins["1"]) != 2 ||
		result.Table.Columns[2].Name != "Lifeform" || *result.Table.Rows[0].Cells[2].Integer != "2" ||
		!reflect.DeepEqual(personalBefore, personal) {
		t.Fatal("source-spelled personal projection did not preserve whole-row UNION or original schema", result, personal, err)
	}
	for _, spelling := range []string{"LIFEFORM", "lifeform", "LifeForm "} {
		bad := vprotest.CloneLifeformReferenceFixture(personal)
		bad.Columns[2].Name = spelling
		result, err := reportcore.PrepareVegetationLifeformReferences(context.Background(), master, bad)
		if err == nil || !reflect.DeepEqual(result, reportcore.VegetationLifeformReferences{}) {
			t.Fatal("unlisted personal spelling silently repaired", spelling, result, err)
		}
	}
	ambiguous := vprotest.CloneLifeformReferenceFixture(personal)
	ambiguous.Columns = append(ambiguous.Columns, vprocore.ProjectMetadataColumn{Name: "Lifeform"})
	ambiguous.Rows[0].Cells = append(ambiguous.Rows[0].Cells, vprotest.MetadataInteger("3"))
	result, err = reportcore.PrepareVegetationLifeformReferences(context.Background(), master, ambiguous)
	if err == nil || !reflect.DeepEqual(result, reportcore.VegetationLifeformReferences{}) {
		t.Fatal("ambiguous personal aliases accepted", result, err)
	}
	master.Columns[2].Name = "LifeForm"
	result, err = reportcore.PrepareVegetationLifeformReferences(context.Background(), master, personal)
	if err == nil || !reflect.DeepEqual(result, reportcore.VegetationLifeformReferences{}) {
		t.Fatal("personal source alias leaked into master", result, err)
	}
}

func TestVegetationLifeformReferencesActualImportedMixedCaseSchemas(t *testing.T) {
	service, _ := vprotest.ReportServiceFixture(t, false)
	owner := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	owner.Mu.Lock()
	defer owner.Mu.Unlock()
	tx, err := owner.BeginReadSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	master, err := vprocore.ReadSQLiteStorageRows(context.Background(), tx, "VLists", "USysAllSpecs", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	personal, err := vprocore.ReadSQLiteStorageRows(context.Background(), tx, "VUser", "USysUserSpp", "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	masterColumns, err := vprocore.SiteUnitTransferColumns(master, "Lifeform")
	if err != nil {
		t.Fatal("imported master spelling changed", err)
	}
	personalColumns, err := vprocore.SiteUnitTransferColumns(personal, "LifeForm")
	if err != nil {
		t.Fatal("imported personal spelling changed", err)
	}
	if master.Columns[masterColumns["Lifeform"]].Name != "Lifeform" ||
		personal.Columns[personalColumns["LifeForm"]].Name != "LifeForm" {
		t.Fatal("fixture did not exercise original mixed-case source schemas")
	}
	original := vprotest.CloneLifeformReferenceFixture(personal)
	result, err := reportcore.PrepareVegetationLifeformReferences(context.Background(), master, personal)
	if err != nil || len(result.Table.Rows) == 0 || result.Table.Columns[2].Name != "Lifeform" ||
		!reflect.DeepEqual(original, personal) {
		t.Fatal("actual imported source UNION projection failed or mutated the source", result, err)
	}
	for _, row := range result.Table.Rows {
		if _, err := vprocore.VegetationLifeformInteger(row.Cells[2]); err != nil {
			t.Fatal("imported UNION lost typed nullable lifeform", row, err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}
