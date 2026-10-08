package main

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/fs882layout"
)

func TestFS1333PackagedAggregateHeightsAndChildLinksRemainUnmapped(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("resources", "fs1333-sivi-layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	var layout fs882layout.Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	if layout.Root != "frmSIVIsite" || len(layout.Forms) != 4 || layout.GeometryUnit != "twips" {
		t.Fatal("original SIVI closure/geometry evidence changed", layout.Root)
	}
	forms := map[string]fs882layout.Form{}
	for _, form := range layout.Forms {
		if forms[form.Name].Name != "" || len(form.SHA256) != 64 {
			t.Fatal("source identity/hash lost", form.Name)
		}
		forms[form.Name] = form
		for _, field := range form.Fields {
			if !field.ReadOnly || field.Implementation != "unmapped" {
				t.Fatal("static source packaging enabled an unavailable workflow", form.Name, field)
			}
		}
	}
	root := forms["frmSIVIsite"]
	if root.RecordSource != "USysEnv" || len(root.Embedded) != 3 || len(root.Pages) != 3 {
		t.Fatal("parent source/pages/child count changed", root)
	}
	expectedChildren := map[string]string{
		"SubVegA-SIVI_BC": "USysVegA", "SubVegC-SIVI": "USysVegC", "SubVegD-SIVI": "USysVegD",
	}
	seen := map[string]bool{}
	for _, child := range root.Embedded {
		if !child.Resolved || !child.ReadOnly || expectedChildren[child.Form] == "" || seen[child.Form] ||
			!reflect.DeepEqual(child.MasterFields, []string{"PlotNumber"}) ||
			!reflect.DeepEqual(child.ChildFields, []string{"PlotNumber"}) {
			t.Fatal("SIVI physical plot child link changed", child)
		}
		seen[child.Form] = true
	}
	for name, source := range expectedChildren {
		form := forms[name]
		if form.RecordSource != source || !seen[name] {
			t.Fatal("SIVI child source changed", name, form.RecordSource)
		}
		heights, labels := []string{}, map[string]bool{}
		beforeUpdate := false
		for _, field := range form.Fields {
			if strings.HasPrefix(field.Binding, "Height") {
				heights = append(heights, field.Binding)
				for _, event := range field.Events {
					if !event.Resolved {
						t.Fatal("height source event unresolved", name, event)
					}
				}
			}
			if field.Caption != "" {
				labels[field.Caption] = true
			}
		}
		for _, control := range form.Controls {
			for _, event := range control.Events {
				if control.ControlName == name && event.Property == "BeforeUpdate" &&
					event.Procedure == "Form_BeforeUpdate" && event.Resolved {
					beforeUpdate = true
				}
			}
		}
		if !beforeUpdate {
			t.Fatal("source BeforeUpdate event path lost", name)
		}
		switch name {
		case "SubVegA-SIVI_BC":
			if !reflect.DeepEqual(heights, []string{"HeightA", "HeightB"}) || !labels["Ht A"] || !labels["Ht B"] {
				t.Fatal("SIVI aggregate heights replaced by FS882 individual heights", heights, labels)
			}
		case "SubVegC-SIVI":
			if !reflect.DeepEqual(heights, []string{"Height6"}) || !labels["Ht"] {
				t.Fatal("SIVI herb height binding/label changed", heights, labels)
			}
		case "SubVegD-SIVI":
			if len(heights) != 0 {
				t.Fatal("unavailable moss height invented", heights)
			}
		}
	}
}

