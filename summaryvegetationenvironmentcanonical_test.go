package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestSummaryEnvironmentalCanonicalAllDefinitionsOwnedSnapshot(t *testing.T) {
	catalogue, err := csv.NewReader(strings.NewReader(summaryEnvironmentalCanonicalFieldListCSV)).ReadAll()
	if err != nil || len(catalogue) != 55 || !reflect.DeepEqual(catalogue[0], summaryVegetationFieldListColumns) {
		t.Fatal("exact54 source definitions required", len(catalogue), err)
	}
	definitions := catalogue[1:]
	sort.SliceStable(definitions, func(i, j int) bool {
		a, errA := strconv.Atoi(definitions[i][0])
		b, errB := strconv.Atoi(definitions[j][0])
		if errA != nil || errB != nil {
			t.Fatal("source order not INTEGER", errA, errB)
		}
		return a < b
	})
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%t", external), func(t *testing.T) {
			service, state := summaryEnvironmentalLifeformServiceFixture(t, external)
			owner := service.environment.report.contexts.projects.sqlite
			env := quoteHeaderIdentifier(state.ActiveProject + "_Env")
			mutateContextFixture(t, owner.attachments["project"], "DELETE FROM "+env)
			mutateContextFixture(t, owner.attachments["VPro64"], "DELETE FROM USysShortVegSummaryFieldList")
			quoted := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
			for _, definition := range definitions {
				report := "-1"
				if definition[5] == "SVHumusDepth" {
					report = "0"
				}
				source, argument := "NULL", "NULL"
				if definition[1] != "" {
					source = quoted(definition[1])
				}
				if definition[6] != "" {
					argument = quoted(definition[6])
				}
				mutateContextFixture(t, owner.attachments["VPro64"],
					"INSERT INTO USysShortVegSummaryFieldList VALUES("+
						definition[0]+","+source+","+quoted(definition[2])+","+quoted(definition[3])+","+
						report+","+quoted(definition[5])+","+argument+")")
			}
			before := databaseBytes(t, owner.attachments)
			config, err := os.ReadFile(service.environment.report.contexts.projects.preferences.path)
			if err != nil {
				t.Fatal(err)
			}
			snapshots := 0
			service.environment.snapshot.commitRead = func(*sql.Tx) error { snapshots++; return nil }
			got, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
			if err != nil || got == nil || len(got.Rows) != 53 || len(got.FieldList.Rows) != 54 || snapshots != 1 {
				t.Fatal("complete canonical supported selection unavailable", got, err, snapshots)
			}
			selected := make([][]string, 0, 53)
			for _, definition := range definitions {
				if definition[5] != "SVHumusDepth" {
					selected = append(selected, definition)
				}
			}
			for i, row := range got.Rows {
				definition := selected[i]
				if *row.Field.Cells[2].Text != definition[2] || *row.Field.Cells[5].Text != definition[5] ||
					len(row.Values) != 2 || len(row.Field.Cells) != 7 {
					t.Fatal("canonical physical order/label/function/witness differs", i, row)
				}
				function := definition[5]
				for unit, value := range row.Values {
					if function == "SVnPlots" {
						want := "4"
						if unit == 1 {
							want = "1"
						}
						if value.Kind != "cell" || value.Cell == nil || value.Cell.Storage != "integer" ||
							value.Cell.Integer == nil || *value.Cell.Integer != want || row.LabelColumn != 3 {
							t.Fatal("canonical independent physical count changed", row)
						}
					} else if function == "SVSeepage" || function == "SVRootingDepth" || function == "SVSlope" ||
						function == "SVSubstrate" || function == "SVStrataCover" {
						if !reflect.DeepEqual(value, SummaryVegetationEnvironmentalValue{Kind: "empty"}) {
							t.Fatal("no-Env canonical numeric Empty changed", row)
						}
					} else {
						want := ""
						if function == "SVLifeformCover" {
							if unit == 0 {
								want = "3.8 - 4.4 - 5.0"
							} else if definition[6] == "1" {
								want = "0.0 - 0.0 - 0.0"
							}
						}
						if value.Kind != "cell" || value.Cell == nil || value.Cell.Storage != "text" ||
							value.Cell.Text == nil || *value.Cell.Text != want {
							t.Fatal("canonical independent no-Env String/lifeform value changed", row, want)
						}
					}
				}
			}
			wire, err := json.Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			var roundtrip SummaryVegetationEnvironmentalLifeformPreview
			decoder := json.NewDecoder(bytes.NewReader(wire))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&roundtrip); err != nil || !reflect.DeepEqual(got, &roundtrip) {
				t.Fatal("complete canonical matrix strict wire changed", err)
			}
			workbook, err := prepareSummaryEnvironmentalWorkbook(t.Context(), summaryEnvironmentalWorkbookInput{
				FirstRow: 5, FieldList: got.FieldList, Units: summaryEnvironmentalPreviewUnits(got.Vegetation), Rows: got.Rows,
			})
			if err != nil || len(workbook.Bytes) == 0 || len(workbook.Header.Fields) != 53 ||
				workbook.Header.LastDataRow != 57 || workbook.Header.NextRow != 60 {
				t.Fatal("complete canonical typed workbook preparation differs", err)
			}
			book := openSummaryEnvironmentalWorkbook(t, workbook)
			for i, definition := range selected {
				want := ""
				if definition[5] == "SVnPlots" {
					want = "4"
				} else if definition[5] == "SVLifeformCover" {
					want = "3.8 - 4.4 - 5.0"
				}
				value, err := book.GetCellValue("Summary Environment", "D"+strconv.Itoa(i+5))
				if err != nil || value != want {
					t.Fatal("canonical first-unit workbook value differs from independent expectation", definition, value, err)
				}
			}
			if !reflect.DeepEqual(databaseBytes(t, owner.attachments), before) {
				t.Fatal("complete canonical read mutated physical source")
			}
			mutateContextFixture(t, owner.attachments["VPro64"],
				"UPDATE USysShortVegSummaryFieldList SET Report=-1 WHERE FunctionName='SVHumusDepth'")
			refusalBefore := databaseBytes(t, owner.attachments)
			refused, err := service.PreviewEnvironmentalLifeform(t.Context(), state.ContextID, 1, 6, 0)
			if err == nil || refused != nil || !strings.Contains(err.Error(), "SVHumusDepth") ||
				!strings.Contains(err.Error(), "unavailable") {
				t.Fatal("unresolved current source HumusDepth returned partial success", refused, err)
			}
			if !reflect.DeepEqual(databaseBytes(t, owner.attachments), refusalBefore) {
				t.Fatal("canonical refusal changed source")
			}
			mutateContextFixture(t, owner.attachments["VPro64"],
				"UPDATE USysShortVegSummaryFieldList SET Report=0 WHERE FunctionName='SVHumusDepth'")
			retry, err := service.PreviewEnvironmentalLifeform(context.Background(), state.ContextID, 1, 6, 0)
			if err != nil || !reflect.DeepEqual(got, retry) {
				t.Fatal("canonical correction/retry changed report", retry, err)
			}
			afterConfig, err := os.ReadFile(service.environment.report.contexts.projects.preferences.path)
			if err != nil || !bytes.Equal(config, afterConfig) {
				t.Fatal("canonical qualification wrote preferences", err)
			}
			if !external {
				if path := os.Getenv("VPRO_SUMMARY_ENVIRONMENT_CANONICAL_WIRE_PATH"); path != "" {
					roundtrip.Vegetation.ContextID = "summary-canonical-wire"
					roundtrip.Vegetation.ProjectPath = `C:\fixture\Sample.db`
					roundtrip.Vegetation.SUPath = `C:\fixture\Sample.db`
					roundtrip.SourcePath = `C:\fixture\VPro64.db`
					roundtrip.EnvSourcePath = `C:\fixture\Sample.db`
					roundtrip.MasterSourcePath = `C:\fixture\VLists.db`
					output, err := json.MarshalIndent(roundtrip, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, append(output, '\n'), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}
