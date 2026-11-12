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
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLongVegetationSelectedScalarStrictGoClientWire(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("frontend", "src", "longVegetationSelectedScalarWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wire struct {
		Preview LongVegetationPreview                   `json:"preview"`
		Cells   map[string]vprocore.ProjectMetadataCell `json:"cells"`
	}
	if err := vprocore.DecodeStrictRequiredJSON(raw, &wire, "selected scalar client wire", "preview", "cells"); err != nil {
		t.Fatal(err)
	}
	if len(wire.Cells) != 4 || wire.Preview.Settings.SelectedField != ` Literal & "field" ` ||
		wire.Preview.Report.SelectedFieldType != "real" || wire.Preview.Settings.ShowEnglishName ||
		wire.Preview.Settings.ShowSpeciesCode {
		t.Fatal("selected scalar wire lost literal/type provenance", wire)
	}
	for _, kind := range []string{"text", "integer", "boolean", "real"} {
		if _, err := longVegetationSelectedScalarValue(kind, wire.Cells[kind]); err != nil {
			t.Fatal(kind, err)
		}
	}
	roundtrip, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var independent, returned any
	if err := json.Unmarshal(raw, &independent); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(roundtrip, &returned); err != nil || !reflect.DeepEqual(independent, returned) {
		t.Fatal("production Go selected scalar wire differs from client fixture", err)
	}
}

func TestLongVegetationSelectedScalarPlannerPreservesDuplicateWeightsNullsAndConstantFanout(t *testing.T) {
	integer := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
	}
	real := func(value float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	for _, test := range []struct {
		declaration, kind string
		values            []vprocore.ProjectMetadataCell
	}{
		{"TEXT", "text", []vprocore.ProjectMetadataCell{vprotest.MetadataText(""), vprotest.MetadataText(""), {Storage: "null"}}},
		{"INTEGER", "integer", []vprocore.ProjectMetadataCell{integer("1"), integer("2"), integer("2"), {Storage: "null"}}},
		{"BOOLEAN", "boolean", []vprocore.ProjectMetadataCell{integer("-1"), integer("0"), integer("0"), {Storage: "null"}}},
		{"REAL", "real", []vprocore.ProjectMetadataCell{real(1.005), real(0.15), real(0.15), {Storage: "null"}}},
	} {
		t.Run(test.kind, func(t *testing.T) {
			prepared, original, layers := vegetationLayerFixture(t)
			species := vprocore.ProjectMetadataTable{Columns: append(append([]vprocore.ProjectMetadataColumn{}, original.Columns...),
				vprocore.ProjectMetadataColumn{Name: "Literal", DeclaredType: test.declaration})}
			for i, value := range test.values {
				cells := append(append([]vprocore.ProjectMetadataCell{}, original.Rows[0].Cells...), vprocore.CloneSiteUnitCell(value))
				species.Rows = append(species.Rows, vprocore.ProjectMetadataRow{RowID: []string{"1", "2", "3", "4"}[i], Cells: cells})
			}
			projected, projection, err := projectLongVegetationSelectedField(context.Background(), species, "Literal")
			if err != nil {
				t.Fatal(err)
			}
			for _, constant := range []bool{false, true} {
				options := layerTestOptions()
				options.ConstantSpeciesList = constant
				options.PresenceGreaterThan, options.MeanCoverGreaterThan = -1, -999
				report, err := planLongVegetationLayers(context.Background(), prepared, projected, layers, options)
				if err != nil {
					t.Fatal(err)
				}
				if err := projection.restore(context.Background(), &report); err != nil {
					t.Fatal(err)
				}
				if err := projection.validateGrouping(context.Background(), &report); err != nil {
					t.Fatal("independent duplicate/NULL/fanout plan became unavailable", err)
				}
				found := 0
				combinations := map[string]bool{}
				for _, unit := range report.Units {
					if unit.Code.Text == nil || *unit.Code.Text != "U" {
						continue
					}
					if unit.NumPlots != 2 {
						t.Fatal("selected field changed physical denominator", unit.NumPlots)
					}
					for _, row := range unit.Rows {
						if row.Layer.Text == nil || *row.Layer.Text != "L1" || row.Species.Text == nil ||
							*row.Species.Text != "Scientific A" {
							continue
						}
						found++
						wantCover := -2.0
						if reflect.DeepEqual(row.MatchedName, test.values[1]) {
							wantCover = -4
						}
						if row.Presence == nil || *row.Presence != 0.5 || row.MeanCover == nil ||
							*row.MeanCover != wantCover || len(row.Plots) == 0 || row.Plots[0].Cover == nil ||
							*row.Plots[0].Cover != wantCover*2 {
							t.Fatal("duplicate selected scalar reference changed join weight/statistics", row, wantCover)
						}
						if !constant && !reflect.DeepEqual(row.EnglishName, row.MatchedName) {
							t.Fatal("nonconstant selected scalar invented a list/matched fanout", row)
						}
						pair, err := json.Marshal([]vprocore.ProjectMetadataCell{row.EnglishName, row.MatchedName})
						if err != nil {
							t.Fatal(err)
						}
						combinations[string(pair)] = true
					}
				}
				want := 3
				if test.kind == "text" {
					want = 2
				}
				if constant {
					want *= want
				}
				if found != want || len(combinations) != want {
					t.Fatal("NULL/empty/duplicate scalar groups or independent list/matched dimensions collapsed", constant, found, combinations)
				}
			}
		})
	}
}

