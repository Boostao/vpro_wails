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
	"time"
)

func summaryVegetationServiceFixture(t *testing.T, external bool) (*SummaryVegetationService, ProjectState) {
	t.Helper()
	contexts, state := reportServiceFixture(t, external)
	owner := contexts.projects.sqlite
	mutateContextFixture(t, owner.attachments["project"], `DELETE FROM `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+
		`; INSERT INTO `+quoteHeaderIdentifier(state.ActiveProject+"_Veg")+`(PlotNumber,Species,Cover1) VALUES('P1','X',10),('P2','X',20),('P3','X',5),('P2','S',2)`)
	mutateContextFixture(t, owner.attachments["su"], `DELETE FROM Report_SU;
		INSERT INTO Report_SU VALUES('P1','A'),('P1','A'),('P2','A'),('P5','A'),('P3','B'),(NULL,'A'),('P9',NULL),('P6','Orphan')`)
	mutateContextFixture(t, owner.attachments["VLists"], `DELETE FROM USysAllSpecs;
		INSERT INTO USysAllSpecs(Code,ScientificName,Lifeform,EnglishName,Codetype) VALUES('X','Alpha',1,'x','U'),('S','Excluded',2,'s','s')`)
	mutateContextFixture(t, owner.attachments["VUser"], `DELETE FROM USysUserSpp`)
	mutateContextFixture(t, owner.attachments["VPro64"], `DELETE FROM LayerCode;
		INSERT INTO LayerCode(LayerText,Layer1234567,Layer) VALUES('1','1','A1'),('10','10',NULL)`)
	service, err := NewSummaryVegetationService(contexts, func(name string) (string, bool) {
		if name != summaryVegetationFeatureEnvironment {
			t.Fatal("gate borrowed another workflow", name)
		}
		return "true", true
	})
	if err != nil {
		t.Fatal(err)
	}
	return service, state
}

func TestSummaryVegetationServiceGateAndOwnedZeroWrites(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", "1", " true"} {
		service, err := NewSummaryVegetationService(nil, func(string) (string, bool) { return flag, flag != "" })
		if flag == "" || flag == "false" || flag == "true" {
			if err != nil || service.enabled != (flag == "true") {
				t.Fatal(flag, service, err)
			}
			if _, err := service.Preview(context.Background(), "owned", 1, 6); err == nil {
				t.Fatal("unavailable service published")
			}
		} else if err == nil {
			t.Fatal("nonliteral feature flag accepted", flag)
		}
	}
	for _, external := range []bool{false, true} {
		service, state := summaryVegetationServiceFixture(t, external)
		contexts := service.contexts
		before := databaseBytes(t, contexts.projects.sqlite.attachments)
		config, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil {
			t.Fatal(err)
		}
		for _, mode := range []int{1, 2, 3, 6} {
			for _, average := range []int{1, 2} {
				result, err := service.Preview(context.Background(), state.ContextID, average, mode)
				if err != nil || result.ContextID != state.ContextID || result.Project != state.ActiveProject ||
					result.ProjectPath != state.ProjectPath || result.SUPath != state.SUPath || result.SU != "Report" ||
					result.Average != average || result.Display != mode || len(result.Memberships) != 8 ||
					len(result.Units) != 2 || result.Units[0].NumPlots != 4 || result.Units[1].NumPlots != 1 ||
					!reflect.DeepEqual(result.Columns, []string{"A", "B"}) || len(result.Rows) != 2 ||
					*result.Rows[0].Layer.Text != "A1" || *result.Rows[0].Species.Text != "Alpha" ||
					len(result.Rows[0].Witnesses) != 4 || len(result.Rows[1].Witnesses) != 1 {
					t.Fatal("owned literal source scope or transport witnesses differ", external, average, mode, result, err)
				}
				for _, row := range result.Rows {
					for _, witness := range row.Witnesses {
						if len(witness.RawVegRowIDs) == 0 || witness.StageRowID == "" || witness.ConversionLayerID == "" || witness.LayerMapID == "" {
							t.Fatal("transport dropped physical source origin", witness)
						}
					}
				}
				again, err := service.Preview(context.Background(), state.ContextID, average, mode)
				if err != nil || !reflect.DeepEqual(result, again) {
					t.Fatal("repeated owned report differs", err)
				}
				*result.Rows[0].Covers[0].Text = "changed"
				result.Rows[0].Witnesses[0].ReferenceOrigins[0].Table = "changed"
				if *again.Rows[0].Covers[0].Text == "changed" || again.Rows[0].Witnesses[0].ReferenceOrigins[0].Table == "changed" {
					t.Fatal("transport results alias another read")
				}
			}
		}
		assertProfileSUFiles(t, contexts, before)
		after, err := os.ReadFile(contexts.projects.preferences.path)
		if err != nil || !bytes.Equal(config, after) {
			t.Fatal("explicit draft preview changed saved preferences", err)
		}
	}
}

