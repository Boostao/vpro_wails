package references

import (
	"context"
	"os"
	"testing"
)

func TestReferenceService_SpeciesAndLists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "vpro-ref-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	svc, err := NewReferenceService(tempDir)
	if err != nil {
		t.Fatalf("NewReferenceService failed: %v", err)
	}
	defer svc.Close()

	// Test 1: Species search
	matches, err := svc.SearchSpecies(context.Background(), "ABIELAS", 10)
	if err != nil {
		t.Fatalf("SearchSpecies failed: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected species matches for 'ABIELAS', got none")
	}

	foundAbielas := false
	for _, m := range matches {
		if m.Code == "ABIELAS" {
			foundAbielas = true
			if m.ScientificName == "" {
				t.Errorf("expected ScientificName for ABIELAS, got empty")
			}
		}
	}
	if !foundAbielas {
		t.Errorf("expected ABIELAS in search results")
	}

	// Test 2: Get single species
	sp, err := svc.GetSpecies(context.Background(), "ABIELAS")
	if err != nil {
		t.Fatalf("GetSpecies failed: %v", err)
	}
	if sp == nil || sp.Code != "ABIELAS" {
		t.Fatalf("expected ABIELAS, got %v", sp)
	}

	// Test 3: List items (MoistureRegime)
	items, err := svc.GetListItems(context.Background(), "MoistureRegime")
	if err != nil {
		t.Fatalf("GetListItems failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected MoistureRegime list items, got none")
	}

	// Test 4: GetAllLists
	all, err := svc.GetAllLists(context.Background())
	if err != nil {
		t.Fatalf("GetAllLists failed: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("expected non-empty lists map")
	}
	if _, ok := all["MoistureRegime"]; !ok {
		t.Errorf("expected MoistureRegime in GetAllLists")
	}

	// Test 5: 2025/2026 Taxonomy & LMH25 (2023) vocabulary extensions
	for _, code := range []string{"CALLNOO", "PINUALB", "PINUCON1", "POPUHAS", "CORNNUT", "BREWBRE", "ACAUTRI"} {
		sp2026, err := svc.GetSpecies(context.Background(), code)
		if err != nil || sp2026 == nil || sp2026.Code != code {
			t.Fatalf("expected 2025/2026 taxon %s in vlists.db, got %+v (%v)", code, sp2026, err)
		}
	}
	for _, expected := range []struct{ list, item string }{
		{"StructuralStage", "3c"},
		{"StructuralStage", "3d"},
		{"SiviStructuralStage", "3c"},
		{"SiviStructuralStage", "3d"},
		{"StructuralStageMod", "v"},
		{"StructuralStageMod", "k"},
		{"SiteDisturbance", "A.w.th"},
		{"SiteDisturbance", "A.w.br"},
		{"Exposure", "CO"},
		{"FloodingRegimeDur", "S"},
		{"FloodingRegimeFreq", "S"},
	} {
		listItems := all[expected.list]
		found := false
		for _, it := range listItems {
			if it.Item == expected.item {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected LMH25 2023 item %s in list %s", expected.item, expected.list)
		}
	}

	// Test 6: Truncated/corrupt vlists.db is atomically repaired on open
	_ = svc.Close()
	if err := os.WriteFile(tempDir+"/vlists.db", []byte("truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	repaired, err := NewReferenceService(tempDir)
	if err != nil {
		t.Fatalf("NewReferenceService failed to repair truncated vlists.db: %v", err)
	}
	defer repaired.Close()
	spRepaired, err := repaired.GetSpecies(context.Background(), "ABIELAS")
	if err != nil || spRepaired == nil || spRepaired.Code != "ABIELAS" {
		t.Fatalf("repaired vlists.db failed species lookup: %+v (%v)", spRepaired, err)
	}
}
