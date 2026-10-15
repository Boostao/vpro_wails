package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"reflect"
	"testing"
)

func projectOwnedSourceFixture(t *testing.T, external, none bool) (*ContextService, ProjectState, longVegetationOptions) {
	t.Helper()
	private, state := vegetationWorkbookServiceFixture(t, external)
	contexts := private.contexts
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`CREATE TABLE FixtureEnv AS SELECT * FROM Sample_Env WHERE 0;
		CREATE TABLE FixtureAdmin AS SELECT * FROM Sample_Admin WHERE 0;
		DROP TABLE Sample_Env; DROP TABLE Sample_Admin;
		CREATE TABLE Sample_Env AS SELECT * FROM FixtureEnv;
		CREATE TABLE Sample_Admin AS SELECT * FROM FixtureAdmin;
		DROP TABLE FixtureEnv; DROP TABLE FixtureAdmin;
		INSERT INTO Sample_Env(PlotNumber) VALUES('108050'),('108050'),('108050x'),(NULL),('env-only');
		INSERT INTO Sample_Admin(Plot,SitePlotQuality,VegPlotQuality,SoilPlotQuality)
		VALUES('108050','Good','Good','Good'),('108050','Good','Good','Good'),('108050x','Fair','Fair','Fair')`)
	if err = errors.Join(err, db.Close()); err != nil {
		t.Fatal(err)
	}
	if none {
		selection := contextSelection(state)
		selection.SU, selection.SUPath = "None", ""
		state, err = contexts.SwitchContext(state.ContextID, selection)
		if err != nil {
			t.Fatal(err)
		}
	}
	options, err := loadLongVegetationOptions(contexts.plots)
	if err != nil {
		t.Fatal(err)
	}
	return contexts, state, options
}

func projectOwnedSourceRead(ctx context.Context, contexts *ContextService, state ProjectState,
	fields []string, options longVegetationOptions) (longVegetationReportSource, error) {
	return withContextPlotRequest(ctx, contexts, state.ContextID, func(plots *PlotService) (longVegetationReportSource, error) {
		return withOwnedPreferenceSnapshot(ctx, plots, publicationReadSnapshotHooks{},
			func(owner *sqliteContext, tx *sql.Tx) (longVegetationReportSource, error) {
				return readLongVegetationProjectSource(ctx, owner, tx, fields, options)
			})
	})
}

func TestProjectOwnedSourceOriginalPhysicalTuplesAcrossAdapters(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, none := range []bool{false, true} {
			contexts, state, base := projectOwnedSourceFixture(t, external, none)
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			for _, all := range []bool{false, true} {
				if all && !none {
					continue
				}
				fields := []string{"PlotNumber"}
				if all {
					fields = nil
				}
				for _, mode := range []string{"layer", "none", "strata", "lifeform"} {
					options := base
					options.NoneGrouping = mode == "none"
					options.StrataGrouping = mode == "strata"
					options.LifeformGrouping = mode == "lifeform"
					source, err := projectOwnedSourceRead(context.Background(), contexts, state, fields, options)
					if err != nil {
						t.Fatalf("external=%v NoneSU=%v all=%v mode=%s: %v", external, none, all, mode, err)
					}
					report := source.Report
					if len(report.ProjectUnits) != 5 ||
						report.SelectedUnits != nil || report.NoneUnits != nil {
						t.Fatal("physical tuples replaced by filtered TEMP view or fabricated SU memberships", report)
					}
					counts := map[string]int{"text:108050": 4, "text:108050x": 1}
					if all {
						counts = map[string]int{"text:All plots": 5}
						if report.ProjectUnits[3].PlotNumber.Storage != "null" {
							t.Fatal("Env-only original NULL denominator lost", report.ProjectUnits)
						}
					}
					if len(report.Units) != len(counts) {
						t.Fatal("unexpected original label grouping", report.Units)
					}
					for _, unit := range report.Units {
						if unit.NumPlots != counts[vegetationTextKey(unit.Code)] ||
							len(unit.ProjectUnitIndexes) != unit.NumPlots || len(unit.MembershipIDs) != 0 {
							t.Fatal("project denominator differs from independent physical joins", unit)
						}
						for _, index := range unit.ProjectUnitIndexes {
							if index < 0 || index >= len(report.ProjectUnits) ||
								!reflect.DeepEqual(unit.Code, report.ProjectUnits[index].SiteUnit) {
								t.Fatal("original tuple index ownership differs", unit)
							}
						}
					}
					if dto, err := longVegetationTransport(context.Background(), report); err == nil ||
						!reflect.DeepEqual(dto, LongVegetationReport{}) {
						t.Fatal("private reader enabled incomplete public wire", dto, err)
					}
				}
			}
			assertProfileSUFiles(t, contexts, before)
			after, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("physical source reader changed preferences", err)
			}
		}
	}
}

