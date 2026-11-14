package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestSummarySpeciesPreviewStrictRequestsAndZeroWrites(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, true)
	request := SiteUnitSpeciesListRequest{1, 1, 1, 1, 0, 0}
	before := vprotest.DatabaseBytes(t, service.Projects.Sqlite.Attachments)
	config, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []int{1, 2} {
		for _, order := range []int{1, 2} {
			request.Method, request.OrderBy = method, order
			got, err := PreviewSiteUnitSummarySpecies(service, context.Background(), state.ContextID, request)
			if err != nil || !reflect.DeepEqual(got.Options, request) || got.Environment.Report.Method != method {
				t.Fatal("explicit owned species output unavailable", got, err)
			}
			expectedFields := 39
			if order == 2 {
				expectedFields = 49
			}
			if len(got.Environment.Report.Fields) != expectedFields ||
				len(got.Units) != len(got.Environment.Report.Units) {
				t.Fatal("species and Environment scopes differ", got)
			}
			for i, unit := range got.Units {
				if unit.Code != got.Environment.Report.Units[i].Code ||
					unit.NPlots != len(got.Environment.Report.Units[i].Plots) {
					t.Fatal("species denominator differs from owned physical Environment count", unit)
				}
			}
		}
	}
	vprotest.AssertProfileSUFiles(t, service, before)
	after, err := os.ReadFile(service.Projects.Preferences.Path)
	if err != nil || string(config) != string(after) {
		t.Fatal("read-only explicit grouping/criteria changed saved configuration", err)
	}
	valid := `{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0,"coverGreaterThan":0}`
	var decoded SiteUnitSpeciesListRequest
	if err := json.Unmarshal([]byte(valid), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, `null`,
		`{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0}`,
		`{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0,"coverGreaterThan":null}`,
		`{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0,"coverGreaterThan":0,"method":2}`,
		`{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0.5,"coverGreaterThan":0}`,
		`{"method":1,"orderBy":1,"coverCalculation":1,"andOr":1,"presenceGreaterThan":0,"coverGreaterThan":0,"unknown":"\ud800"}`,
	} {
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			t.Fatal("malformed/missing/duplicate/Unicode criteria accepted", raw)
		}
	}
}

func TestSummarySpeciesPreviewCancellationRefusalOwnershipAndRetry(t *testing.T) {
	service, state := vprotest.ReportServiceFixture(t, false)
	request := SiteUnitSpeciesListRequest{1, 1, 1, 1, 0, 0}
	for _, invalid := range []SiteUnitSpeciesListRequest{
		{0, 1, 1, 1, 0, 0}, {1, 3, 1, 1, 0, 0}, {1, 1, 0, 1, 0, 0},
		{1, 1, 1, 0, 0, 0}, {1, 1, 1, 1, 0, 32768},
	} {
		if got, err := PreviewSiteUnitSummarySpecies(service, context.Background(), state.ContextID, invalid); err == nil || !reflect.DeepEqual(got, SiteUnitSpeciesListPreview{}) {
			t.Fatal("invalid criteria returned partial public output", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := PreviewSiteUnitSummarySpecies(service, ctx, state.ContextID, request); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SiteUnitSpeciesListPreview{}) {
		t.Fatal("cancelled source returned partial output", got, err)
	}
	if _, err := PreviewSiteUnitSummarySpecies(service, nil, state.ContextID, request); err == nil {
		t.Fatal("missing context authorized")
	}
	next, err := service.SwitchContext(state.ContextID, vprotest.ContextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewSiteUnitSummarySpecies(service, context.Background(), state.ContextID, request); err == nil {
		t.Fatal("stale source ownership authorized")
	}
	if _, err := PreviewSiteUnitSummarySpecies(service, context.Background(), next.ContextID, request); err != nil {
		t.Fatal("refusal/cancellation leaked ownership lease", err)
	}
}
