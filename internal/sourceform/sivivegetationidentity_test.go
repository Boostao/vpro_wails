package sourceform

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIIdentityPlanningNullableSigned32AndSharedPhysicalRows(t *testing.T) {
	for _, id := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("-2147483648"), vprotest.MetadataInteger("2147483647"), vprotest.MetadataInteger("-1"), vprotest.MetadataInteger("1"), {Storage: "null"}} {
		veg := vprotest.SiviProjectionFixture()
		before := vprotest.SiviProjectionFixture()
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
			[]SiviHeightEdit{{"7", "SubVegD-SIVI", "ID", vprotest.MetadataInteger("0"), id}}, nil)
		if err != nil || len(got) != 1 || got[0].RowID != "7" || got[0].Column != "ID" {
			t.Fatal("nullable signed32/source-shared ID rejected", got, err)
		}
		planned, err := applySIVIIdentityPlan(veg, got)
		if err != nil || !reflect.DeepEqual(planned.Rows[6].Cells[0], id) || !reflect.DeepEqual(veg, before) {
			t.Fatal("mutable ID changed physical row or aliased original", planned, err)
		}
		for i, row := range planned.Rows {
			if row.RowID != before.Rows[i].RowID || !reflect.DeepEqual(row.Cells[1:], before.Rows[i].Cells[1:]) {
				t.Fatal("ID plan changed unrelated source cells")
			}
		}
		for _, group := range []int{0, 1, 2} {
			projections, err := ProjectSIVIVegetation(context.Background(), "P", false, planned)
			if err != nil {
				t.Fatal(err)
			}
			last := projections[group].Rows[len(projections[group].Rows)-1]
			if last.RowID != "7" || !reflect.DeepEqual(last.Cells[0], id) {
				t.Fatal("shared group retained stale application ID")
			}
		}
	}
}

func TestSIVIIdentityPlanningPreservesHistoricalInvalidNoops(t *testing.T) {
	for _, id := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("historical"), vprotest.MetadataInteger("2147483648"), vprotest.SiviReal(1), {Storage: "null"}} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[0].Cells[0] = id
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", id, id}}, map[string]bool{"0": true})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged historical ID was validated as a new assignment", got, err)
		}
	}
	veg := vprotest.SiviProjectionFixture()
	veg.Rows[0].Cells[0] = vprotest.MetadataText("historical")
	got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
		[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataText("historical"), vprotest.MetadataInteger("1")}}, nil)
	if got != nil || err == nil {
		t.Fatal("ungranted historical-storage repair returned assignments", got, err)
	}
}

func TestSIVIIdentityPlanningRejectsCollisionReservationSwapAndRawValues(t *testing.T) {
	for _, cell := range []vprocore.ProjectMetadataCell{
		vprotest.MetadataInteger("2147483648"), vprotest.MetadataInteger("-2147483649"), vprotest.MetadataInteger("01"), vprotest.MetadataInteger("+1"),
		vprotest.MetadataInteger("-0"), vprotest.MetadataInteger(" 1 "), vprotest.MetadataText("1"), vprotest.SiviReal(1), vprotest.SiviReal(math.NaN()),
	} {
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(),
			[]SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), cell}}, nil)
		if err == nil || got != nil {
			t.Fatal("new ID was silently coerced", cell, got, err)
		}
	}
	edits := []SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")}}
	for _, owner := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.SiviReal(1), vprotest.MetadataText(" 1e0 ")} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[7].Cells[0] = owner
		if got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg, edits, nil); err == nil || got != nil {
			t.Fatal("ID collision outside plot/group was overlooked", got, err)
		}
	}
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), edits, map[string]bool{"1": true}); err == nil || got != nil {
		t.Fatal("deleted/reserved ID was reassigned", got, err)
	}
	repeated := append(append([]SiviHeightEdit{}, edits...), SiviHeightEdit{"2", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")})
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), repeated, nil); err == nil || got != nil {
		t.Fatal("batch introduced a duplicate identity", got, err)
	}
	veg := vprotest.SiviProjectionFixture()
	veg.Rows[0].Cells[0], veg.Rows[1].Cells[0] = vprotest.MetadataInteger("1"), vprotest.MetadataInteger("2")
	swap := []SiviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("1"), vprotest.MetadataInteger("2")}, {"2", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("2"), vprotest.MetadataInteger("1")}}
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg, swap, nil); err == nil || got != nil {
		t.Fatal("identity swap was implicitly authorized", got, err)
	}
}

