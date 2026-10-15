package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func summaryVegetationGroupedServiceFixture(t *testing.T, external bool) (*SummaryVegetationService, ProjectState) {
	t.Helper()
	service, state := summaryVegetationServiceFixture(t, external)
	owner := service.contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["VPro64"], `UPDATE LayerCode SET Strata='A',LayerCode='01' WHERE Layer1234567='1'`)
	return service, state
}

func TestSummaryVegetationGroupedServiceOwnedDraftZeroWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state := summaryVegetationGroupedServiceFixture(t, external)
		before := databaseBytes(t, service.contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, grouping := range []int{12, 13, 14} {
			for _, average := range []int{1, 2} {
				got, err := service.PreviewGrouped(context.Background(), state.ContextID, average, grouping)
				if err != nil || got.ContextID != state.ContextID || got.Average != average || got.Display != 6 ||
					got.GroupAndOrder != grouping || got.ShowClass != 0 || got.GroupField == nil || got.OrderField == nil ||
					got.OrderField.DAOType != 7 || got.Source != "literal-normal-su-grouped" ||
					got.ProjectPath != state.ProjectPath || got.SUPath != state.SUPath || len(got.Rows) == 0 {
					t.Fatal(external, average, grouping, got, err)
				}
				for _, row := range got.Rows {
					if row.Order.Storage != "real" || grouping == 14 && row.Layer.Storage != "null" {
						t.Fatal("typed grouped projection lost", row)
					}
					for _, witness := range row.Witnesses {
						if witness.ConversionLayerID != "" || len(witness.RawVegRowIDs) == 0 {
							t.Fatal("raw builder provenance lost", witness)
						}
					}
				}
				again, err := service.PreviewGrouped(context.Background(), state.ContextID, average, grouping)
				if err != nil || !reflect.DeepEqual(got, again) {
					t.Fatal("owned grouped reread differs", err)
				}
				got.GroupField.Name = "changed"
				*got.Rows[0].Covers[0].Text = "changed"
				got.Rows[0].Witnesses[0].RawVegRowIDs[0] = "changed"
				if again.GroupField.Name == "changed" || *again.Rows[0].Covers[0].Text == "changed" ||
					again.Rows[0].Witnesses[0].RawVegRowIDs[0] == "changed" {
					t.Fatal("grouped transport aliases read")
				}
			}
		}
		assertProfileSUFiles(t, service.contexts, before)
		after, err := os.ReadFile(service.contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("grouped draft persisted settings", err)
		}
	}
}

func TestSummaryVegetationGroupedServiceErrorsCleanupRetry(t *testing.T) {
	service, state := summaryVegetationGroupedServiceFixture(t, true)
	for _, options := range [][2]int{{0, 12}, {1, 11}, {1, 15}} {
		got, err := service.PreviewGrouped(context.Background(), state.ContextID, options[0], options[1])
		if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
			t.Fatal("invalid explicit grouping produced output", got, err)
		}
	}
	if _, err := service.PreviewGrouped(nil, state.ContextID, 1, 12); err == nil {
		t.Fatal("nil context accepted")
	}
	for _, identity := range []string{"", "foreign"} {
		if _, err := service.PreviewGrouped(context.Background(), identity, 1, 12); err == nil {
			t.Fatal("foreign context accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := service.PreviewGrouped(ctx, state.ContextID, 1, 12)
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("cancelled grouped read published", got, err)
	}
	owner := service.contexts.projects.sqlite
	service.snapshot.commitRead = func(tx *sql.Tx) error {
		return errors.New("held grouping snapshot failure")
	}
	got, err = service.PreviewGrouped(context.Background(), state.ContextID, 1, 12)
	if err == nil || !reflect.DeepEqual(got, SummaryVegetationPreview{}) {
		t.Fatal("failed snapshot published", got, err)
	}
	service.snapshot = publicationReadSnapshotHooks{}
	if _, err := service.PreviewGrouped(context.Background(), state.ContextID, 1, 12); err != nil {
		t.Fatal("snapshot cleanup retry failed", err)
	}
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+` SET Cover10=1e100 WHERE PlotNumber='P1'`)
	if _, err := service.PreviewGrouped(context.Background(), state.ContextID, 1, 14); err == nil {
		t.Fatal("unused TempReportVeg SINGLE overflow accepted")
	}
	mutateContextFixture(t, owner.attachments["project"], `UPDATE `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+` SET Cover10=NULL WHERE PlotNumber='P1'`)
	if _, err := service.PreviewGrouped(context.Background(), state.ContextID, 1, 14); err != nil {
		t.Fatal("raw correction retry failed", err)
	}
	service.enabled = false
	if _, err := service.PreviewGrouped(context.Background(), state.ContextID, 1, 12); err == nil {
		t.Fatal("grouped gate bypassed")
	}
}

func TestSummaryVegetationGroupedClientWireFixture(t *testing.T) {
	service, state := summaryVegetationGroupedServiceFixture(t, false)
	cases := []SummaryVegetationPreview{}
	for _, grouping := range []int{12, 13, 14} {
		for _, average := range []int{1, 2} {
			result, err := service.PreviewGrouped(context.Background(), state.ContextID, average, grouping)
			if err != nil {
				t.Fatal(err)
			}
			result.ContextID, result.Project, result.ProjectPath, result.SU, result.SUPath =
				"owned", "Sample", `C:\fixture\project.db`, "Report", `C:\fixture\project.db`
			cases = append(cases, result)
		}
	}
	encoded, err := json.MarshalIndent(cases, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	fixture := filepath.Join("frontend", "src", "summaryVegetationGroupedWire.json")
	if os.Getenv("VPRO_UPDATE_SUMMARY_VEGETATION_GROUPED_WIRE") == "true" {
		if err := os.WriteFile(fixture, append(encoded, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var expected []SummaryVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil || !reflect.DeepEqual(cases, expected) || len(cases) != 6 {
		t.Fatal("grouped wire differs from production DTO/owned physical read", err)
	}
}
