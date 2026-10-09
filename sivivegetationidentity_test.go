package main

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestSIVIIdentityPlanningNullableSigned32AndSharedPhysicalRows(t *testing.T) {
	for _, id := range []ProjectMetadataCell{metadataInteger("-2147483648"), metadataInteger("2147483647"), metadataInteger("-1"), metadataInteger("1"), {Storage: "null"}} {
		veg := siviProjectionFixture()
		before := siviProjectionFixture()
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
			[]siviHeightEdit{{"7", "SubVegD-SIVI", "ID", metadataInteger("0"), id}}, nil)
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
			projections, err := projectSIVIVegetation(context.Background(), "P", false, planned)
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
	for _, id := range []ProjectMetadataCell{metadataText("historical"), metadataInteger("2147483648"), siviReal(1), {Storage: "null"}} {
		veg := siviProjectionFixture()
		veg.Rows[0].Cells[0] = id
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", id, id}}, map[string]bool{"0": true})
		if err != nil || got == nil || len(got) != 0 {
			t.Fatal("unchanged historical ID was validated as a new assignment", got, err)
		}
	}
	veg := siviProjectionFixture()
	veg.Rows[0].Cells[0] = metadataText("historical")
	got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg,
		[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", metadataText("historical"), metadataInteger("1")}}, nil)
	if got != nil || err == nil {
		t.Fatal("ungranted historical-storage repair returned assignments", got, err)
	}
}

func TestSIVIIdentityPlanningRejectsCollisionReservationSwapAndRawValues(t *testing.T) {
	for _, cell := range []ProjectMetadataCell{
		metadataInteger("2147483648"), metadataInteger("-2147483649"), metadataInteger("01"), metadataInteger("+1"),
		metadataInteger("-0"), metadataInteger(" 1 "), metadataText("1"), siviReal(1), siviReal(math.NaN()),
	} {
		got, err := planSIVIIdentityEdits(context.Background(), "P", false, siviProjectionFixture(),
			[]siviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), cell}}, nil)
		if err == nil || got != nil {
			t.Fatal("new ID was silently coerced", cell, got, err)
		}
	}
	edits := []siviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")}}
	for _, owner := range []ProjectMetadataCell{metadataInteger("1"), siviReal(1), metadataText(" 1e0 ")} {
		veg := siviProjectionFixture()
		veg.Rows[7].Cells[0] = owner
		if got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg, edits, nil); err == nil || got != nil {
			t.Fatal("ID collision outside plot/group was overlooked", got, err)
		}
	}
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, siviProjectionFixture(), edits, map[string]bool{"1": true}); err == nil || got != nil {
		t.Fatal("deleted/reserved ID was reassigned", got, err)
	}
	repeated := append(append([]siviHeightEdit{}, edits...), siviHeightEdit{"2", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")})
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, siviProjectionFixture(), repeated, nil); err == nil || got != nil {
		t.Fatal("batch introduced a duplicate identity", got, err)
	}
	veg := siviProjectionFixture()
	veg.Rows[0].Cells[0], veg.Rows[1].Cells[0] = metadataInteger("1"), metadataInteger("2")
	swap := []siviHeightEdit{{"1", "SubVegA-SIVI_BC", "ID", metadataInteger("1"), metadataInteger("2")}, {"2", "SubVegA-SIVI_BC", "ID", metadataInteger("2"), metadataInteger("1")}}
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, veg, swap, nil); err == nil || got != nil {
		t.Fatal("identity swap was implicitly authorized", got, err)
	}
}

func TestSIVIIdentityPlanningExactOwnershipContextsAndCancellation(t *testing.T) {
	for _, edit := range []siviHeightEdit{
		{"01", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")},
		{"3", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")},
		{"8", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")},
		{"1", "SubVegA-SIVI", "ID", metadataInteger("0"), metadataInteger("1")},
		{"1", "SubVegA-SIVI_BC", "PlotNumber", metadataText("P"), metadataText("Other")},
		{"1", "SubVegA-SIVI_BC", "ID", metadataInteger("2"), metadataInteger("1")},
	} {
		if got, err := planSIVIIdentityEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{edit}, nil); err == nil || got != nil {
			t.Fatal("foreign/hidden/stale identity edit returned assignments", edit, got, err)
		}
	}
	duplicate := siviHeightEdit{"7", "SubVegA-SIVI_BC", "ID", metadataInteger("0"), metadataInteger("1")}
	other := duplicate
	other.Form = "SubVegD-SIVI"
	if got, err := planSIVIIdentityEdits(context.Background(), "P", false, siviProjectionFixture(), []siviHeightEdit{duplicate, other}, nil); err == nil || got != nil {
		t.Fatal("shared physical ID was written twice", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planSIVIIdentityEdits(ctx, "P", false, siviProjectionFixture(), []siviHeightEdit{duplicate}, nil); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled identity planning returned assignments", got, err)
	}
}

func TestSIVIIdentityReservationsKeepTableAliasesAndNULLDistinct(t *testing.T) {
	table := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "Table"}, {Name: "ID"}}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("_vEg"), metadataInteger("-1")}},
		{"2", []ProjectMetadataCell{metadataText("SAMPLE_VEG"), metadataInteger("2")}},
		{"3", []ProjectMetadataCell{metadataText("_Other"), metadataInteger("3")}},
		{"4", []ProjectMetadataCell{metadataText("_Veg"), {Storage: "null"}}},
	}}
	ledger := ProjectMetadataTable{Columns: []ProjectMetadataColumn{{Name: "ChildTable"}, {Name: "ID"}}, Rows: []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText(`"Sample_Veg"`), metadataInteger("4")}},
		{"2", []ProjectMetadataCell{metadataText(`"Sample_Other"`), metadataInteger("5")}},
	}}
	got, err := siviIdentityReservedIDs("Sample", map[string]ProjectMetadataTable{"Sample_Audit": table, "__VPRO_ChildIdentity": ledger})
	if err != nil || !reflect.DeepEqual(got, map[string]bool{"-1": true, "2": true, "4": true}) {
		t.Fatal("reservation aliases/NULL/families collapsed", got, err)
	}
	for _, invalid := range []ProjectMetadataCell{metadataText("4"), {Storage: "null"}, metadataInteger("2147483648")} {
		ledger.Rows[0].Cells[1] = invalid
		if got, err := siviIdentityReservedIDs("Sample", map[string]ProjectMetadataTable{"Sample_Audit": table, "__VPRO_ChildIdentity": ledger}); err == nil || got != nil {
			t.Fatal("invalid owned reservation silently ignored", got, err)
		}
	}
}

func TestSIVIIdentityReturnOccupantsPreserveHistoricalDuplicates(t *testing.T) {
	veg := siviProjectionFixture()
	assignments := []siviHeightAssignment{{RowID: "1", Column: "ID", Before: metadataInteger("0"), After: metadataInteger("1")}}
	got, err := siviIdentityReturnOccupants(veg, assignments)
	if err != nil || len(got) != 8 || got[0].RowID != "2" || got[7].RowID != "9" {
		t.Fatal("restoration lost pre-existing duplicates outside the selected plot", got, err)
	}
	*got[0].Cells[0].Integer = "99"
	if *veg.Rows[1].Cells[0].Integer != "0" {
		t.Fatal("return occupancy aliases physical source")
	}
	if err := validateSIVIIdentityCell("ID", metadataInteger(strings.Repeat("1", 30))); err == nil {
		t.Fatal("overlong ID text parsed as an integer")
	}
}
