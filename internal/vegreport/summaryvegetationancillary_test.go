package vegreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func summaryAncillaryTestTable(columns []string, rows ...[]vprocore.ProjectMetadataCell) vprocore.ProjectMetadataTable {
	result := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}}
	for _, name := range columns {
		result.Columns = append(result.Columns, vprocore.ProjectMetadataColumn{Name: name})
	}
	for i, cells := range rows {
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
	}
	return result
}

func summaryAncillaryTestInput() summaryVegetationAncillaryInput {
	text, null := vprotest.MetadataText, vprocore.ProjectMetadataCell{Storage: "null"}
	return summaryVegetationAncillaryInput{
		CreatedAt: "2026-10-17T02:00:00Z",
		SU: summaryAncillaryTestTable([]string{"PlotNumber", "SiteUnit"},
			[]vprocore.ProjectMetadataCell{text("P1"), text("A")}, []vprocore.ProjectMetadataCell{text("P1"), text("A")},
			[]vprocore.ProjectMetadataCell{text("P2"), text("A")}, []vprocore.ProjectMetadataCell{null, text("A")},
			[]vprocore.ProjectMetadataCell{text("P3"), text("B")}, []vprocore.ProjectMetadataCell{text("P4"), text("C")},
			[]vprocore.ProjectMetadataCell{text(""), text("")}, []vprocore.ProjectMetadataCell{text("P5"), null},
			[]vprocore.ProjectMetadataCell{text("P6"), text("N")}, []vprocore.ProjectMetadataCell{text("P7"), text("E")}),
		Veg: summaryAncillaryTestTable([]string{"PlotNumber"},
			[]vprocore.ProjectMetadataCell{text("P1")}, []vprocore.ProjectMetadataCell{text("P1")},
			[]vprocore.ProjectMetadataCell{text("P3")}, []vprocore.ProjectMetadataCell{text("P5")},
			[]vprocore.ProjectMetadataCell{text("P6")}, []vprocore.ProjectMetadataCell{text("P7")},
			[]vprocore.ProjectMetadataCell{text("")}, []vprocore.ProjectMetadataCell{null}),
		Tables: summaryVegetationAncillaryTables{
			Env: summaryAncillaryTestTable([]string{"PlotNumber"},
				[]vprocore.ProjectMetadataCell{text("unrelated")}, []vprocore.ProjectMetadataCell{null}),
			UnitNames: summaryAncillaryTestTable([]string{"SiteSeries", "SiteSeriesLongName"},
				[]vprocore.ProjectMetadataCell{text("A"), text("Alpha")}, []vprocore.ProjectMetadataCell{text("A"), text("Alpha")},
				[]vprocore.ProjectMetadataCell{text("C"), text("No vegetation")}, []vprocore.ProjectMetadataCell{text("a"), text("Literal peer")},
				[]vprocore.ProjectMetadataCell{text(""), text("")}, []vprocore.ProjectMetadataCell{text("N"), null},
				[]vprocore.ProjectMetadataCell{text("E"), text("")}, []vprocore.ProjectMetadataCell{null, text("NULL unit")}),
			Descriptions: summaryAncillaryTestTable([]string{"table_name", "description"},
				[]vprocore.ProjectMetadataCell{text("USysAllSpecs"), text("")}),
		},
	}
}

func TestSummaryAncillaryPhysicalJoinCountsAndRetainedMemberships(t *testing.T) {
	input := summaryAncillaryTestInput()
	before, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := prepareSummaryVegetationAncillary(t.Context(), input)
	if err != nil {
		t.Fatal(err)
	}
	want := []summaryVegetationAncillaryUnit{
		{"", 1, vprotest.MetadataText("")}, {"A", 6, vprotest.MetadataText("Alpha")},
		{"E", 1, vprotest.MetadataText("")}, {"N", 1, vprocore.ProjectMetadataCell{Storage: "null"}},
	}
	if got.CreatedAt != input.CreatedAt || got.EnvironmentRows != 2 || !got.EnvironmentCountAvailable ||
		got.SelectedPlotRows != 9 || len(got.Memberships) != 10 || !reflect.DeepEqual(got.Units, want) ||
		got.VersionStatus != "literal" || !vprocore.MetadataCellEqual(got.SpeciesVersion, vprotest.MetadataText("")) {
		t.Fatal("independent SnTable/COUNT/master fanout or metadata expectations differ", got)
	}
	for i, member := range got.Memberships {
		if member.SourceRowID != strconv.Itoa(i+1) ||
			!vprocore.MetadataCellEqual(member.PlotNumber, input.SU.Rows[i].Cells[0]) ||
			!vprocore.MetadataCellEqual(member.SiteUnit, input.SU.Rows[i].Cells[1]) {
			t.Fatal("physical SU membership reduced or reordered", i, member)
		}
	}
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("source tables mutated")
	}
	*got.Memberships[0].PlotNumber.Text = "changed"
	*got.Units[1].LongName.Text = "changed"
	*got.SpeciesVersion.Text = "changed"
	after, _ = json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("output retains mutable source pointers")
	}
}

