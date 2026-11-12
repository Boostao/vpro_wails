package integration

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sourceform/sftest"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSIVISpeciesListedDecisionsUnicodeAndDomains(t *testing.T) {
	ctx := context.Background()
	refs := sftest.SiviSpeciesTestReferences(t)
	text := func(s string) *string { return &s }
	for _, test := range []struct {
		form, value, decision string
		entered, selected     *string
		valid                 bool
	}{
		{"SubVegA-SIVI_BC", "A", "", nil, nil, true},
		{"SubVegA-SIVI", "A", "", nil, nil, true},
		{"SubVegC-SIVI", "C", "", nil, nil, true},
		{"SubVegD-SIVI", "D", "", nil, nil, true},
		{"SubVegC-SIVI", "A", "", nil, nil, false},
		{"SubVegA-SIVI", "C", "", nil, nil, false},
		{"SubVegD-SIVI", "C", "", nil, nil, false},
		{"SubVegAXL", "A", "", nil, nil, false},
		{"SubVegA-SIVI_BC", "é😀😀😀", "", nil, nil, true},
		{"SubVegA-SIVI_BC", "éé😀😀😀", "", nil, nil, true},
		{"SubVegA-SIVI_BC", "é😀😀😀😀", "", nil, nil, false},
		{"SubVegC-SIVI", "OLDDUP", "keep", text("olddup"), nil, true},
		{"SubVegC-SIVI", "NEW_A", "replace", text("olddup"), text("new_a"), true},
		{"SubVegD-SIVI", "NEW_B", "replace", text("OLDDUP"), text("NEW_B"), true},
		{"SubVegA-SIVI", "PERSONAL", "user", text("personal"), text("Personal"), true},
		{"SubVegA-SIVI", "OLDDUP", "user", text("olddup"), text("olddup"), false},
		{"SubVegA-SIVI", "A", "", text("A"), nil, false},
		{"SubVegA-SIVI", "PERSONAL", "user", text("personal"), text("PERSONAL"), false},
		{"SubVegA-SIVI", "new_a", "replace", text("olddup"), text("new_a"), false},
		{"SubVegA-SIVI", "A", "create", text("A"), nil, false},
		{"SubVegA-SIVI", "É", "keep", text("é"), nil, false},
		{"SubVegA-SIVI", "", "", nil, nil, false},
		{"SubVegA-SIVI", " A", "", nil, nil, false},
	} {
		edit := sourceform.SIVISpeciesEdit{Form: test.form, Value: test.value, Decision: test.decision, Entered: test.entered, Selected: test.selected}
		err := sourceform.ValidateSIVISpeciesMembership(ctx, edit, refs)
		if (err == nil) != test.valid {
			t.Fatalf("%+v: %v", test, err)
		}
	}
	for _, form := range []string{"SubVegA-SIVI_BC", "SubVegA-SIVI", "SubVegC-SIVI", "SubVegD-SIVI"} {
		for life := 0; life <= 13; life++ {
			refs.Master.Rows[0].Cells[2] = vprotest.MetadataInteger(strconv.Itoa(life))
			want := (strings.HasPrefix(form, "SubVegA") && life >= 1 && life <= 4) ||
				(form == "SubVegC-SIVI" && (life >= 5 && life <= 8 || life == 12)) ||
				(form == "SubVegD-SIVI" && (life == 1 || life == 2 || life >= 9 && life <= 11))
			err := sourceform.ValidateSIVISpeciesMembership(ctx, sourceform.SIVISpeciesEdit{Form: form, Value: "A"}, refs)
			if (err == nil) != want {
				t.Fatal(form, life, want, err)
			}
		}
	}
}

