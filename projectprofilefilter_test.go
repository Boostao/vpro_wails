package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestProfileNavigationRevalidatesExactScopedPreviewAndPreservesEveryFile(t *testing.T) {
	service, state, input := profileRunFixture(t)
	ctx := context.Background()
	preview, err := service.RunProjectPlotProfile(ctx, state.ContextID, input)
	if err != nil {
		t.Fatal(err)
	}
	request := ProjectPlotProfileFilterRequest{Input: input, Preview: preview}
	files := databaseBytes(t, service.projects.sqlite.attachments)
	navigation, err := service.ResolveProjectPlotProfileNavigation(ctx, state.ContextID, request)
	if err != nil || navigation.ContextID != state.ContextID || len(navigation.Plots) != 11 || !reflect.DeepEqual(navigation.Result, preview) {
		t.Fatal("canonical52/11 scoped navigation unavailable", navigation, err)
	}
	for index, plot := range navigation.Plots {
		if plot.PlotNumber != preview.PlotNumbers[index] {
			t.Fatal("navigation reordered/normalized/excluded a reviewed identity", plot)
		}
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decoded ProjectPlotProfileFilterRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveProjectPlotProfileNavigation(ctx, state.ContextID, decoded); err != nil {
		t.Fatal("actual transport altered preview identity", err)
	}
	for _, contextID := range []string{"", "stale"} {
		if _, err := service.ResolveProjectPlotProfileNavigation(ctx, contextID, request); err == nil {
			t.Fatal("unowned navigation accepted", contextID)
		}
	}
	for _, alter := range []func(*ProjectPlotProfileResult){
		func(p *ProjectPlotProfileResult) { p.Project = "Other" },
		func(p *ProjectPlotProfileResult) { p.SU = "Other" },
		func(p *ProjectPlotProfileResult) { p.PlotNumbers[0] = "off-project" },
		func(p *ProjectPlotProfileResult) { p.Steps[0].PlotCount++ },
	} {
		var bad ProjectPlotProfileFilterRequest
		if err := json.Unmarshal(data, &bad); err != nil {
			t.Fatal(err)
		}
		alter(&bad.Preview)
		if _, err := service.ResolveProjectPlotProfileNavigation(ctx, state.ContextID, bad); err == nil {
			t.Fatal("forged/stale scoped membership/count accepted", bad.Preview)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.ResolveProjectPlotProfileNavigation(cancelled, state.ContextID, request); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled navigation was published", err)
	}
	for role, original := range files {
		now, err := os.ReadFile(service.projects.sqlite.attachments[role])
		if err != nil || !bytes.Equal(original, now) {
			t.Fatal("navigation wrote project/support/config/history/counts", role, err)
		}
	}
}

func TestProfileNavigationStrictTransportRejectsRepairAndMissingInputs(t *testing.T) {
	for _, raw := range []string{
		`{"input":null,"preview":{}}`, `{"input":{}}`,
		`{"input":{},"preview":{},"unknown":true}`,
		`{"input":{"unknown":true},"preview":{}}`,
		`{"input":{},"preview":{"plotNumbers":["\ud800"]}}`,
		`{"input":{"originalRules":{},"subvarieties":null},"preview":{}}`,
		`{"input":{"originalRules":{}},"preview":{}}`,
	} {
		var request ProjectPlotProfileFilterRequest
		if err := json.Unmarshal([]byte(raw), &request); err == nil {
			t.Fatal("repaired/missing/unknown filter transport accepted", raw)
		}
	}
}

func TestProfileNavigationRejectsChangedStoredInputs(t *testing.T) {
	service, state, input := profileRunFixture(t)
	ctx := context.Background()
	preview, err := service.RunProjectPlotProfile(ctx, state.ContextID, input)
	if err != nil {
		t.Fatal(err)
	}
	db, _, release, err := service.plots.getActiveDB()
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`DELETE FROM Sample_Veg WHERE PlotNumber='108050'`)
	release()
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ResolveProjectPlotProfileNavigation(ctx, state.ContextID, ProjectPlotProfileFilterRequest{input, preview})
	if err == nil || !strings.Contains(err.Error(), "changed since preview") {
		t.Fatal("changed stored profiling inputs did not invalidate navigation", err)
	}
}

func TestProfileNavigationExactThresholdsAndLiteralIdentities(t *testing.T) {
	for _, count := range []int{0, 200, 201} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			service, state, _ := profileRunFixture(t)
			db, _, release, err := service.plots.getActiveDB()
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			if _, err := db.Exec(`DELETE FROM Sample_Env; DELETE FROM Sample_Admin;
				DELETE FROM Sample_Profile WHERE rowid<>3;
				UPDATE Sample_Profile SET "Order"=1,"Table"='Env',"Field"='PlotNumber',"Operator"='Like',
				Layer=NULL,Species=NULL,Criteria='*',Operation='Add plots' WHERE rowid=3`); err != nil {
				t.Fatal(err)
			}
			for index := 0; index < count; index++ {
				plot := fmt.Sprintf("P%03d", index)
				if index == 0 {
					plot = " O'Brien # "
				}
				if _, err := db.Exec(`INSERT INTO Sample_Env(PlotNumber) VALUES(?); INSERT INTO Sample_Admin(Plot) VALUES(?)`, plot, plot); err != nil {
					t.Fatal(err)
				}
			}
			ctx := context.Background()
			review, err := service.ReviewProjectPlotProfile(ctx, state.ContextID)
			if err != nil {
				t.Fatal(err)
			}
			input := ProjectPlotProfileRunRequest{OriginalRules: review.Rules}
			preview, err := service.RunProjectPlotProfile(ctx, state.ContextID, input)
			if err != nil || len(preview.PlotNumbers) != count {
				t.Fatal("threshold did not exercise actual execution output", preview, err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			navigation, err := service.ResolveProjectPlotProfileNavigation(ctx, state.ContextID, ProjectPlotProfileFilterRequest{input, preview})
			if count == 200 {
				if err != nil || len(navigation.Plots) != 200 || navigation.Plots[0].PlotNumber != " O'Brien # " {
					t.Fatal("200-result boundary/literal identity changed", navigation, err)
				}
			} else if err == nil || count == 0 && !strings.Contains(err.Error(), "zero plots") ||
				count == 201 && !strings.Contains(err.Error(), "exceeds 200") {
				t.Fatal("empty/over-limit result became a success-shaped fallback", count, err)
			}
			for role, prior := range before {
				now, err := os.ReadFile(service.projects.sqlite.attachments[role])
				if err != nil || !bytes.Equal(prior, now) {
					t.Fatal("threshold navigation wrote stored data", count, role, err)
				}
			}
		})
	}
}
