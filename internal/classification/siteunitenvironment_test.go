package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"reflect"
	"strings"
	"testing"
)

func reverseSiteUnitFixture() (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable, vprocore.ProjectMetadataTable) {
	env, _, su := siteUnitTransferFixture()
	env.Columns = append(env.Columns, vprocore.ProjectMetadataColumn{Name: "Flag", DeclaredType: "BOOLEAN"})
	for i := range env.Rows {
		env.Rows[i].Cells = append(env.Rows[i].Cells, vprocore.ProjectMetadataCell{Storage: "null"})
	}
	admin := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
		{Name: "Plot"}, {Name: "UserSiteUnit"}, {Name: "SiteUnitShortName"}, {Name: "SiteUnitLongName"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P1"), vprotest.MetadataText("Old"), vprotest.MetadataText("Short"), vprotest.MetadataText("Long")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P2"), vprotest.MetadataText("Old"), vprotest.MetadataText("Preserve short"), vprotest.MetadataText("Preserve long")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P3"), vprotest.MetadataText("Old"), vprotest.MetadataText("Preserve short"), vprotest.MetadataText("Preserve long")}},
		}}
	su.Rows = su.Rows[:3]
	su.Rows[0].Cells[1] = vprotest.MetadataText("Unit")
	su.Rows[1].Cells[1] = vprotest.MetadataText("")
	su.Rows[2].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
	master := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "SiteSeries"}, {Name: "SiteSeriesLongName"}},
		Rows: []vprocore.ProjectMetadataRow{{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Unit"), vprotest.MetadataText("Master long")}}}}
	personal := vprocore.ProjectMetadataTable{Columns: master.Columns, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Unit"), vprotest.MetadataText("Personal long")}}}}
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
			duplicate := func(table *vprocore.ProjectMetadataTable) {
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
				env.Rows[0].Cells[1] = vprotest.MetadataInteger("-1")
			case "malformed-lock":
				env.Rows[0].Cells[1].Integer = vprotest.MetadataInteger("0").Integer
			case "unit-length":
				su.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("\U0001f332", 51))
			case "name-length":
				personal.Rows[0].Cells[1] = vprotest.MetadataText(strings.Repeat("x", 101))
			case "nul":
				su.Rows[0].Cells[1] = vprotest.MetadataText("Unit\x00")
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
	su.Rows[0].Cells[1] = vprotest.MetadataText(value)
	changes, err := planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || *changes[0].After.Text != value {
		t.Fatal("exact100 UTF16 destination rejected", err)
	}
	historical := vprotest.MetadataText(strings.Repeat("x", 101))
	su.Rows[0].Cells[1], admin.Rows[0].Cells[1] = historical, vprocore.CloneSiteUnitCell(historical)
	changes, err = planSiteUnitEnvironment(env, admin, su, master, personal)
	if err != nil || len(changes) != 2 || !reflect.DeepEqual(su.Rows[0].Cells[1], historical) {
		t.Fatal("unchanged historical invalid unit rewritten", err)
	}
}
