package main

import (
	"context"
	"database/sql"
	"testing"
)

func TestSelectedUnitOwnedQualityRetainsOriginalFanoutWithoutReweighting(t *testing.T) {
	private, state := vegetationWorkbookServiceFixture(t, false)
	contexts, owner := private.contexts, private.contexts.projects.sqlite
	project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer project.Close()
	if _, err := project.Exec(`DELETE FROM Sample_Admin WHERE Plot IN ('108050','108050x');
		INSERT INTO Sample_Admin(Plot,SitePlotQuality,VegPlotQuality,SoilPlotQuality)
		VALUES('108050','Good','Good','Good'),('108050x','Fair','Fair','Fair')`); err != nil {
		t.Fatal(err)
	}
	lists, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer lists.Close()
	if _, err := lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName='DataQuality';
		INSERT INTO USysTableOfLists(ListName,Item,ItemOrder) VALUES
		('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),('DataQuality','Good',3)`); err != nil {
		t.Fatal(err)
	}
	values, err := contexts.projects.preferences.snapshot()
	if err != nil {
		t.Fatal(err)
	}
	options, err := decodeLongVegetationOptions(values)
	if err != nil {
		t.Fatal(err)
	}
	options.SelectedUnitFields = []string{"PlotNumber"}
	criteria := [3]vegetationQualityCriterion{{Minimum: "Poor"}, {Minimum: "Poor"}, {Minimum: "Poor"}}
	options.Quality = &criteria
	before := databaseBytes(t, owner.attachments)
	source, err := withContextPlotRequest(context.Background(), contexts, state.ContextID,
		func(plots *PlotService) (longVegetationReportSource, error) {
			return withOwnedPreferenceSnapshot(context.Background(), plots, publicationReadSnapshotHooks{},
				func(owner *sqliteContext, tx *sql.Tx) (longVegetationReportSource, error) {
					return readLongVegetationReportSource(context.Background(), owner, tx, options)
				})
		})
	if err != nil {
		t.Fatal(err)
	}
	report, err := longVegetationTransport(context.Background(), source.Report)
	if err != nil || report.Quality == nil || len(report.Quality.Occurrences) != 9 || len(report.SelectedUnits) != 2 ||
		len(report.Units) != 2 || len(source.Tables) != 8 {
		t.Fatal("QCLV8+1 must existence-filter source2 to two original prepared rows", report, err)
	}
	for _, unit := range report.Units {
		if unit.NumPlots != 1 || len(unit.MembershipIDs) != 1 {
			t.Fatal("original reference multiplicity was reapplied to derived source2 units", unit)
		}
	}
	found := false
	for _, diagnostic := range report.Diagnostics {
		if diagnostic.Code == "quality_reference_multiplicity" {
			found = true
			if diagnostic.Identity != "1" || diagnostic.Count != 8 {
				t.Fatal("source2 lost independent original quality fanout diagnostic", diagnostic)
			}
		}
		if diagnostic.Code == "selected_unit_join_multiplicity" {
			t.Fatal("two unique original parents cannot invent derived multiplicity", diagnostic)
		}
	}
	if !found {
		t.Fatal("QCLV provenance fanout diagnostic missing")
	}
	base, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-10",
		source.Tables[5], source.Tables[1], ProjectMetadataTable{})
	if err != nil {
		t.Fatal(err)
	}
	summary, err := vegetationWorkbookSelectedUnitSummary(context.Background(), base, source.Tables[5],
		source.Tables[6], source.Tables[1], options.SelectedUnitFields, report)
	if err != nil || summary.SelectedPlotRows != 2 || len(summary.Memberships) != 2 {
		t.Fatal("source2 summary accidentally reused nine QCLV occurrences", summary, err)
	}
	assertProfileSUFiles(t, contexts, before)
}
