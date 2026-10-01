package main

import (
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
	matches, err := svc.SearchSpecies("ABIELAS", 10)
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
	sp, err := svc.GetSpecies("ABIELAS")
	if err != nil {
		t.Fatalf("GetSpecies failed: %v", err)
	}
	if sp == nil || sp.Code != "ABIELAS" {
		t.Fatalf("expected ABIELAS, got %v", sp)
	}

	// Test 3: List items (MoistureRegime)
	items, err := svc.GetListItems("MoistureRegime")
	if err != nil {
		t.Fatalf("GetListItems failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("expected MoistureRegime list items, got none")
	}

	// Test 4: GetAllLists
	all, err := svc.GetAllLists()
	if err != nil {
		t.Fatalf("GetAllLists failed: %v", err)
	}
	if len(all) == 0 {
		t.Fatalf("expected non-empty lists map")
	}
	if _, ok := all["MoistureRegime"]; !ok {
		t.Errorf("expected MoistureRegime in GetAllLists")
	}
}
