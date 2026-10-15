package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestSummaryEnvironmentalDerivedOriginalPhysicalCases(t *testing.T) {
	for _, spec := range []struct {
		function, environment, fixture string
		count                          int
	}{
		{"SVBioGeoClimaticUnit", "VPRO_SUMMARY_BGC_SOURCE", summaryEnvironmentalBGCSourceFixture, 7},
		{"SVAspect", "VPRO_SUMMARY_ASPECT_SOURCE", summaryEnvironmentalASPECTSourceFixture, 44},
	} {
		t.Run(spec.function, func(t *testing.T) {
			path := os.Getenv(spec.environment)
			raw := []byte(spec.fixture)
			if path != "" {
				var err error
				raw, err = os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
			}
			var source struct {
				Physical map[string][][]json.RawMessage `json:"physical"`
			}
			if err := json.Unmarshal(raw, &source); err != nil {
				t.Fatal(err)
			}
			build := func(name string, columns []ProjectMetadataColumn) ProjectMetadataTable {
				table := ProjectMetadataTable{Columns: columns, Rows: []ProjectMetadataRow{}}
				for i, row := range source.Physical[name] {
					cells := []ProjectMetadataCell{}
					for j, raw := range row {
						cell := ProjectMetadataCell{Storage: "null"}
						if string(raw) != "null" {
							if spec.function == "SVAspect" && name == "Sample_Env" && j == 1 {
								var number int64
								if err := json.Unmarshal(raw, &number); err != nil {
									t.Fatal(err)
								}
								cell = metadataInteger(strconv.FormatInt(number, 10))
							} else {
								var text string
								if err := json.Unmarshal(raw, &text); err != nil {
									t.Fatal(err)
								}
								cell = ProjectMetadataCell{Storage: "text", Text: &text}
							}
						}
						cells = append(cells, cell)
					}
					table.Rows = append(table.Rows, ProjectMetadataRow{RowID: strconv.Itoa(i + 1), Cells: cells})
				}
				return table
			}
			columns := []ProjectMetadataColumn{{Name: "PlotNumber", DeclaredType: "TEXT(7)"}}
			if spec.function == "SVAspect" {
				columns = append(columns, ProjectMetadataColumn{Name: "Aspect", DeclaredType: "INTEGER"})
			} else {
				columns = append(columns, ProjectMetadataColumn{Name: "Zone", DeclaredType: "TEXT(4)"},
					ProjectMetadataColumn{Name: "SubZone", DeclaredType: "TEXT(8)"})
			}
			env := build("Sample_Env", columns)
			su := build("UnitInputs", []ProjectMetadataColumn{{Name: "PlotNumber"}, {Name: "SiteUnit"}})
			receipt := []byte(spec.fixture)
			if path != "" {
				var err error
				receipt, err = os.ReadFile(os.Getenv(spec.environment + "_RECEIPT"))
				if err != nil {
					t.Fatal(err)
				}
			}
			var observations struct {
				Observations []struct {
					Unit   *string  `json:"unit"`
					Actual []string `json:"actual"`
				} `json:"observations"`
			}
			if err := json.Unmarshal(receipt, &observations); err != nil || len(observations.Observations) != spec.count {
				t.Fatal("complete source observations required", err)
			}
			for _, observation := range observations.Observations {
				got, err := prepareSummaryVegetationEnvironmentalCategories(t.Context(), spec.function, observation.Unit, env, su)
				if observation.Unit == nil {
					if err == nil || !reflect.DeepEqual(observation.Actual[:2], []string{"error", "94"}) {
						t.Fatal(got, err, observation)
					}
				} else if err != nil || got.Text == nil || !reflect.DeepEqual(observation.Actual, []string{"ok", "String", *got.Text}) {
					t.Fatal(got, err, observation)
				}
			}
		})
	}
}

func TestSummaryEnvironmentalDerivedOwnedRefusalsAndRetry(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, false)
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FunctionName='SVAspect' WHERE FunctionName='SVSnr'`)
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET Aspect=CASE PlotNumber WHEN 'P1' THEN 0 WHEN 'P2' THEN 360 ELSE NULL END`)
	got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || got == nil || *got.Rows[1].Values[0].Text != "N(3)  " {
		t.Fatal("converted-label grouping differs", got, err)
	}
	for _, invalid := range []string{"32768", "CAST('0' AS BLOB)", "0.5"} {
		mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
			SET Aspect=`+invalid+` WHERE PlotNumber='P1'`)
		if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); got != nil || err == nil {
			t.Fatal("invalid original Aspect storage accepted", invalid, got, err)
		}
	}
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FunctionName='SVBioGeoClimaticUnit' WHERE FunctionName='SVAspect'`)
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET Zone=CASE PlotNumber WHEN 'P1' THEN 'A' ELSE NULL END, SubZone=CASE PlotNumber WHEN 'P1' THEN 'B' WHEN 'P2' THEN 'AB' ELSE NULL END`)
	got, err = service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || got == nil || *got.Rows[1].Values[0].Text != "AB(3)  " || *got.Rows[1].Values[1].Text != "(Null 1)" {
		t.Fatal("NULL-zone count vs loop admission differs", got, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if got, err := service.PreviewEnvironmental(ctx, state.ContextID, 1, 6, 0); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
}