func TestFS1333HeightBRemainsPhysicalTextNotNumericCoercion(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("resources", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for name, expected := range map[string]string{"HeightA": "REAL", "HeightB": "VARCHAR", "Height6": "REAL"} {
		var physical string
		if err := db.QueryRow(`SELECT type FROM pragma_table_info('Sample_Veg') WHERE name=?`, name).Scan(&physical); err != nil {
			t.Fatal(err)
		}
		if !strings.EqualFold(physical, expected) {
			t.Fatal("aggregate physical storage was guessed from its UI label", name, physical)
		}
	}
}

func TestFS1333ParentBindingsAndEventOwnershipRemainDistinct(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("resources", "fs1333-sivi-layout.json"))
	if err != nil {
		t.Fatal(err)
	}
	var layout fs882layout.Layout
	if err := json.Unmarshal(data, &layout); err != nil {
		t.Fatal(err)
	}
	parent := layout.Forms[0]
	expectedEvents := map[string][]string{
		"frmSIVIsite":            {"OnCurrent:Form_Current", "OnLoad:Form_Load"},
		"optPlotType":            {"AfterUpdate:optPlotType_AfterUpdate"},
		"btnClose2":              {"OnClick:btnClose2_Click"},
		"ProjectID":              {"OnGotFocus:ProjectID_GotFocus", "OnNotInList:ProjectID_NotInList"},
		"btnEditMetadata":        {"OnClick:btnEditMetadata_Click"},
		"optSpeciesListComplete": {"AfterUpdate:optSpeciesListComplete_AfterUpdate"},
		"optProjectID":           {"AfterUpdate:optProjectID_AfterUpdate"},
	}
	actualEvents := map[string][]string{}
	for _, control := range parent.Controls {
		for _, event := range control.Events {
			if !event.Resolved {
				t.Fatal("parent event lost its source procedure", control.ControlName, event)
			}
			actualEvents[control.ControlName] = append(actualEvents[control.ControlName], event.Property+":"+event.Procedure)
		}
	}
	if !reflect.DeepEqual(actualEvents, expectedEvents) {
		t.Fatal("SIVI parent events drifted or inherited XL Lock/audit events", actualEvents)
	}
	path, err := filepath.Abs(filepath.Join("resources", "Sample.db"))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	bound := map[string]bool{}
	for _, field := range parent.Fields {
		if field.Binding == "" {
			continue
		}
		if !field.ReadOnly || field.Implementation != "unmapped" || bound[strings.ToLower(field.Binding)] {
			t.Fatal("parent binding was duplicated or enabled without its workflow", field)
		}
		bound[strings.ToLower(field.Binding)] = true
		var definitions int
		if err := db.QueryRow(`SELECT COUNT(*) FROM (
			SELECT name FROM pragma_table_info('Sample_Env')
			UNION ALL SELECT name FROM pragma_table_info('Sample_Admin')
		) WHERE name=? COLLATE NOCASE`, field.Binding).Scan(&definitions); err != nil {
			t.Fatal(err)
		}
		if definitions != 1 {
			t.Fatal("source USysEnv binding is unavailable or has ambiguous physical ownership", field.Binding, definitions)
		}
	}
	if len(bound) != 77 {
		t.Fatal("source parent binding count changed", len(bound))
	}
	for _, name := range []string{"sv_polygonnumber", "sv_floodplain", "sv_standheight", "sv_ahorizondepth", "sv_rootzonetexture", "humusthickness", "stratacovertotal", "plotnumber", "plottype", "startdate"} {
		if !bound[name] {
			t.Fatal("distinct SIVI parent storage binding lost", name)
		}
	}
}

func TestFS1333ExtendedChildOnlyAddsThreeShrubCoverBindings(t *testing.T) {
	load := func(name string) fs882layout.Layout {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("resources", name))
		if err != nil {
			t.Fatal(err)
		}
		var layout fs882layout.Layout
		if err := json.Unmarshal(data, &layout); err != nil {
			t.Fatal(err)
		}
		return layout
	}
	normal := load("fs1333-sivi-layout.json").Forms[1]
	extended := load("fs1333-extended-shrub-layout.json")
	if normal.Name != "SubVegA-SIVI_BC" || extended.Root != "SubVegA-SIVI" || len(extended.Forms) != 1 ||
		extended.Forms[0].RecordSource != normal.RecordSource {
		t.Fatal("extended presentation substituted parent identity/source")
	}
	bindings := map[string]bool{}
	for _, field := range normal.Fields {
		if field.Binding != "" {
			bindings[strings.ToLower(field.Binding)] = true
		}
	}
	added, labels := map[string]bool{}, map[string]bool{}
	for _, field := range extended.Forms[0].Fields {
		if !field.ReadOnly || field.Implementation != "unmapped" {
			t.Fatal("extended static packaging enabled a workflow", field)
		}
		labels[field.Caption] = true
		if field.Binding != "" {
			key := strings.ToLower(field.Binding)
			if bindings[key] {
				delete(bindings, key)
			} else {
				added[key] = true
			}
		}
	}
	if len(bindings) != 0 || !reflect.DeepEqual(added, map[string]bool{"cover5a": true, "cover5b": true, "cover5c": true}) ||
		!labels["B3"] || !labels["B4"] || !labels["B5"] {
		t.Fatal("extended SIVI lost aggregate heights or guessed shrub columns/labels", bindings, added, labels)
	}
}
