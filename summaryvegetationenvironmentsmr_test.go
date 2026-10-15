package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSummaryEnvironmentalSMROriginalCases(t *testing.T) {
	labels := []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "", "bad", "00", " 0", "0 "}
	expected := []string{"VX", "X", "SX", "SM", "M", "SHG", "HG", "SHD", "HD", "", "", "", "", ""}
	path := os.Getenv("VPRO_SUMMARY_SMR_SOURCE")
	raw := []byte(summaryEnvironmentalSMRSourceFixture)
	if path != "" {
		var err error
		raw, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	var source struct {
		Observations []struct {
			Unit   *string  `json:"unit"`
			Actual []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal(raw, &source); err != nil || len(source.Observations) != 19 {
		t.Fatal("nineteen original source cases required", err)
	}
	for i, label := range labels {
		if got := summaryVegetationSMRClassCode(label); got != expected[i] {
			t.Fatalf("independent source mapping %q: %q", label, got)
		}
	}
	for _, observation := range source.Observations {
		var categories []summaryVegetationEnvironmentalCategory
		var nulls int64
		want := ""
		if observation.Unit != nil {
			unit := *observation.Unit
			switch unit {
			case "mixed":
				categories = []summaryVegetationEnvironmentalCategory{{"", 1}, {"0", 2}, {"8", 1}}
				nulls, want = 1, "(1)  VX(2)  HD(1)  (Null 1)"
			case "allnull":
				nulls, want = 2, "(Null 2)"
			case "missing", "noenv":
			default:
				for i := range labels {
					if unit == "code"+strconv.Itoa(i) {
						categories = []summaryVegetationEnvironmentalCategory{{labels[i], 1}}
						want = expected[i] + "(1)  "
					}
				}
			}
		}
		got, err := formatSummaryVegetationEnvironmentalCategories(t.Context(), "SVSmr", observation.Unit, categories, nulls)
		if observation.Unit == nil {
			if err == nil || observation.Actual[0] != "error" || observation.Actual[1] != "94" {
				t.Fatal(got, err, observation)
			}
		} else if err != nil || got.Storage != "text" || got.Text == nil || *got.Text != want ||
			!reflect.DeepEqual(observation.Actual, []string{"ok", "String", want}) {
			t.Fatal("independent original summary differs", got, err, want, observation)
		}
	}
}

func TestSummaryEnvironmentalSMRLiteralGroupingAndOwnedConsumer(t *testing.T) {
	service, state := summaryEnvironmentalCombinedFixture(t, false)
	owner := service.report.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE USysShortVegSummaryFieldList SET FunctionName='SVSmr' WHERE FunctionName='SVSnr'`)
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET MoistureRegime=CASE PlotNumber WHEN 'P1' THEN '0' WHEN 'P2' THEN '00' ELSE NULL END`)
	got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || got == nil || *got.Rows[1].Values[0].Text != "VX(2)  (1)  " ||
		*got.Rows[1].Values[1].Text != "(Null 1)" {
		t.Fatal("literal codes grouped before label conversion", got, err)
	}
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET MoistureRegime=CASE PlotNumber WHEN 'P1' THEN '' WHEN 'P2' THEN 'bad' ELSE NULL END`)
	got, err = service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0)
	if err != nil || *got.Rows[1].Values[0].Text != "(2)  (1)  " {
		t.Fatal("unknown/empty categories merged by display label", got, err)
	}
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+`
		SET MoistureRegime=CAST(X'FF' AS TEXT) WHERE PlotNumber='P1'`)
	if got, err := service.PreviewEnvironmental(t.Context(), state.ContextID, 1, 6, 0); got != nil || err == nil ||
		!strings.Contains(err.Error(), "malformed Unicode") {
		t.Fatal("malformed source repaired before mapping", got, err)
	}
}