func TestSummaryVegetationServiceOwnershipCancellationCleanupAndRetry(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, true)
	owner := service.contexts.projects.sqlite
	owner.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	result, err := service.Preview(ctx, state.ContextID, 1, 6)
	cancel()
	owner.mu.Unlock()
	if !errors.Is(err, context.DeadlineExceeded) || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
		t.Fatal("cancelled owner lease published", result, err)
	}
	original := owner.attachmentInfo["su"]
	owner.attachmentInfo["su"] = owner.attachmentInfo["project"]
	result, err = service.Preview(context.Background(), state.ContextID, 1, 6)
	if err == nil || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
		t.Fatal("changed file identity published", result, err)
	}
	owner.attachmentInfo["su"] = original
	service.snapshot.commitRead = func(*sql.Tx) error { return errors.New("read completion failed") }
	if result, err := service.Preview(context.Background(), state.ContextID, 1, 6); err == nil || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
		t.Fatal("failed snapshot commit published", result, err)
	}
	service.snapshot.commitRead = nil
	service.snapshot.rollbackRead = func(*sql.Tx) error { return errors.New("read cleanup failed") }
	if result, err := service.Preview(context.Background(), state.ContextID, 1, 6); err == nil || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
		t.Fatal("failed snapshot cleanup published", result, err)
	}
	service.snapshot.rollbackRead = nil
	cancelled, stop := context.WithCancel(context.Background())
	service.snapshot.commitRead = func(*sql.Tx) error { stop(); return nil }
	if result, err := service.Preview(cancelled, state.ContextID, 1, 6); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, SummaryVegetationPreview{}) {
		t.Fatal("in-flight late cancellation published or discarded pinned coordinator", result, err)
	}
	service.snapshot.commitRead = nil
	if _, err := service.Preview(context.Background(), state.ContextID, 1, 6); err != nil {
		t.Fatal("cleanup/cancellation broke next pinned snapshot", err)
	}
	next, err := service.contexts.SwitchContext(state.ContextID, contextSelection(state))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Preview(context.Background(), state.ContextID, 1, 6); err == nil {
		t.Fatal("stale context published")
	}
	if _, err := service.Preview(context.Background(), next.ContextID, 1, 6); err != nil {
		t.Fatal("switched owned retry failed", err)
	}
}

func TestSummaryVegetationServicePhysicalSchemaAndIndependentScope(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, false)
	owner := service.contexts.projects.sqlite
	// QC-disabled raw admission must not depend on Env/Admin or navigation rows.
	mutateContextFixture(t, owner.attachments["project"], `DROP TABLE `+quoteHeaderIdentifier(state.ActiveProject+"_Env")+
		`; DROP TABLE `+quoteHeaderIdentifier(state.ActiveProject+"_Admin"))
	if _, err := service.Preview(context.Background(), state.ContextID, 1, 6); err != nil {
		t.Fatal("borrowed unrelated Env/Admin prerequisite", err)
	}
	verify := func(expect string) {
		t.Helper()
		result, err := service.Preview(context.Background(), state.ContextID, 1, 6)
		if expect == "" && err != nil || expect != "" && (err == nil || !strings.Contains(err.Error(), expect) || !reflect.DeepEqual(result, SummaryVegetationPreview{})) {
			t.Fatal("source refusal/correction differs", expect, result, err)
		}
	}
	mutateContextFixture(t, owner.attachments["VUser"], `ALTER TABLE USysUserSpp RENAME TO MissingPersonal`)
	verify("VUser.USysUserSpp")
	mutateContextFixture(t, owner.attachments["VUser"], `ALTER TABLE MissingPersonal RENAME TO USysUserSpp`)
	verify("")
	mutateContextFixture(t, owner.attachments["VLists"], `UPDATE USysAllSpecs SET ScientificName=CAST(X'FF' AS TEXT) WHERE Code='X'`)
	verify("malformed Unicode")
	mutateContextFixture(t, owner.attachments["VLists"], `UPDATE USysAllSpecs SET ScientificName='Alpha' WHERE Code='X'`)
	verify("")
	mutateContextFixture(t, owner.attachments["VPro64"], `ALTER TABLE LayerCode RENAME COLUMN Layer TO LegacyLayer`)
	verify("Layer")
	mutateContextFixture(t, owner.attachments["VPro64"], `ALTER TABLE LayerCode RENAME COLUMN LegacyLayer TO Layer`)
	verify("")
	owner.selection.SU = "USysSuTableDynamic"
	verify("hierarchy/dynamic-break")
	owner.selection.SU = "None"
	verify("selected normal SU")
	owner.selection.SU = "Report"
	verify("")
	for _, args := range [][2]int{{0, 6}, {3, 6}, {1, 4}, {1, 5}} {
		if _, err := service.Preview(context.Background(), state.ContextID, args[0], args[1]); err == nil {
			t.Fatal("unsupported draft option accepted", args)
		}
	}
	if _, err := service.Preview(nil, state.ContextID, 1, 6); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestSummaryVegetationProductionWireRoundTrip(t *testing.T) {
	service, state := summaryVegetationServiceFixture(t, false)
	result, err := service.Preview(context.Background(), state.ContextID, 1, 6)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var reopened SummaryVegetationPreview
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(result, reopened) {
		t.Fatal("production nullable-array DTO did not strictly roundtrip", err)
	}
	if strings.Contains(string(raw), `"Rows"`) || strings.Contains(string(raw), `"ReferenceOrigins"`) {
		t.Fatal("private capitalized names leaked into wire")
	}
}
