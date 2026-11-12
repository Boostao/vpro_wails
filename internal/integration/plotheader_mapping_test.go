package integration

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

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
	arcticExtended := map[string]bool{
		"SnowCoverregime":  true,
		"ActiveLayerDepth": true,
	}
	if len(bound) != 98 || len(vprocore.HeaderFields) != 101 {
		t.Fatalf("bound columns %d, mappings %d; want 98 FS882-6x4XL + 2 Arctic extended + runtime lock", len(bound), len(vprocore.HeaderFields))
	}
	_, db := vprotest.HeaderFixture(t)
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
	for _, field := range vprocore.HeaderFields {
		if properties[field.Property] || members[field.Member] {
			t.Fatalf("duplicate DTO mapping: %#v", field)
		}
		properties[field.Property], members[field.Member] = true, true
		member, ok := reflect.TypeOf(vprocore.FS882Header{}).FieldByName(field.Member)
		if !ok || member.Tag.Get("json") != field.Property {
			t.Fatalf("invalid DTO mapping: %#v", field)
		}
		if field.Property == "locked" {
			if field.Column != "" || field.Table != "" || member.Type.Kind() != reflect.Bool {
				t.Fatal("lock must remain runtime-only")
			}
			continue
		}
		if (!bound[field.Column] && !arcticExtended[field.Column]) || mapped[field.Column] {
			t.Fatalf("unbound or duplicate source mapping: %#v", field)
		}
		mapped[field.Column] = true
		typ, exists := schemas[field.Table][field.Column]
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
		if field.Property != "plotNumber" {
			if got != reflect.Pointer {
				t.Fatalf("stored field must be nullable: %#v", field)
			}
			got = member.Type.Elem().Kind()
		}
		if got != want {
			t.Fatalf("%s is %s, schema %s requires %s", field.Property, got, typ, want)
		}
		counts[field.Table]++
		types[got]++
	}
	expectedMapped := map[string]bool{}
	for k := range bound {
		expectedMapped[k] = true
	}
	for k := range arcticExtended {
		expectedMapped[k] = true
	}
	if !reflect.DeepEqual(mapped, expectedMapped) {
		t.Fatal("parent bindings and stored mappings differ")
	}
	t.Logf("verified mappings: tables=%v types=%v", counts, types)
}

func TestPlotHeader_AllStoredColumnsAndAuditValues(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := fullHeader("HDRCOLS")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	for _, field := range vprocore.HeaderFields {
		if field.Column == "" {
			continue
		}
		key := "PlotNumber"
		if field.Table == "Admin" {
			key = "Plot"
		}
		member := reflect.ValueOf(h).FieldByName(field.Member)
		column := vprocore.QuoteHeaderIdentifier(field.Column)
		if member.Kind() == reflect.Pointer && member.Type().Elem().Kind() == reflect.Bool {
			column = "CAST(" + column + " AS INTEGER)"
		}
		var stored string
		query := `SELECT CAST(` + column + ` AS TEXT) FROM Sample_` + field.Table +
			` WHERE ` + vprocore.QuoteHeaderIdentifier(key) + ` = ?`
		if err := db.QueryRow(query, h.PlotNumber).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		want := vprocore.HeaderAuditValue(vprocore.HeaderValue(h, field))
		if stored != want {
			t.Errorf("%s.%s stored %q, want %v", field.Table, field.Column, stored, want)
		}
		if field.Property == "plotNumber" {
			continue
		}
		var after string
		var count int
		if err := db.QueryRow(`SELECT COUNT(*), AfterEdit FROM Sample_Audit
			WHERE PlotNumber = ? AND "Table" = ? AND EditField = ? AND BeforeEdit IS NULL`,
			h.PlotNumber, "_"+field.Table, field.Column).Scan(&count, &after); err != nil {
			t.Fatal(err)
		}
		if count != 1 || after != want {
			t.Errorf("%s audit count %d after %q, want %v", field.Property, count, after, want)
		}
	}
}

