package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

var soilPlanSourceFields = map[string][]string{
	"SoilHumusXL": {
		"Horizon", "UpperDepth", "LowerDepth", "HumusFormpH", "Comment", "HumusStructureDegree",
		"HumusStructureKind", "MycelAbundance", "FecalAbundance", "RootsAbundance", "RootsSize", "vonPost",
	},
	"SoilMineralXL": {
		"Horizon", "UpperDepth", "LowerDepth", "Texture", "Colour", "Comments", "PitDepthLimit", "ASP",
		"PercentCoarseFragsGravel", "PercentCoarseFragsCobbles", "PercentCoarseFragsStones",
		"PercentCoarseFragsTotal", "PercentCoarseFragsShape", "RootsAbundance", "RootsSize",
		"MineralStructureClass", "MineralStructureKind", "MineralFormpH",
	},
}

func soilPlanInteger(value string) ProjectMetadataCell {
	return ProjectMetadataCell{Storage: "integer", Integer: &value}
}

func soilPlanFixture(form string) *twoPageSoilOriginal {
	kind, source := "Humus", "UsysHumus"
	if form == "SoilMineralXL" {
		kind, source = "Mineral", "UsysMineral"
	}
	original := &twoPageSoilOriginal{ContextID: "owned-context", Project: "Sample", Plot: "0108050",
		Form: form, Table: "Sample_" + kind, RecordSource: source}
	original.Records.Columns = []ProjectMetadataColumn{{Name: "ID"}, {Name: "PlotNumber"}}
	row := ProjectMetadataRow{RowID: "-7", Cells: []ProjectMetadataCell{soilPlanInteger("-2147483648"), metadataText("0108050")}}
	for _, column := range soilPlanSourceFields[form] {
		original.Records.Columns = append(original.Records.Columns, ProjectMetadataColumn{Name: column})
		row.Cells = append(row.Cells, ProjectMetadataCell{Storage: "null"})
	}
	blob := "ff00"
	original.Records.Columns = append(original.Records.Columns, ProjectMetadataColumn{Name: "Unmapped"})
	row.Cells = append(row.Cells, ProjectMetadataCell{Storage: "blob", BlobHex: &blob})
	original.Records.Rows = []ProjectMetadataRow{row}
	return original
}

func soilPlanEdit(original *twoPageSoilOriginal, column string, value ProjectMetadataCell) twoPageSoilEdit {
	expected := ProjectMetadataCell{Storage: "null"}
	for i, field := range original.Records.Columns {
		if field.Name == column {
			expected = original.Records.Rows[0].Cells[i]
			break
		}
	}
	return twoPageSoilEdit{original.ContextID, original.Form, "-7", column, expected, value}
}

func TestTwoPageSoilPlanSourceBindingsAndDetachedAssignments(t *testing.T) {
	integerFields := map[string]bool{"vonPost": true, "ASP": true, "PercentCoarseFragsGravel": true,
		"PercentCoarseFragsCobbles": true, "PercentCoarseFragsStones": true, "PercentCoarseFragsTotal": true}
	realFields := map[string]bool{"UpperDepth": true, "LowerDepth": true, "HumusFormpH": true, "MineralFormpH": true}
	for form, fields := range soilPlanSourceFields {
		t.Run(form, func(t *testing.T) {
			original := soilPlanFixture(form)
			before, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var edits []twoPageSoilEdit
			var want []twoPageSoilAssignment
			for _, column := range fields {
				cell, scalar := metadataText("X"), any("X")
				switch {
				case integerFields[column]:
					cell, scalar = soilPlanInteger("-32768"), int(-32768)
				case realFields[column]:
					cell, scalar = siviReal(-1.25), float64(-1.25)
				}
				edits = append(edits, soilPlanEdit(original, column, cell))
				want = append(want, twoPageSoilAssignment{"-7", column, -2147483648,
					ProjectMetadataCell{Storage: "null"}, cell, scalar})
			}
			got, err := planTwoPageSoilEdits(context.Background(), original, edits)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("all source-bound assignments = %#v, %v; want %#v", got, err, want)
			}
			after, err := json.Marshal(original)
			if err != nil || string(after) != string(before) {
				t.Fatal("planning changed complete originals/unmapped BLOB")
			}
			*edits[0].Value.Text = "changed caller"
			if *got[0].After.Text != "X" || got[0].Value != "X" {
				t.Fatal("assignment retained a mutable caller value")
			}
		})
	}
}

