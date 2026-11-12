package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func loadedChildRecord(t *testing.T, s *vprocore.PlotService, kind, plot string) reflect.Value {
	t.Helper()
	var record any
	switch kind {
	case "Veg":
		rows, err := s.ListVegRecords(plot)
		if err != nil || len(rows) != 1 {
			t.Fatalf("list %s: %v %v", kind, rows, err)
		}
		record = &rows[0]
	case "Humus":
		rows, err := s.ListHumusRecords(plot)
		if err != nil || len(rows) != 1 {
			t.Fatalf("list %s: %v %v", kind, rows, err)
		}
		record = &rows[0]
	case "Mineral":
		rows, err := s.ListMineralRecords(plot)
		if err != nil || len(rows) != 1 {
			t.Fatalf("list %s: %v %v", kind, rows, err)
		}
		record = &rows[0]
	case "Other":
		rows, err := s.ListOtherRecords(plot)
		if err != nil || len(rows) != 1 {
			t.Fatalf("list %s: %v %v", kind, rows, err)
		}
		record = &rows[0]
	}
	return reflect.ValueOf(record).Elem()
}

func TestPlotChild_XLStoredBindingCoverage(t *testing.T) {
	forms := map[string]string{
		"SubVegAXL_BC": "Veg", "SubVegCXL": "Veg", "SubVegDXL": "Veg",
		"SubVegAhtXL": "Veg", "SubVegChtXL": "Veg", "USysVegOtherXL": "Veg",
		"SoilHumusXL": "Humus", "SoilMineralXL": "Mineral", "SubOtherXL": "Other",
	}
	var layout struct {
		Forms []struct {
			Name   string `json:"name"`
			Fields []struct {
				Properties map[string]struct {
					Value string `json:"value"`
				} `json:"properties"`
			} `json:"fields"`
		} `json:"forms"`
	}
	data, err := os.ReadFile(filepath.Join("resources", "fs882-xl-layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	s, db := vprotest.ChildFixture(t)
	stored := map[string]map[string]bool{}
	for kind, fields := range vprocore.ChildFields {
		stored[kind] = map[string]bool{"id": true, "plotnumber": true}
		columns, err := vprocore.ChildCapabilities(db, "Sample", kind)
		if err != nil {
			t.Fatal(err)
		}
		value := childRecord(kind, "", 0, false)
		seenMembers := map[string]bool{}
		for _, field := range fields {
			column := strings.ToLower(field.Column)
			if !columns[column] || seenMembers[field.Member] {
				t.Fatalf("unsupported/duplicate allowlist entry: %s.%s", kind, field.Column)
			}
			seenMembers[field.Member] = true
			stored[kind][column] = true
			var typ string
			if err := db.QueryRow(`SELECT type FROM pragma_table_info(?) WHERE lower(name) = ?`, "Sample_"+kind, column).Scan(&typ); err != nil {
				t.Fatal(err)
			}
			member := value.FieldByName(field.Member)
			scalar := member.Kind()
			if scalar == reflect.Pointer {
				scalar = member.Type().Elem().Kind()
			}
			expected := map[string]reflect.Kind{
				"REAL": reflect.Float64, "SMALLINT": reflect.Int, "INTEGER": reflect.Int,
				"VARCHAR": reflect.String, "TEXT": reflect.String, "BOOLEAN": reflect.Bool,
			}[typ]
			if scalar != expected {
				t.Fatalf("DTO type for %s.%s = %v, schema %s requires %v", kind, field.Column, scalar, typ, expected)
			}
		}
		caps, err := s.GetChildCapabilities(kind)
		if err != nil || len(caps) != len(fields)+2 {
			t.Fatalf("DTO capabilities for %s: %v %v", kind, caps, err)
		}
	}
	bound := map[string]map[string]bool{}
	for _, form := range layout.Forms {
		kind, included := forms[form.Name]
		if !included {
			continue
		}
		delete(forms, form.Name)
		if bound[kind] == nil {
			bound[kind] = map[string]bool{}
		}
		for _, field := range form.Fields {
			source := strings.ToLower(strings.TrimSpace(field.Properties["ControlSource"].Value))
			if source == "" {
				continue
			}
			if !stored[kind][source] {
				t.Errorf("unmapped XL source binding %s.%s", form.Name, source)
			}
			if source != "id" && source != "plotnumber" {
				bound[kind][source] = true
			}
		}
	}
	if len(forms) != 0 {
		t.Fatalf("missing source forms: %v", forms)
	}
	for kind, count := range map[string]int{"Veg": 31, "Humus": 12, "Mineral": 18, "Other": 8} {
		if len(bound[kind]) != count {
			t.Fatalf("%s mapped XL stored binding count = %d, want %d", kind, len(bound[kind]), count)
		}
	}
	if _, err := s.GetChildCapabilities("Unknown"); err == nil {
		t.Fatal("unknown child kind advertised as supported")
	}
}

func TestPlotChild_AllFieldTypedListAndJSONRoundtrip(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := vprotest.ChildFixture(t)
			s.SetAuditStrength(3)
			original := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, original); err != nil {
				t.Fatal(err)
			}

			loaded := loadedChildRecord(t, s, kind, "CHILD1")
			id := loaded.FieldByName("ID").Int()
			if !reflect.DeepEqual(childValues(kind, loaded), childValues(kind, original)) {
				t.Fatalf("typed List lost stored fields: %v != %v", childValues(kind, loaded), childValues(kind, original))
			}
			data, err := json.Marshal(loaded.Interface())
			if err != nil {
				t.Fatal(err)
			}
			decoded := childRecord(kind, "", 0, false)
			if err := json.Unmarshal(data, decoded.Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(childValues(kind, decoded), childValues(kind, loaded)) {
				t.Fatal("JSON roundtrip changed integer/float/string/bool types or values")
			}
			clearChildAudit(t, db)
			if err := updateChildRecord(s, decoded); err != nil {
				t.Fatal(err)
			}
			checkChildAudits(t, s, kind, id, nil, nil)
			// The loaded DTO supports direct Go null clearing as well as JSON.
			for _, field := range vprocore.ChildFields[kind] {
				member := loaded.FieldByName(field.Member)
				if member.Kind() == reflect.Pointer {
					member.SetZero()
				}
			}
			if err := updateChildRecord(s, loaded); err != nil {
				t.Fatal(err)
			}
			cleared := loadedChildRecord(t, s, kind, "CHILD1")
			if len(childValues(kind, cleared)) != map[string]int{"Veg": 1, "Humus": 0, "Mineral": 0, "Other": 0}[kind] {
				t.Fatalf("null clearing failed: %v", childValues(kind, cleared))
			}
			before := childValues(kind, decoded)
			expected := map[string]any{}
			for _, field := range vprocore.ChildFields[kind] {
				if loaded.FieldByName(field.Member).Kind() == reflect.Pointer {
					expected[field.Column] = nil
				}
			}
			checkChildAudits(t, s, kind, id, before, expected)
		})
	}
}

