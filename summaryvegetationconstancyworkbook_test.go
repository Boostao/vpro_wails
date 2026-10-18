package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func summaryConstancyWorkbookOptions() SummaryVegetationWorkbookOptions {
	grouping := 13
	return SummaryVegetationWorkbookOptions{Title: "=Constancy", Average: 1, Display: 1, ShowClass: 20,
		GroupAndOrder: &grouping, References: &SummaryVegetationReferenceOptions{}, Constancy: true}
}

func TestSummaryConstancyWorkbookStrictRequests(t *testing.T) {
	const base = `{"title":"=Constancy","average":1,"display":1,"showClass":20,"includeEnv":false,"references":{"showCommon":false,"useGuideNames":false},"groupAndOrder":13`
	for _, suffix := range []string{
		`,"constancy":false}`, `,"constancy":null}`, `,"constancy":1}`, `,"constancy":"true"}`,
		`,"Constancy":true}`, `,"constancy":true,"constancy":true}`,
		`,"constancy":true,"\u0063onstancy":true}`, `,"constancy":true} {}`,
	} {
		if err := json.Unmarshal([]byte(base+suffix), new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("ambiguous Constancy request accepted", suffix)
		}
	}
	valid := base + `,"constancy":true}`
	for _, mutation := range [][2]string{
		{`"groupAndOrder":13`, `"groupAndOrder":12`},
		{`"groupAndOrder":13`, `"groupAndOrder":14`},
		{`,"groupAndOrder":13`, ""},
		{`"display":1`, `"display":6`},
		{`"showCommon":false`, `"showCommon":true`},
		{`"useGuideNames":false`, `"useGuideNames":true`},
		{`,"references":{"showCommon":false,"useGuideNames":false}`, ""},
	} {
		if err := json.Unmarshal([]byte(strings.Replace(valid, mutation[0], mutation[1], 1)),
			new(SummaryVegetationWorkbookOptions)); err == nil {
			t.Fatal("incompatible Constancy calculation accepted", mutation)
		}
	}
	for _, selected := range []bool{false, true} {
		request := base + "}"
		if selected {
			request = valid
		}
		var options SummaryVegetationWorkbookOptions
		if err := json.Unmarshal([]byte(request), &options); err != nil || options.Constancy != selected || options.ShowClass != 20 {
			t.Fatal("explicit mode or retained class changed", options, err)
		}
		data, err := json.Marshal(options)
		if err != nil || string(data) != request {
			t.Fatal("legacy omission or Constancy wire changed", string(data), request, err)
		}
	}
}

func summaryConstancyWorkbookFixture() summaryVegetationWorkbookInput {
	input := summaryVegetationWorkbookFixture()
	preview := &input.Vegetation
	preview.Display, preview.GroupAndOrder = 1, 13
	preview.Source, preview.Ordering = summaryVegetationConstancySource, summaryVegetationConstancyOrdering
	preview.GroupField = &SummaryVegetationField{Name: "Lifeform", DAOType: 4, Size: 4}
	preview.OrderField = &SummaryVegetationField{Name: "Order", DAOType: 10}
	preview.References = &SummaryVegetationReferenceReview{MasterRows: 5, UnionRows: 5}
	preview.Rows = nil
	for i, record := range []struct{ lifeform, name, code string }{
		{"", "Missing", "S"}, {"-1", "Negative", "U"},
		{"1", "Alpha", "X"}, {"1", "Alpha", "Y"},
		{"2", "Second", "Z"}, {"128", "Wide", "W"},
	} {
		group := ProjectMetadataCell{Storage: "null"}
		if record.lifeform != "" {
			group = metadataInteger(record.lifeform)
		}
		raw := metadataText(record.code)
		preview.Rows = append(preview.Rows, SummaryVegetationRow{Layer: group, Species: metadataText(record.name),
			RawSpecies: &raw, Order: metadataText("1.00"),
			Covers: []ProjectMetadataCell{metadataText("0.75 - 14.00"), ProjectMetadataCell{Storage: "null"}},
			Witnesses: []SummaryVegetationWitness{{StageRowID: fmt.Sprint(i + 1), MembershipID: "1",
				Unit: "A", Cover: ProjectMetadataCell{Storage: "null"}}}})
	}
	return input
}

