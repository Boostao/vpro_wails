package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func lifeformWorkbookFixture(t *testing.T, change func(*ProjectMetadataTable)) (LifeformSummaryPreview, SpeciesAttributeSummaryPreview) {
	t.Helper()
	veg, su, species, catalogue := lifeformFixture()
	if change != nil {
		change(&su)
	}
	names := []string{"Code", "Codetype"}
	for _, def := range speciesAttributeDefinitions() {
		names = append(names, def.Field)
	}
	attributes := lifeformTable(names,
		[]ProjectMetadataCell{metadataText("A"), metadataText("U"), metadataText("S1"), metadataText("1"),
			metadataText("I"), metadataText("R"), metadataText("0"), metadataText("6")},
		[]ProjectMetadataCell{metadataText("A"), metadataText("U"), metadataText("unlisted"), {Storage: "null"},
			metadataText("P"), metadataText("B"), metadataText("1"), {Storage: "null"}})
	lf, err := planLifeformSummary(context.Background(), "Project", "Selected", veg, su, species, catalogue)
	if err != nil {
		t.Fatal(err)
	}
	at, err := planSpeciesAttributeSummary(context.Background(), "Project", "Selected", veg, su, attributes)
	if err != nil {
		t.Fatal(err)
	}
	return LifeformSummaryPreview{"ctx", "project.sqlite", "su.sqlite", lf},
		SpeciesAttributeSummaryPreview{"ctx", "project.sqlite", "su.sqlite", at}
}

func openLifeformWorkbook(t *testing.T, result lifeformWorkbook) *excelize.File {
	t.Helper()
	if len(result.Bytes) == 0 {
		t.Fatal("no XLSX bytes")
	}
	book, err := excelize.OpenReader(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := book.Close(); err != nil {
			t.Error(err)
		}
	})
	return book
}

func lifeformWorkbookCell(t *testing.T, book *excelize.File, sheet, address, want string) {
	t.Helper()
	got, err := book.GetCellValue(sheet, address, excelize.Options{RawCellValue: true})
	if err != nil || got != want {
		t.Fatalf("%s!%s = %q, want %q (%v)", sheet, address, got, want, err)
	}
}

func renameLifeformWorkbookUnit(lf *LifeformSummaryPreview, at *SpeciesAttributeSummaryPreview, index int, code string) {
	old := vegetationTextKey(lf.Report.Units[index].Code)
	lf.Report.Units[index].Code = metadataText(code)
	at.Report.Units[index].Code = metadataText(code)
	for i := range lf.Report.Memberships {
		if vegetationTextKey(lf.Report.Memberships[i].SiteUnit) == old {
			lf.Report.Memberships[i].SiteUnit = metadataText(code)
			at.Report.Memberships[i].SiteUnit = metadataText(code)
		}
	}
	for i := range lf.Report.Entries {
		if vegetationTextKey(lf.Report.Entries[i].ProjectID) == old {
			lf.Report.Entries[i].ProjectID = metadataText(code)
		}
	}
}

