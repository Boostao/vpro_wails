package main

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestVegetationReadAvailabilityPreservesLegacyRowsAndRejectsPartialNumericIdentities(t *testing.T) {
	for _, fixture := range []struct {
		name   string
		values string
		reason string
	}{
		{"exact zero and negative", "(0,'ZERO'),(-1,'NEGATIVE')", ""},
		{"NULL after complete row", "(1,'AAA'),(NULL,'ZZZ')", "NULL ID"},
		{"historical duplicate", "(1,'AAA'),(1,'ZZZ')", "duplicated"},
		{"unsafe positive numeric transport", "(9007199254740992,'UNSAFE')", "numeric transport"},
		{"unsafe negative numeric transport", "(-9007199254740992,'UNSAFE')", "numeric transport"},
		{"exact transport endpoints", "(9007199254740991,'HIGH'),(-9007199254740991,'LOW')", ""},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			plots, db := childFixture(t)
			query := `INSERT INTO Sample_Veg (PlotNumber,ID,Species) SELECT 'CHILD1',column1,column2 FROM (VALUES ` + fixture.values + `)`
			if _, err := db.Exec(query); err != nil {
				t.Fatal(err)
			}
			before := heightTableSnapshot(t, db, "Sample_Veg")
			result, err := vegetationReadAvailability(plots, "CHILD1")
			if err != nil || result == nil {
				t.Fatal(result, err)
			}
			if fixture.reason != "" {
				if result.Available || result.Records != nil || !strings.Contains(result.Reason, fixture.reason) {
					t.Fatal("unsupported identity became partial/synthetic numeric rows", result)
				}
			} else {
				legacy, err := plots.ListVegRecords("CHILD1")
				if err != nil || !result.Available || result.Reason != "" || !reflect.DeepEqual(result.Records, legacy) {
					t.Fatal("valid legacy rows changed", result, legacy, err)
				}
			}
			if heightTableSnapshot(t, db, "Sample_Veg") != before || auditCount(t, db, "CHILD1") != 0 {
				t.Fatal("read-only availability changed vegetation/audits")
			}
		})
	}
	plots, db := childFixture(t)
	empty, err := vegetationReadAvailability(plots, "CHILD1")
	if err != nil || !empty.Available || empty.Reason != "" || len(empty.Records) != 0 {
		t.Fatal("empty ordinary result became an identity error", empty, err)
	}
	if _, err := db.Exec(`DROP TABLE Sample_Veg`); err != nil {
		t.Fatal(err)
	}
	if result, err := vegetationReadAvailability(plots, "CHILD1"); err == nil || result != nil {
		t.Fatal("storage failure became an availability acknowledgement", result, err)
	}
}

func TestVegetationReadAvailabilityOwnedCancellationAndRecovery(t *testing.T) {
	contexts, state, db, _, _ := siviWriteFixture(t, false, 0)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := contexts.GetVegetationReadAvailability(ctx, state.ContextID, "108050"); result != nil || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled availability returned rows/reason", result, err)
	}
	if result, err := contexts.GetVegetationReadAvailability(context.Background(), "foreign", "108050"); result != nil || err == nil {
		t.Fatal("foreign context obtained availability", result, err)
	}
	result, err := contexts.GetVegetationReadAvailability(context.Background(), state.ContextID, "108050")
	if err != nil || !result.Available || len(result.Records) == 0 {
		t.Fatal("owned retry failed", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
	if _, err := db.Exec(`UPDATE Sample_Veg SET ID=NULL WHERE PlotNumber='108050' AND ID=10000001`); err != nil {
		t.Fatal(err)
	}
	before = databaseBytes(t, contexts.projects.sqlite.attachments)
	result, err = contexts.GetVegetationReadAvailability(context.Background(), state.ContextID, "108050")
	if err != nil || result.Available || result.Records != nil || !strings.Contains(result.Reason, "NULL ID") {
		t.Fatal("owned NULL read became partial/failed parent load", result, err)
	}
	assertProfileSUFiles(t, contexts, before)
}