func TestSummaryConstancyWorkbookTypedBodyAndCaption(t *testing.T) {
	for _, include := range []bool{false, true} {
		input := summaryConstancyWorkbookFixture()
		first, caption := 6, "Vegetation Table - Constancy Format"
		if include {
			environment := summaryEnvironmentalWorkbookFixture(t)
			environment.Units = summaryEnvironmentalPreviewUnits(input.Vegetation)
			input.Environment = &environment
			first, caption = 10, "Site Unit Summary Table"
		}
		before, err := json.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
		if err != nil || result.FirstVegetationRow != first {
			t.Fatal("numeric Constancy body unavailable", result.FirstVegetationRow, err)
		}
		book := openSummaryEnvironmentalWorkbook(t, summaryEnvironmentalWorkbook{Bytes: result.Bytes})
		actual, err := book.GetCellValue(result.Sheet, "A2")
		if err != nil || actual != caption {
			t.Fatal("Constancy/environment caption precedence changed", actual, caption, err)
		}
		for i, expected := range [][]string{
			{"", "Missing"}, {"-1", "Negative"}, {"1", "Alpha"},
			{"1", "Alpha"}, {"2", "Second"}, {"128", "Wide"},
		} {
			for column, want := range append(expected, "1.00", "0.75 - 14.00", "") {
				address, err := excelize.CoordinatesToCellName(column+1, first+i)
				if err != nil {
					t.Fatal(err)
				}
				value, err := book.GetCellValue(result.Sheet, address, excelize.Options{RawCellValue: true})
				formula, formulaErr := book.GetCellFormula(result.Sheet, address)
				if err != nil || formulaErr != nil || value != want || formula != "" {
					t.Fatal("independent complete Constancy body differs", address, value, want, err, formulaErr)
				}
				if column == 0 && want != "" {
					kind, err := book.GetCellType(result.Sheet, address)
					if err != nil || kind != excelize.CellTypeNumber && kind != excelize.CellTypeUnset {
						t.Fatal("original LONG group became text", address, kind, err)
					}
				}
			}
		}
		for i := range input.Vegetation.Rows {
			address := fmt.Sprintf("F%d", first+i)
			value, err := book.GetCellValue(result.Sheet, address)
			if err != nil || value != "" {
				t.Fatal("invented visible raw-code column", address, value, err)
			}
		}
		if !bytes.Equal(summaryEnvironmentalWorkbookSource(t, book), before) {
			t.Fatal("complete numeric/raw-species provenance was lost")
		}
		after, err := json.Marshal(input)
		again, repeatErr := prepareSummaryVegetationWorkbook(t.Context(), input)
		if err != nil || repeatErr != nil || !bytes.Equal(before, after) || !bytes.Equal(result.Bytes, again.Bytes) {
			t.Fatal("Constancy renderer mutated input or became nondeterministic", err, repeatErr)
		}
	}
}

func TestSummaryConstancyWorkbookRejectsAuthorityAndIdentityDrift(t *testing.T) {
	for name, mutate := range map[string]func(*SummaryVegetationPreview){
		"ordinary-source": func(p *SummaryVegetationPreview) { p.Source = "literal-normal-su-effective-references" },
		"ordering":        func(p *SummaryVegetationPreview) { p.Ordering = "literal-null-first-group-species" },
		"ordinary-field":  func(p *SummaryVegetationPreview) { p.GroupField = &SummaryVegetationField{"LifeForm", 10, 3} },
		"group":           func(p *SummaryVegetationPreview) { p.GroupAndOrder = 12 },
		"display":         func(p *SummaryVegetationPreview) { p.Display = 6 },
		"reference":       func(p *SummaryVegetationPreview) { p.References = nil },
		"common":          func(p *SummaryVegetationPreview) { p.References.ShowCommon = true },
		"guide":           func(p *SummaryVegetationPreview) { p.References.UseGuideNames = true },
		"raw-missing":     func(p *SummaryVegetationPreview) { p.Rows[0].RawSpecies = nil },
		"raw-duplicate":   func(p *SummaryVegetationPreview) { p.Rows[3].RawSpecies = p.Rows[2].RawSpecies },
		"raw-overlength":  func(p *SummaryVegetationPreview) { raw := metadataText("123456789"); p.Rows[0].RawSpecies = &raw },
		"text-group":      func(p *SummaryVegetationPreview) { p.Rows[1].Layer = metadataText("-1") },
		"wide-group":      func(p *SummaryVegetationPreview) { p.Rows[5].Layer = metadataInteger("2147483648") },
		"numeric-order":   func(p *SummaryVegetationPreview) { p.Rows[4], p.Rows[5] = p.Rows[5], p.Rows[4] },
		"raw-order":       func(p *SummaryVegetationPreview) { p.Rows[2], p.Rows[3] = p.Rows[3], p.Rows[2] },
	} {
		t.Run(name, func(t *testing.T) {
			input := summaryConstancyWorkbookFixture()
			mutate(&input.Vegetation)
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if err == nil || len(result.Bytes) != 0 {
				t.Fatal("Constancy drift returned success/partial bytes", err)
			}
		})
	}
}