func TestLifeformWorkbookSourceCellsStylesProvenanceArithmeticAndClones(t *testing.T) {
	lf, at := lifeformWorkbookFixture(t, nil)
	beforeLF, _ := json.Marshal(lf)
	beforeAT, _ := json.Marshal(at)
	layout := lifeformWorkbookLayout{Details: [6]bool{true, true, true, true, true, true}}
	result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, layout)
	if err != nil {
		t.Fatal(err)
	}
	book := openLifeformWorkbook(t, result)
	if len(result.Sheets) != 5 || len(book.GetSheetList()) != 6 {
		t.Fatal("source units omitted", result.Sheets, book.GetSheetList())
	}
	for i, name := range []string{"U", "V", "Empty", "Zero", "NoName4"} {
		if result.Sheets[i].Name != name || !reflect.DeepEqual(result.Sheets[i].Unit, lf.Report.Units[i].Code) {
			t.Fatal("source order or typed unit mapping changed", result.Sheets)
		}
	}
	for cell, want := range map[string]string{
		"A1": "Project: Project", "A2": "Site Unit Table: Selected", "A3": "Unit: U",
		"A4": "nPlots: 2", "A5": "Number of unique species: 2", "A6": "Total species occurences : 24",
		"B8": "^Count", "C8": "^Number of plot occurences",
		"A10": "SRank", "A11": "Wetland_Ind", "A12": "WeedStatus", "A13": "RedBlueList",
		"A14": "ASMR", "A15": "Climate",
		"B10": "8", "C10": "1", "B11": "4", "C11": "1",
		"A18": "Lifeform", "B18": "Presence", "C18": "Mean Cover",
		"A19": "2", "B19": "0.5", "C19": "0", "A20": "1", "B20": "0.5", "C20": "1.6",
		"E8": "SiteUnit", "E9": "nSRankDetail", "F8": "S1", "F9": "4", "G8": "S2", "G9": "",
		"Y8": "SX", "Y9": "", "E12": "SiteUnit", "E13": "nWetland_Ind",
		"E17": "nWeedStatusDetail", "E21": "nRedBlueListDetail",
		"E25": "nASMRDetail", "E29": "nClimateDetail",
	} {
		lifeformWorkbookCell(t, book, "U", cell, want)
	}
	merges, err := book.GetMergeCells("U")
	if err != nil || len(merges) != 2 {
		t.Fatal("summary header merges", merges, err)
	}
	mergeNames := map[string]bool{}
	for _, merge := range merges {
		mergeNames[merge.GetStartAxis()+":"+merge.GetEndAxis()] = true
	}
	if !mergeNames["B8:B9"] || !mergeNames["C8:C9"] {
		t.Fatal(mergeNames)
	}
	for _, cell := range []string{"B19", "C20", "B31", "C31"} {
		id, err := book.GetCellStyle("U", cell)
		if err != nil {
			t.Fatal(err)
		}
		style, err := book.GetStyle(id)
		if err != nil || style.CustomNumFmt == nil || *style.CustomNumFmt != "0.0%" {
			t.Fatal("source numeric percent style lost", cell, style, err)
		}
	}
	for _, cell := range []string{"A1", "A6", "B8", "A18", "E8"} {
		id, err := book.GetCellStyle("U", cell)
		if err != nil {
			t.Fatal(err)
		}
		style, err := book.GetStyle(id)
		if err != nil || style.Font == nil || !style.Font.Bold {
			t.Fatal("source bold header style lost", cell, style, err)
		}
	}
	for _, cell := range []string{"A19", "B10", "C10", "B19", "C20", "F9"} {
		kind, err := book.GetCellType("U", cell)
		if err != nil || kind == excelize.CellTypeSharedString || kind == excelize.CellTypeInlineString {
			t.Fatal("numeric source output coerced to text", cell, kind, err)
		}
	}
	for _, sheet := range []string{"Empty", "Zero", "NoName4"} {
		lifeformWorkbookCell(t, book, sheet, "B10", "")
		lifeformWorkbookCell(t, book, sheet, "C10", "0")
		lifeformWorkbookCell(t, book, sheet, "C19", "")
		lifeformWorkbookCell(t, book, sheet, "F9", "")
	}
	lifeformWorkbookCell(t, book, "Zero", "B19", "")
	lifeformWorkbookCell(t, book, "Empty", "B19", "0")
	var encoded strings.Builder
	for row := 1; ; row++ {
		chunk, err := book.GetCellValue("_VPRO_Source", "A"+strconv.Itoa(row))
		if err != nil {
			t.Fatal(err)
		}
		if chunk == "" {
			break
		}
		encoded.WriteString(chunk)
	}
	source, err := hex.DecodeString(encoded.String())
	if err != nil {
		t.Fatal(err)
	}
	var provenance struct {
		Lifeform   LifeformSummaryPreview
		Attributes SpeciesAttributeSummaryPreview
		Layout     lifeformWorkbookLayout
	}
	if err := json.Unmarshal(source, &provenance); err != nil ||
		!reflect.DeepEqual(provenance.Lifeform, lf) || !reflect.DeepEqual(provenance.Attributes, at) ||
		provenance.Layout != layout {
		t.Fatal("complete typed provenance changed", err)
	}
	archive, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range archive.File {
		if file.Name != "xl/workbook.xml" {
			continue
		}
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil || closeErr != nil {
			t.Fatal(err, closeErr)
		}
		found = strings.Contains(string(data), `name="_VPRO_Source"`) && strings.Contains(string(data), `state="veryHidden"`)
	}
	if !found {
		t.Fatal("provenance is not very hidden")
	}
	afterLF, _ := json.Marshal(lf)
	afterAT, _ := json.Marshal(at)
	if !bytes.Equal(beforeLF, afterLF) || !bytes.Equal(beforeAT, afterAT) {
		t.Fatal("preparation mutated input previews")
	}
	*result.Sheets[0].Unit.Text = "changed output"
	if *lf.Report.Units[0].Code.Text != "U" || *at.Report.Units[0].Code.Text != "U" {
		t.Fatal("sheet mapping aliases input typed identities")
	}
}