func TestTwoPageSoilPlanPhysicalDomainsAndNull(t *testing.T) {
	textLimits := map[string]map[string]int{
		"SoilHumusXL": {"Horizon": 8, "HumusStructureDegree": 1, "HumusStructureKind": 5,
			"MycelAbundance": 1, "FecalAbundance": 1, "RootsAbundance": 6, "RootsSize": 6, "Comment": 0},
		"SoilMineralXL": {"Horizon": 8, "Texture": 4, "Colour": 14, "PitDepthLimit": 1,
			"PercentCoarseFragsShape": 1, "RootsAbundance": 6, "RootsSize": 6,
			"MineralStructureClass": 3, "MineralStructureKind": 7, "Comments": 0},
	}
	for form, limits := range textLimits {
		for column, maximum := range limits {
			t.Run(form+"/"+column, func(t *testing.T) {
				original := soilPlanFixture(form)
				limit := maximum
				if limit == 0 {
					limit = 4096
				}
				for _, text := range []string{"", strings.Repeat("x", limit), strings.Repeat("\U0001f600", limit/2)} {
					edit := soilPlanEdit(original, column, metadataText(text))
					if got, err := planTwoPageSoilEdits(context.Background(), original, []twoPageSoilEdit{edit}); err != nil || len(got) != 1 {
						t.Fatalf("valid literal text %q rejected: %v, %v", text, got, err)
					}
				}
				for _, cell := range []ProjectMetadataCell{metadataText(string([]byte{0xff})), siviReal(1), soilPlanInteger("1")} {
					if got, err := planTwoPageSoilEdits(context.Background(), original,
						[]twoPageSoilEdit{soilPlanEdit(original, column, cell)}); err == nil || got != nil {
						t.Fatalf("invalid text storage accepted: %v, %v", got, err)
					}
				}
				if maximum > 0 {
					for _, text := range []string{strings.Repeat("x", maximum+1), strings.Repeat("\U0001f600", maximum/2+1)} {
						if got, err := planTwoPageSoilEdits(context.Background(), original,
							[]twoPageSoilEdit{soilPlanEdit(original, column, metadataText(text))}); err == nil || got != nil {
							t.Fatal("new UTF-16 overflow accepted")
						}
					}
				}
			})
		}
	}
	for _, test := range []struct {
		form, column string
		valid, bad   []ProjectMetadataCell
	}{
		{"SoilHumusXL", "vonPost", []ProjectMetadataCell{soilPlanInteger("-32768"), soilPlanInteger("32767"), soilPlanInteger("0")},
			[]ProjectMetadataCell{soilPlanInteger("-32769"), soilPlanInteger("32768"), soilPlanInteger("01"), siviReal(1), metadataText("1")}},
		{"SoilMineralXL", "ASP", []ProjectMetadataCell{soilPlanInteger("-32768"), soilPlanInteger("32767")},
			[]ProjectMetadataCell{soilPlanInteger("9223372036854775807"), siviReal(1.5)}},
		{"SoilHumusXL", "UpperDepth", []ProjectMetadataCell{siviReal(-math.MaxFloat32), siviReal(math.MaxFloat32), siviReal(0)},
			[]ProjectMetadataCell{siviReal(math.Nextafter(math.MaxFloat32, math.Inf(1))), siviReal(math.NaN()), siviReal(math.Inf(1)), soilPlanInteger("1")}},
		{"SoilMineralXL", "MineralFormpH", []ProjectMetadataCell{siviReal(1.234567890123456)},
			[]ProjectMetadataCell{metadataText("1"), siviReal(-math.MaxFloat64)}},
	} {
		t.Run(test.form+"/"+test.column, func(t *testing.T) {
			original := soilPlanFixture(test.form)
			for _, cell := range test.valid {
				if got, err := planTwoPageSoilEdits(context.Background(), original,
					[]twoPageSoilEdit{soilPlanEdit(original, test.column, cell)}); err != nil || len(got) != 1 {
					t.Fatalf("valid physical number rejected: %v, %v", got, err)
				}
			}
			for _, cell := range test.bad {
				if got, err := planTwoPageSoilEdits(context.Background(), original,
					[]twoPageSoilEdit{soilPlanEdit(original, test.column, cell)}); err == nil || got != nil {
					t.Fatal("invalid physical number accepted")
				}
			}
		})
	}
	for form, fields := range soilPlanSourceFields {
		for _, column := range fields {
			original := soilPlanFixture(form)
			for i, field := range original.Records.Columns {
				if field.Name == column {
					original.Records.Rows[0].Cells[i] = metadataText("historical invalid type or overlength")
				}
			}
			edit := soilPlanEdit(original, column, ProjectMetadataCell{Storage: "null"})
			if got, err := planTwoPageSoilEdits(context.Background(), original, []twoPageSoilEdit{edit}); err != nil || len(got) != 1 || got[0].Value != nil {
				t.Fatalf("explicit nullable correction %s.%s failed: %v, %v", form, column, got, err)
			}
			edit.Value = edit.Expected
			if got, err := planTwoPageSoilEdits(context.Background(), original, []twoPageSoilEdit{edit}); err != nil || len(got) != 0 {
				t.Fatalf("unchanged historical value was validated or assigned: %v, %v", got, err)
			}
		}
	}
}