func TestPlotHeader_NullableBooleans(t *testing.T) {
	for _, field := range []vprocore.HeaderField{
		{Property: "speciesListComplete", Member: "SpeciesListComplete", Table: "Env", Column: "SpeciesListComplete"},
		{Property: "updatedFromCards", Member: "UpdatedFromCards", Table: "Admin", Column: "UpdatedFromCards"},
	} {
		t.Run(field.Property, func(t *testing.T) {
			s, db := vprotest.HeaderFixture(t)
			h := vprocore.FS882Header{PlotNumber: "HDRBOOL"}
			if err := s.CreatePlot(h); err != nil {
				t.Fatal(err)
			}
			for i, value := range []*bool{boolPointer(false), boolPointer(true), boolPointer(true), boolPointer(false), nil} {
				reflect.ValueOf(&h).Elem().FieldByName(field.Member).Set(reflect.ValueOf(value))
				if field.Member == "SpeciesListComplete" && value == nil {
					before := vprotest.SubstrateSnapshot(t, db)
					if err := s.UpdatePlot(h); err == nil || !strings.Contains(err.Error(), "select No explicitly") {
						t.Fatalf("step %d accepted new Species NULL: %v", i, err)
					}
					vprotest.AssertSubstrateSnapshot(t, db, before)
					continue
				}
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
				if decoded[field.Property] != want {
					t.Fatalf("boolean JSON value %v, want %v", decoded[field.Property], want)
				}
			}
			entries, err := s.ListAuditEntries(h.PlotNumber)
			if err != nil {
				t.Fatal(err)
			}
			wantChanges := [][2]any{{nil, "0"}, {"0", "-1"}, {"-1", "0"}}
			if field.Member == "UpdatedFromCards" {
				wantChanges = append(wantChanges, [2]any{"0", nil})
			}
			if len(entries) != len(wantChanges) {
				t.Fatalf("boolean audit count %d, want %d", len(entries), len(wantChanges))
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
			for _, want := range wantChanges {
				found := false
				for _, change := range changes {
					found = found || reflect.DeepEqual(change, want)
				}
				if !found {
					t.Fatalf("missing boolean audit transition %v: %v", want, changes)
				}
			}
			key := "PlotNumber"
			if field.Table == "Admin" {
				key = "Plot"
			}
			for _, stored := range []int{-1, 1, 0} {
				if _, err := db.Exec(`UPDATE Sample_`+field.Table+` SET `+vprocore.QuoteHeaderIdentifier(field.Column)+
					` = ? WHERE `+vprocore.QuoteHeaderIdentifier(key)+` = ?`, stored, h.PlotNumber); err != nil {
					t.Fatal(err)
				}
				got, err := s.GetPlot(h.PlotNumber)
				if err != nil {
					t.Fatal(err)
				}
				if actual := vprocore.HeaderValue(*got, field); actual != (stored != 0) {
					t.Fatalf("stored %d read as %v", stored, actual)
				}
				count := vprotest.AuditCount(t, db, h.PlotNumber)
				if err := s.UpdatePlot(*got); err != nil {
					t.Fatal(err)
				}
				if vprotest.AuditCount(t, db, h.PlotNumber) != count {
					t.Fatal("equivalent boolean encodings generated an audit")
				}
			}
		})
	}
}

func boolPointer(value bool) *bool { return &value }

func TestPlotHeader_BooleanAuditStrengths(t *testing.T) {
	for strength := 0; strength <= 3; strength++ {
		s, db := vprotest.HeaderFixture(t)
		if err := s.SetAuditStrength(strength); err != nil {
			t.Fatal(err)
		}
		h := vprocore.FS882Header{PlotNumber: "HDRBSTR"}
		want := 0
		for i, value := range []*bool{nil, boolPointer(false), boolPointer(true), nil} {
			h.SpeciesListComplete, h.UpdatedFromCards = value, value
			if i == 3 {
				before := vprotest.SubstrateSnapshot(t, db)
				if err := s.SavePlot(h); err == nil || !strings.Contains(err.Error(), "select No explicitly") {
					t.Fatalf("strength %d accepted mixed new Species NULL: %v", strength, err)
				}
				vprotest.AssertSubstrateSnapshot(t, db, before)
				h.SpeciesListComplete = boolPointer(false)
			}
			if err := s.SavePlot(h); err != nil {
				t.Fatal(err)
			}
			if (i == 1 && strength >= 2) || (i == 2 && strength >= 1) {
				want += 2
			}
			if i == 3 && strength >= 1 {
				want++
				if strength == 3 {
					want++
				}
			}
			if got := vprotest.AuditCount(t, db, h.PlotNumber); got != want {
				t.Fatalf("strength %d step %d: audit count %d, want %d", strength, i, got, want)
			}
		}
	}
}

func TestPlotHeader_NewFieldsAuditUpdateRollback(t *testing.T) {
	s, db := vprotest.HeaderFixture(t)
	h := fullHeader("HDRAUDR")
	if err := s.CreatePlot(h); err != nil {
		t.Fatal(err)
	}
	before := vprotest.AuditCount(t, db, h.PlotNumber)
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
	if err != nil || !reflect.DeepEqual(*got, h) || vprotest.AuditCount(t, db, h.PlotNumber) != before {
		t.Fatalf("failed audit left partial new fields: %#v %v", got, err)
	}
}
