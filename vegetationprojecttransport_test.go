package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
)

func projectTransportFixture(t *testing.T, none bool) vegetationLayerReport {
	t.Helper()
	project, master, layers := projectReportFixture(t, none)
	prepared, err := projectReportPreparation(context.Background(), project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestProjectTransportOriginalSourceKindTuplesIndexesAndExactWire(t *testing.T) {
	for _, none := range []bool{false, true} {
		source := projectTransportFixture(t, none)
		kind := vegetationProjectFieldsScope
		if none {
			kind = vegetationProjectAllScope
		}
		report, err := longVegetationProjectTransport(context.Background(), source, kind)
		wantCount := 7
		if none {
			wantCount = 6
		}
		if err != nil || report.SourceKind != kind || len(report.ProjectUnits) != wantCount ||
			report.SelectedUnits != nil || report.NoneUnits != nil || report.SU != "None" {
			t.Fatal("project transport lost separate original provenance", report, err)
		}
		if !none && (report.ProjectUnits[0].EnvRowID != "101" || report.ProjectUnits[0].AdminRowID == nil ||
			*report.ProjectUnits[0].AdminRowID != "201" || !reflect.DeepEqual(report.ExcludedProjectUnitIndexes, []int{5})) {
			t.Fatal("project source2 original physical tuple differs", report.ProjectUnits[0])
		}
		for i, row := range report.ProjectUnits {
			if !reflect.DeepEqual(row.PlotNumber, source.ProjectUnits[i].PlotNumber) ||
				!reflect.DeepEqual(row.SiteUnit, source.ProjectUnits[i].SiteUnit) ||
				(row.AdminRowID == nil) != none {
				t.Fatal("project wire invented or repaired original tuple cells", row)
			}
		}
		for i, unit := range report.Units {
			if unit.MembershipIDs == nil || len(unit.MembershipIDs) != 0 ||
				!reflect.DeepEqual(unit.ProjectUnitIndexes, source.Units[i].ProjectUnitIndexes) ||
				len(unit.ProjectUnitIndexes) != unit.NumPlots {
				t.Fatal("project wire fabricated selected-list memberships", unit)
			}
		}
		raw, err := json.Marshal(report)
		if err != nil || !bytes.Contains(raw, []byte(`"sourceKind":"`+kind+`"`)) ||
			!bytes.Contains(raw, []byte(`"projectUnitIndexes":`)) ||
			!bytes.Contains(raw, []byte(`"membershipIds":[]`)) ||
			!bytes.Contains(raw, []byte(`"projectUnits":`)) ||
			bytes.Contains(raw, []byte(`"selectedUnits":`)) || bytes.Contains(raw, []byte(`"noneUnits":`)) {
			t.Fatal("distinct project wire shape differs", string(raw), err)
		}
		var decoded LongVegetationReport
		if err := decodeProfileLifecycleJSON(raw, &decoded,
			"project", "su", "title", "units", "diagnostics", "sourceKind", "projectUnits"); err != nil ||
			!reflect.DeepEqual(decoded, report) {
			t.Fatal("strict-Go project fixture roundtrip lost nullable/int64/index shape", err)
		}
		if none && (report.ProjectUnits[5].PlotNumber.Storage != "null" || len(report.ExcludedProjectUnitIndexes) != 0) {
			t.Fatal("source3 NULL Env tuple lost", report.ProjectUnits)
		}
		*report.ProjectUnits[0].PlotNumber.Text = "mutated"
		*report.ProjectUnits[0].SiteUnit.Text = "mutated"
		if report.ProjectUnits[0].AdminRowID != nil {
			*report.ProjectUnits[0].AdminRowID = "mutated"
		}
		report.Units[0].ProjectUnitIndexes[0] = 999
		report.Units[0].Code = metadataText("mutated")
		if *source.ProjectUnits[0].PlotNumber.Text != "P1" || source.Units[0].ProjectUnitIndexes[0] == 999 {
			t.Fatal("project transport aliases original prepared provenance")
		}
		if dto, err := longVegetationTransport(context.Background(), source); err == nil ||
			!reflect.DeepEqual(dto, LongVegetationReport{}) {
			t.Fatal("distinct project transport enabled legacy selected consumer", dto, err)
		}
	}
}

func TestProjectTransportRejectsForgedMixedMissingAndRepeatedOwnership(t *testing.T) {
	for _, mutate := range []func(*vegetationLayerReport){
		func(r *vegetationLayerReport) { r.Project = "" },
		func(r *vegetationLayerReport) { r.SU = "" },
		func(r *vegetationLayerReport) { r.ProjectUnits = nil },
		func(r *vegetationLayerReport) { r.ProjectUnits = []vegetationProjectUnitInput{} },
		func(r *vegetationLayerReport) { r.SelectedUnits = []vegetationSelectedUnitOccurrence{} },
		func(r *vegetationLayerReport) { r.NoneUnits = []vegetationNoneUnitOccurrence{} },
		func(r *vegetationLayerReport) { r.ProjectUnits[0].EnvRowID = "01" },
		func(r *vegetationLayerReport) { r.ProjectUnits[0].AdminRowID = nil },
		func(r *vegetationLayerReport) { r.ProjectUnits[1] = r.ProjectUnits[0] },
		func(r *vegetationLayerReport) { r.Units[0].MembershipIDs = []string{"1"} },
		func(r *vegetationLayerReport) { r.Units[0].NumPlots++ },
		func(r *vegetationLayerReport) { r.Units[0].ProjectUnitIndexes = nil },
		func(r *vegetationLayerReport) { r.Units[0].ProjectUnitIndexes[0] = -1 },
		func(r *vegetationLayerReport) { r.Units[0].ProjectUnitIndexes[0] = len(r.ProjectUnits) },
		func(r *vegetationLayerReport) {
			for i := range r.Units {
				if len(r.Units[i].ProjectUnitIndexes) > 1 {
					r.Units[i].ProjectUnitIndexes[1] = r.Units[i].ProjectUnitIndexes[0]
					return
				}
			}
			t.Fatal("duplicate ownership case requires a fanout unit")
		},
		func(r *vegetationLayerReport) { r.Units[0].Code = metadataText("wrong") },
		func(r *vegetationLayerReport) { r.Units = r.Units[1:] },
		func(r *vegetationLayerReport) { r.Units = append(r.Units, r.Units[0]) },
		func(r *vegetationLayerReport) { r.Diagnostics = nil },
		func(r *vegetationLayerReport) {
			for i := range r.Diagnostics {
				if r.Diagnostics[i].Code == "excluded_unit_without_vegetation" {
					r.Diagnostics[i].Count++
				}
			}
		},
		func(r *vegetationLayerReport) {
			r.Diagnostics = append(r.Diagnostics, vegetationLayerDiagnostic{"excluded_unit_without_vegetation", "text:U", 2})
		},
		func(r *vegetationLayerReport) { r.Quality = &vegetationQualitySelection{} },
	} {
		source := projectTransportFixture(t, false)
		mutate(&source)
		report, err := longVegetationProjectTransport(context.Background(), source, vegetationProjectFieldsScope)
		if err == nil || !reflect.DeepEqual(report, LongVegetationReport{}) {
			t.Fatal("forged project ownership accepted", report, err)
		}
	}
	for _, none := range []bool{false, true} {
		source := projectTransportFixture(t, none)
		wrong := vegetationProjectAllScope
		if none {
			wrong = vegetationProjectFieldsScope
		}
		for _, kind := range []string{"", vegetationSelectedListScope, "project", wrong} {
			if report, err := longVegetationProjectTransport(context.Background(), source, kind); err == nil ||
				!reflect.DeepEqual(report, LongVegetationReport{}) {
				t.Fatal("caller source kind overrides original provenance", kind, report, err)
			}
		}
	}
	source := projectTransportFixture(t, true)
	source.SU = "Report"
	if _, err := longVegetationProjectTransport(context.Background(), source, vegetationProjectAllScope); err == nil {
		t.Fatal("source3 invented original SU context")
	}
}

func TestProjectTransportOwnedAdaptersRetainOriginalProvenance(t *testing.T) {
	for _, none := range []bool{false, true} {
		contexts, state, base := projectOwnedSourceFixture(t, true, none)
		fields, kind := []string{"PlotNumber"}, vegetationProjectFieldsScope
		if none {
			fields, kind = nil, vegetationProjectAllScope
		}
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		for _, mode := range []string{"layer", "none", "strata", "lifeform"} {
			options := base
			options.NoneGrouping, options.StrataGrouping, options.LifeformGrouping = mode == "none", mode == "strata", mode == "lifeform"
			source, err := projectOwnedSourceRead(context.Background(), contexts, state, fields, options)
			if err != nil {
				t.Fatal(err)
			}
			report, err := longVegetationProjectTransport(context.Background(), source.Report, kind)
			if err != nil || report.SourceKind != kind || len(report.ProjectUnits) != 5 || len(report.Units) == 0 {
				t.Fatal("owned adapter output lost distinct original project transport", none, mode, report, err)
			}
			for _, unit := range report.Units {
				if len(unit.ProjectUnitIndexes) != unit.NumPlots || len(unit.MembershipIDs) != 0 {
					t.Fatal("adapter transport invented physical SU membership IDs", unit)
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
	}
}

func TestProjectTransportCancellationAndLegacyWireRemainDistinct(t *testing.T) {
	source := projectTransportFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, ctx} {
		report, err := longVegetationProjectTransport(ctx, source, vegetationProjectFieldsScope)
		if err == nil || ctx != nil && !errors.Is(err, context.Canceled) || !reflect.DeepEqual(report, LongVegetationReport{}) {
			t.Fatal("invalid/cancelled transport returned partial report", report, err)
		}
	}
	if _, err := longVegetationProjectTransport(context.Background(), source, vegetationProjectFieldsScope); err != nil {
		t.Fatal("cancelled pure transport broke retry", err)
	}
	prepared, master, layers := vegetationLayerFixture(t)
	ordinary, err := planLongVegetationLayers(context.Background(), prepared, master, layers, layerTestOptions())
	if err != nil {
		t.Fatal(err)
	}
	report, err := longVegetationTransport(context.Background(), ordinary)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(report)
	if err != nil || bytes.Contains(raw, []byte(`"sourceKind":`)) || bytes.Contains(raw, []byte(`"projectUnits":`)) ||
		bytes.Contains(raw, []byte(`"projectUnitIndexes":`)) {
		t.Fatal("ordinary wire shape gained project source/ownership fields", string(raw), err)
	}
	ordinary.Units[0].ProjectUnitIndexes = []int{}
	if _, err := longVegetationTransport(context.Background(), ordinary); err == nil {
		t.Fatal("legacy transport silently discarded forged project unit ownership")
	}
}
