package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"testing"
)

func TestLongVegetationNoneUnitsPhysicalJoinsAndNulls(t *testing.T) {
	env := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "Other"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("env1")}},
			{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("env2")}},
			{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("Q"), vprotest.MetadataText("env3")}},
			{RowID: "4", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("env4")}},
			{RowID: "5", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("env5")}},
		}}
	su := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}},
		Rows: []vprocore.ProjectMetadataRow{
			{RowID: "11", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("unit1")}},
			{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), {Storage: "null"}}},
			{RowID: "13", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("absent"), vprotest.MetadataText("unit2")}},
			{RowID: "14", Cells: []vprocore.ProjectMetadataCell{{Storage: "null"}, vprotest.MetadataText("unit3")}},
			{RowID: "15", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText("")}},
		}}
	got, err := planLongVegetationNoneUnits(context.Background(), env, su, nil)
	if err != nil || len(got) != 5 {
		t.Fatal("Env2*SU2 plus empty1; orphan/NULL not members", got, err)
	}
	want := [][2]string{{"1", "11"}, {"1", "12"}, {"2", "11"}, {"2", "12"}, {"5", "15"}}
	for i, row := range got {
		if row.EnvRowID != want[i][0] || row.MembershipRowID != want[i][1] || row.QualityOccurrence != nil ||
			row.SiteUnit.Text == nil || *row.SiteUnit.Text != "All plots" ||
			row.PlotNumber.Text == nil || *row.PlotNumber.Text != map[bool]string{true: "", false: "P"}[i == 4] {
			t.Fatal("literal scope/order/witness lost", i, row)
		}
	}
	*got[0].PlotNumber.Text = "mutated"
	*got[0].SiteUnit.Text = "mutated unit"
	if *env.Rows[0].Cells[0].Text != "P" || *su.Rows[0].Cells[0].Text != "P" ||
		*got[1].SiteUnit.Text != "All plots" {
		t.Fatal("derived joins alias source or peers")
	}
}

func TestLongVegetationNoneUnitsQualityReferenceFanoutThenOuterEnvJoin(t *testing.T) {
	su, env, admin, lists := vegetationQualityFixture()
	env.Rows = append(env.Rows, vprocore.ProjectMetadataRow{RowID: "22", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P")}})
	admin.Rows = append(admin.Rows, vprocore.ProjectMetadataRow{RowID: "32", Cells: []vprocore.ProjectMetadataCell{
		vprotest.MetadataText("P"), vprotest.MetadataText("Good"), vprotest.MetadataText("Good"), vprotest.MetadataText("Good")}})
	su.Rows = append(su.Rows, vprocore.ProjectMetadataRow{RowID: "12", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("P"), vprotest.MetadataText("other")}})
	// The duplicate DataQuality definition doubles each of the three ranks.
	lists.Rows = append(lists.Rows, vprocore.ProjectMetadataRow{RowID: "20", Cells: []vprocore.ProjectMetadataCell{
		vprotest.MetadataText("Good"), vprotest.MetadataText("DataQuality"), vprotest.MetadataInteger("3")}})
	if _, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Good", 0)); err == nil {
		t.Fatal("existing uncalibrated duplicate Env/Admin boundary was weakened")
	}
	// Independent source joins:2 memberships*2 Env*2 Admin*2^3 references.
	quality := vegetationQualitySelection{Occurrences: []vegetationQualityOccurrence{}}
	for _, member := range []string{"11", "12"} {
		for _, parent := range []string{"21", "22"} {
			for _, adminID := range []string{"31", "32"} {
				for _, site := range []string{"3", "20"} {
					for _, veg := range []string{"3", "20"} {
						for _, soil := range []string{"3", "20"} {
							unit := "  Unit  "
							if member == "12" {
								unit = "other"
							}
							quality.Occurrences = append(quality.Occurrences, vegetationQualityOccurrence{
								Membership: reportcore.VegetationReportMembership{RowID: member, PlotNumber: vprotest.MetadataText("P"), SiteUnit: vprotest.MetadataText(unit)},
								EnvRowID:   parent, AdminRowID: adminID, ListRowIDs: [3]*string{&site, &veg, &soil},
								Values: [3]vprocore.ProjectMetadataCell{vprotest.MetadataText("Good"), vprotest.MetadataText("Good"), vprotest.MetadataText("Good")},
							})
						}
					}
				}
			}
		}
	}
	got, err := planLongVegetationNoneUnits(context.Background(), env, su, &quality)
	if err != nil || len(got) != 128 {
		t.Fatal("WriteBlankSiteUnits must join2 Env to64 qualified occurrences, not existence-filter or single Env", len(got), err)
	}
	tuples := map[[2]int]bool{}
	for i, row := range got {
		if row.QualityOccurrence == nil || *row.QualityOccurrence < 0 || *row.QualityOccurrence >= 64 ||
			row.MembershipRowID != quality.Occurrences[*row.QualityOccurrence].Membership.RowID {
			t.Fatal("derived quality occurrence lost original member index", row)
		}
		envIndex := 0
		if row.EnvRowID == "22" {
			envIndex = 1
		} else if row.EnvRowID != "21" {
			t.Fatal("invented outer Env", row)
		}
		key := [2]int{envIndex, *row.QualityOccurrence}
		if tuples[key] {
			t.Fatal("duplicate/missing derived join tuple", i, row)
		}
		tuples[key] = true
	}
	if len(tuples) != 128 {
		t.Fatal("complete independent outer join coverage lost", len(tuples))
	}
	*got[0].QualityOccurrence = -1
	if *got[64].QualityOccurrence != 0 {
		t.Fatal("outer Env peers alias their occurrence index")
	}
	quality.Occurrences = []vegetationQualityOccurrence{}
	if got, err := planLongVegetationNoneUnits(context.Background(), env, su, &quality); err != nil || got == nil || len(got) != 0 {
		t.Fatal("empty quality selection fell back to unqualified units", got, err)
	}
}