func TestSummaryAncillaryEmptyEnvironmentAndDescriptionStates(t *testing.T) {
	for _, state := range []string{"missing", "null", "empty"} {
		t.Run(state, func(t *testing.T) {
			input := summaryAncillaryTestInput()
			input.Tables.Env.Rows = []vprocore.ProjectMetadataRow{}
			expected, status := vprotest.MetadataText("Unknown"), "missing-definition"
			switch state {
			case "missing":
				input.Tables.Descriptions.Rows = []vprocore.ProjectMetadataRow{}
			case "null":
				input.Tables.Descriptions.Rows[0].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
				expected, status = vprocore.ProjectMetadataCell{Storage: "null"}, "null"
			case "empty":
				expected, status = vprotest.MetadataText(""), "literal"
			}
			got, err := prepareSummaryVegetationAncillary(t.Context(), input)
			if err != nil || got.EnvironmentRows != 0 || got.EnvironmentCountAvailable ||
				got.VersionStatus != status || !vprocore.MetadataCellEqual(got.SpeciesVersion, expected) {
				t.Fatal("empty Env or NULL/empty/missing Description collapsed", got, err)
			}
		})
	}
}

func TestSummaryAncillaryRefusalsAndCancellation(t *testing.T) {
	for _, test := range []struct {
		name string
		edit func(*summaryVegetationAncillaryInput)
	}{
		{"missing-time", func(p *summaryVegetationAncillaryInput) { p.CreatedAt = "" }},
		{"noncanonical-time", func(p *summaryVegetationAncillaryInput) { p.CreatedAt = "2026-10-17t02:00:00Z" }},
		{"fractional-time", func(p *summaryVegetationAncillaryInput) { p.CreatedAt = "2026-10-17T02:00:00.1Z" }},
		{"ancient-time", func(p *summaryVegetationAncillaryInput) { p.CreatedAt = "0001-10-17T02:00:00Z" }},
		{"missing-SU-column", func(p *summaryVegetationAncillaryInput) { p.SU.Columns[0].Name = "other" }},
		{"duplicate-row", func(p *summaryVegetationAncillaryInput) { p.SU.Rows[1].RowID = "1" }},
		{"partial-row", func(p *summaryVegetationAncillaryInput) { p.SU.Rows[0].Cells = nil }},
		{"unavailable-Env", func(p *summaryVegetationAncillaryInput) { p.Tables.Env = vprocore.ProjectMetadataTable{} }},
		{"coerced-plot", func(p *summaryVegetationAncillaryInput) {
			n := "1"
			p.Veg.Rows[0].Cells[0] = vprocore.ProjectMetadataCell{Storage: "integer", Integer: &n}
		}},
		{"ambiguous-First", func(p *summaryVegetationAncillaryInput) {
			p.Tables.UnitNames.Rows[1].Cells[1] = vprotest.MetadataText("Different")
		}},
		{"NULL-empty-First", func(p *summaryVegetationAncillaryInput) {
			p.Tables.UnitNames.Rows[1].Cells[1] = vprocore.ProjectMetadataCell{Storage: "null"}
		}},
		{"invalid-unused-text", func(p *summaryVegetationAncillaryInput) {
			p.Tables.UnitNames.Rows[3].Cells[1] = vprotest.MetadataText(string([]byte{0xff}))
		}},
		{"duplicate-description", func(p *summaryVegetationAncillaryInput) {
			row := p.Tables.Descriptions.Rows[0]
			row.RowID = "2"
			p.Tables.Descriptions.Rows = append(p.Tables.Descriptions.Rows, row)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := summaryAncillaryTestInput()
			test.edit(&input)
			got, err := prepareSummaryVegetationAncillary(t.Context(), input)
			if err == nil || !reflect.DeepEqual(got, summaryVegetationAncillaryPlan{}) {
				t.Fatal("unqualified source returned success/partial plan", err, got)
			}
		})
	}
	for _, ctx := range []context.Context{nil,
		&summaryHeaderCancellingContext{Context: context.Background(), remaining: 8}} {
		got, err := prepareSummaryVegetationAncillary(ctx, summaryAncillaryTestInput())
		if err == nil || !reflect.DeepEqual(got, summaryVegetationAncillaryPlan{}) ||
			ctx != nil && !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation returned success/partial plan", got, err)
		}
	}
}