func TestPlotChild_LegacyOmissionAndExplicitNull(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := vprotest.ChildFixture(t)
			s.SetAuditStrength(3)
			original := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, original); err != nil {
				t.Fatal(err)
			}
			id := childID(t, db, kind, "CHILD1")
			clearChildAudit(t, db)
			legacy := childRecord(kind, "", 0, false)
			payload := fmt.Sprintf(`{"id":%d,"plotNumber":"CHILD1","species":"PSEUMEN"}`, id)
			if err := json.Unmarshal([]byte(payload), legacy.Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			if err := updateChildRecord(s, legacy); err != nil {
				t.Fatal(err)
			}
			if err := updateChildRecord(s, childRecord(kind, "CHILD1", id, false)); err != nil {
				t.Fatal(err)
			}
			loaded := loadedChildRecord(t, s, kind, "CHILD1")
			for _, field := range vprocore.ChildFields[kind][vprocore.ChildLegacyFields[kind]:] {
				if !reflect.DeepEqual(vprocore.ChildValue(loaded, field), vprocore.ChildValue(original, field)) {
					t.Fatalf("legacy omitted %s was overwritten", field.Column)
				}
			}
			audits, err := s.ListAuditEntries("CHILD1")
			if err != nil {
				t.Fatal(err)
			}
			for _, audit := range audits {
				for _, field := range vprocore.ChildFields[kind][vprocore.ChildLegacyFields[kind]:] {
					if audit.EditField == field.Column {
						t.Fatalf("omitted new field audited: %s", field.Column)
					}
				}
			}
			clearChildAudit(t, db)
			first := vprocore.ChildFields[kind][vprocore.ChildLegacyFields[kind]]
			nullPayload := strings.TrimSuffix(payload, "}") + fmt.Sprintf(`,"%s":null}`, vprocore.ChildJSONKey(legacy, first))
			if err := json.Unmarshal([]byte(nullPayload), legacy.Addr().Interface()); err != nil {
				t.Fatal(err)
			}
			if err := updateChildRecord(s, legacy); err != nil {
				t.Fatal(err)
			}
			checkChildAudits(t, s, kind, id, map[string]any{first.Column: vprocore.ChildValue(original, first)}, map[string]any{first.Column: nil})
			loaded = loadedChildRecord(t, s, kind, "CHILD1")
			if vprocore.ChildValue(loaded, first) != nil {
				t.Fatalf("explicit JSON null did not clear %s", first.Column)
			}
		})
	}
}

