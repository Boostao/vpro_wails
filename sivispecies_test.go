package main

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func siviSpeciesTestReferences(t *testing.T) SIVISpeciesReferences {
	t.Helper()
	table := func(personal bool) ProjectMetadataTable {
		names := siviSpeciesMasterColumns
		if personal {
			names = siviSpeciesPersonalColumns
		}
		result := ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}}
		for _, name := range names {
			kind := "TEXT"
			if strings.EqualFold(name, "Lifeform") {
				kind = "INTEGER"
			}
			result.Columns = append(result.Columns, ProjectMetadataColumn{name, kind})
		}
		return result
	}
	result := SIVISpeciesReferences{ContextID: "ctx", Project: "Sample", Plot: "108050", Master: table(false), Personal: table(true)}
	result.Master.Rows = []ProjectMetadataRow{
		{"-9223372036854775808", []ProjectMetadataCell{metadataText("A"), {Storage: "null"}, metadataInteger("3"), metadataText(""), metadataText("U"), {Storage: "null"}}},
		{"2", []ProjectMetadataCell{metadataText("C"), metadataText(""), metadataInteger("12"), {Storage: "null"}, metadataText("x"), {Storage: "null"}}},
		{"3", []ProjectMetadataCell{metadataText("D"), {Storage: "null"}, metadataInteger("9"), {Storage: "null"}, metadataText("u"), {Storage: "null"}}},
		{"4", []ProjectMetadataCell{metadataText("new_a"), {Storage: "null"}, metadataInteger("99"), {Storage: "null"}, metadataText("S"), metadataText("olddup")}},
		{"5", []ProjectMetadataCell{metadataText("NEW_B"), metadataText("duplicate"), metadataInteger("3"), metadataText(""), metadataText("U"), metadataText("olddup")}},
		{"6", []ProjectMetadataCell{metadataText("é😀😀😀"), {Storage: "null"}, metadataInteger("1"), {Storage: "null"}, metadataText("X"), {Storage: "null"}}},
		{"7", []ProjectMetadataCell{metadataText("éé😀😀😀"), {Storage: "null"}, metadataInteger("1"), {Storage: "null"}, metadataText("X"), {Storage: "null"}}},
	}
	result.Personal.Rows = []ProjectMetadataRow{
		{"1", []ProjectMetadataCell{metadataText("Personal"), {Storage: "null"}, metadataInteger("99"), metadataText(""), metadataText("S")}},
		{"2", []ProjectMetadataCell{metadataText("olddup"), {Storage: "null"}, metadataInteger("1"), {Storage: "null"}, metadataText("U")}},
	}
	return result
}

func TestSIVISpeciesListedDecisionsUnicodeAndDomains(t *testing.T) {
	ctx := context.Background()
	refs := siviSpeciesTestReferences(t)
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
		edit := SIVISpeciesEdit{Form: test.form, Value: test.value, Decision: test.decision, Entered: test.entered, Selected: test.selected}
		err := validateSIVISpeciesMembership(ctx, edit, refs)
		if (err == nil) != test.valid {
			t.Fatalf("%+v: %v", test, err)
		}
	}
	for _, form := range []string{"SubVegA-SIVI_BC", "SubVegA-SIVI", "SubVegC-SIVI", "SubVegD-SIVI"} {
		for life := 0; life <= 13; life++ {
			refs.Master.Rows[0].Cells[2] = metadataInteger(strconv.Itoa(life))
			want := (strings.HasPrefix(form, "SubVegA") && life >= 1 && life <= 4) ||
				(form == "SubVegC-SIVI" && (life >= 5 && life <= 8 || life == 12)) ||
				(form == "SubVegD-SIVI" && (life == 1 || life == 2 || life >= 9 && life <= 11))
			err := validateSIVISpeciesMembership(ctx, SIVISpeciesEdit{Form: form, Value: "A"}, refs)
			if (err == nil) != want {
				t.Fatal(form, life, want, err)
			}
		}
	}
}

func TestSIVISpeciesDefinitionProjectionRetainsDuplicatesNullsAndRecasing(t *testing.T) {
	ctx := context.Background()
	raw := siviSpeciesTestReferences(t).Master
	raw.Columns[2].Name = "LIFEFORM"
	raw.Rows = append(raw.Rows, ProjectMetadataRow{"9223372036854775807", raw.Rows[0].Cells})
	projected, err := projectSIVISpeciesDefinitions(ctx, raw, false)
	if err != nil || !reflect.DeepEqual(projected, raw) {
		t.Fatal("projection normalized/deduplicated physical metadata", projected, err)
	}
	for _, cell := range []ProjectMetadataCell{metadataText("3"), siviReal(3), metadataInteger("32768")} {
		bad := siviSpeciesTestReferences(t).Master
		bad.Rows[0].Cells[2] = cell
		if _, err := projectSIVISpeciesDefinitions(ctx, bad, false); err == nil {
			t.Fatal("coerced malformed Lifeform", cell)
		}
	}
	for _, mutate := range []func(*ProjectMetadataTable){
		func(table *ProjectMetadataTable) { table.Columns[0].Name = "Missing" },
		func(table *ProjectMetadataTable) { table.Rows[1].RowID = table.Rows[0].RowID },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[1] = metadataInteger("1") },
		func(table *ProjectMetadataTable) { table.Rows[0].Cells[5] = siviReal(2) },
		func(table *ProjectMetadataTable) { table.Rows[0].RowID = "+1" },
	} {
		bad := siviSpeciesTestReferences(t).Master
		mutate(&bad)
		if _, err := projectSIVISpeciesDefinitions(ctx, bad, false); err == nil {
			t.Fatal("malformed physical definition accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := projectSIVISpeciesDefinitions(canceled, raw, false); err == nil {
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
		if err := json.Unmarshal([]byte(malformed), &SIVISpeciesEdit{}); err == nil {
			t.Fatal("malformed edit accepted", malformed)
		}
	}
	var decoded SIVISpeciesEdit
	if err := json.Unmarshal([]byte(edit), &decoded); err != nil || decoded.RowID != "9223372036854775807" {
		t.Fatal(decoded, err)
	}
	ref, err := json.Marshal(siviSpeciesTestReferences(t))
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
		if err := json.Unmarshal([]byte(bad), &SIVISpeciesWrite{}); err == nil {
			t.Fatal("malformed reviewed authority accepted", bad)
		}
	}
	if err := json.Unmarshal([]byte(request), &SIVISpeciesWrite{}); err != nil {
		t.Fatal(err)
	}
}
