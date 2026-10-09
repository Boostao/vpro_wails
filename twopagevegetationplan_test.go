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

func twoPageVegetationPlanFixture(t *testing.T, form string) *twoPageVegetationOriginal {
	t.Helper()
	service, state, db := twoPageVegetationReadFixture(t, false, "TEXT")
	if _, err := db.Exec(`INSERT INTO Sample_Veg(rowid,ID,PlotNumber,Species,Cover1,Cover2,cover3,
		TotalA,Cover4,Cover5,TotalB,Cover5a,Cover5b,Cover5c,Cover6,Cover7,Cover8,Cover9,Collected) VALUES
		(-7,-2147483648,'108050','TEST',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL),
		(3,-2147483648,'108050','PEER',0,0,0,0,0,0,0,0,0,0,0,0,0,0,'V'),
		(8,NULL,'108050','NULLID',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL),
		(9,'01','108050','TEXTID',0,0,0,0,0,0,0,0,0,0,0,0,0,0,NULL)`); err != nil {
		t.Fatal(err)
	}
	original, err := service.readTwoPageVegetationOriginal(context.Background(), state.ContextID, "108050", form)
	if err != nil {
		t.Fatal(err)
	}
	return original
}

func vegetationPlanCell(original *twoPageVegetationOriginal, rowID, column string) *ProjectMetadataCell {
	for i, field := range original.Records.Columns {
		if strings.EqualFold(field.Name, column) {
			for j := range original.Records.Rows {
				if original.Records.Rows[j].RowID == rowID {
					return &original.Records.Rows[j].Cells[i]
				}
			}
		}
	}
	panic("planned test field absent")
}

func vegetationPlanEdit(original *twoPageVegetationOriginal, column string, value ProjectMetadataCell) twoPageVegetationEdit {
	return twoPageVegetationEdit{original.ContextID, original.Form, "-7", column,
		cloneSiteUnitCell(*vegetationPlanCell(original, "-7", column)), value}
}

func cloneVegetationPlanOriginal(t *testing.T, original *twoPageVegetationOriginal) *twoPageVegetationOriginal {
	t.Helper()
	wire, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var clone twoPageVegetationOriginal
	if err := json.Unmarshal(wire, &clone); err != nil {
		t.Fatal(err)
	}
	return &clone
}

func TestTwoPageVegetationPlanBoundFieldsAndDuplicateLogicalIDs(t *testing.T) {
	for _, test := range []struct {
		form   string
		fields []string
	}{
		{"SubVegAXL_BC", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Collected"}},
		{"SubVegAXL", []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c", "Collected"}},
		{"SubVegCXL", []string{"Cover6", "Collected"}},
		{"SubVegDXL", []string{"Cover7", "Cover8", "Cover9", "Collected"}},
	} {
		t.Run(test.form, func(t *testing.T) {
			original := twoPageVegetationPlanFixture(t, test.form)
			before := cloneVegetationPlanOriginal(t, original)
			for _, column := range test.fields {
				for _, value := range []ProjectMetadataCell{siviReal(1.25), {Storage: "null"}} {
					if column == "Collected" && value.Storage == "real" {
						value = metadataText("X")
					}
					if column == "Collected" && value.Storage == "null" {
						*vegetationPlanCell(original, "-7", "Collected") = metadataText("C")
					}
					edit := vegetationPlanEdit(original, column, value)
					got, err := planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{edit})
					if err != nil || len(got) != 1 || got[0].RowID != "-7" || got[0].Column != column ||
						got[0].ID != -2147483648 || !reflect.DeepEqual(got[0].Before, edit.Expected) || !reflect.DeepEqual(got[0].After, value) {
						t.Fatalf("bound %s plan = %+v %v", column, got, err)
					}
					if column == "Collected" {
						*vegetationPlanCell(original, "-7", "Collected") = ProjectMetadataCell{Storage: "null"}
					}
				}
			}
			if !reflect.DeepEqual(original, before) {
				t.Fatal("planning changed complete physical original or duplicate-ID peer")
			}
		})
	}
}

func TestTwoPageVegetationPlanRetainsHistoricalValuesAndSourceDeparture(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegCXL")
	*vegetationPlanCell(original, "-7", "Cover6") = metadataText("historical invalid numeric")
	*vegetationPlanCell(original, "-7", "Collected") = metadataText("historical overlength")
	unchanged := []twoPageVegetationEdit{
		vegetationPlanEdit(original, "Cover6", metadataText("historical invalid numeric")),
		vegetationPlanEdit(original, "Collected", metadataText("historical overlength")),
	}
	got, err := planTwoPageVegetationEdits(context.Background(), original, unchanged)
	if err != nil || len(got) != 0 {
		t.Fatalf("unchanged historical invalid assignments were normalized: %+v %v", got, err)
	}
	got, err = planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{
		vegetationPlanEdit(original, "Cover6", ProjectMetadataCell{Storage: "null"}),
	})
	if err != nil || len(got) != 1 || got[0].Value != nil {
		t.Fatalf("clearing last source cover became a forbidden membership constraint: %+v %v", got, err)
	}
	if got[0].Before.Text == vegetationPlanCell(original, "-7", "Cover6").Text {
		t.Fatal("assignment retained a mutable original pointer")
	}
	*got[0].Before.Text = "independent assignment"
	if *vegetationPlanCell(original, "-7", "Cover6").Text != "historical invalid numeric" {
		t.Fatal("assignment mutation changed original")
	}
}

