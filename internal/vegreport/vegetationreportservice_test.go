package vegreport

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLongVegetationServiceOwnedIdentitySettingsAndZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := vprotest.ReportServiceFixture(t, external)
			before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
			config, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil {
				t.Fatal(err)
			}
			options, err := GetLongVegetationOptions(service, context.Background(), state.ContextID)
			if err != nil || options.ContextID != state.ContextID || options.Project != state.ActiveProject ||
				options.ProjectPath != state.ProjectPath || options.SU != state.ActiveSU || options.SUPath != state.SUPath ||
				options.Settings.Title != "Long Vegetation Report" || options.Settings.Average != "all-plots" ||
				!options.Settings.ConstantSpeciesList {
				t.Fatal("retained owned settings differ", options, err)
			}
			preview, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
			if err != nil || preview.ContextID != state.ContextID || preview.ProjectPath != state.ProjectPath ||
				preview.SUPath != state.SUPath || preview.Report.Project != state.ActiveProject ||
				preview.Report.SU != state.ActiveSU || preview.Report.Title != options.Settings.Title ||
				!reflect.DeepEqual(preview.Settings, options.Settings) || len(preview.Report.Units) != 2 {
				t.Fatal("typed preview lost source/owner/settings", preview, err)
			}
			again, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
			if err != nil || !reflect.DeepEqual(preview, again) {
				t.Fatal("repeated public preview differs", err)
			}
			for _, unit := range preview.Report.Units {
				if unit.NameCandidates == nil ||
					unit.Code.Storage == "null" && (unit.NameStatus != "unassigned" || unit.LongName == nil || *unit.LongName != "") ||
					unit.Code.Storage == "text" && (unit.NameStatus != "missing" || unit.LongName != nil) {
					t.Fatal("source unit-name metadata/defaults differ", unit)
				}
			}
			raw, err := json.Marshal(preview)
			if err != nil || bytes.Contains(raw, []byte(`"Units"`)) || !bytes.Contains(raw, []byte(`"membershipIds"`)) ||
				!bytes.Contains(raw, []byte(`"matchedName"`)) || !bytes.Contains(raw, []byte(`"settings"`)) {
				t.Fatal("transport exposes internal field names or loses metadata", string(raw), err)
			}
			var decoded LongVegetationPreview
			if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, preview) {
				t.Fatal("typed JSON roundtrip differs", err)
			}
			vprotest.AssertProfileSUFiles(t, service, before)
			after, err := os.ReadFile(service.Projects.Preferences.Path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("public read wrote preferences", err)
			}
		})
	}
}