func TestLongVegetationNoneUnitsOwnershipMalformedAndCancellation(t *testing.T) {
	su, env, admin, lists := vegetationQualityFixture()
	quality, err := planLongVegetationQuality(context.Background(), su, env, admin, lists, vegetationQualityCriteria("Good", 0))
	if err != nil {
		t.Fatal(err)
	}
	quality.Occurrences[0].Membership.SiteUnit = vprotest.MetadataText("foreign")
	if got, err := planLongVegetationNoneUnits(context.Background(), env, su, &quality); got != nil || err == nil {
		t.Fatal("quality selection invented original member", got, err)
	}
	for _, invalid := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("0"), vprotest.MetadataText("P\x00"), vprotest.MetadataText("界"), vprotest.MetadataText(string([]byte{0xff}))} {
		su, env, _, _ := vegetationQualityFixture()
		env.Rows[0].Cells[0] = invalid
		if got, err := planLongVegetationNoneUnits(context.Background(), env, su, nil); got != nil || err == nil {
			t.Fatal("invalid plot identity repaired", invalid, got, err)
		}
	}
	su, env, _, _ = vegetationQualityFixture()
	env.Rows[0].Cells[0] = vprotest.MetadataText("p")
	if got, err := planLongVegetationNoneUnits(context.Background(), env, su, nil); got != nil || err == nil {
		t.Fatal("uncalibrated case-equivalent source equality guessed", got, err)
	}
	env.Rows[0].Cells[0] = vprotest.MetadataText("P")
	env.Rows = append(env.Rows, env.Rows[0])
	if got, err := planLongVegetationNoneUnits(context.Background(), env, su, nil); got != nil || err == nil {
		t.Fatal("duplicate physical row identities accepted", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planLongVegetationNoneUnits(ctx, env, su, nil); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled plan returned partial units", got, err)
	}
	if got, err := planLongVegetationNoneUnits(nil, env, su, nil); got != nil || err == nil {
		t.Fatal("nil context authorized plan", got, err)
	}
}