func TestSIVISpeciesDefinitionProjectionRetainsDuplicatesNullsAndRecasing(t *testing.T) {
	ctx := context.Background()
	raw := sftest.SiviSpeciesTestReferences(t).Master
	raw.Columns[2].Name = "LIFEFORM"
	raw.Rows = append(raw.Rows, vprocore.ProjectMetadataRow{RowID: "9223372036854775807", Cells: raw.Rows[0].Cells})
	projected, err := sourceform.ProjectSIVISpeciesDefinitions(ctx, raw, false)
	if err != nil || !reflect.DeepEqual(projected, raw) {
		t.Fatal("projection normalized/deduplicated physical metadata", projected, err)
	}
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataText("3"), vprotest.SiviReal(3), vprotest.MetadataInteger("32768")} {
		bad := sftest.SiviSpeciesTestReferences(t).Master
		bad.Rows[0].Cells[2] = cell
		if _, err := sourceform.ProjectSIVISpeciesDefinitions(ctx, bad, false); err == nil {
			t.Fatal("coerced malformed Lifeform", cell)
		}
	}
	for _, mutate := range []func(*vprocore.ProjectMetadataTable){
		func(table *vprocore.ProjectMetadataTable) { table.Columns[0].Name = "Missing" },
		func(table *vprocore.ProjectMetadataTable) { table.Rows[1].RowID = table.Rows[0].RowID },
		func(table *vprocore.ProjectMetadataTable) { table.Rows[0].Cells[1] = vprotest.MetadataInteger("1") },
		func(table *vprocore.ProjectMetadataTable) { table.Rows[0].Cells[5] = vprotest.SiviReal(2) },
		func(table *vprocore.ProjectMetadataTable) { table.Rows[0].RowID = "+1" },
	} {
		bad := sftest.SiviSpeciesTestReferences(t).Master
		mutate(&bad)
		if _, err := sourceform.ProjectSIVISpeciesDefinitions(ctx, bad, false); err == nil {
			t.Fatal("malformed physical definition accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := sourceform.ProjectSIVISpeciesDefinitions(canceled, raw, false); err == nil {
		t.Fatal("canceled projection accepted")
	}
}

func TestSIVISpeciesStrictRawTransport(t *testing.T) {
	cell := `{"storage":"text","text":"RAW","integer":null,"real":null,"blobHex":null}`
	edit := `{"rowId":"9223372036854775807","form":"SubVegC-SIVI","expected":` + cell + `,"value":"C"}`
	for _, malformed := range []string{
		strings.Replace(edit, `"rowId":`, `"RowId":`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","value":"D"`, 1),
		strings.Replace(edit, `"value":"C"`, `"column":"Species","value":"C"`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":null`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":1`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"\ud800"`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"`+string([]byte{0xff})+`"`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","decision":null`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","entered":null`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","selected":null`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","decision":"create"`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","decision":"keep"`, 1),
		strings.Replace(edit, `"value":"C"`, `"value":"C","entered":"C"`, 1),
		strings.Replace(edit, `"storage":"text"`, `"storage":"text","storage":"null"`, 1),
		strings.Replace(edit, `"text":"RAW"`, `"text":"\udfff"`, 1),
		strings.Replace(edit, `"blobHex":null`, `"other":null`, 1),
		`null`, `{}`, edit + `{}`,
	} {
		if err := json.Unmarshal([]byte(malformed), &sourceform.SIVISpeciesEdit{}); err == nil {
			t.Fatal("malformed edit accepted", malformed)
		}
	}
	var decoded sourceform.SIVISpeciesEdit
	if err := json.Unmarshal([]byte(edit), &decoded); err != nil || decoded.RowID != "9223372036854775807" {
		t.Fatal(decoded, err)
	}
	ref, err := json.Marshal(sftest.SiviSpeciesTestReferences(t))
	if err != nil {
		t.Fatal(err)
	}
	request := `{"original":[],"references":` + string(ref) + `,"edits":[` + edit + `]}`
	for _, bad := range []string{
		strings.Replace(request, `"original":[]`, `"original":null`, 1),
		strings.Replace(request, `"references":`, `"References":`, 1),
		strings.Replace(request, `"contextId":"ctx"`, `"contextId":"ctx","contextId":"foreign"`, 1),
		strings.Replace(request, `"name":"Code"`, `"name":"Code","name":"other"`, 1),
		strings.Replace(request, `"rows":[`, `"rows":null,"rows":[`, 1),
		strings.Replace(request, `"cells":[`, `"cells":null,"cells":[`, 1),
		strings.Replace(request, `"integer":"3"`, `"integer":"32768"`, 1),
		strings.Replace(request, `"storage":"integer","text":null,"integer":"3"`, `"storage":"text","text":"3","integer":null`, 1),
	} {
		if err := json.Unmarshal([]byte(bad), &sourceform.SIVISpeciesWrite{}); err == nil {
			t.Fatal("malformed reviewed authority accepted", bad)
		}
	}
	if err := json.Unmarshal([]byte(request), &sourceform.SIVISpeciesWrite{}); err != nil {
		t.Fatal(err)
	}
}
