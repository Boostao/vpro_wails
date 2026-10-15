package main

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
)

func summaryEnvironmentalNumericServiceFixture(t *testing.T, external bool) (*SummaryVegetationEnvironmentalService, ProjectState) {
	t.Helper()
	service, state := summaryEnvironmentalCombinedFixture(t, external)
	service.numericEnabled = true
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET SeepageDepth=1,RootingDepth=2,StandAge=3,Elevation=4,SlopeGradient=1.25,
		SubstrateDecWood=1.25,SubstrateBedRock=2.5,SubstrateRocks=4,
		SubstrateMineralSoil=5,SubstrateOrganicMatter=6,SubstrateWater=7,
		StrataCoverTree=8,StrataCoverShrub=9,StrataCoverHerb=10,StrataCoverMoss=11 WHERE PlotNumber='P1';
		UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET SeepageDepth=5,RootingDepth=6,StandAge=7,Elevation=8,SlopeGradient=5.75,
		SubstrateDecWood=5.75,SubstrateBedRock=2.5,SubstrateRocks=4,
		SubstrateMineralSoil=5,SubstrateOrganicMatter=6,SubstrateWater=7,
		StrataCoverTree=8,StrataCoverShrub=9,StrataCoverHerb=10,StrataCoverMoss=11 WHERE PlotNumber='P2'`)
	mutateContextFixture(t, owner.attachments["VPro64"], `INSERT INTO USysShortVegSummaryFieldList VALUES
		(NULL,'Env','Seepage',NULL,-1,'SVSeepage',NULL),
		(2,'Env','Root',NULL,-1,'SVRootingDepth',NULL),
		(3,'Env','Age',NULL,-1,'SVStandAge',NULL),
		(4,'Env','Elevation',NULL,-1,'SVElevation',NULL),
		(5,'Env','Slope',NULL,-1,'SVSlope',NULL),
		(6,'Env','DecWood',NULL,-1,'SVSubstrate','DecWood'),
		(7,'Env','BedRock',NULL,-1,'SVSubstrate','BedRock'),
		(8,'Env','Rocks',NULL,-1,'SVSubstrate','Rocks'),
		(9,'Env','MineralSoil',NULL,-1,'SVSubstrate','MineralSoil'),
		(10,'Env','OrganicMatter',NULL,-1,'SVSubstrate','OrganicMatter'),
		(11,'Env','Water',NULL,-1,'SVSubstrate','Water'),
		(12,'Env','Tree',NULL,-1,'SVStrataCover','Tree'),
		(13,'Env','Shrub',NULL,-1,'SVStrataCover','Shrub'),
		(14,'Env','Herb',NULL,-1,'SVStrataCover','Herb'),
		(15,'Env','Moss',NULL,-1,'SVStrataCover','Moss')`)
	return service, state
}

func TestSummaryEnvironmentalNumericSnapshotOwnedAndWire(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryEnvironmentalNumericServiceFixture(t, external)
		owner := service.report.contexts.projects.sqlite
		before := databaseBytes(t, owner.attachments)
		got, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0)
		if err != nil || got == nil || len(got.Rows) != 17 || got.Source != summaryEnvironmentalNumericSource {
			t.Fatal(got, err)
		}
		want := map[string]string{"SVSeepage": "1 - 2.3 - 5", "SVRootingDepth": "2 - 3.3 - 6",
			"SVStandAge": "3 - 04 - 7", "SVElevation": "4 - 05 - 8", "SVSlope": "1.25 - 3 - 5.75"}
		for _, row := range got.Rows {
			function := *row.Field.Cells[5].Text
			if text, ok := want[function]; ok {
				if row.Values[0].Cell == nil || row.Values[0].Cell.Text == nil || *row.Values[0].Cell.Text != text {
					t.Fatal(row, text)
				}
			}
			if summaryEnvironmentalIsNumeric(function) {
				if function == "SVStandAge" || function == "SVElevation" {
					text := "(Null 1)"
					if function == "SVElevation" {
						text = "  (Null 1)"
					}
					if row.Values[1].Kind != "cell" || row.Values[1].Cell.Text == nil || *row.Values[1].Cell.Text != text {
						t.Fatal(row)
					}
				} else if !reflect.DeepEqual(row.Values[1], SummaryVegetationEnvironmentalValue{Kind: "empty"}) {
					t.Fatal(row)
				}
			}
		}
		if !reflect.DeepEqual(databaseBytes(t, owner.attachments), before) {
			t.Fatal("snapshot mutated physical inputs")
		}
		encoded, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		var decoded SummaryVegetationEnvironmentalNumericPreview
		decoder := json.NewDecoder(bytes.NewReader(encoded))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&decoded); err != nil || !reflect.DeepEqual(got, &decoded) {
			t.Fatal("strict typed snapshot wire", err)
		}
		if !external {
			if path := os.Getenv("VPRO_SUMMARY_ENVIRONMENT_NUMERIC_PREVIEW_WIRE_PATH"); path != "" {
				got.Vegetation.ContextID = "summary-numeric-wire"
				got.Vegetation.ProjectPath = `C:\fixture\Sample.db`
				got.Vegetation.SUPath = `C:\fixture\Sample.db`
				got.SourcePath = `C:\fixture\VPro64.db`
				got.EnvSourcePath = `C:\fixture\Sample.db`
				encoded, err := json.MarshalIndent(got, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				encoded = append(encoded, '\n')
				if os.Getenv("VPRO_UPDATE_SUMMARY_ENVIRONMENT_NUMERIC_PREVIEW_WIRE") == "true" {
					if err := os.WriteFile(path, encoded, 0600); err != nil {
						t.Fatal(err)
					}
				}
				actual, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(actual, encoded) {
					t.Fatal("numeric preview wire changed", err)
				}
			}
		}
		service.enabled = false
		if result, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0); err == nil || result != nil {
			t.Fatal("gate", result, err)
		}
	}
}

func TestSummaryEnvironmentalNumericSnapshotCorrectionCleanup(t *testing.T) {
	service, state := summaryEnvironmentalNumericServiceFixture(t, true)
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FieldNumber='decwood' WHERE Label='DecWood'`)
	if got, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil || !strings.Contains(err.Error(), "FieldNumber") {
		t.Fatal(got, err)
	}
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FieldNumber='DecWood' WHERE Label='DecWood'`)
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("numeric snapshot commit failed") }
	if got, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil {
		t.Fatal(got, err)
	}
	service.snapshot.commitRead = nil
	ctx, cancel := context.WithCancel(t.Context())
	service.snapshot.commitRead = func(*sql.Tx) error { cancel(); return nil }
	if got, err := service.PreviewEnvironmentalNumeric(ctx, state.ContextID, 1, 6, 0); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal(got, err)
	}
	service.snapshot.commitRead = nil
	if got, err := service.PreviewEnvironmentalNumeric(t.Context(), state.ContextID, 1, 6, 0); err != nil || got == nil {
		t.Fatal("retry", got, err)
	}
	if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); err == nil || got != nil {
		t.Fatal("old facade numeric enabling", got, err)
	}
}
