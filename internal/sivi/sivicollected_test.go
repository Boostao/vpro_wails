package sivi

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestSIVICollectedPlanningSourceGroupsAndCycles(t *testing.T) {
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	for _, extended := range []bool{false, true} {
		a := "SubVegA-SIVI_BC"
		if extended {
			a = "SubVegA-SIVI"
		}
		for _, source := range []struct{ row, form string }{{"1", a}, {"4", "SubVegC-SIVI"}, {"6", "SubVegD-SIVI"}} {
			for _, cell := range []vprocore.ProjectMetadataCell{
				null, vprotest.MetadataText("C"), vprotest.MetadataText("V"), vprotest.MetadataText("c"), vprotest.MetadataText("v"),
				vprotest.MetadataText("Ｃ"), vprotest.MetadataText("ｃ"), vprotest.MetadataText("Ｖ"), vprotest.MetadataText("ｖ"),
				vprotest.MetadataText("?"), vprotest.MetadataText(""), vprotest.MetadataText(" C"), vprotest.MetadataText(strings.Repeat("x", 256)),
			} {
				for clicks := 1; clicks <= 3; clicks++ {
					veg := vprotest.SiviProjectionFixture()
					for i := range veg.Rows {
						if veg.Rows[i].RowID == source.row {
							veg.Rows[i].Cells[15] = vprocore.CloneSiteUnitCell(cell)
						}
					}
					draft := SIVICollectedEdit{source.row, source.form, cell, clicks}
					got, err := planSIVICollectedEdits(context.Background(), "P", extended, veg, []SIVICollectedEdit{draft})
					next := vprocore.CollectedAfterClicks(cell.Text, clicks)
					want := null
					if next != nil {
						want = vprotest.MetadataText(*next)
					}
					changed := !reflect.DeepEqual(cell, want)
					if err != nil || got == nil || len(got) != map[bool]int{false: 0, true: 1}[changed] {
						t.Fatalf("%s/%v/%d: %v %v", source.form, cell, clicks, got, err)
					}
					if changed && (got[0].RowID != source.row || got[0].Column != "Collected" ||
						!reflect.DeepEqual(got[0].Before, cell) || !reflect.DeepEqual(got[0].After, want)) {
						t.Fatal("typed cycle differs", got, want)
					}
				}
			}
		}
	}
}

func TestSIVICollectedPlanningRejectsUnavailableStaleRepeatedAndNonText(t *testing.T) {
	null := vprocore.ProjectMetadataCell{Storage: "null"}
	good := SIVICollectedEdit{"1", "SubVegA-SIVI_BC", null, 1}
	bad := []SIVICollectedEdit{
		{"1", good.Form, null, 0}, {"1", good.Form, null, -1}, {"1", good.Form, null, 4},
		{"1", good.Form, vprotest.MetadataText(""), 1}, {"1", good.Form, vprotest.MetadataText("C"), 1},
		{"1", "SubVegA-SIVI", null, 1}, {"1", "SubVegC-SIVI", null, 1},
		{"1", "SubVegD-SIVI", null, 1}, {"1", "foreign", null, 1},
		{"3", good.Form, null, 1}, {"8", good.Form, null, 1}, {"9", good.Form, null, 1},
		{"foreign", good.Form, null, 1}, {"01", good.Form, null, 1},
		{"9223372036854775808", good.Form, null, 1},
		{"1", good.Form, vprocore.ProjectMetadataCell{Storage: "text"}, 1},
		{"1", good.Form, vprocore.ProjectMetadataCell{Storage: "null", Text: vprotest.MetadataText("C").Text}, 1},
		{"1", good.Form, vprotest.MetadataText("\xff"), 1},
	}
	for _, edit := range bad {
		if got, err := planSIVICollectedEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), []SIVICollectedEdit{good, edit}); err == nil || got != nil {
			t.Fatal("invalid trailing draft returned partial plan", edit, got, err)
		}
	}
	for _, edits := range [][]SIVICollectedEdit{
		nil, {good, good},
		{{"7", good.Form, null, 1}, {"7", "SubVegC-SIVI", null, 1}},
		{{"7", good.Form, null, 3}, {"7", "SubVegD-SIVI", null, 1}},
	} {
		if got, err := planSIVICollectedEdits(context.Background(), "P", false, vprotest.SiviProjectionFixture(), edits); err == nil || got != nil {
			t.Fatal("empty/repeated physical cell accepted", got, err)
		}
	}
	for _, cell := range []vprocore.ProjectMetadataCell{vprotest.MetadataInteger("1"), vprotest.SiviReal(1), {Storage: "blob", BlobHex: vprotest.MetadataText("ff").Text}} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[0].Cells[15] = cell
		got, err := planSIVICollectedEdits(context.Background(), "P", false, veg, []SIVICollectedEdit{{"1", good.Form, cell, 3}})
		if err == nil || got != nil || !strings.Contains(err.Error(), "not cyclable") {
			t.Fatal("non-text historical Collected was coerced or silently defaulted", got, err)
		}
	}
	if got, err := planSIVICollectedEdits(context.Background(), "Other", false, vprotest.SiviProjectionFixture(), []SIVICollectedEdit{good}); err == nil || got != nil {
		t.Fatal("foreign plot accepted", got, err)
	}
}

