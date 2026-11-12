package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestEnvironmentSiteUnitPlanReadsOriginalSQLiteTablesWithoutWrites(t *testing.T) {
	service, state := vprotest.ContextServiceFixture(t)
	c := service.Projects.Sqlite
	before := vprotest.DatabaseBytes(t, c.Attachments)
	read := func(suffix string) vprocore.ProjectMetadataTable {
		t.Helper()
		table, err := vprocore.ReadSQLiteStorageRows(context.Background(), c.Conn, "project", state.ActiveProject+suffix, "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return table
	}
	if _, err := planEnvironmentSiteUnits(read("_Env"), read("_Admin"), read("_SU")); err != nil {
		t.Fatal("retained original SQLite bindings unavailable", err)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
}

func siteUnitTransferFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	table := func(columns []string, cells [][]vprocore.ProjectMetadataCell) vprocore.ProjectMetadataTable {
		result := vprocore.ProjectMetadataTable{Rows: []vprocore.ProjectMetadataRow{}}
		for _, column := range columns {
			result.Columns = append(result.Columns, vprocore.ProjectMetadataColumn{Name: column, DeclaredType: "VARCHAR"})
		}
		for i, row := range cells {
			result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: row})
		}
		return result
	}
	env := table([]string{"PlotNumber"}, [][]vprocore.ProjectMetadataCell{
		{vprotest.MetadataText("P1")}, {vprotest.MetadataText("P2")}, {vprotest.MetadataText("P3")},
		{vprotest.MetadataText("P4")}, {vprotest.MetadataText("Other")},
	})
	admin := table([]string{"Plot", "UserSiteUnit"}, [][]vprocore.ProjectMetadataCell{
		{vprotest.MetadataText("P1"), vprotest.MetadataText("  Literal ' unit  ")},
		{vprotest.MetadataText("P2"), vprotest.MetadataText("")},
		{vprotest.MetadataText("P3"), {Storage: "null"}},
		{vprotest.MetadataText("Other"), vprotest.MetadataText("Outside selected SU")},
	})
	su := table([]string{"PlotNumber", "SiteUnit"}, [][]vprocore.ProjectMetadataCell{
		{vprotest.MetadataText("P1"), vprotest.MetadataText("Before")},
		{vprotest.MetadataText("P2"), {Storage: "null"}},
		{vprotest.MetadataText("P3"), vprotest.MetadataText("")},
		{vprotest.MetadataText("P4"), vprotest.MetadataText("Unlinked")},
		{vprotest.MetadataText("Foreign"), {Storage: "blob", BlobHex: vprotest.MetadataText("ff").Text}},
		{{Storage: "null"}, vprotest.MetadataText("Null identity")},
	})
	return env, admin, su
}

func TestEnvironmentSiteUnitPlanLiteralLinksNULLAndNoMutation(t *testing.T) {
	env, admin, su := siteUnitTransferFixture()
	original := *su.Rows[0].Cells[1].Text
	changes, err := planEnvironmentSiteUnits(env, admin, su)
	if err != nil || len(changes) != 3 {
		t.Fatal("exact original links not planned", changes, err)
	}
	for i, change := range changes {
		if change.PlotNumber != "P"+strconv.Itoa(i+1) || change.SURowID != strconv.Itoa(i+1) ||
			!reflect.DeepEqual(change.Before, su.Rows[i].Cells[1]) || !reflect.DeepEqual(change.After, admin.Rows[i].Cells[1]) {
			t.Fatal("plan changed a source value/physical identity", change)
		}
	}
	*changes[0].Before.Text = "Changed plan"
	*changes[0].After.Text = "Changed plan"
	if *su.Rows[0].Cells[1].Text != original || *admin.Rows[0].Cells[1].Text != "  Literal ' unit  " {
		t.Fatal("plan aliases caller-owned source observations")
	}
	env.Rows[0].Cells[0] = vprotest.MetadataText("p1")
	changes, err = planEnvironmentSiteUnits(env, admin, su)
	if err != nil || len(changes) != 2 {
		t.Fatal("literal match silently changed case", err)
	}
}

func TestEnvironmentSiteUnitPlanRejectsAmbiguousPhysicalLinksAndMalformedRows(t *testing.T) {
	for _, role := range []string{"env", "admin", "su", "rowid", "column", "width", "storage", "tag", "plot-storage"} {
		t.Run(role, func(t *testing.T) {
			env, admin, su := siteUnitTransferFixture()
			duplicate := func(table *vprocore.ProjectMetadataTable) {
				row := table.Rows[0]
				row.RowID = "-9223372036854775808"
				table.Rows = append(table.Rows, row)
			}
			switch role {
			case "env":
				duplicate(&env)
			case "admin":
				duplicate(&admin)
			case "su":
				duplicate(&su)
			case "rowid":
				su.Rows[0].RowID = "01"
			case "column":
				admin.Columns[1].Name = "Plot"
			case "width":
				admin.Rows[0].Cells = nil
			case "storage":
				admin.Rows[0].Cells[1] = vprotest.MetadataInteger("7")
			case "tag":
				admin.Rows[0].Cells[1].Integer = vprotest.MetadataInteger("7").Integer
			case "plot-storage":
				env.Rows[0].Cells[0] = vprotest.MetadataInteger("7")
			}
			if _, err := planEnvironmentSiteUnits(env, admin, su); err == nil {
				t.Fatal("ambiguous/malformed source accepted")
			}
		})
	}
}

func TestEnvironmentSiteUnitPlanUTF16AndHistoricalOmission(t *testing.T) {
	for _, value := range []string{strings.Repeat("x", 255), strings.Repeat("\U0001f332", 127) + "x"} {
		env, admin, su := siteUnitTransferFixture()
		admin.Rows[0].Cells[1] = vprotest.MetadataText(value)
		if _, err := planEnvironmentSiteUnits(env, admin, su); err != nil {
			t.Fatal("exact255 UTF-16-unit boundary rejected", err)
		}
	}
	for _, value := range []string{strings.Repeat("x", 256), strings.Repeat("\U0001f332", 128), "Embedded\x00NUL"} {
		env, admin, su := siteUnitTransferFixture()
		admin.Rows[0].Cells[1] = vprotest.MetadataText(value)
		if _, err := planEnvironmentSiteUnits(env, admin, su); err == nil {
			t.Fatal("invalid new assignment accepted")
		}
		su.Rows[0].Cells[1] = vprotest.MetadataText(value)
		changes, err := planEnvironmentSiteUnits(env, admin, su)
		if err != nil || len(changes) != 2 {
			t.Fatal("unchanged historical invalid value was assigned or repaired", err)
		}
	}
}
