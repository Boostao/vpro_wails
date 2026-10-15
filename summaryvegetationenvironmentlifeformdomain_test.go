package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strconv"
	"testing"
)

func TestSummaryEnvironmentalLifeformCompleteCanonicalDomain(t *testing.T) {
	quick, master, su := summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformDomainSourceFixture)
	var source struct {
		Observations []struct {
			Unit     *string  `json:"unit"`
			Argument *string  `json:"argument"`
			Actual   []string `json:"actual"`
		} `json:"observations"`
	}
	if err := json.Unmarshal([]byte(summaryEnvironmentalLifeformDomainSourceFixture), &source); err != nil ||
		len(source.Observations) != 93 {
		t.Fatal("complete93 original observations required", err)
	}
	before, err := json.Marshal([]ProjectMetadataTable{quick, master, su})
	if err != nil {
		t.Fatal(err)
	}
	forms := map[string]int{}
	for _, item := range source.Observations {
		if item.Argument == nil {
			if !reflect.DeepEqual(item.Actual[:2], []string{"error", "94"}) {
				t.Fatal("original NULL String argument differs", item)
			}
			continue
		}
		got, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), *item.Argument, item.Unit, quick, master, su)
		if item.Unit == nil {
			if err == nil || !reflect.DeepEqual(item.Actual[:2], []string{"error", "94"}) {
				t.Fatal("original NULL SiteUnit must refuse", got, err, item)
			}
			continue
		}
		forms[*item.Argument]++
		if *item.Unit == "MIXED" {
			if err != nil || got.Storage != "text" || got.Text == nil || *got.Text != "" {
				t.Fatal("native literal-scope adaptation changed", got, err)
			}
			continue
		}
		if err != nil || got.Storage != "text" || got.Text == nil ||
			!reflect.DeepEqual(item.Actual, []string{"ok", "String", *got.Text}) {
			t.Fatal("original complete-domain value differs", got, err, item)
		}
	}
	if len(forms) != 13 {
		t.Fatal("all13 original field arguments must be covered", forms)
	}
	for form := 1; form <= 13; form++ {
		if forms[strconv.Itoa(form)] != 7 {
			t.Fatal("seven original unit observations required per argument", form, forms)
		}
	}
	after, err := json.Marshal([]ProjectMetadataTable{quick, master, su})
	if err != nil || string(after) != string(before) {
		t.Fatal("complete-domain input mutation", err)
	}
}

func TestSummaryEnvironmentalLifeformCompleteDomainStrictRefusals(t *testing.T) {
	quick, master, su := summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformDomainSourceFixture)
	unit := "mixed"
	for _, argument := range []string{"0", "14", "-1", "3.0", "03", " 3", "3 ", "3\x00", "３", "999999999999999999"} {
		if _, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), argument, &unit, quick, master, su); err == nil {
			t.Fatal("unavailable/nonliteral argument accepted", argument)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for form := 1; form <= 13; form++ {
		if _, err := prepareSummaryVegetationEnvironmentalLifeform(ctx, strconv.Itoa(form), &unit, quick, master, su); !errors.Is(err, context.Canceled) {
			t.Fatal("cancellation must preserve identity for every form", form, err)
		}
	}
	for _, form := range []string{"3", "7", "12"} {
		for _, target := range []string{"quick", "master", "su"} {
			quick, master, su := summaryEnvironmentalLifeformFixtureFromJSON(t, summaryEnvironmentalLifeformDomainSourceFixture)
			bad := string([]byte{0xff})
			switch target {
			case "quick":
				quick.Rows[len(quick.Rows)-1].Cells[0] = ProjectMetadataCell{Storage: "text", Text: &bad}
			case "master":
				master.Rows[len(master.Rows)-1].Cells[1] = metadataInteger("32768")
			case "su":
				su.Rows[len(su.Rows)-1].Cells[1] = ProjectMetadataCell{Storage: "text", Text: &bad}
			}
			if _, err := prepareSummaryVegetationEnvironmentalLifeform(t.Context(), form, &unit, quick, master, su); err == nil {
				t.Fatal("new argument bypassed complete physical-source validation", form, target)
			}
		}
	}
}
