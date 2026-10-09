package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestLongVegetationCodeReferenceBridgePreservesLiteralCellsAndGrouping(t *testing.T) {
	prepared, species, layers := vegetationLayerFixture(t)
	species.Rows = append(species.Rows, ProjectMetadataRow{RowID: "2", Cells: []ProjectMetadataCell{
		metadataText("A"), metadataText("Scientific A"), metadataText("Different English"), metadataText("U"),
	}})
	options := layerTestOptions()
	options.ShowEnglishName, options.ShowSpeciesCode = false, true
	bridge, internal, err := longVegetationCodeReferences(context.Background(), species, options)
	if err != nil || !internal.ShowEnglishName || internal.ShowSpeciesCode ||
		*bridge.Rows[1].Cells[2].Text != "A" || *species.Rows[1].Cells[2].Text != "Different English" {
		t.Fatal("Code replaced source data or reused English grouping", bridge, internal, err)
	}
	report, err := planLongVegetationLayers(context.Background(), prepared, bridge, layers, internal)
	if err != nil {
		t.Fatal(err)
	}
	for _, unit := range report.Units {
		if vegetationTextKey(unit.Code) != "text:U" {
			continue
		}
		var matched []vegetationLayerRow
		for _, row := range unit.Rows {
			if vegetationTextKey(row.Layer) == "text:L1" && vegetationTextKey(row.Species) == "text:Scientific A" {
				matched = append(matched, row)
			}
		}
		if len(matched) != 1 || *matched[0].EnglishName.Text != "A" ||
			*matched[0].MatchedName.Text != "A" || *matched[0].MeanCover != -4 {
			t.Fatal("Code grouping retained English-name split or lost physical fanout", matched)
		}
	}
	*bridge.Rows[0].Cells[0].Text = "changed"
	*bridge.Rows[0].Cells[2].Text = "changed"
	if *species.Rows[0].Cells[0].Text != "A" || *species.Rows[0].Cells[2].Text != "English A" {
		t.Fatal("Code bridge cells alias borrowed source")
	}
	species.Rows[0].Cells[0] = metadataText("  LongerThanEight  ")
	literal, _, err := longVegetationCodeReferences(context.Background(), species, options)
	if err != nil || *literal.Rows[0].Cells[2].Text != "  LongerThanEight  " {
		t.Fatal("source Code caption caused truncation or normalization", literal, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := longVegetationCodeReferences(ctx, species, options); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled Code reference bridge accepted", err)
	}
}

func TestLongVegetationCodeDefaultGateAndOwnedAllGroupingModes(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "internal", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVShowEnglishName": 2}); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			service.longVegetationNoneEnabled, service.longVegetationLifeformEnabled, service.longVegetationStrataEnabled = true, true, true
			if _, err := service.GetLongVegetationOptions(context.Background(), state.ContextID); err == nil {
				t.Fatal("other grouping gates authorized supplementary Code")
			}
			if _, err := service.PreviewLongVegetation(context.Background(), state.ContextID); err == nil {
				t.Fatal("default-off Code report accepted")
			}
			service.longVegetationCodeEnabled = true
			tx, err := service.projects.sqlite.beginReadSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			rows, err := tx.QueryContext(context.Background(), `SELECT Code FROM VLists.USysAllSpecs WHERE typeof(Code)='text'`)
			if err != nil {
				t.Fatal(err)
			}
			originalCodes := map[string]bool{}
			for rows.Next() {
				var code string
				if err := rows.Scan(&code); err != nil {
					t.Fatal(err)
				}
				originalCodes[code] = true
			}
			if err := errors.Join(rows.Err(), rows.Close(), tx.Rollback()); err != nil {
				t.Fatal(err)
			}
			for _, grouping := range []int{1, 2, 3, 4} {
				if err := service.projects.preferences.update("ReportOptions", map[string]any{"LVGroupBy": grouping}); err != nil {
					t.Fatal(err)
				}
				preview, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
				if err != nil || !preview.Settings.ShowSpeciesCode || preview.Settings.ShowEnglishName ||
					len(preview.Report.Units) == 0 {
					t.Fatal("Code mode failed owned grouping", grouping, preview, err)
				}
				again, err := service.PreviewLongVegetation(context.Background(), state.ContextID)
				if err != nil || !reflect.DeepEqual(preview, again) {
					t.Fatal("Code reads differ", grouping, err)
				}
				for _, unit := range preview.Report.Units {
					for _, row := range unit.Rows {
						for _, cell := range []ProjectMetadataCell{row.EnglishName, row.MatchedName} {
							if cell.Storage != "text" && cell.Storage != "null" {
								t.Fatal("Code mode lost original nullable text", cell)
							}
							if cell.Text != nil && !originalCodes[*cell.Text] {
								t.Fatal("supplementary Code was invented or came from synthetic grouping identity", grouping, cell)
							}
						}
					}
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, err := service.PreviewLongVegetation(ctx, state.ContextID); !errors.Is(err, context.Canceled) {
				t.Fatal("cancelled Code read accepted", err)
			}
			if _, err := service.PreviewLongVegetation(context.Background(), "stale"); err == nil {
				t.Fatal("stale Code owner accepted")
			}
			assertProfileSUFiles(t, service, before)
			options := layerTestOptions()
			options.ShowSpeciesCode = true
			if err := service.checkLongVegetationGrouping(options); err == nil {
				t.Fatal("simultaneous English/Code fields accepted")
			}
		})
	}
}
