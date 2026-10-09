package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPlotHeader_SourceBindingsAndTypes(t *testing.T) {
	data, err := os.ReadFile("resources/fs882-xl-layout.json")
	if err != nil {
		t.Fatal(err)
	}
	var layout struct {
		Forms []struct {
			Name     string
			Controls []struct {
				Properties map[string]struct {
					Value string
				}
			}
		}
	}
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	bound := map[string]bool{}
	for _, form := range layout.Forms {
		if form.Name != "FS882-6x4XL" {
			continue
		}
		for _, control := range form.Controls {
			source := control.Properties["ControlSource"].Value
			if source != "" && !strings.HasPrefix(source, "=") {
				bound[source] = true
			}
		}
	}
	if len(bound) != 98 || len(headerFields) != 99 {
		t.Fatalf("bound columns %d, mappings %d; want 98 stored plus runtime lock", len(bound), len(headerFields))
	}
	_, db := headerFixture(t)
	schemas := map[string]map[string]string{}
	for _, table := range []string{"Env", "Admin"} {
		rows, err := db.Query(`PRAGMA table_info(Sample_` + table + `)`)
		if err != nil {
			t.Fatal(err)
		}
		schemas[table] = map[string]string{}
		for rows.Next() {
			var cid, required, pk int
			var column, typ string
			var defaultValue any
			if err := rows.Scan(&cid, &column, &typ, &required, &defaultValue, &pk); err != nil {
				t.Fatal(err)
			}
			schemas[table][column] = typ
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
	properties, members, mapped := map[string]bool{}, map[string]bool{}, map[string]bool{}
	counts := map[string]int{}
	types := map[reflect.Kind]int{}
	for _, field := range headerFields {
		if properties[field.property] || members[field.member] {
			t.Fatalf("duplicate DTO mapping: %#v", field)
		}
		properties[field.property], members[field.member] = true, true
		member, ok := reflect.TypeOf(FS882Header{}).FieldByName(field.member)
		if !ok || member.Tag.Get("json") != field.property {
			t.Fatalf("invalid DTO mapping: %#v", field)
		}
		if field.property == "locked" {
			if field.column != "" || field.table != "" || member.Type.Kind() != reflect.Bool {
				t.Fatal("lock must remain runtime-only")
			}
			continue
		}
		if !bound[field.column] || mapped[field.column] {
			t.Fatalf("unbound or duplicate source mapping: %#v", field)
		}
		mapped[field.column] = true
		typ, exists := schemas[field.table][field.column]
		if !exists {
			t.Fatalf("missing canonical schema column: %#v", field)
		}
		want := reflect.String
		switch typ {
		case "REAL", "DOUBLE":
			want = reflect.Float64
		case "SMALLINT":
			want = reflect.Int
		case "BOOLEAN":
			want = reflect.Bool
		case "VARCHAR", "TEXT", "TIMESTAMP":
		default:
			t.Fatalf("unverified type %s for %#v", typ, field)
		}
		got := member.Type.Kind()
		if field.property != "plotNumber" {
			if got != reflect.Pointer {
				t.Fatalf("stored field must be nullable: %#v", field)
			}
			got = member.Type.Elem().Kind()
		}
		if got != want {
			t.Fatalf("%s is %s, schema %s requires %s", field.property, got, typ, want)
		}
		counts[field.table]++
		types[got]++
	}
	if !reflect.DeepEqual(mapped, bound) {
		t.Fatal("parent bindings and stored mappings differ")
	}
	t.Logf("verified mappings: tables=%v types=%v", counts, types)
}

func TestPlotHeader_AllStoredColumnsAndAuditValues(t *testing.T) {
	s, db := headerFixture(t)
	h := fullHeader("HDRCOLS")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	for _, field := range headerFields {
		if field.column == "" {
			continue
		}
		key := "PlotNumber"
		if field.table == "Admin" {
			key = "Plot"
		}
		member := reflect.ValueOf(h).FieldByName(field.member)
		column := quoteHeaderIdentifier(field.column)
		if member.Kind() == reflect.Pointer && member.Type().Elem().Kind() == reflect.Bool {
			column = "CAST(" + column + " AS INTEGER)"
		}
		var stored string
		query := `SELECT CAST(` + column + ` AS TEXT) FROM Sample_` + field.table +
			` WHERE ` + quoteHeaderIdentifier(key) + ` = ?`
		if err := db.QueryRow(query, h.PlotNumber).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		want := headerAuditValue(headerValue(h, field))
		if stored != want {
			t.Errorf("%s.%s stored %q, want %v", field.table, field.column, stored, want)
		}
		if field.property == "plotNumber" {
			continue
		}
		var after string
		var count int
		if err := db.QueryRow(`SELECT COUNT(*), AfterEdit FROM Sample_Audit
			WHERE PlotNumber = ? AND "Table" = ? AND EditField = ? AND BeforeEdit IS NULL`,
			h.PlotNumber, "_"+field.table, field.column).Scan(&count, &after); err != nil {
			t.Fatal(err)
		}
		if count != 1 || after != want {
			t.Errorf("%s audit count %d after %q, want %v", field.property, count, after, want)
		}
	}
}

func TestPlotHeader_NullableBooleans(t *testing.T) {
	for _, field := range []headerField{
		{"speciesListComplete", "SpeciesListComplete", "Env", "SpeciesListComplete"},
		{"updatedFromCards", "UpdatedFromCards", "Admin", "UpdatedFromCards"},
	} {
		t.Run(field.property, func(t *testing.T) {
			s, db := headerFixture(t)
			h := FS882Header{PlotNumber: "HDRBOOL"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			for i, value := range []*bool{boolPointer(false), boolPointer(true), boolPointer(true), boolPointer(false), nil} {
				reflect.ValueOf(&h).Elem().FieldByName(field.member).Set(reflect.ValueOf(value))
				if err := s.UpdatePlot(h); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil || !reflect.DeepEqual(*got, h) {
					t.Fatalf("step %d: %#v %v", i, got, err)
				}
				data, err := json.Marshal(got)
				if err != nil {
					t.Fatal(err)
				}
				var decoded map[string]any
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				var want any
				if value != nil {
					want = *value
				}
				if decoded[field.property] != want {
					t.Fatalf("boolean JSON value %v, want %v", decoded[field.property], want)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 4 {
				t.Fatalf("boolean audit count %d, want 4", len(entries))
			}
			var changes [][2]any
			for _, entry := range entries {
				var before, after any
				if entry.BeforeEdit != nil {
					before = *entry.BeforeEdit
				}
				if entry.AfterEdit != nil {
					after = *entry.AfterEdit
				}
				changes = append(changes, [2]any{before, after})
			}
			for _, want := range [][2]any{{nil, "0"}, {"0", "-1"}, {"-1", "0"}, {"0", nil}} {
				found := false
				for _, change := range changes {
					found = found || reflect.DeepEqual(change, want)
				}
				if !found {
					t.Fatalf("missing boolean audit transition %v: %v", want, changes)
				}
			}
			key := "PlotNumber"
			if field.table == "Admin" {
				key = "Plot"
			}
			for _, stored := range []int{-1, 1, 0} {
				if _, err := db.Exec(`UPDATE Sample_`+field.table+` SET `+quoteHeaderIdentifier(field.column)+
					` = ? WHERE `+quoteHeaderIdentifier(key)+` = ?`, stored, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if actual := headerValue(*got, field); actual != (stored != 0) {
					t.Fatalf("stored %d read as %v", stored, actual)
				}
				count := auditCount(t, db, h.PlotNumber)
				if err := s.UpdatePlot(*got); err != nil {
					t.Fatal(err)
				}
				if auditCount(t, db, h.PlotNumber) != count {
					t.Fatal("equivalent boolean encodings generated an audit")
				}
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

func TestPlotHeader_BooleanAuditStrengths(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		s, db := headerFixture(t)
		if err := s.SetAuditStrength(strength); err != nil {
			t.Fatal(err)
		}
		h := FS882Header{PlotNumber: "HDRBSTR"}
		want := 0
		for i, value := range []*bool{nil, boolPointer(false), boolPointer(true), nil} {
			h.SpeciesListComplete, h.UpdatedFromCards = value, value
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if (i == 1 && strength >= 2) || (i == 2 && strength >= 1) || (i == 3 && strength == 3) {
				want += 2
			}
			if got := auditCount(t, db, h.PlotNumber); got != want {
				t.Fatalf("strength %d step %d: audit count %d, want %d", strength, i, got, want)
			}
		}
	}
}

func TestPlotHeader_NewFieldsAuditUpdateRollback(t *testing.T) {
	s, db := headerFixture(t)
	h := fullHeader("HDRAUDR")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := auditCount(t, db, h.PlotNumber)
	if _, err := db.Exec(`CREATE TRIGGER fail_audit BEFORE INSERT ON Sample_Audit
		BEGIN SELECT RAISE(ABORT, 'audit insert failure'); END`); err != nil {
		t.Fatal(err)
	}
	next := fullHeader(h.PlotNumber)
	next.SpeciesListComplete, next.UpdatedFromCards = boolPointer(false), boolPointer(false)
	next.SoilNotes, next.HumusThickness = nil, nil
	if err := s.UpdatePlot(next); err == nil {
		t.Fatal("expected audit failure")
	}
	got, err := s.GetPlot(h.PlotNumber)
	if err != nil || !reflect.DeepEqual(*got, h) || auditCount(t, db, h.PlotNumber) != before {
		t.Fatalf("failed audit left partial new fields: %#v %v", got, err)
	}
}