func TestTwoPageSoilPlanOwnershipSchemaAndIdentityRefusals(t *testing.T) {
	for name, change := range map[string]func(*twoPageSoilOriginal, *twoPageSoilEdit){
		"context": func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.ContextID += "-stale" },
		"form":    func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.Form = "SoilMineralXL" },
		"row":     func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.RowID = "0" },
		"identity edit": func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) {
			e.Column, e.Expected, e.Value = "ID", soilPlanInteger("-2147483648"), soilPlanInteger("1")
		},
		"parent edit": func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.Column = "PlotNumber" },
		"unmapped":    func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.Column = "Unmapped" },
		"missing field": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Columns[2].Name = "WrongHorizon"
		},
		"ambiguous schema": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) { o.Records.Columns[2].Name = "ID" },
		"missing cells":    func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) { o.Records.Rows[0].Cells = nil },
		"rowid alias":      func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) { o.Records.Rows[0].RowID = "-07" },
		"wrong source":     func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) { o.RecordSource = "UsysMineral" },
		"wrong table":      func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) { o.Table = "Other_Humus" },
		"parent whitespace": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[1] = metadataText("0108050 ")
		},
		"numeric parent": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[1] = soilPlanInteger("108050")
		},
		"NULL ID": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[0] = ProjectMetadataCell{Storage: "null"}
		},
		"text ID": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[0] = metadataText("-2147483648")
		},
		"overflow ID": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[0] = soilPlanInteger("2147483648")
		},
		"expected": func(_ *twoPageSoilOriginal, e *twoPageSoilEdit) { e.Expected = metadataText("not original") },
		"malformed unmapped text": func(o *twoPageSoilOriginal, _ *twoPageSoilEdit) {
			o.Records.Rows[0].Cells[len(o.Records.Columns)-1] = metadataText(string([]byte{0xff}))
		},
	} {
		t.Run(name, func(t *testing.T) {
			original := soilPlanFixture("SoilHumusXL")
			edit := soilPlanEdit(original, "Horizon", metadataText("LF"))
			change(original, &edit)
			if got, err := planTwoPageSoilEdits(context.Background(), original, []twoPageSoilEdit{edit}); err == nil || got != nil {
				t.Fatalf("invalid original/draft returned a plan: %v, %v", got, err)
			}
		})
	}
	for _, identity := range []ProjectMetadataCell{soilPlanInteger("-2147483648"), siviReal(-2147483648), metadataText(" -2147483648 ")} {
		original := soilPlanFixture("SoilHumusXL")
		peer := ProjectMetadataRow{RowID: "2", Cells: append([]ProjectMetadataCell{}, original.Records.Rows[0].Cells...)}
		peer.Cells[0] = identity
		original.Records.Rows = append(original.Records.Rows, peer)
		if got, err := planTwoPageSoilEdits(context.Background(), original,
			[]twoPageSoilEdit{soilPlanEdit(original, "Horizon", metadataText("LF"))}); err == nil || got != nil {
			t.Fatal("colliding physical peer logical ID accepted")
		}
	}
	original := soilPlanFixture("SoilHumusXL")
	edit := soilPlanEdit(original, "Horizon", metadataText("LF"))
	if got, err := planTwoPageSoilEdits(context.Background(), original, []twoPageSoilEdit{edit, edit}); err == nil || got != nil {
		t.Fatal("duplicate edit accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planTwoPageSoilEdits(ctx, original, []twoPageSoilEdit{edit}); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal("cancelled planning succeeded")
	}
	if got, err := planTwoPageSoilEdits(nil, original, []twoPageSoilEdit{edit}); err == nil || got != nil {
		t.Fatal("nil planning context accepted")
	}
}