func TestTwoPageVegetationPlanValueRefusalsAndTextBounds(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegCXL")
	for _, test := range []struct {
		column string
		value  ProjectMetadataCell
	}{
		{"Cover6", metadataText("1.25")}, {"Cover6", metadataInteger("1")},
		{"Cover6", siviReal(math.Inf(1))}, {"Cover6", siviReal(math.NaN())},
		{"Cover6", siviReal(math.MaxFloat64)},
		{"Cover6", siviReal(math.Copysign(0, -1))},
		{"Collected", metadataText("CC")}, {"Collected", metadataText("\U0001f600")},
		{"Collected", metadataText(string([]byte{255}))}, {"Collected", siviReal(1)},
	} {
		if got, err := planTwoPageVegetationEdits(context.Background(), original,
			[]twoPageVegetationEdit{vegetationPlanEdit(original, test.column, test.value)}); err == nil || got != nil {
			t.Fatalf("malformed/new overlength %s value accepted: %+v %v", test.column, got, err)
		}
	}
	for _, text := range []string{"", "C", "V", "X", " ", "\u00e9"} {
		got, err := planTwoPageVegetationEdits(context.Background(), original,
			[]twoPageVegetationEdit{vegetationPlanEdit(original, "Collected", metadataText(text))})
		if err != nil || len(got) != 1 || got[0].Value != text {
			t.Fatalf("literal one-UTF16-unit/empty Collected text was restricted or trimmed: %+v %v", got, err)
		}
	}
}