func TestLongVegetationSelectedThresholdsCannotHideUnverifiedJoinedRepresentatives(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	value := 2.0
	for i := range prepared.Observations {
		prepared.Observations[i].Cover = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	species.Columns = append(species.Columns, vprocore.ProjectMetadataColumn{Name: "Literal", DeclaredType: "TEXT"})
	species.Rows[0].Cells[1] = vprotest.MetadataText("A")
	species.Rows[0].Cells = append(species.Rows[0].Cells, vprotest.MetadataText("same scalar"))
	peer := vprocore.ProjectMetadataRow{RowID: "2", Cells: append([]vprocore.ProjectMetadataCell{}, species.Rows[0].Cells...)}
	peer.Cells[1] = vprotest.MetadataText("a")
	species.Rows = append(species.Rows, peer)
	projected, projection, err := projectLongVegetationSelectedField(context.Background(), species, "Literal")
	if err != nil {
		t.Fatal(err)
	}
	options := layerTestOptions()
	options.ConstantSpeciesList, options.PresenceGreaterThan, options.MeanCoverGreaterThan = false, -1, 3
	report, err := planLongVegetationLayers(context.Background(), prepared, projected, layers, options)
	if err != nil {
		t.Fatal(err)
	}
	units, rows := 0, 0
	for _, unit := range report.Units {
		if unit.Code.Text == nil || *unit.Code.Text != "U" {
			continue
		}
		units++
		for _, row := range unit.Rows {
			if row.Species.Text != nil && (*row.Species.Text == "A" || *row.Species.Text == "a") {
				rows++
			}
		}
	}
	if units != 1 || rows != 0 {
		t.Fatal("threshold witness did not suppress the separate byte buckets", units, rows)
	}
	options.selectedFieldGrouping, err = projection.newGroupingGuard()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := planLongVegetationLayers(context.Background(), prepared, projected, layers, options); err == nil {
		t.Fatal("thresholds hid joined unverified identities from the operation-local guard")
	}
}

func TestLongVegetationSelectedGroupingGuardsActualScopeNotUnjoinedMasterIdentities(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	species.Columns = append(species.Columns, vprocore.ProjectMetadataColumn{Name: "Literal", DeclaredType: "TEXT"})
	species.Rows[0].Cells = append(species.Rows[0].Cells, vprotest.MetadataText("observed"))
	species.Rows = append(species.Rows, vprocore.ProjectMetadataRow{RowID: "2", Cells: []vprocore.ProjectMetadataCell{
		vprotest.MetadataText("UNJOINED"), vprotest.MetadataText("\u00e9"), vprotest.MetadataText("unjoined"), vprotest.MetadataText("U"), vprotest.MetadataText("unjoined"),
	}})
	for _, joined := range []bool{false, true} {
		if joined {
			species.Rows[1].Cells[0] = vprotest.MetadataText("A")
		}
		projected, projection, err := projectLongVegetationSelectedField(context.Background(), species, "Literal")
		if err != nil {
			t.Fatal("staging normalized or rejected an unjoined source identity", err)
		}
		options := layerTestOptions()
		options.selectedFieldGrouping, err = projection.newGroupingGuard()
		if err != nil {
			t.Fatal(err)
		}
		report, err := planLongVegetationLayers(context.Background(), prepared, projected, layers, options)
		if (err != nil) != joined {
			t.Fatal("prethreshold guard let unjoined master veto scope or joined unknown identity succeed", joined, err)
		}
		if joined {
			continue
		}
		if err := projection.restore(context.Background(), &report); err != nil {
			t.Fatal(err)
		}
		err = projection.validateGrouping(context.Background(), &report)
		if err != nil {
			t.Fatal("restored verified scope became unavailable", err)
		}
		if *species.Rows[1].Cells[1].Text != "\u00e9" {
			t.Fatal("availability guard normalized a physical source identity")
		}
	}
	projection := &longVegetationSelectedFieldProjection{Kind: "text"}
	for _, input := range []struct {
		ctx    context.Context
		report *vegetationLayerReport
	}{{nil, &vegetationLayerReport{}}, {context.Background(), nil}} {
		if err := projection.validateGrouping(input.ctx, input.report); err == nil {
			t.Fatal("missing grouping authority became success")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := projection.validateGrouping(ctx, &vegetationLayerReport{}); err == nil {
		t.Fatal("empty scope bypassed cancellation")
	}
}

func TestLongVegetationSelectedSpeciesRefusalKeyDoesNotRewriteOrInventNativeGroups(t *testing.T) {
	for _, test := range []struct {
		value, key string
	}{{"A", "text:a"}, {"A - B", "text:ab"}, {"", "text:"}, {"01", "text:01"}} {
		cell := vprotest.MetadataText(test.value)
		key, err := longVegetationSelectedSpeciesKey(cell)
		if err != nil || key != test.key || *cell.Text != test.value {
			t.Fatal("refusal key rewrote an identity or invented grouping", test, key, err)
		}
	}
	if key, err := longVegetationSelectedSpeciesKey(vprocore.ProjectMetadataCell{Storage: "null"}); err != nil || key != "null" {
		t.Fatal("NULL identity became empty text", key, err)
	}
	for _, value := range []string{"\u00e9", "a\u0301", "bad\x00identity", string([]byte{0xff})} {
		if _, err := longVegetationSelectedSpeciesKey(vprotest.MetadataText(value)); err == nil {
			t.Fatal("uncalibrated identity acquired a successful comparison key", value)
		}
	}
	for _, test := range []struct {
		declaration string
		cell        vprocore.ProjectMetadataCell
	}{
		{"TEXT", vprotest.MetadataText("A")},
		{"INTEGER", vprocore.ProjectMetadataCell{Storage: "integer", Integer: new(string)}},
		{"BOOLEAN", vprocore.ProjectMetadataCell{Storage: "integer", Integer: new(string)}},
		{"REAL", vprocore.ProjectMetadataCell{Storage: "real", Real: new(float64)}},
	} {
		if test.cell.Integer != nil {
			*test.cell.Integer = "0"
		}
		for _, pair := range [][2]string{{"A", "a"}, {"A B", "AB"}, {"A-B", "AB"}, {"e", "\u00e9"}} {
			source := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
				{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Literal", DeclaredType: test.declaration},
			}, Rows: []vprocore.ProjectMetadataRow{
				{RowID: "1", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(pair[0]), {Storage: "null"}, vprotest.MetadataText("original"), vprocore.CloneSiteUnitCell(test.cell)}},
				{RowID: "2", Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText(pair[0]), vprotest.MetadataText(pair[1]), vprotest.MetadataText("original"), vprocore.CloneSiteUnitCell(test.cell)}},
			}}
			projected, projection, err := projectLongVegetationSelectedField(context.Background(), source, "Literal")
			if err != nil || projection == nil || len(projected.Rows) != 2 {
				t.Fatal("typed staging rejected unobserved identity data", test.declaration, pair, projected, err)
			}
			report := vegetationLayerReport{Units: []vegetationLayerUnit{{Rows: []vegetationLayerRow{
				{Species: vprotest.MetadataText(pair[0]), EnglishName: projected.Rows[0].Cells[2], MatchedName: projected.Rows[0].Cells[2]},
				{Species: vprotest.MetadataText(pair[1]), EnglishName: projected.Rows[1].Cells[2], MatchedName: projected.Rows[1].Cells[2]},
			}}}}
			if err := projection.restore(context.Background(), &report); err != nil {
				t.Fatal(err)
			}
			if err := projection.validateGrouping(context.Background(), &report); err == nil {
				t.Fatal("equivalent/unverified effective identities split into successful groups", test.declaration, pair, report)
			}
			for i := range report.Units[0].Rows {
				report.Units[0].Rows[i].EnglishName, report.Units[0].Rows[i].MatchedName = vprocore.ProjectMetadataCell{Storage: "null"}, vprocore.ProjectMetadataCell{Storage: "null"}
			}
			if err := projection.validateGrouping(context.Background(), &report); err == nil {
				t.Fatal("NULL selected scalars bypassed effective identity refusal", test.declaration, pair, report)
			}
		}
	}
}