func TestSummaryConstancyWorkbookNumericLeadingOrdering(t *testing.T) {
	for name, test := range map[string]struct {
		rows    []int
		species []string
		valid   bool
	}{
		"negative-leading":   {rows: []int{1, 2, 3, 4, 5}, valid: true},
		"raw-ties":           {rows: []int{2, 3}, valid: true},
		"duplicate":          {rows: []int{2, 2}},
		"descending-groups":  {rows: []int{4, 2}},
		"descending-species": {rows: []int{2, 3}, species: []string{"Zulu", "Alpha"}},
		"descending-raw":     {rows: []int{3, 2}},
		"null-after-numeric": {rows: []int{2, 0}},
	} {
		t.Run(name, func(t *testing.T) {
			input := summaryConstancyWorkbookFixture()
			rows := input.Vegetation.Rows
			input.Vegetation.Rows = nil
			for i, index := range test.rows {
				row := rows[index]
				if test.species != nil {
					row.Species = metadataText(test.species[i])
				}
				input.Vegetation.Rows = append(input.Vegetation.Rows, row)
			}
			result, err := prepareSummaryVegetationWorkbook(t.Context(), input)
			if test.valid {
				if err != nil || len(result.Bytes) == 0 {
					t.Fatal("valid numeric-leading Constancy rows rejected", err)
				}
			} else if err == nil || len(result.Bytes) != 0 {
				t.Fatal("invalid numeric-leading Constancy rows returned success/partial bytes", err)
			}
		})
	}
}

func TestSummaryConstancyWorkbookIndependentBackendGates(t *testing.T) {
	service, state := summaryReferenceWorkbookFixture(t, false)
	contexts := service.source.environment.report.contexts
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	config, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil {
		t.Fatal(err)
	}

	options := summaryConstancyWorkbookOptions()
	for mask := 0; mask < 8; mask++ {
		service.groupedEnabled, service.referencesEnabled, service.presentationEnabled = mask&1 != 0, mask&2 != 0, mask&4 != 0
		review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, options))
		if mask != 7 {
			if err == nil || review != nil {
				t.Fatal("independent Constancy prerequisite bypassed", mask, review, err)
			}
			continue
		}
		if err != nil || review == nil || review.Scope != summaryVegetationConstancyScope ||
			review.Preview.Source != summaryVegetationConstancySource ||
			review.Preview.Ordering != summaryVegetationConstancyOrdering ||
			review.Options.ShowClass != 20 || review.Preview.ShowClass != 0 ||
			!reflect.DeepEqual(review.Preview.GroupField, &SummaryVegetationField{"Lifeform", 4, 4}) ||
			len(review.Preview.Rows) == 0 {
			t.Fatal("explicit Constancy inspection unavailable or metadata drifted", review, err)
		}
		for _, row := range review.Preview.Rows {
			if row.RawSpecies == nil || row.Layer.Storage != "integer" && row.Layer.Storage != "null" {
				t.Fatal("Constancy transport lost raw grouping or numeric lifeform", row)
			}
		}
	}
	service.groupedEnabled, service.referencesEnabled, service.presentationEnabled = false, false, false
	ordinary := SummaryVegetationWorkbookOptions{Title: "ordinary", Average: 1, Display: 6}
	review, err := service.GetReview(t.Context(), state.ContextID, workbookRequest(t, ordinary))
	if err != nil || review == nil || review.Scope != summaryVegetationWorkbookDraftScope {
		t.Fatal("Constancy prerequisites disabled unrelated ordinary workbook", err)
	}
	for _, row := range review.Preview.Rows {
		data, err := json.Marshal(row)
		if err != nil || row.RawSpecies != nil || bytes.Contains(data, []byte(`"rawSpecies"`)) {
			t.Fatal("Constancy field leaked into ordinary wire", string(data), err)
		}
	}
	after, err := os.ReadFile(contexts.projects.preferences.path)
	if err != nil || !bytes.Equal(config, after) || !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("read-only Constancy changed source data/audits/preferences", err)
	}
}