type siviCollectedCancelContext struct {
	context.Context
	calls, at int
}

func (ctx *siviCollectedCancelContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.at {
		return context.Canceled
	}
	return nil
}

func TestSIVICollectedPlanningClonesAndCancellation(t *testing.T) {
	veg := vprotest.SiviProjectionFixture()
	veg.Rows[0].Cells[15] = vprotest.MetadataText("c")
	original := vprotest.SiviProjectionFixture()
	original.Rows[0].Cells[15] = vprotest.MetadataText("c")
	edit := SIVICollectedEdit{"1", "SubVegA-SIVI_BC", vprotest.MetadataText("c"), 1}
	got, err := planSIVICollectedEdits(context.Background(), "P", false, veg, []SIVICollectedEdit{edit})
	if err != nil || len(got) != 1 || got[0].Value != "V" {
		t.Fatal(got, err)
	}
	*got[0].Before.Text, *got[0].After.Text = "changed", "changed"
	if !reflect.DeepEqual(veg, original) || *edit.Expected.Text != "c" {
		t.Fatal("plan aliases source or draft")
	}
	for _, ctx := range []context.Context{nil, &siviCollectedCancelContext{Context: context.Background(), at: 1}, &siviCollectedCancelContext{Context: context.Background(), at: 4}} {
		if got, err := planSIVICollectedEdits(ctx, "P", false, veg, []SIVICollectedEdit{edit}); err == nil || got != nil {
			t.Fatal("nil/canceled planning succeeded", got, err)
		}
	}
	complete := &siviCollectedCancelContext{Context: context.Background(), at: 1000}
	if _, err := planSIVICollectedEdits(complete, "P", false, veg, []SIVICollectedEdit{edit}); err != nil {
		t.Fatal(err)
	}
	for _, at := range []int{complete.calls - 1, complete.calls} {
		ctx := &siviCollectedCancelContext{Context: context.Background(), at: at}
		if got, err := planSIVICollectedEdits(ctx, "P", false, veg, []SIVICollectedEdit{edit}); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("late cancellation returned partial/successful plan", at, got, err)
		}
	}
}

func TestSIVICollectedPlanningKeepsSigned64PhysicalIdentity(t *testing.T) {
	for _, rowID := range []string{"-9223372036854775808", "9223372036854775807", "9007199254740993", "0"} {
		veg := vprotest.SiviProjectionFixture()
		veg.Rows[0].RowID = rowID
		got, err := planSIVICollectedEdits(context.Background(), "P", false, veg,
			[]SIVICollectedEdit{{rowID, "SubVegA-SIVI_BC", vprocore.ProjectMetadataCell{Storage: "null"}, 1}})
		if err != nil || len(got) != 1 || got[0].RowID != rowID || got[0].Value != "C" {
			t.Fatal("physical signed64 identity rounded or replaced with application ID", rowID, got, err)
		}
	}
}

func TestSIVICollectedStrictTransport(t *testing.T) {
	valid := `{"rowId":"1","form":"SubVegA-SIVI_BC","expected":{"storage":"text","text":"C"},"clicks":1}`
	var edit SIVICollectedEdit
	if err := json.Unmarshal([]byte(valid), &edit); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{
		`{}`, `null`, `[]`, valid + `{}`,
		strings.Replace(valid, `"rowId"`, `"RowId"`, 1),
		strings.Replace(valid, `"clicks":1`, `"clicks":null`, 1),
		strings.Replace(valid, `"clicks":1`, `"clicks":1.5`, 1),
		strings.Replace(valid, `"clicks":1`, `"value":null,"clicks":1`, 1),
		strings.Replace(valid, `"clicks":1`, `"column":"Collected","clicks":1`, 1),
		strings.Replace(valid, `"clicks":1`, `"clicks":2,"clicks":1`, 1),
		strings.Replace(valid, `"expected":`, `"expected":null,"expected":`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"\ud800"`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"\udc00"`, 1),
		strings.Replace(valid, `"text":"C"`, `"text":"`+"\xff"+`"`, 1),
	} {
		if err := json.Unmarshal([]byte(raw), &edit); err == nil {
			t.Fatal("invalid/repaired Collected transport accepted", raw)
		}
	}
	for _, property := range []string{"rowId", "form", "expected", "clicks"} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(valid), &fields); err != nil {
			t.Fatal(err)
		}
		delete(fields, property)
		raw, _ := json.Marshal(fields)
		if err := json.Unmarshal(raw, &edit); err == nil {
			t.Fatal("missing explicit draft property", property)
		}
	}
}
