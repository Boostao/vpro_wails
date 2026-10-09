package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestExtendedShrubsPackagedSourceAndAuthorizedNumericFields(t *testing.T) {
	data, err := os.ReadFile("resources/fs882-extended-shrub-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var layout fs882layout.Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	if layout.Root != "SubVegAXL" || len(layout.Forms) != 1 || layout.Forms[0].RecordSource != "USysVegA" {
		t.Fatal("extended child lost its original source")
	}
	var fields, allowed []string
	for _, field := range layout.Forms[0].Fields {
		if strings.HasPrefix(strings.ToLower(field.Binding), "cover") || strings.HasPrefix(strings.ToLower(field.Binding), "total") {
			property := strings.ToLower(field.Binding[:1]) + field.Binding[1:]
			fields = append(fields, property)
			if strings.HasPrefix(field.Binding, "Cover5") && len(field.Binding) == 7 &&
				field.Properties["ValidationRule"].Value != "<100 Or Is Null" {
				t.Fatal("extended shrub source validation differs", field)
			}
		}
	}
	for property := range vegetationNumberForms["SubVegAXL"] {
		allowed = append(allowed, property)
	}
	sort.Strings(fields)
	sort.Strings(allowed)
	if len(fields) != 10 || !reflect.DeepEqual(fields, allowed) {
		t.Fatal("source/authorization mismatch", fields, allowed)
	}
}

func TestExtendedShrubsOnlyMembershipAtomicAuditRollbackAndIndependentTotals(t *testing.T) {
	service, db := childFixture(t)
	service.SetAuditStrength(3)
	if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover5a,TotalB)
		VALUES ('CHILD1','RAW',0,0,NULL),('CHILD1','RAW',-9,NULL,NULL);
		UPDATE Sample_Veg SET Cover5b=120 WHERE ID=0;
		CREATE TRIGGER protect_extended_historical BEFORE UPDATE OF Cover5b ON Sample_Veg
		BEGIN SELECT RAISE(ABORT,'historical reassigned'); END;
		CREATE TRIGGER reject_extended_audit BEFORE INSERT ON Sample_Audit WHEN NEW.EditField='Cover5c'
		BEGIN SELECT RAISE(ABORT,'extended audit rejected'); END`); err != nil {
		t.Fatal(err)
	}
	number := func(value float64) *float64 { return &value }
	update := VegetationNumberUpdate{ID: 0,
		Values:   map[string]*float64{"cover5a": number(-1.234567890123), "cover5b": number(120), "cover5c": number(99.999)},
		Expected: map[string]*float64{"cover5a": number(0), "cover5b": number(120), "cover5c": nil},
		Forms:    map[string]string{"cover5a": "SubVegAXL", "cover5b": "SubVegAXL", "cover5c": "SubVegAXL"}}
	before := substrateSnapshot(t, db)
	if err := service.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{update}); err == nil {
		t.Fatal("late extended audit rejection committed")
	}
	assertSubstrateSnapshot(t, db, before)
	if _, err := db.Exec("DROP TRIGGER reject_extended_audit"); err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{update}); err != nil {
		t.Fatal("retained extended retry/historical omission failed", err)
	}
	var a, b, c float64
	var total any
	if err := db.QueryRow("SELECT Cover5a,Cover5b,Cover5c,TotalB FROM Sample_Veg WHERE ID=0").Scan(&a, &b, &c, &total); err != nil ||
		a != -1.234567890123 || b != 120 || c != 99.999 || total != nil || auditCount(t, db, "CHILD1") != 2 {
		t.Fatal("precision, independent total, historical value or exact audits changed", a, b, c, total, err)
	}
	for _, form := range []string{"SubVegAXL_BC", "SubVegAXL"} {
		predicate, err := vegetationSpeciesRowPredicate(form)
		if err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow("SELECT count(*) FROM Sample_Veg WHERE PlotNumber='CHILD1' AND (" + predicate + ")").Scan(&count); err != nil || count != 1 {
			t.Fatal("extended-only row vanished from shared A membership", form, count, err)
		}
	}
	before = substrateSnapshot(t, db)
	for _, request := range []VegetationNumberUpdate{
		{ID: -9, Values: map[string]*float64{"cover5a": number(0)}, Expected: map[string]*float64{"cover5a": nil}, Forms: map[string]string{"cover5a": "SubVegAXL"}},
		{ID: 0, Values: map[string]*float64{"cover5c": number(100)}, Expected: map[string]*float64{"cover5c": number(99.999)}, Forms: map[string]string{"cover5c": "SubVegAXL"}},
		{ID: 0, Values: map[string]*float64{"cover5c": nil}, Expected: map[string]*float64{"cover5c": number(99.999)}, Forms: map[string]string{"cover5c": "SubVegAhtXL"}},
	} {
		if err := service.UpdateVegetationNumbers("CHILD1", []VegetationNumberUpdate{request}); err == nil {
			t.Fatal("unavailable source/value/membership accepted", request)
		}
		assertSubstrateSnapshot(t, db, before)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	_, err = inspectVegetationDeletion(context.Background(), tx, `"Sample_Veg"`, "CHILD1", "SubVegAXL", 0)
	tx.Rollback()
	if err != nil {
		t.Fatal("extended source lost existing reviewed deletion boundary", err)
	}
}

func TestExtendedShrubsCreationDeletionIdentityAndScope(t *testing.T) {
	service, db := childFixture(t)
	service.SetAuditStrength(3)
	if _, err := db.Exec(`CREATE TABLE USysAllSpecs(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		CREATE TABLE USysUserSpp(Code TEXT,ScientificName TEXT,Lifeform INTEGER,EnglishName TEXT,Codetype TEXT);
		INSERT INTO USysAllSpecs VALUES ('A',NULL,3,NULL,'U');
		DELETE FROM Sample_Veg`); err != nil {
		t.Fatal(err)
	}
	zero := 0.0
	request := VegetationCreationRequest{Form: "SubVegAXL", Species: "A", Values: map[string]*float64{"cover5a": &zero, "cover5b": nil, "cover5c": nil}}
	id, err := service.CreateSourceVegetation("CHILD1", request)
	if err != nil || id != 1 {
		t.Fatal("extended-only zero creation failed", id, err)
	}
	var cover, total any
	if err := db.QueryRow("SELECT Cover1,TotalB FROM Sample_Veg WHERE ID=?", id).Scan(&cover, &total); err != nil || cover != nil || total != nil {
		t.Fatal("extended creation invented standard cover/total", cover, total, err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	review, err := inspectVegetationDeletion(context.Background(), tx, `"Sample_Veg"`, "CHILD1", request.Form, id)
	tx.Rollback()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteReviewedVegetation("CHILD1", VegetationDeletionRequest{ID: id, Form: request.Form, Expected: review.review.Expected}); err != nil {
		t.Fatal(err)
	}
	id, err = service.CreateSourceVegetation("CHILD1", request)
	if err != nil || id != 2 {
		t.Fatal("extended creation reused deleted/reserved identity", id, err)
	}
}
