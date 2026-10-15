package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTwoPageSourceAuditProductionWireFixture(t *testing.T) {
	service, state, db := sourceAuditFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Audit VALUES
		('Sample','source user','108050','_Veg','Species',NULL,NULL,'',-1,0,NULL)`); err != nil {
		t.Fatal(err)
	}
	got, err := service.GetOriginal(context.Background(), state.ContextID, "108050", "FS882-8x6XL")
	if err != nil {
		t.Fatal(err)
	}
	got.ContextID = "wire-owned"
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("frontend", "src", "twoPageAuditWire.json"))
	if err != nil {
		t.Fatal(err)
	}
	var expected, actual any
	if err := json.Unmarshal(fixture, &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("actual Go service wire differs from shared frontend fixture:\n%s", encoded)
	}
}