func TestLifeformWorkbookIdenticalInputsProduceIdenticalBytes(t *testing.T) {
	lf, at := lifeformWorkbookFixture(t, nil)
	for _, details := range [][6]bool{{}, {true, true, true, true, true, true}} {
		t.Run(fmt.Sprint(details), func(t *testing.T) {
			layout := lifeformWorkbookLayout{Details: details}
			baseline, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, layout)
			if err != nil {
				t.Fatal(err)
			}
			book := openLifeformWorkbook(t, baseline)
			lifeformWorkbookCell(t, book, "U", "B8", "^Count")
			lifeformWorkbookCell(t, book, "U", "C8", "^Number of plot occurences")
			for repeat := 0; repeat < 8; repeat++ {
				result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, layout)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(result.Bytes, baseline.Bytes) || !reflect.DeepEqual(result.Sheets, baseline.Sheets) {
					t.Fatalf("identical owned inputs changed approval bytes or sheet mappings on repetition %d", repeat+1)
				}
			}
		})
	}
}

func TestLifeformWorkbookAllDetailVectorsOnlyChangeDetails(t *testing.T) {
	lf, at := lifeformWorkbookFixture(t, func(su *ProjectMetadataTable) { su.Rows = su.Rows[:2] })
	labels := []string{"nSRankDetail", "nWetland_Ind", "nWeedStatusDetail", "nRedBlueListDetail", "nASMRDetail", "nClimateDetail"}
	for mask := 0; mask < 64; mask++ {
		t.Run(fmt.Sprintf("%06b", mask), func(t *testing.T) {
			var layout lifeformWorkbookLayout
			for j := range layout.Details {
				layout.Details[j] = mask&(1<<j) != 0
			}
			result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, layout)
			if err != nil {
				t.Fatal(err)
			}
			book := openLifeformWorkbook(t, result)
			detailRow := 8
			for j, enabled := range layout.Details {
				row := at.Report.Units[0].Rows[j]
				want := ""
				if row.Count != nil {
					want = strconv.Itoa(*row.Count)
				}
				lifeformWorkbookCell(t, book, "U", fmt.Sprintf("B%d", j+10), want)
				lifeformWorkbookCell(t, book, "U", fmt.Sprintf("C%d", j+10), strconv.Itoa(row.PlotOccurrences))
				if !enabled {
					continue
				}
				lifeformWorkbookCell(t, book, "U", fmt.Sprintf("E%d", detailRow), "SiteUnit")
				lifeformWorkbookCell(t, book, "U", fmt.Sprintf("E%d", detailRow+1), labels[j])
				for k, category := range at.Report.Definitions[j].Categories {
					cell, _ := excelize.CoordinatesToCellName(k+6, detailRow)
					lifeformWorkbookCell(t, book, "U", cell, category)
					cell, _ = excelize.CoordinatesToCellName(k+6, detailRow+1)
					want := ""
					if row.Categories[k] != nil {
						want = strconv.Itoa(*row.Categories[k])
					}
					lifeformWorkbookCell(t, book, "U", cell, want)
				}
				detailRow += 4
			}
			lifeformWorkbookCell(t, book, "U", fmt.Sprintf("E%d", detailRow), "")
			lifeformWorkbookCell(t, book, "U", "C20", "0.8")
		})
	}
}