func TestPlotChild_UnboundColumnsStayExcluded(t *testing.T) {
	excluded := map[string][]string{
		"Veg":   {"HeightA", "Height5a", "Height5b", "Height5c", "HeightB", "Flag"},
		"Humus": {"Consistence", "Character", "Fauna", "Flag"},
		"Mineral": {"MottlesAbundance", "MottlesSize", "MottlesContrast", "ClayFilmsFreq",
			"ClayFilmThickness", "Effervescence", "Porosity", "Flag"},
		"Other": {"Flag"},
	}
	for kind, columns := range excluded {
		t.Run(kind, func(t *testing.T) {
			s, db := vprotest.ChildFixture(t)
			s.SetAuditStrength(3)
			record := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			record.FieldByName("ID").SetInt(childID(t, db, kind, "CHILD1"))
			want := map[string]any{}
			for _, column := range columns {
				var typ string
				if err := db.QueryRow(`SELECT type FROM pragma_table_info(?) WHERE lower(name) = ?`, "Sample_"+kind, strings.ToLower(column)).Scan(&typ); err != nil {
					t.Fatal(err)
				}
				var value any = "unbound-'source'"
				switch typ {
				case "REAL":
					value = 123.75
				case "BOOLEAN":
					value = true
				}
				if _, err := db.Exec(`UPDATE "Sample_`+kind+`" SET `+vprocore.QuoteHeaderIdentifier(column)+` = ? WHERE PlotNumber = 'CHILD1'`, value); err != nil {
					t.Fatal(err)
				}
				want[column] = value
			}
			clearChildAudit(t, db)
			for _, field := range vprocore.ChildFields[kind] {
				editChildMember(record.FieldByName(field.Member), kind, field)
			}
			if err := updateChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			for column, expected := range want {
				var stored any
				if err := db.QueryRow(`SELECT ` + vprocore.QuoteHeaderIdentifier(column) + ` FROM "Sample_` + kind + `" WHERE PlotNumber = 'CHILD1'`).Scan(&stored); err != nil ||
					!reflect.DeepEqual(stored, expected) {
					t.Fatalf("unbound %s was changed: %v != %v; %v", column, stored, expected, err)
				}
			}
			audits, err := s.ListAuditEntries("CHILD1")
			if err != nil {
				t.Fatal(err)
			}
			for _, audit := range audits {
				if _, excluded := want[audit.EditField]; excluded {
					t.Fatalf("unbound field audited: %s", audit.EditField)
				}
			}
		})
	}
}

func TestPlotChild_StrictJSONScalarTypes(t *testing.T) {
	for _, test := range []struct{ kind, key, invalid string }{
		{"Veg", "cultural1", "1.5"}, {"Humus", "vonPost", `"2"`},
		{"Mineral", "percentCoarseFragsGravel", "2.5"},
		{"Other", "userFlag1", "1"}, {"Other", "userFlag2", `"false"`},
	} {
		t.Run(test.kind+"/"+test.key, func(t *testing.T) {
			record := childRecord(test.kind, "", 0, false)
			payload := fmt.Sprintf(`{"%s":%s}`, test.key, test.invalid)
			if err := json.Unmarshal([]byte(payload), record.Addr().Interface()); err == nil {
				t.Fatalf("invalid scalar silently coerced: %s", payload)
			}
		})
	}
}

func TestPlotChild_MissingStoredColumnUnsupported(t *testing.T) {
	for _, kind := range []string{"Veg", "Humus", "Mineral", "Other"} {
		t.Run(kind, func(t *testing.T) {
			s, db := vprotest.ChildFixture(t)
			record := childRecord(kind, "CHILD1", 0, true)
			if err := saveChildRecord(s, record); err != nil {
				t.Fatal(err)
			}
			id := childID(t, db, kind, "CHILD1")
			record.FieldByName("ID").SetInt(id)
			missing := vprocore.ChildFields[kind][vprocore.ChildLegacyFields[kind]]
			if _, err := db.Exec(`ALTER TABLE "Sample_` + kind + `" DROP COLUMN ` + vprocore.QuoteHeaderIdentifier(missing.Column)); err != nil {
				t.Fatal(err)
			}
			caps, err := s.GetChildCapabilities(kind)
			if err != nil || caps[vprocore.ChildJSONKey(record, missing)] {
				t.Fatalf("missing column advertised as supported: %v %v", caps, err)
			}
			initialAudit := vprotest.AuditCount(t, db, "CHILD1")
			if err := updateChildRecord(s, record); err == nil || !strings.Contains(err.Error(), "unsupported") ||
				!strings.Contains(err.Error(), missing.Column) {
				t.Fatalf("missing column not explicitly rejected: %v", err)
			}
			if vprotest.AuditCount(t, db, "CHILD1") != initialAudit || childCount(t, db, kind, "CHILD1") != 1 {
				t.Fatal("unsupported property changed row/audit")
			}
			loaded := loadedChildRecord(t, s, kind, "CHILD1")
			if vprocore.ChildValue(loaded, missing) != nil {
				t.Fatal("missing field synthesized a value")
			}
			if err := updateChildRecord(s, loaded); err != nil {
				t.Fatalf("supported fields cannot be saved when a new column is missing: %v", err)
			}
		})
	}
}
