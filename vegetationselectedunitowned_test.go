package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestSelectedUnitOwnedSourceTypedTransportAndSummary(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "attached", true: "external"}[external], func(t *testing.T) {
			service, state := vegetationWorkbookServiceFixture(t, external)
			contexts := service.contexts
			before := databaseBytes(t, contexts.projects.sqlite.attachments)
			config, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			options, err := loadLongVegetationOptions(contexts.plots)
			if err != nil {
				t.Fatal(err)
			}
			options.SelectedUnitFields = []string{"PlotNumber"}
			source, err := withContextPlotRequest(context.Background(), contexts, state.ContextID,
				func(plots *PlotService) (longVegetationReportSource, error) {
					return withOwnedPreferenceSnapshot(context.Background(), plots, publicationReadSnapshotHooks{},
						func(owner *sqliteContext, tx *sql.Tx) (longVegetationReportSource, error) {
							return readLongVegetationReportSource(context.Background(), owner, tx, options)
						})
				})
			if err != nil || len(source.Tables) != 7 || len(source.Report.SelectedUnits) != 3 {
				t.Fatal("source2 must read physical Env/Admin without quality and retain orphan", len(source.Tables), source.Report.SelectedUnits, err)
			}
			transport, err := longVegetationTransport(context.Background(), source.Report)
			if err != nil || len(transport.SelectedUnits) != 3 || transport.NoneUnits != nil {
				t.Fatal("source2 transport lost separate derived witness", transport, err)
			}
			for i, plot := range []string{"108050", "missing", "108050x"} {
				row := transport.SelectedUnits[i]
				if row.MembershipID != []string{"1", "2", "3"}[i] || *row.PlotNumber.Text != plot {
					t.Fatal("original selected-SU identity/order", i, row)
				}
				if plot == "missing" {
					if row.EnvRowID != nil || row.AdminRowID != nil || row.SiteUnit.Storage != "null" {
						t.Fatal("orphan invented parents/label", row)
					}
				} else if row.EnvRowID == nil || row.AdminRowID == nil || *row.SiteUnit.Text != plot {
					t.Fatal("physical field projection lost original parent witnesses", row)
				}
			}
			settings := vegetationSettings(options)
			if settings.UnitGrouping != "selected-fields" || !reflect.DeepEqual(settings.UnitFields, []string{"PlotNumber"}) {
				t.Fatal("explicit ordered selection not represented", settings)
			}
			raw, err := json.Marshal(LongVegetationPreview{ContextID: state.ContextID, ProjectPath: state.ProjectPath,
				SUPath: state.SUPath, Settings: settings, Report: transport})
			var reopened LongVegetationPreview
			if err != nil || json.Unmarshal(raw, &reopened) != nil || !reflect.DeepEqual(reopened.Report, transport) ||
				!bytes.Contains(raw, []byte(`"envRowId":null`)) || !bytes.Contains(raw, []byte(`"adminRowId":null`)) {
				t.Fatal("source2 typed JSON roundtrip repaired NULL/derived evidence", string(raw), err)
			}
			summary, err := prepareVegetationWorkbookSummary(context.Background(), "2026-10-10",
				source.Tables[5], source.Tables[1], ProjectMetadataTable{})
			if err != nil {
				t.Fatal(err)
			}
			derived, err := vegetationWorkbookSelectedUnitSummary(context.Background(), summary,
				source.Tables[5], source.Tables[6], source.Tables[1], options.SelectedUnitFields, transport)
			if err != nil || derived.SelectedPlotRows != 3 || len(derived.Memberships) != 3 ||
				derived.Memberships[1].SelectedUnit == nil || derived.Memberships[1].SiteUnit.Storage != "null" {
				t.Fatal("derived source2 summary lost nonnull orphan count/original witness", derived, err)
			}
			*derived.Memberships[0].SelectedUnit.PlotNumber.Text = "mutated"
			if *transport.SelectedUnits[0].PlotNumber.Text != "108050" {
				t.Fatal("derived summary aliases transport")
			}
			assertProfileSUFiles(t, contexts, before)
			after, err := os.ReadFile(contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, after) {
				t.Fatal("source2 read projection changed YAML", err)
			}
			values, err := contexts.projects.preferences.snapshot()
			if err != nil {
				t.Fatal(err)
			}
			values["ReportOptions"].(map[string]any)["LVUnitGroups"] = 2
			if _, err := decodeLongVegetationOptions(values); err == nil {
				t.Fatal("internal draft routing enabled unreviewed source2 selection/config")
			}
		})
	}
}