func TestLongVegetationSelectedScalarTextRefusalIncludesSourceSpeciesFallbackAndMixedIdentity(t *testing.T) {
	name := func(value string) *string { return &value }
	for _, pair := range [][2]string{{"A", "a"}, {"A", "B"}, {"word", "word "}, {"a", "a\u0301"}} {
		for _, scientific := range [][2]*string{
			{nil, nil},
			{nil, name("X")},
			{name("X"), nil},
			{name("X"), name("X")},
		} {
			source := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
				{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Literal", DeclaredType: "TEXT"},
			}}
			for i := range pair {
				name := vprocore.ProjectMetadataCell{Storage: "null"}
				if scientific[i] != nil {
					name = vprotest.MetadataText(*scientific[i])
				}
				source.Rows = append(source.Rows, vprocore.ProjectMetadataRow{RowID: []string{"1", "2"}[i],
					Cells: []vprocore.ProjectMetadataCell{vprotest.MetadataText("X"), name, vprotest.MetadataText("original"), vprotest.MetadataText(pair[i])}})
			}
			projected, projection, err := projectLongVegetationSelectedField(context.Background(), source, "Literal")
			if err != nil || projection == nil || len(projected.Rows) != 2 {
				t.Fatal("typed staging could not preserve source representatives", pair, scientific, projected, err)
			}
			report := vegetationLayerReport{Units: []vegetationLayerUnit{{Rows: []vegetationLayerRow{
				{Species: vprotest.MetadataText("X"), EnglishName: projected.Rows[0].Cells[2], MatchedName: projected.Rows[0].Cells[2]},
				{Species: vprotest.MetadataText("X"), EnglishName: projected.Rows[1].Cells[2], MatchedName: projected.Rows[1].Cells[2]},
			}}}}
			if err := projection.restore(context.Background(), &report); err != nil {
				t.Fatal(err)
			}
			if err := projection.validateGrouping(context.Background(), &report); err == nil {
				t.Fatal("fallback/mixed/uncalibrated TEXT representatives produced successful groups", pair, scientific, report)
			}
			source.Rows[1].Cells[1] = vprotest.MetadataText("Distinct species")
			projected, projection, err = projectLongVegetationSelectedField(context.Background(), source, "Literal")
			if err != nil || projection == nil || len(projected.Rows) != 2 {
				t.Fatal("single literal per independent effective species became unavailable", projected, err)
			}
			report.Units[0].Rows[1].Species = vprotest.MetadataText("Distinct species")
			if err := projection.validateGrouping(context.Background(), &report); err != nil {
				t.Fatal("separate effective species could not retain their individual literals", err)
			}
		}
	}
}