func TestTwoPageVegetationPlanOwnerAndPhysicalWitnessRefusals(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegAXL_BC")
	edit := vegetationPlanEdit(original, "Cover1", siviReal(1))
	for _, change := range []func(*twoPageVegetationEdit){
		func(e *twoPageVegetationEdit) { e.ContextID += "-stale" },
		func(e *twoPageVegetationEdit) { e.Form = "SubVegAXL" },
		func(e *twoPageVegetationEdit) { e.RowID = "8" },
		func(e *twoPageVegetationEdit) { e.RowID = "9" },
		func(e *twoPageVegetationEdit) { e.RowID = "03" },
		func(e *twoPageVegetationEdit) { e.Column = "Species" },
		func(e *twoPageVegetationEdit) { e.Column = "Cover5a" },
		func(e *twoPageVegetationEdit) { e.Column = "Cover6" },
		func(e *twoPageVegetationEdit) { e.Expected = siviReal(0) },
	} {
		wrong := edit
		change(&wrong)
		if got, err := planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{wrong}); err == nil || got != nil {
			t.Fatalf("wrong source/identity/expected witness accepted: %+v %v", got, err)
		}
	}
	if got, err := planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{edit, edit}); err == nil || got != nil {
		t.Fatal("repeated physical field accepted")
	}
	for _, change := range []func(*twoPageVegetationOriginal){
		func(o *twoPageVegetationOriginal) { o.Table = "Other_Veg" },
		func(o *twoPageVegetationOriginal) { o.RecordSource = "USysVegC" },
		func(o *twoPageVegetationOriginal) { o.SourceRowIDs = append(o.SourceRowIDs, "-7") },
		func(o *twoPageVegetationOriginal) { o.Records.Rows[0].RowID = "-07" },
		func(o *twoPageVegetationOriginal) { o.Records.Rows[1].RowID = "-7" },
		func(o *twoPageVegetationOriginal) { o.Records.Rows[1].Cells = o.Records.Rows[1].Cells[:2] },
		func(o *twoPageVegetationOriginal) { *vegetationPlanCell(o, "3", "PlotNumber") = metadataText("other") },
		func(o *twoPageVegetationOriginal) {
			*vegetationPlanCell(o, "3", "Unmapped") = metadataText(string([]byte{255}))
		},
		func(o *twoPageVegetationOriginal) { o.Records.Columns[19].Name = "cover1" },
		func(o *twoPageVegetationOriginal) {
			for _, column := range []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover5a", "Cover5b", "Cover5c"} {
				*vegetationPlanCell(o, "-7", column) = ProjectMetadataCell{Storage: "null"}
			}
			o.SourceRowIDs = []string{"3", "8", "9"}
		},
	} {
		wrong := cloneVegetationPlanOriginal(t, original)
		change(wrong)
		if got, err := planTwoPageVegetationEdits(context.Background(), wrong, []twoPageVegetationEdit{edit}); err == nil || got != nil {
			t.Fatalf("malformed physical original accepted: %+v %v", got, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, err := planTwoPageVegetationEdits(ctx, original, []twoPageVegetationEdit{edit}); got != nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled planning accepted: %+v %v", got, err)
	}
	if got, err := planTwoPageVegetationEdits(nil, original, []twoPageVegetationEdit{edit}); got != nil || err == nil {
		t.Fatal("nil planning context accepted")
	}
}

func TestTwoPageVegetationEditStrictJSON(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegCXL")
	edit := vegetationPlanEdit(original, "Collected", metadataText("C"))
	wire, err := json.Marshal(edit)
	if err != nil {
		t.Fatal(err)
	}
	var decoded twoPageVegetationEdit
	if err := json.Unmarshal(wire, &decoded); err != nil || !reflect.DeepEqual(edit, decoded) {
		t.Fatalf("strict roundtrip failed: %+v %v", decoded, err)
	}
	for _, invalid := range []string{
		strings.Replace(string(wire), `"column":"Collected"`, `"column":"Collected","column":"Collected"`, 1),
		strings.Replace(string(wire), `"column":"Collected"`, `"column":"Collected","\u0063olumn":"Collected"`, 1),
		strings.Replace(string(wire), `"text":"C"`, `"text":"\ud800"`, 1),
		strings.Replace(string(wire), `"text":"C"`, `"text":"C","text":"V"`, 1),
		strings.Replace(string(wire), `"form":`, `"unknown":1,"form":`, 1),
		strings.Replace(string(wire), `"value":{`, `"value":null,"other":{`, 1),
		`null`, `[]`,
	} {
		if err := json.Unmarshal([]byte(invalid), &decoded); err == nil {
			t.Fatalf("strict vegetation transport accepted %s", invalid)
		}
	}

}

func TestTwoPageVegetationPlanExactRealWitness(t *testing.T) {
	original := twoPageVegetationPlanFixture(t, "SubVegCXL")
	*vegetationPlanCell(original, "-7", "Cover6") = siviReal(0)
	edit := vegetationPlanEdit(original, "Cover6", siviReal(1))
	edit.Expected = siviReal(math.Copysign(0, -1))
	if got, err := planTwoPageVegetationEdits(context.Background(), original, []twoPageVegetationEdit{edit}); err == nil || got != nil {
		t.Fatalf("different real-bit expected value was treated as an exact witness: %+v %v", got, err)
	}
}
