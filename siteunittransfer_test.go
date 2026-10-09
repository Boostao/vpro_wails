package main

import (
	"context"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestEnvironmentSiteUnitPlanReadsOriginalSQLiteTablesWithoutWrites(t *testing.T) {
	service, state := contextServiceFixture(t)
	c := service.projects.sqlite
	before := databaseBytes(t, c.attachments)
	read := func(suffix string) ProjectMetadataTable {
		t.Helper()
		table, err := readSQLiteStorageRows(context.Background(), c.conn, "project", state.ActiveProject+suffix, "", nil, "")
		if err != nil {
			t.Fatal(err)
		}
		return table
	}
	if _, err := planEnvironmentSiteUnits(read("_Env"), read("_Admin"), read("_SU")); err != nil {
		t.Fatal("retained original SQLite bindings unavailable", err)
	}
	assertProfileSUFiles(t, service, before)
}

func siteUnitTransferFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	table := func(columns []string, cells [][]ProjectMetadataCell) ProjectMetadataTable {
		result := ProjectMetadataTable{Rows: []ProjectMetadataRow{}}
		for _, column := range columns {
			result.Columns = append(result.Columns, ProjectMetadataColumn{Name: column, DeclaredType: "VARCHAR"})
		}
		for i, row := range cells {
			result.Rows = append(result.Rows, ProjectMetadataRow{strconv.Itoa(i + 1), row})
		}
		return result
	}
	env := table([]string{"PlotNumber"}, [][]ProjectMetadataCell{
		{metadataText("P1")}, {metadataText("P2")}, {metadataText("P3")},
		{metadataText("P4")}, {metadataText("Other")},
	})
	admin := table([]string{"Plot", "UserSiteUnit"}, [][]ProjectMetadataCell{
		{metadataText("P1"), metadataText("  Literal ' unit  ")},
		{metadataText("P2"), metadataText("")},
		{metadataText("P3"), {Storage: "null"}},
		{metadataText("Other"), metadataText("Outside selected SU")},
	})
	su := table([]string{"PlotNumber", "SiteUnit"}, [][]ProjectMetadataCell{
		{metadataText("P1"), metadataText("Before")},
		{metadataText("P2"), {Storage: "null"}},
		{metadataText("P3"), metadataText("")},
		{metadataText("P4"), metadataText("Unlinked")},
		{metadataText("Foreign"), {Storage: "blob", BlobHex: metadataText("ff").Text}},
		{{Storage: "null"}, metadataText("Null identity")},
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
	env.Rows[0].Cells[0] = metadataText("p1")
	changes, err = planEnvironmentSiteUnits(env, admin, su)
	if err != nil || len(changes) != 2 {
		t.Fatal("literal match silently changed case", err)
	}
}

func TestEnvironmentSiteUnitPlanRejectsAmbiguousPhysicalLinksAndMalformedRows(t *testing.T) {
	for _, role := range []string{"env", "admin", "su", "rowid", "column", "width", "storage", "tag", "plot-storage"} {
		t.Run(role, func(t *testing.T) {
			env, admin, su := siteUnitTransferFixture()
			duplicate := func(table *ProjectMetadataTable) {
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
				admin.Rows[0].Cells[1] = metadataInteger("7")
			case "tag":
				admin.Rows[0].Cells[1].Integer = metadataInteger("7").Integer
			case "plot-storage":
				env.Rows[0].Cells[0] = metadataInteger("7")
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
		admin.Rows[0].Cells[1] = metadataText(value)
		if _, err := planEnvironmentSiteUnits(env, admin, su); err != nil {
			t.Fatal("exact255 UTF-16-unit boundary rejected", err)
		}
	}
	for _, value := range []string{strings.Repeat("x", 256), strings.Repeat("\U0001f332", 128), "Embedded\x00NUL"} {
		env, admin, su := siteUnitTransferFixture()
		admin.Rows[0].Cells[1] = metadataText(value)
		if _, err := planEnvironmentSiteUnits(env, admin, su); err == nil {
			t.Fatal("invalid new assignment accepted")
		}
		su.Rows[0].Cells[1] = metadataText(value)
		changes, err := planEnvironmentSiteUnits(env, admin, su)
		if err != nil || len(changes) != 2 {
			t.Fatal("unchanged historical invalid value was assigned or repaired", err)
		}
	}
}