func TestProjectOwnedSourceQualityUsesOriginalSUWithoutProjectReweighting(t *testing.T) {
	for _, external := range []bool{false, true} {
		contexts, state, options := projectOwnedSourceFixture(t, external, false)
		project, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = project.Exec(`DELETE FROM Sample_Env WHERE rowid=2; DELETE FROM Sample_Admin WHERE rowid=2`)
		if err = errors.Join(err, project.Close()); err != nil {
			t.Fatal(err)
		}
		lists, err := sql.Open("sqlite3", sqliteFileURI(contexts.projects.sqlite.attachments["VLists"], "rw"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = lists.Exec(`DELETE FROM USysTableOfLists WHERE ListName='DataQuality';
			INSERT INTO USysTableOfLists(ListName,Item,ItemOrder)
			VALUES('DataQuality','Poor',1),('DataQuality','Fair',2),('DataQuality','Good',3),('DataQuality','Good',3)`)
		if err = errors.Join(err, lists.Close()); err != nil {
			t.Fatal(err)
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		criteria := [3]vegetationQualityCriterion{{Minimum: "Poor"}, {Minimum: "Poor"}, {Minimum: "Poor"}}
		options.Quality = &criteria
		source, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options)
		if err != nil || source.Report.Quality == nil || len(source.Report.Quality.Occurrences) != 9 ||
			len(source.Report.ProjectUnits) != 2 || len(source.Report.Units) != 2 {
			t.Fatal("original QCLV8+1 must not reweight two project tuples", source.Report, err)
		}
		found := false
		for _, diagnostic := range source.Report.Diagnostics {
			if diagnostic.Code == "quality_reference_multiplicity" {
				found = true
				if diagnostic.Identity != "1" || diagnostic.Count != 8 {
					t.Fatal("original SU quality fanout evidence lost", diagnostic)
				}
			}
		}
		if !found {
			t.Fatal("quality multiplicity evidence missing")
		}
		for _, unit := range source.Report.Units {
			want := 1
			if unit.NumPlots != want || len(unit.MembershipIDs) != 0 {
				t.Fatal("QCLV fanout became project denominator", unit)
			}
		}
		assertProfileSUFiles(t, contexts, before)
		strict := [3]vegetationQualityCriterion{{Minimum: "Good"}, {Minimum: "Good"}, {Minimum: "Good"}}
		options.Quality = &strict
		source, err = projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options)
		if err != nil || len(source.Report.ProjectUnits) != 1 || len(source.Report.Units) != 1 ||
			source.Report.Units[0].NumPlots != 1 || *source.Report.ProjectUnits[0].PlotNumber.Text != "108050" {
			t.Fatal("actual original SU qualification failed to prune project tuples by existence", source.Report, err)
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestProjectOwnedSourcePhysicalReadErrorsRemainUnavailableUntilCorrection(t *testing.T) {
	contexts, state, options := projectOwnedSourceFixture(t, true, false)
	db, err := sql.Open("sqlite3", sqliteFileURI(state.ProjectPath, "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, corruption := range []struct{ damage, repair string }{
		{`UPDATE Sample_Env SET PlotNumber=CAST(X'FF' AS TEXT) WHERE rowid=1`,
			`UPDATE Sample_Env SET PlotNumber='108050' WHERE rowid=1`},
		{`CREATE TABLE FixtureAdmin AS SELECT * FROM Sample_Admin; DROP TABLE Sample_Admin`,
			`CREATE TABLE Sample_Admin AS SELECT * FROM FixtureAdmin; DROP TABLE FixtureAdmin`},
	} {
		if _, err := db.Exec(corruption.damage); err != nil {
			t.Fatal(err)
		}
		if source, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options); err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
			t.Fatal("physical source read failure became successful repaired/default report", source, err)
		}
		if _, err := db.Exec(corruption.repair); err != nil {
			t.Fatal(err)
		}
		if source, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options); err != nil || len(source.Report.ProjectUnits) != 5 {
			t.Fatal("source correction/retry lost physical tuples or snapshot attachments", source.Report, err)
		}
	}
	criteria := [3]vegetationQualityCriterion{{Minimum: "Poor"}, {Minimum: "Poor"}, {Minimum: "Poor"}}
	options.Quality = &criteria
	if source, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options); err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
		t.Fatal("project reader invented unsupported duplicate-parent QCLV authority", source, err)
	}
}

func TestProjectOwnedSourceRefusalCancellationAndRetry(t *testing.T) {
	contexts, state, options := projectOwnedSourceFixture(t, false, false)
	criteria := [3]vegetationQualityCriterion{{Minimum: "Poor"}, {Minimum: "Poor"}, {Minimum: "Poor"}}
	for _, fields := range [][]string{nil, {}, {"missing"}, {"PlotNumber", "PlotNumber"}} {
		source, err := projectOwnedSourceRead(context.Background(), contexts, state, fields, options)
		if err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
			t.Fatal("ambiguous/invalid source accepted", fields, source, err)
		}
	}
	for _, mixed := range []longVegetationOptions{
		{NoneUnits: true}, {SelectedUnitFields: []string{"PlotNumber"}},
	} {
		if source, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, mixed); err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
			t.Fatal("mixed selected-list/project authority accepted", source, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if source, err := projectOwnedSourceRead(ctx, contexts, state, []string{"PlotNumber"}, options); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(source, longVegetationReportSource{}) {
		t.Fatal("cancelled source returned output", source, err)
	}
	if _, err := projectOwnedSourceRead(context.Background(), contexts, state, []string{"PlotNumber"}, options); err != nil {
		t.Fatal("cancelled source leaked lease", err)
	}
	contexts, state, options = projectOwnedSourceFixture(t, true, true)
	options.Quality = &criteria
	for _, fields := range [][]string{nil, {"PlotNumber"}} {
		if source, err := projectOwnedSourceRead(context.Background(), contexts, state, fields, options); err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
			t.Fatal("None source inferred original SU quality authority", source, err)
		}
	}
	options.Quality = nil
	owner := contexts.projects.sqlite
	original := owner.attachmentInfo["project"]
	owner.attachmentInfo["project"] = owner.attachmentInfo["VLists"]
	if source, err := projectOwnedSourceRead(context.Background(), contexts, state, nil, options); err == nil || !reflect.DeepEqual(source, longVegetationReportSource{}) {
		t.Fatal("changed physical file ownership accepted", source, err)
	}
	owner.attachmentInfo["project"] = original
	if _, err := projectOwnedSourceRead(context.Background(), contexts, state, nil, options); err != nil {
		t.Fatal("ownership refusal leaked lease", err)
	}
}
