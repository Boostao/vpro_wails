package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"testing"
)

func TestSelectedUnitPreparationUnmatchedAndNullPlotWitnesses(t *testing.T) {
	veg, su, layers, _, env, admin := selectedUnitReportFixture(t)
	su.Rows = append(su.Rows,
		vprocore.ProjectMetadataRow{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("absent"), vprotest.MetadataText("historical")}},
		vprocore.ProjectMetadataRow{RowID: "4", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("historical")}})
	prepared, occurrences, err := prepareLongVegetationSelectedUnits(context.Background(), "Project", "Selected",
		veg, su, layers, env, admin, []string{"Zone"}, nil)
	if err != nil || len(occurrences) != 10 || len(prepared.Memberships) != 4 {
		t.Fatal("outer-SU orphan/NULL rows were lost before report", prepared, occurrences, err)
	}
	for _, i := range []int{8, 9} {
		row := occurrences[i]
		if row.EnvRowID != nil || row.AdminRowID != nil || row.SiteUnit.Storage != "null" {
			t.Fatal("unmatched outer row invented Env/Admin/label", row)
		}
	}
	reportMembers, err := selectedUnitReportMemberships(context.Background(), prepared)
	if err != nil || len(reportMembers) != 10 || reportMembers[8].RowID != "3" ||
		reportMembers[8].PlotNumber.Text == nil || *reportMembers[8].PlotNumber.Text != "absent" ||
		reportMembers[9].RowID != "4" || reportMembers[9].PlotNumber.Storage != "null" {
		t.Fatal("typed derived membership mapping changed original identities", reportMembers, err)
	}
	*reportMembers[0].SiteUnit.Text = "changed"
	if *prepared.selectedUnitOccurrences[0].SiteUnit.Text != "Alpha" {
		t.Fatal("kernel projection aliases retained preparation")
	}
	prepared.selectedUnitOccurrences[9].EnvRowID = sqlutil.CloneCatalogueValue(prepared.selectedUnitOccurrences[0].EnvRowID)
	prepared.selectedUnitOccurrences[9].AdminRowID = sqlutil.CloneCatalogueValue(prepared.selectedUnitOccurrences[0].AdminRowID)
	if rows, err := selectedUnitReportMemberships(context.Background(), prepared); err == nil || rows != nil {
		t.Fatal("NULL plot acquired fake matched parents", rows, err)
	}
}
