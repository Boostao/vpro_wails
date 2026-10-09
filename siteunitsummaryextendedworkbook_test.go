package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSummaryExtendedWorkbookBothMethodsGroupsAndExactCells(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := reportServiceFixture(t, external)
		before := databaseBytes(t, service.projects.sqlite.attachments)
		config, err := os.ReadFile(service.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []int{1, 2} {
			input, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, method, publicationReadSnapshotHooks{})
			if err != nil {
				t.Fatal(err)
			}
			original, err := json.Marshal(input)
			if err != nil {
				t.Fatal(err)
			}
			lifeform, err := prepareSiteUnitSummaryLifeformWorkbook(context.Background(), input)
			if err != nil {
				t.Fatal("lifeform workbook", err)
			}
			lifeformPreview, err := siteUnitSummaryLifeformPreview(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			book := openSiteUnitSummaryWorkbook(t, lifeform)
			for index, sheet := range lifeform.Sheets {
				for field := range 49 {
					row := field + 7
					if field >= 21 {
						row += 2
					}
					if field >= 37 {
						row += 2
					}
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A"+strconv.Itoa(row), lifeformPreview.Report.Fields[field].Label)
					siteUnitSummaryWorkbookCell(t, book, sheet.Name, "B"+strconv.Itoa(row), lifeformPreview.Report.Units[index].Values[field])
				}
				siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A47", "SOILS")
				siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A59", "Bedrock Type 3")
			}
			for _, grouping := range []int{1, 2} {
				for _, average := range []int{1, 2} {
					for _, thresholds := range [][3]int{{1, 0, 0}, {1, 32767, 32767}, {2, 0, 1}} {
						request := SiteUnitSpeciesListRequest{method, grouping, average, thresholds[0], thresholds[1], thresholds[2]}
						preview, err := siteUnitSummarySpeciesPreview(context.Background(), input, request)
						if err != nil {
							t.Fatal(err)
						}
						result, err := prepareSiteUnitSummarySpeciesWorkbook(context.Background(), input, request)
						if err != nil {
							t.Fatal("species workbook", err)
						}
						again, err := prepareSiteUnitSummarySpeciesWorkbook(context.Background(), input, request)
						if err != nil || !bytes.Equal(result.Bytes, again.Bytes) {
							t.Fatal("workbook output is nondeterministic", err)
						}
						book := openSiteUnitSummaryWorkbook(t, result)
						for index, sheet := range result.Sheets {
							unit := preview.Environment.Report.Units[index]
							siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A1", unit.Code)
							siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A3", "Plots in unit: "+strconv.Itoa(len(unit.Plots)))
							siteUnitSummaryWorkbookCell(t, book, sheet.Name, "F6", "SITE")
							siteUnitSummaryWorkbookCell(t, book, sheet.Name, "F29", "VEGETATION")
							soils := 37
							if grouping == 2 {
								soils = 47
							}
							siteUnitSummaryWorkbookCell(t, book, sheet.Name, "F"+strconv.Itoa(soils), "SOILS")
							for column, heading := range []string{"Scientific Name", "Common Name", "%Cover", "%Presence"} {
								siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("%c4", 'A'+column), heading)
							}
							position := 5
							for _, group := range preview.Units[index].Groups {
								siteUnitSummaryWorkbookCell(t, book, sheet.Name, "A"+strconv.Itoa(position), group.Caption)
								styleID, err := book.GetCellStyle(sheet.Name, "A"+strconv.Itoa(position))
								if err != nil {
									t.Fatal(err)
								}
								style, err := book.GetStyle(styleID)
								if err != nil || (style.Font != nil && style.Font.Bold) != (grouping == 2) {
									t.Fatal("layer versus lifeform group bold differs", style, err)
								}
								position++
								for _, row := range group.Rows {
									if !row.Included {
										continue
									}
									for column, value := range []*string{row.ScientificName.Text, row.EnglishName.Text, &row.Cover, &row.Presence} {
										expected := ""
										if value != nil {
											expected = *value
										}
										siteUnitSummaryWorkbookCell(t, book, sheet.Name, fmt.Sprintf("%c%d", 'A'+column, position), expected)
									}
									position++
								}
								position++
							}
						}
						rows, err := book.GetRows("_VPRO_Source")
						if err != nil {
							t.Fatal(err)
						}
						var encoded strings.Builder
						for _, row := range rows {
							encoded.WriteString(row[0])
						}
						data, err := hex.DecodeString(encoded.String())
						if err != nil {
							t.Fatal(err)
						}
						var actual SiteUnitSpeciesListPreview
						if err := json.Unmarshal(data, &actual); err != nil || !reflect.DeepEqual(actual, preview) {
							t.Fatal("typed criteria/excluded rows/NULL source provenance changed", err)
						}
					}
				}
			}
			after, err := json.Marshal(input)
			if err != nil || !bytes.Equal(original, after) {
				t.Fatal("workbook preparation mutated its input", err)
			}
		}
		assertProfileSUFiles(t, service, before)
		after, err := os.ReadFile(service.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("private workbook capture wrote preferences", err)
		}
	}
}

func TestSummaryExtendedWorkbookRefusalCancellationAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	input, err := service.readSiteUnitQuickVegetationInput(context.Background(), state.ContextID, 1, publicationReadSnapshotHooks{})
	if err != nil {
		t.Fatal(err)
	}
	request := SiteUnitSpeciesListRequest{1, 1, 1, 1, 0, 0}
	if result, err := prepareSiteUnitSummarySpeciesWorkbook(nil, input, request); err == nil || len(result.Bytes) != 0 {
		t.Fatal("nil context accepted", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := prepareSiteUnitSummarySpeciesWorkbook(ctx, input, request); !errors.Is(err, context.Canceled) || len(result.Bytes) != 0 {
		t.Fatal("cancelled workbook returned bytes", err)
	}
	for _, remaining := range []int{1, 10, 100, 300} {
		ctx := &vegetationCancelContext{Context: context.Background(), remaining: remaining}
		if result, err := prepareSiteUnitSummarySpeciesWorkbook(ctx, input, request); !errors.Is(err, context.Canceled) || len(result.Bytes) != 0 {
			t.Fatal("late cancellation returned partial workbook", remaining, err)
		}
	}
	for _, mutate := range []func(*SiteUnitSpeciesListRequest){
		func(r *SiteUnitSpeciesListRequest) { r.Method = 2 },
		func(r *SiteUnitSpeciesListRequest) { r.OrderBy = 3 },
		func(r *SiteUnitSpeciesListRequest) { r.AndOr = 9 },
		func(r *SiteUnitSpeciesListRequest) { r.CoverGreaterThan = 32768 },
	} {
		invalid := request
		mutate(&invalid)
		if result, err := prepareSiteUnitSummarySpeciesWorkbook(context.Background(), input, invalid); err == nil || len(result.Bytes) != 0 {
			t.Fatal("malformed options accepted", err)
		}
	}
	lifeform, err := siteUnitSummaryLifeformPreview(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := prepareSiteUnitSummaryWorkbook(context.Background(), lifeform); err == nil || len(result.Bytes) != 0 {
		t.Fatal("original layer-only kernel widened implicitly", err)
	}
	if result, err := prepareSiteUnitSummarySpeciesWorkbook(context.Background(), input, request); err != nil || len(result.Bytes) == 0 {
		t.Fatal("retry unavailable", err)
	}
}