func TestLongVegetationServiceReadsUnitNamesInOwnedSnapshot(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner := service.Projects.Sqlite
	db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(owner.Attachments["VLists"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	code := "  Unit 'quoted'  "
	for _, name := range []any{"  Reference name  ", nil, "  Reference name  "} {
		if _, err := db.Exec(`INSERT INTO MasterSiteUnitList(ID,SiteSeries,SiteSeriesLongName)
			SELECT COALESCE(MAX(ID),0)+1,?,? FROM MasterSiteUnitList`, code, name); err != nil {
			t.Fatal(err)
		}
	}
	before := vprotest.DatabaseBytes(t, owner.Attachments)
	preview, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	var unit LongVegetationUnit
	for _, candidate := range preview.Report.Units {
		if candidate.Code.Text != nil && *candidate.Code.Text == code {
			unit = candidate
		}
	}
	if unit.NameStatus != "unique" || unit.LongName == nil || *unit.LongName != "  Reference name  " ||
		len(unit.NameCandidates) != 3 || unit.NumPlots != 1 {
		t.Fatal("owned names guessed or duplicate references changed denominator", unit)
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	if _, err := db.Exec(`INSERT INTO MasterSiteUnitList(ID,SiteSeries,SiteSeriesLongName)
		SELECT COALESCE(MAX(ID),0)+1,?,'Other' FROM MasterSiteUnitList`, code); err != nil {
		t.Fatal(err)
	}
	conflict, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range conflict.Report.Units {
		if candidate.Code.Text != nil && *candidate.Code.Text == code &&
			(candidate.NameStatus != "conflicting" || candidate.LongName != nil || len(candidate.NameCandidates) != 4 ||
				!reflect.DeepEqual(candidate.Rows, unit.Rows) || candidate.NumPlots != unit.NumPlots) {
			t.Fatal("name ambiguity altered statistics or arbitrarily chose a definition", candidate)
		}
	}
}

func TestLongVegetationServiceCancellationStaleNoneAndOwnership(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	owner := service.Projects.Sqlite
	owner.Mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	preview, err := PreviewLongVegetation(service, ctx, state.ContextID)
	cancel()
	owner.Mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(preview, LongVegetationPreview{}) {
		t.Fatal("waiting cancellation returned output", preview, err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	cancel()
	if options, err := GetLongVegetationOptions(service, ctx, state.ContextID); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(options, LongVegetationOptions{}) {
		t.Fatal("cancelled options returned output", options, err)
	}
	original := owner.AttachmentInfo["su"]
	owner.AttachmentInfo["su"] = owner.AttachmentInfo["project"]
	for _, read := range []func() error{
		func() error {
			value, err := GetLongVegetationOptions(service, context.Background(), state.ContextID)
			if !reflect.DeepEqual(value, LongVegetationOptions{}) {
				t.Fatal("invalid owner returned options", value)
			}
			return err
		},
		func() error {
			value, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
			if !reflect.DeepEqual(value, LongVegetationPreview{}) {
				t.Fatal("invalid owner returned preview", value)
			}
			return err
		},
	} {
		if err := read(); err == nil {
			t.Fatal("changed SU ownership accepted")
		}
	}
	owner.AttachmentInfo["su"] = original
	if _, err := PreviewLongVegetation(service, context.Background(), state.ContextID); err != nil {
		t.Fatal("failed read leaked lease", err)
	}
	selection := vprotest.ContextSelection(state)
	selection.SU, selection.SUPath = "None", ""
	next, err := service.SwitchContext(state.ContextID, selection)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{state.ContextID, next.ContextID} {
		if _, err := GetLongVegetationOptions(service, context.Background(), id); err == nil {
			t.Fatal("stale/None options accepted")
		}
		if _, err := PreviewLongVegetation(service, context.Background(), id); err == nil {
			t.Fatal("stale/None preview expanded scope")
		}
	}
}

func TestLongVegetationServiceUsesCurrentStrictConfiguration(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{
		"LVReportTitle": "  Remembered \u00e9 title  ", "LVAvgType": 20,
		"LVConstantSppList": 0, "LVPresenceGreaterThan": -2.5, "LVCoverGreaterThan": -3.5,
	}); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
	if err != nil || preview.Settings.Title != "  Remembered \u00e9 title  " ||
		preview.Settings.Average != "observations" || preview.Settings.ConstantSpeciesList ||
		preview.Settings.PresenceGreaterThan != -2.5 || preview.Settings.MeanCoverGreaterThan != -3.5 ||
		preview.Report.Title != preview.Settings.Title {
		t.Fatal("preview did not use current unchanged settings", preview, err)
	}
	for _, update := range []map[string]any{{"LVGroupBy": 3}, {"LVGroupBy": 1, "DataQualityFilterEnforceLV": -1, "DataQualityFilterSiteNullLV": nil}} {
		if err := service.Projects.Preferences.Update("ReportOptions", update); err != nil {
			t.Fatal(err)
		}
		if value, err := GetLongVegetationOptions(service, context.Background(), state.ContextID); err == nil ||
			!reflect.DeepEqual(value, LongVegetationOptions{}) {
			t.Fatal("unsupported configured options silently accepted", value, err)
		}
		if value, err := PreviewLongVegetation(service, context.Background(), state.ContextID); err == nil ||
			!reflect.DeepEqual(value, LongVegetationPreview{}) {
			t.Fatal("unsupported configured mode silently accepted", value, err)
		}
	}
}

func TestLongVegetationTransportPreservesNullEmptyFanoutAndAliases(t *testing.T) {
	empty, name, zero := "", "name", 0.0
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	text := vprocore.ProjectMetadataCell{Storage: "text", Text: &empty}
	source := vegetationLayerReport{Project: "P", SU: "S", Title: "T",
		Units: []vegetationLayerUnit{{Code: null, NumPlots: 2, MembershipIDs: []string{"1", "2"},
			Rows: []vegetationLayerRow{
				{Layer: text, Species: text, EnglishName: null, MatchedName: text,
					Presence: &zero, MeanCover: &zero, Plots: []vegetationCrosstabPlot{{"", &zero}}},
				{Layer: text, Species: text, EnglishName: text, MatchedName: vprocore.ProjectMetadataCell{Storage: "text", Text: &name},
					Plots: []vegetationCrosstabPlot{{"", nil}}},
			}}},
		Diagnostics: []vegetationLayerDiagnostic{{"constant_list_name_fanout", "text:/text:", 2}},
	}
	report, err := longVegetationTransport(context.Background(), source)
	if err != nil || len(report.Units[0].Rows) != 2 || report.Units[0].Code.Storage != "null" ||
		*report.Units[0].Rows[0].Species.Text != "" || report.Units[0].Rows[1].Presence != nil ||
		report.Units[0].Rows[1].Plots[0].Cover != nil || report.Units[0].Rows[0].Presence == nil ||
		*report.Units[0].Rows[0].Plots[0].Cover != 0 {
		t.Fatal("typed fanout/NULL/empty/zero changed", report, err)
	}
	*report.Units[0].Rows[0].Species.Text = "changed"
	*report.Units[0].Rows[0].Presence = 5
	*report.Units[0].Rows[0].Plots[0].Cover = 6
	report.Units[0].MembershipIDs[0] = "changed"
	if empty != "" || zero != 0 || source.Units[0].MembershipIDs[0] != "1" ||
		*report.Units[0].Rows[1].Species.Text != "" {
		t.Fatal("transport aliases source or adjacent fanout")
	}
	raw, err := json.Marshal(LongVegetationReport{Units: []LongVegetationUnit{}, Diagnostics: []LongVegetationDiagnostic{}})
	if err != nil || !strings.Contains(string(raw), `"units":[]`) || !strings.Contains(string(raw), `"diagnostics":[]`) {
		t.Fatal("empty collections changed to NULL", string(raw), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if value, err := longVegetationTransport(ctx, source); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(value, LongVegetationReport{}) {
		t.Fatal("cancelled serialization returned partial output", value, err)
	}
}