func TestSIVIIdentityPlanningExactOwnershipContextsAndCancellation(t *testing.T) {
	for _, edit := range []SiviHeightEdit{
		{"01", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")},
		{"3", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")},
		{"8", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")},
		{"1", "SubVegA-SIVI", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")},
		{"1", "SubVegA-SIVI_BC", "PlotNumber", vprotest.MetadataText("P"), vprotest.MetadataText("Other")},
		{"1", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("2"), vprotest.MetadataInteger("1")},
	} {
		if got, err := planSIVIIdentityEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), []SiviHeightEdit{edit}, nil); err == nil || got != nil {
			t.Fatal("foreign/hidden/stale identity edit returned assignments", edit, got, err)
		}
	}
	duplicate := SiviHeightEdit{"7", "SubVegA-SIVI_BC", "ID", vprotest.MetadataInteger("0"), vprotest.MetadataInteger("1")}
	other := duplicate
	other.Form = "SubVegD-SIVI"
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), []SiviHeightEdit{duplicate, other}, nil); err == nil || got != nil {
		t.Fatal("shared physical ID was written twice", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSIVIIdentityEdits(ctx, "P", false, vprotest.SiviProjectionFixture(), []SiviHeightEdit{duplicate}, nil); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled identity planning returned assignments", got, err)
	}
}

func TestSIVIIdentityReservationsKeepTableAliasesAndNULLDistinct(t *testing.T) {
	table := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Table"}, {Name: "ID"}}, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("_vEg"), vprotest.MetadataInteger("-1")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("SAMPLE_VEG"), vprotest.MetadataInteger("2")}},
		{RowID: "3", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("_Other"), vprotest.MetadataInteger("3")}},
		{RowID: "4", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("_Veg"), {Storage: "null"}}},
	}}
	ledger := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "ChildTable"}, {Name: "ID"}}, Rows: []vprocore.ProjectMetadataRow{
		{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(`"Sample_Veg"`), vprotest.MetadataInteger("4")}},
		{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(`"Sample_Other"`), vprotest.MetadataInteger("5")}},
	}}
	got, err := siviIdentityReservedIDs("Sample", map[string]vprocore.ProjectMetadataTable{"Sample_Audit": table, "__VPRO_ChildIdentity": ledger})
	if err != nil || !reflect.DeepEqual(got, map[string]bool{"-1": true, "2": true, "4": true}) {
		t.Fatal("reservation aliases/NULL/families collapsed", got, err)
	}
	for _, invalid := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("4"), {Storage: "null"}, vprotest.MetadataInteger("2147483648")} {
		ledger.Rows[0].Cells[1] = invalid
		if got, err := siviIdentityReservedIDs("Sample", map[string]vprocore.ProjectMetadataTable{"Sample_Audit": table, "__VPRO_ChildIdentity": ledger}); err == nil || got != nil {
			t.Fatal("invalid owned reservation silently ignored", got, err)
		}
	}
}

func TestSIVIIdentityReturnOccupantsPreserveHistoricalDuplicates(t *testing.T) {
	veg := vprotest.SiviProjectionFixture()
	assignments := []SiviHeightAssignment{{RowID: "1", Column: "ID", Before: vprotest.MetadataInteger("0"), After: vprotest.MetadataInteger("1")}}
	got, err := siviIdentityReturnOccupants(veg, assignments)
	if err != nil || len(got) != 8 || got[0].RowID != "2" || got[7].RowID != "9" {
		t.Fatal("restoration lost pre-existing duplicates outside the selected plot", got, err)
	}
	*got[0].Cells[0].Integer = "99"
	if *veg.Rows[1].Cells[0].Integer != "0" {
		t.Fatal("return occupancy aliases physical source")
	}
	if err := ValidateSIVIIdentityCell("ID", vprotest.MetadataInteger(strings.Repeat("1", 30))); err == nil {
		t.Fatal("overlong ID text parsed as an integer")
	}
}