func TestLongVegetationSelectedFieldOwnedIndependentGateAndAllGroupings(t *testing.T) {
	for _, external := range []bool{false, true} {
		for _, test := range []struct {
			declaration, kind string
			value             any
		}{{"TEXT", "text", "literal native"}, {"INTEGER", "integer", int64(2147483647)},
			{"BOOLEAN", "boolean", int64(-1)}, {"REAL", "real", 1.005}} {
			t.Run(test.kind+map[bool]string{false: "-internal", true: "-external"}[external], func(t *testing.T) {
				service, state := vprotest.ReportServiceFixture(t, external)
				lists, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(service.Projects.Sqlite.Attachments["VLists"], "rw"))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := lists.Exec(`ALTER TABLE USysAllSpecs ADD COLUMN Literal ` + test.declaration); err != nil {
					lists.Close()
					t.Fatal(err)
				}
				if _, err := lists.Exec(`UPDATE USysAllSpecs SET Literal=?`, test.value); err != nil {
					lists.Close()
					t.Fatal(err)
				}
				if err := lists.Close(); err != nil {
					t.Fatal(err)
				}
				if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LVShowEnglishName": 4, "LVShowField": "literal"}); err != nil {
					t.Fatal(err)
				}
				before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
				service.LongVegetationCodeEnabled = true
				service.LongVegetationNoneEnabled, service.LongVegetationStrataEnabled, service.LongVegetationLifeformEnabled = true, true, true
				if _, err := GetLongVegetationOptions(service, context.Background(), state.ContextID); err == nil {
					t.Fatal("other flags enabled selected scalar")
				}
				if _, err := PreviewLongVegetation(service, context.Background(), state.ContextID); err == nil {
					t.Fatal("default-off selected scalar report succeeded")
				}
				service.LongVegetationSelectedFieldEnabled = true
				for _, grouping := range []int{1, 2, 3, 4} {
					if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LVGroupBy": grouping}); err != nil {
						t.Fatal(err)
					}
					options, err := GetLongVegetationOptions(service, context.Background(), state.ContextID)
					if err != nil || options.Settings.SelectedField != "literal" ||
						options.Settings.ShowEnglishName || options.Settings.ShowSpeciesCode {
						t.Fatal("mode4 option shape changed", grouping, options, err)
					}
					preview, err := PreviewLongVegetation(service, context.Background(), state.ContextID)
					if err != nil || preview.Settings.SelectedField != "literal" || preview.Report.SelectedFieldType != test.kind {
						t.Fatal("typed selected-field preview failed grouping", grouping, preview, err)
					}
					observed := 0
					for _, unit := range preview.Report.Units {
						for _, row := range unit.Rows {
							for _, cell := range []vprocore.ProjectMetadataCell{row.EnglishName, row.MatchedName} {
								if cell.Storage == "null" {
									continue
								}
								value, err := vprocore.MetadataCellValue(cell)
								if err != nil || !reflect.DeepEqual(value, test.value) {
									t.Fatal("selected scalar became a string or changed value", grouping, cell, value, err)
								}
								observed++
							}
						}
					}
					if observed == 0 {
						t.Fatal("selected scalar assertion resolved no rows")
					}
				}
				if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)) {
					t.Fatal("read-only mode4 changed owned physical data")
				}
				if err := service.Projects.Preferences.Update("ReportOptions", map[string]any{"LVShowField": "WeedStatus"}); err != nil {
					t.Fatal(err)
				}
				if _, err := GetLongVegetationOptions(service, context.Background(), state.ContextID); err == nil {
					t.Fatal("missing source suggestion silently became a physical field")
				}
			})
		}
	}
}

