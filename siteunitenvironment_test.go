package main

import (
	"reflect"
	"strings"
	"testing"
)

func reverseSiteUnitFixture() (ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable, ProjectMetadataTable) {
	env, _, su := siteUnitTransferFixture()
	env.Columns = append(env.Columns, ProjectMetadataColumn{Name: "Flag", DeclaredType: "BOOLEAN"})
	for i := range env.Rows {
		env.Rows[i].Cells = append(env.Rows[i].Cells, ProjectMetadataCell{Storage: "null"})
	}
	admin := ProjectMetadataTable{Columns: []ProjectMetadataColumn{
		{Name: "Plot"}, {Name: "UserSiteUnit"}, {Name: "SiteUnitShortName"}, {Name: "SiteUnitLongName"}},
		Rows: []ProjectMetadataRow{
			{RowID: "1", Cells: []ProjectMetadataCell{metadataText("P1"), metadataText("Old"), metadataText("Short"), metadataText("Long")}},
			{RowID: "2", Cells: []ProjectMetadataCell{metadataText("P2"), metadataText("Old"), metadataText("Preserve short"), metadataText("Preserve long")}},
			{RowID: "3", Cells: []ProjectMetadataCell{metadataText("P3"), metadataText("Old"), metadataText("Preserve short"), metadataText("Preserve long")}},
		}}
	su.Rows = su.Rows[:3]
	su.Rows[0].Cells[1] = metadataText("Unit")
	su.Rows[1].Cells[1] = metadataText("")
	su.Rows[2].Cells[1] = ProjectMetadataCell{Storage: "null"}
	master := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}},
		Rows: []ProjectMetadataRow{{RowID: "1", Cells: []ProjectMetadataCell{metadataText("Unit"), metadataText("Master long")}}}}
	personal := ProjectMetadataTable{Columns: master.Columns, Rows: []ProjectMetadataRow{
		{RowID: "1", Cells: []ProjectMetadataCell{metadataText("Unit"), metadataText("Personal long")}}}}
	return env, admin, su, master, personal
}

func TestSiteUnitEnvironmentPlanOriginalLinksPersonalPrecedenceAndMissingNames(t *testing.T) {
	env, admin, su, master, personal := reverseSiteUnitFixture()
	changes, err := planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || len(changes) != 5 {
		t.Fatal("original reverse assignments not planned", changes, err)
	}
	if changes[1].Field != "SiteUnitShortName" || *changes[1].After.Text != "Unit" ||
		*changes[2].After.Text != "Personal long" || changes[2].Origin != "personal override" ||
		changes[3].After.Storage != "text" || *changes[3].After.Text != "" || changes[4].After.Storage != "null" {
		t.Fatal("literal NULL/empty/personal precedence lost", changes)
	}
	*changes[2].After.Text = "Mutated"
	if *personal.Rows[0].Cells[1].Text != "Personal long" {
		t.Fatal("plan aliases reference observations")
	}
	personal.Rows = nil
	changes, err = planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || *changes[2].After.Text != "Master long" {
		t.Fatal("unique master fallback failed", err)
	}
	master.Rows = nil
	changes, err = planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || len(changes) != 3 {
		t.Fatal("unmatched names were guessed or cleared", err)
	}
}

func TestSiteUnitEnvironmentPlanRejectsAmbiguityLocksAndDestinationBounds(t *testing.T) {
	for _, variant := range []string{"master-duplicate", "personal-duplicate", "su-duplicate", "admin-duplicate",
		"env-duplicate", "locked", "malformed-lock", "unit-length", "name-length", "nul"} {
		t.Run(variant, func(t *testing.T) {
			env, admin, su, master, personal := reverseSiteUnitFixture()
			duplicate := func(table *ProjectMetadataTable) {
				row := table.Rows[0]
				row.RowID = "99"
				table.Rows = append(table.Rows, row)
			}
			switch variant {
			case "master-duplicate":
				duplicate(&master)
			case "personal-duplicate":
				duplicate(&personal)
			case "su-duplicate":
				duplicate(&su)
			case "admin-duplicate":
				duplicate(&admin)
			case "env-duplicate":
				duplicate(&env)
			case "locked":
				env.Rows[0].Cells[1] = metadataInteger("-1")
			case "malformed-lock":
				env.Rows[0].Cells[1].Integer = metadataInteger("0").Integer
			case "unit-length":
				su.Rows[0].Cells[1] = metadataText(strings.Repeat("\U0001f332", 51))
			case "name-length":
				personal.Rows[0].Cells[1] = metadataText(strings.Repeat("x", 101))
			case "nul":
				su.Rows[0].Cells[1] = metadataText("Unit\x00")
			}
			if _, err := planSiteUnitEnvironment(env, admin, su, master, personal); err == nil {
				t.Fatal("unsafe reverse assignment accepted")
			}
		})
	}
}

func TestSiteUnitEnvironmentPlanHistoricalOmissionAndExactUTF16Bounds(t *testing.T) {
	env, admin, su, master, personal := reverseSiteUnitFixture()
	value := strings.Repeat("\U0001f332", 50)
	su.Rows[0].Cells[1] = metadataText(value)
	changes, err := planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || *changes[0].After.Text != value {
		t.Fatal("exact100 UTF16 destination rejected", err)
	}
	historical := metadataText(strings.Repeat("x", 101))
	su.Rows[0].Cells[1], admin.Rows[0].Cells[1] = historical, cloneSiteUnitCell(historical)
	changes, err = planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || len(changes) != 2 || !reflect.DeepEqual(su.Rows[0].Cells[1], historical) {
		t.Fatal("unchanged historical invalid unit rewritten", err)
	}
}