func TestSummaryAncillaryOwnedSnapshotAndReadErrorRetry(t *testing.T) {
	service, state := summaryEnvironmentalLifeformServiceFixture(t, true)
	contexts := service.environment.report.contexts
	owner := contexts.Projects.Sqlite
	vprotest.MutateContextFixture(t, owner.Attachments["VLists"], `
		DROP TABLE IF EXISTS MasterSiteUnitList;
		CREATE TABLE MasterSiteUnitList(SiteSeries TEXT, SiteSeriesLongName TEXT);
		INSERT INTO MasterSiteUnitList VALUES('UNUSED','Retained physical peer');
		DROP TABLE IF EXISTS _table_metadata;
		CREATE TABLE _table_metadata(table_name TEXT, description TEXT);
		INSERT INTO _table_metadata VALUES('USysAllSpecs','fixture version');`)
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	config, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	snapshots := 0
	read := func(ctx context.Context) (summaryVegetationAncillaryInput, error) {
		return vprocore.WithContextPlotRequest(ctx, contexts, state.ContextID, func(plots *vprocore.PlotService) (summaryVegetationAncillaryInput, error) {
			return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{
				CommitRead: func(*sql.Tx) error { snapshots++; return nil },
			}, func(owned *vprocore.SqliteContext, tx *sql.Tx) (summaryVegetationAncillaryInput, error) {
				raw, err := readSummaryVegetationWorkbookSource(ctx, service, plots, owned, tx, state.ContextID,
					summaryVegetationWorkbookOptions{Average: 1, Display: 6})
				if err != nil {
					return summaryVegetationAncillaryInput{}, err
				}
				tables, err := readSummaryVegetationAncillaryTables(ctx, owned, tx)
				if err != nil {
					return summaryVegetationAncillaryInput{}, err
				}
				return summaryVegetationAncillaryInput{
					CreatedAt: "2026-10-17T02:00:00Z", SU: raw.Raw.SU, Veg: raw.Raw.Veg, Tables: tables,
				}, nil
			})
		})
	}
	got, err := read(t.Context())
	if err != nil || snapshots != 1 || len(got.SU.Rows) != 8 || len(got.Veg.Rows) != 27 ||
		len(got.Tables.Env.Rows) != 3 || len(got.Tables.UnitNames.Rows) != 1 ||
		len(got.Tables.Descriptions.Rows) != 1 {
		t.Fatal("one-snapshot complete physical input unavailable", snapshots, got, err)
	}
	if _, err := prepareSummaryVegetationAncillary(t.Context(), got); err != nil {
		t.Fatal(err)
	}
	afterConfig, err := os.ReadFile(contexts.Projects.Preferences.Path)
	if err != nil || string(config) != string(afterConfig) || !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, owner.Attachments)) {
		t.Fatal("ancillary read changed data/audits/configuration", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VLists"], `UPDATE MasterSiteUnitList
		SET SiteSeriesLongName=CAST(X'FF' AS TEXT)`)
	failed, err := read(t.Context())
	if err == nil || !reflect.DeepEqual(failed, summaryVegetationAncillaryInput{}) {
		t.Fatal("malformed unused physical text escaped the reader", failed, err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VLists"], `UPDATE MasterSiteUnitList SET SiteSeriesLongName='Corrected'`)
	if _, err := read(t.Context()); err != nil {
		t.Fatal("read-error correction/retry unavailable", err)
	}
	vprotest.MutateContextFixture(t, owner.Attachments["VLists"], `DROP TABLE MasterSiteUnitList;
		CREATE VIEW MasterSiteUnitList AS SELECT 'A' AS SiteSeries, 'fake' AS SiteSeriesLongName`)
	if _, err := read(t.Context()); err == nil || !strings.Contains(err.Error(), "physical") {
		t.Fatal("view accepted as physical master source", err)
	}
}