func TestLifeformWorkbookNullEmptyNamesFormulaTextAndSurrogateBoundaries(t *testing.T) {
	lf, at := lifeformWorkbookFixture(t, func(su *ProjectMetadataTable) {
		su.Rows = append(su.Rows, ProjectMetadataRow{RowID: "7", Cells: []ProjectMetadataCell{
			metadataText("P"), metadataText("")}})
	})
	result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, lifeformWorkbookLayout{})
	if err != nil {
		t.Fatal(err)
	}
	book := openLifeformWorkbook(t, result)
	if result.Sheets[4].Unit.Storage != "null" || result.Sheets[5].Unit.Storage != "text" ||
		result.Sheets[4].Name != "NoName4" || result.Sheets[5].Name != "NoName5" {
		t.Fatal("NULL and empty identity merged", result.Sheets)
	}
	lifeformWorkbookCell(t, book, "NoName5", "A3", "Unit: NoName5")
	for _, code := range []string{"=1+1", "+formula", "@formula", "-formula", strings.Repeat("x", 29) + "😀tail", "a/b"} {
		t.Run(code, func(t *testing.T) {
			lf, at := lifeformWorkbookFixture(t, func(su *ProjectMetadataTable) { su.Rows = su.Rows[:2] })
			renameLifeformWorkbookUnit(&lf, &at, 0, code)
			lf.Report.Project, at.Report.Project = "=SUM(1,2)", "=SUM(1,2)"
			result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, lifeformWorkbookLayout{})
			if err != nil {
				t.Fatal(err)
			}
			book := openLifeformWorkbook(t, result)
			name := result.Sheets[0].Name
			if code == "a/b" && name != "a-b" || strings.HasSuffix(code, "tail") && name != strings.Repeat("x", 29)+"😀" {
				t.Fatal("source sanitization/truncation changed", name)
			}
			for _, address := range []string{"A1", "A3"} {
				formula, err := book.GetCellFormula(name, address)
				if err != nil || formula != "" {
					t.Fatal("formula-looking text interpreted", address, formula, err)
				}
			}
			lifeformWorkbookCell(t, book, name, "A1", "Project: =SUM(1,2)")
		})
	}
}