func TestTwoPageSoilPlanStrictEditTransport(t *testing.T) {
	original := soilPlanFixture("SoilHumusXL")
	edit := soilPlanEdit(original, "Horizon", metadataText("LF"))
	raw, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	var decoded twoPageSoilEdit
	if err := json.Unmarshal(raw, &decoded); err != nil || !reflect.DeepEqual(decoded, edit) {
		t.Fatalf("strict production-shaped round trip = %v, %v", decoded, err)
	}
	valid := string(raw)
	for _, invalid := range []string{
		strings.Replace(valid, `"column":"Horizon"`, `"column":"Horizon","column":"UpperDepth"`, 1),
		strings.Replace(valid, `"column":"Horizon"`, `"column":"Horizon","\u0063olumn":"UpperDepth"`, 1),
		strings.Replace(valid, `"column":"Horizon"`, `"Column":"Horizon"`, 1),
		strings.Replace(valid, `"column":"Horizon"`, `"column":null`, 1),
		strings.Replace(valid, `"column":"Horizon",`, "", 1),
		strings.Replace(valid, `"text":"LF"`, `"text":"LF","text":"OF"`, 1),
		strings.Replace(valid, `"text":"LF"`, `"text":"\ud800"`, 1),
		strings.Replace(valid, `"text":"LF"`, `"text":"\udc00"`, 1),
		strings.Replace(valid, `"text":"LF"`, `"text":"`+string([]byte{0xff})+`"`, 1),
		strings.Replace(valid, `"text":"LF"`, `"text":"LF","unknown":true`, 1),
		strings.Replace(valid, `"storage":"null"`, `"storage":"null","storage":"text"`, 1),
		strings.Replace(valid, `"expected":{`, `"expected":null,"ignored":{`, 1),
		valid[:len(valid)-1] + `,"unknown":true}`,
	} {
		retained := decoded
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil || !reflect.DeepEqual(decoded, retained) {
			t.Fatalf("invalid request repaired/decoded or overwrote editor: %q, %v", invalid, err)
		}
	}
}

func TestTwoPageSoilPlanActualReadBoundaryAndNoMutation(t *testing.T) {
	for _, external := range []bool{false, true} {
		service, state, db := twoPageSoilReadFixture(t, external, "TEXT")
		for _, test := range []struct{ form, kind string }{{"SoilHumusXL", "Humus"}, {"SoilMineralXL", "Mineral"}} {
			if _, err := db.Exec(`INSERT INTO "Sample_` + test.kind + `"(rowid,ID,PlotNumber,Horizon,Unmapped)
				VALUES(-7,0,'108050',NULL,X'FF00'),(2,NULL,'108050','historical too long',NULL)`); err != nil {
				t.Fatal(err)
			}
			original, err := service.readTwoPageSoilOriginal(context.Background(), state.ContextID, "108050", test.form)
			if err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, service.projects.sqlite.attachments)
			got, err := planTwoPageSoilEdits(context.Background(), original,
				[]twoPageSoilEdit{soilPlanEdit(original, "Horizon", metadataText(""))})
			want := []twoPageSoilAssignment{{"-7", "Horizon", 0, ProjectMetadataCell{Storage: "null"}, metadataText(""), ""}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("actual original preserves ID0/NULL/empty/uneditable peer: %v, %v", got, err)
			}
			if after := databaseBytes(t, service.projects.sqlite.attachments); !reflect.DeepEqual(after, before) {
				t.Fatal("plan changed physical source/audit/history")
			}
		}
	}
}