func TestLongVegetationSelectedScalarProjectionTypedGroupingAndRestoration(t *testing.T) {
	text := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
	}
	integer := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
	}
	real := func(value float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	for _, test := range []struct {
		kind   string
		values []vprocore.ProjectMetadataCell
	}{
		{"TEXT", []vprocore.ProjectMetadataCell{text(""), text(""), text(""), {Storage: "null"}}},
		{"INTEGER", []vprocore.ProjectMetadataCell{integer("1"), integer("2"), integer("1"), {Storage: "null"}}},
		{"BOOLEAN", []vprocore.ProjectMetadataCell{integer("-1"), integer("0"), integer("-1"), {Storage: "null"}}},
		{"REAL", []vprocore.ProjectMetadataCell{real(1.005), real(0.15), real(1.005), {Storage: "null"}}},
	} {
		t.Run(test.kind, func(t *testing.T) {
			source := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{
				{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Literal", DeclaredType: test.kind},
			}, Rows: []vprocore.ProjectMetadataRow{}}
			for i, value := range test.values {
				source.Rows = append(source.Rows, vprocore.ProjectMetadataRow{
					RowID: []string{"1", "2", "3", "4"}[i], Cells: []vprocore.ProjectMetadataCell{text("X"), text("Sample"), text("original"), value},
				})
			}
			before, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			projected, projection, err := projectLongVegetationSelectedField(context.Background(), source, "literal")
			if err != nil {
				t.Fatal(err)
			}
			after, err := json.Marshal(source)
			if err != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(projected.Columns, source.Columns) ||
				projected.Rows[0].Cells[2].Storage != "text" || projected.Rows[3].Cells[2].Storage != "null" {
				t.Fatal("projection mutated physical source or conflated NULL/text", source, projected)
			}
			if test.kind != "TEXT" && !reflect.DeepEqual(projected.Rows[0].Cells[2], projected.Rows[2].Cells[2]) {
				t.Fatal("equal typed values did not group together", projected)
			}
			if test.kind == "TEXT" && !reflect.DeepEqual(projected.Rows[0].Cells[2], projected.Rows[1].Cells[2]) {
				t.Fatal("equal literal TEXT values lost grouping identity", projected)
			}
			if test.kind != "TEXT" && reflect.DeepEqual(projected.Rows[0].Cells[2], projected.Rows[1].Cells[2]) {
				t.Fatal("distinct physical scalar values lost grouping identity", projected)
			}
			rows := []vegetationLayerRow{}
			for _, row := range projected.Rows {
				rows = append(rows, vegetationLayerRow{EnglishName: vprocore.CloneSiteUnitCell(row.Cells[2]), MatchedName: vprocore.CloneSiteUnitCell(row.Cells[2])})
			}
			report := vegetationLayerReport{Units: []vegetationLayerUnit{{Rows: rows}}}
			if err := projection.restore(context.Background(), &report); err != nil {
				t.Fatal(err)
			}
			for i, row := range report.Units[0].Rows {
				if !reflect.DeepEqual(row.EnglishName, test.values[i]) || !reflect.DeepEqual(row.MatchedName, test.values[i]) {
					t.Fatal("selected scalar did not survive typed restoration", row, test.values[i])
				}
			}
			*source.Rows[0].Cells[2].Text = "changed original"
			if *projected.Rows[0].Cells[2].Text == "changed original" {
				t.Fatal("projection inherited a mutable source pointer")
			}
		})
	}
}