func TestLifeformWorkbookRejectsMalformedOwnedSourceWithoutPartialBytes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LifeformSummaryPreview, *SpeciesAttributeSummaryPreview)
	}{
		{"context", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { a.ContextID = "different" }},
		{"project", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { a.Report.Project = "different" }},
		{"su", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { a.Report.SU = "different" }},
		{"project path", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { a.ProjectPath = "different" }},
		{"su path", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { a.SUPath = "different" }},
		{"empty ownership", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { l.ContextID, a.ContextID = "", "" }},
		{"no units", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units, a.Report.Units = nil, nil
		}},
		{"missing memberships", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Memberships, a.Report.Memberships = nil, nil
		}},
		{"duplicate memberships", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Memberships[1] = l.Report.Memberships[0]
			a.Report.Memberships[1] = a.Report.Memberships[0]
		}},
		{"membership order", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Memberships[0], a.Report.Memberships[1] = a.Report.Memberships[1], a.Report.Memberships[0]
		}},
		{"omitted units", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units, a.Report.Units = l.Report.Units[:4], a.Report.Units[:4]
		}},
		{"unit identity", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].Code = metadataText("V")
		}},
		{"physical plot count", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[0].NPlots++
			a.Report.Units[0].NPlots++
		}},
		{"unit rowids", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].SURowIDs[0] = "missing"
		}},
		{"catalogue omitted row", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[0].Rows = l.Report.Units[0].Rows[1:]
		}},
		{"catalogue association", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[0].Rows[0].CatalogueRowID = "missing"
		}},
		{"catalogue numeric association", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[0].Rows[0].Lifeform++
		}},
		{"duplicate catalogue", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Catalogue[1] = l.Report.Catalogue[0]
		}},
		{"attribute definitions", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Definitions[0].Categories[0] = "unknown"
		}},
		{"attribute labels", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Definitions[0].Label = "changed"
		}},
		{"attribute category shape", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].Rows[0].Categories = nil
		}},
		{"attribute summary omitted", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].Rows = a.Report.Units[0].Rows[:5]
		}},
		{"attribute field", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].Rows[0].Field = "other"
		}},
		{"nan", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*l.Report.Units[0].Rows[0].Presence = math.NaN()
		}},
		{"inf", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*l.Report.Units[0].Rows[0].MeanCover = math.Inf(1)
		}},
		{"presence denominator", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*l.Report.Units[0].Rows[0].Presence = .9
		}},
		{"zero denominator ratio", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[3].Rows[0].Presence = new(float64)
		}},
		{"negative occurrence", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) { l.Report.Units[0].Occurrences = -1 }},
		{"negative attribute", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*a.Report.Units[0].Rows[0].Count = -1
		}},
		{"zero nonnull category", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*a.Report.Units[0].Rows[0].Categories[0] = 0
		}},
		{"category beyond total", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*a.Report.Units[0].Rows[0].Categories[0] = 9
		}},
		{"invalid cover count", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Units[0].Rows[0].CoverCount = -1
		}},
		{"null count with occurrence", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Units[0].Rows[0].Count = nil
		}},
		{"malformed typed null", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Catalogue[0].Definition = metadataText("ignored")
			l.Report.Catalogue[0].Definition.Storage = "null"
		}},
		{"nontext metadata", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Catalogue[0].Label = metadataInteger("1")
		}},
		{"bad unicode", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*l.Report.Catalogue[0].Label.Text = string([]byte{0xff})
		}},
		{"xml control", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Entries[0].Species = "A\x00"
		}},
		{"xml noncharacter", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*a.Report.Matches[0].Values[0].Text = "\ufffe"
		}},
		{"overlength text", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			*l.Report.Catalogue[0].Label.Text = strings.Repeat("😀", 16384)
		}},
		{"entry owner", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Entries[0].InsertSURowID = "missing"
		}},
		{"entry nonfinite", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Entries[0].Cover = math.Inf(1)
		}},
		{"entry missing single assignment", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			l.Report.Entries[0].Cover = 0.1
		}},
		{"attribute match owner", func(l *LifeformSummaryPreview, a *SpeciesAttributeSummaryPreview) {
			a.Report.Matches[0].SURowID = "missing"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lf, at := lifeformWorkbookFixture(t, nil)
			test.mutate(&lf, &at)
			result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, lifeformWorkbookLayout{})
			if err == nil || !reflect.DeepEqual(result, lifeformWorkbook{}) {
				t.Fatal("invalid source returned partial workbook", result.Sheets, len(result.Bytes), err)
			}
		})
	}
	for _, codes := range [][2]string{{"a/b", "a\\b"}, {"U", "u"},
		{strings.Repeat("a", 31) + "1", strings.Repeat("a", 31) + "2"},
		{"NoName4", "other"}, {"_VPRO_Source", "other"},
		{strings.Repeat("a", 30) + "😀", "other"}} {
		t.Run("names/"+codes[0], func(t *testing.T) {
			lf, at := lifeformWorkbookFixture(t, nil)
			renameLifeformWorkbookUnit(&lf, &at, 0, codes[0])
			renameLifeformWorkbookUnit(&lf, &at, 1, codes[1])
			result, err := prepareLifeformSummaryWorkbook(context.Background(), lf, at, lifeformWorkbookLayout{})
			if err == nil || !reflect.DeepEqual(result, lifeformWorkbook{}) {
				t.Fatal("worksheet collision/split accepted", result.Sheets, err)
			}
		})
	}
}

type lifeformWorkbookCancelContext struct {
	context.Context
	calls int
	after int
}

func (ctx *lifeformWorkbookCancelContext) Err() error {
	ctx.calls++
	if ctx.calls >= ctx.after {
		return context.Canceled
	}
	return nil
}

func TestLifeformWorkbookCancellationNoBytesIncludingFinalSerialization(t *testing.T) {
	lf, at := lifeformWorkbookFixture(t, nil)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, ctx := range []context.Context{nil, canceled} {
		result, err := prepareLifeformSummaryWorkbook(ctx, lf, at, lifeformWorkbookLayout{})
		if err == nil || !reflect.DeepEqual(result, lifeformWorkbook{}) {
			t.Fatal("nil/cancelled context returned bytes", err)
		}
	}
	counter := &lifeformWorkbookCancelContext{Context: context.Background(), after: math.MaxInt}
	if _, err := prepareLifeformSummaryWorkbook(counter, lf, at, lifeformWorkbookLayout{}); err != nil {
		t.Fatal(err)
	}
	for _, after := range []int{3, counter.calls - 1, counter.calls} {
		ctx := &lifeformWorkbookCancelContext{Context: context.Background(), after: after}
		result, err := prepareLifeformSummaryWorkbook(ctx, lf, at, lifeformWorkbookLayout{})
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, lifeformWorkbook{}) {
			t.Fatal("cancellation failed to clear final workbook", after, len(result.Bytes), err)
		}
	}
}