func TestLongVegetationSelectedScalarProjectionRefusesUnknownTypesConversionsAndRepresentatives(t *testing.T) {
	text := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "text", Text: &value}
	}
	integer := func(value string) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &value}
	}
	real := func(value float64) vprocore.ProjectMetadataCell {
		return vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
	}
	for _, test := range []struct {
		kind   string
		values []vprocore.ProjectMetadataCell
	}{
		{"BLOB", []vprocore.ProjectMetadataCell{{Storage: "null"}}},
		{"", []vprocore.ProjectMetadataCell{text("unknown type")}},
		{"INTEGER", []vprocore.ProjectMetadataCell{text("1")}},
		{"INTEGER", []vprocore.ProjectMetadataCell{integer("2147483648")}},
		{"INTEGER", []vprocore.ProjectMetadataCell{integer("01")}},
		{"BOOLEAN", []vprocore.ProjectMetadataCell{integer("1")}},
		{"BOOLEAN", []vprocore.ProjectMetadataCell{real(-1)}},
		{"REAL", []vprocore.ProjectMetadataCell{integer("1")}},
		{"REAL", []vprocore.ProjectMetadataCell{real(math.NaN())}},
		{"REAL", []vprocore.ProjectMetadataCell{real(0), real(math.Copysign(0, -1))}},
		{"TEXT", []vprocore.ProjectMetadataCell{text("new\u0000value")}},
		{"TEXT", []vprocore.ProjectMetadataCell{text(string([]byte{0xff}))}},
		{"TEXT", []vprocore.ProjectMetadataCell{{Storage: "null", Text: new(string)}}},
	} {
		source := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "Code", DeclaredType: "TEXT"}, {Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "Literal", DeclaredType: test.kind}}, Rows: []vprocore.ProjectMetadataRow{}}
		for i, value := range test.values {
			source.Rows = append(source.Rows, vprocore.ProjectMetadataRow{RowID: []string{"1", "2"}[i], Cells: []vprocore.ProjectMetadataCell{text("X"), text("Sample"), text("original"), value}})
		}
		projected, projection, err := projectLongVegetationSelectedField(context.Background(), source, "Literal")
		if err == nil || projection != nil || len(projected.Rows) != 0 {
			t.Fatal("unsupported selected scalar returned successful projection", test, projected, projection, err)
		}
	}
	source := vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{{Name: "ScientificName", DeclaredType: "TEXT"}, {Name: "EnglishName", DeclaredType: "TEXT"}, {Name: "S", DeclaredType: "TEXT"}}}
	for _, field := range []string{"", "unknown", "\u017f", "Literal\u0000", string([]byte{0xff})} {
		if _, projection, err := projectLongVegetationSelectedField(context.Background(), source, field); err == nil || projection != nil {
			t.Fatal("unowned/empty/non-SQLite-equivalent identifier resolved", field, projection, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, projection, err := projectLongVegetationSelectedField(ctx, source, "S"); err == nil || projection != nil {
		t.Fatal("cancelled projection succeeded", projection, err)
	}
	projection := &longVegetationSelectedFieldProjection{Kind: "text", values: map[string]vprocore.ProjectMetadataCell{}}
	report := vegetationLayerReport{Units: []vegetationLayerUnit{{Rows: []vegetationLayerRow{{EnglishName: text("unowned"), MatchedName: vprocore.ProjectMetadataCell{Storage: "null"}}}}}}
	if err := projection.restore(context.Background(), &report); err == nil {
		t.Fatal("unowned scalar token was silently passed through")
	}
}
