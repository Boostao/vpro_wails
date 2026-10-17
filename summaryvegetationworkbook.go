package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/xuri/excelize/v2"
)

type summaryVegetationWorkbookInput struct {
	Title       string                             `json:"title"`
	Vegetation  SummaryVegetationPreview           `json:"vegetation"`
	Environment *summaryEnvironmentalWorkbookInput `json:"environment"`
}

type summaryVegetationWorkbook struct {
	Bytes              []byte
	Sheet              string
	FirstVegetationRow int
	Ancillary          *summaryVegetationAncillaryPlan
}

func prepareSummaryVegetationWorkbook(ctx context.Context, input summaryVegetationWorkbookInput) (summaryVegetationWorkbook, error) {
	return prepareSummaryVegetationWorkbookWithSheets(ctx, input, input, nil)
}

func prepareSummaryVegetationWorkbookWithSheets(ctx context.Context, input summaryVegetationWorkbookInput,
	provenance any, writeSheets func(*excelize.File) error) (summaryVegetationWorkbook, error) {
	fail := func(err error) (summaryVegetationWorkbook, error) { return summaryVegetationWorkbook{}, err }
	if ctx == nil {
		return fail(errors.New("Summary workbook requires a context"))
	}
	preview := input.Vegetation
	if err := validateSummaryVegetationDisplayOptions(ctx, preview.Average, preview.Display, preview.ShowClass); err != nil {
		return fail(err)
	}
	source, ordering, common := "literal-normal-su-layer", "literal-null-first-layer-species", false
	if reference := preview.References; reference != nil {
		source = "literal-normal-su-effective-references"
		common = reference.ShowCommon
		if reference.UseGuideNames && !common || reference.MasterRows < 0 || reference.PersonalRows < 0 ||
			reference.DescriptionRows < 0 || reference.RemovedMasterRows < 0 ||
			reference.RemovedMasterRows > reference.MasterRows || reference.DerivedRows < 0 ||
			reference.DerivedRows > reference.DescriptionRows || reference.UnionRows < 0 ||
			int64(reference.UnionRows) > int64(reference.MasterRows-reference.RemovedMasterRows)+
				int64(reference.PersonalRows)+int64(reference.DerivedRows) {
			return fail(errors.New("Summary workbook effective reference metadata is inconsistent; no repairs applied"))
		}
		if common {
			ordering = "literal-null-first-layer-species-common"
		}
	}
	if preview.ContextID == "" || preview.ProjectPath == "" || preview.SUPath == "" || preview.Project == "" ||
		preview.SU == "" || preview.SU == "None" || preview.SU == "USysSuTableDynamic" ||
		preview.Source != source || preview.Ordering != ordering ||
		(preview.GroupAndOrder != 0 && preview.GroupAndOrder != 11) || preview.GroupField != nil || preview.OrderField != nil ||
		len(preview.Units) == 0 || len(preview.Units) > 250 || len(preview.Rows) == 0 || len(preview.Memberships) == 0 ||
		len(preview.Columns) != len(preview.Units) {
		return fail(errors.New("Summary workbook requires a complete normal-SU layer preview with aligned units and nonempty vegetation"))
	}
	sheet, err := environmentWorkbookSheetName(preview.Project, 0)
	if err != nil {
		return fail(err)
	}
	if sheet != preview.Project {
		return fail(errors.New("Summary workbook project worksheet identity requires alteration; no truncation or replacement"))
	}
	for i, unit := range preview.Units {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if unit.NumPlots < 1 || len(unit.MembershipIDs) == 0 || len(unit.RawVegRowIDs) == 0 ||
			preview.Columns[i] != unit.Code || i > 0 && preview.Units[i-1].Code >= unit.Code {
			return fail(errors.New("Summary workbook requires positive physical counts and exact ordered SV1/pivot alignment"))
		}
	}
	cells := []summaryEnvironmentalHeaderCell{}
	add := func(row, column int, cell ProjectMetadataCell) {
		copied := cloneSiteUnitCell(cell)
		cells = append(cells, summaryEnvironmentalHeaderCell{Row: row, Column: column,
			Value: SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &copied}})
	}
	text := func(row, column int, value string) {
		add(row, column, ProjectMetadataCell{Storage: "text", Text: &value})
	}
	first, separator, headerRow := 6, 0, 5
	leading := 3
	labels := []string{"Layer", "Spp", "Order"}
	if common {
		leading = 4
		labels = []string{"Layer", "Spp", "Common Name", "Order"}
	}
	caption := "Vegetation Table"
	if environment := input.Environment; environment != nil {
		if environment.FirstRow != 5 || environment.English != common || !reflect.DeepEqual(environment.Units, summaryEnvironmentalPreviewUnits(preview)) {
			return fail(errors.New("Summary workbook environmental header requires row5, the same common-name layout and complete SV1 frame"))
		}
		header, err := prepareSummaryEnvironmentalHeader(ctx, 5, common, environment.FieldList, environment.Units, environment.Rows)
		if err != nil {
			return fail(err)
		}
		cells = append(cells, header.Cells...)
		first, separator, headerRow = header.NextRow-1, header.NextRow-2, 4
		text(separator, 1, "StartVeg")
		caption = "Site Unit Summary Table"
	} else {
		for i, unit := range preview.Units {
			count := strconv.Itoa(unit.NumPlots)
			add(4, i+leading+1, ProjectMetadataCell{Storage: "integer", Integer: &count})
			text(5, i+leading+1, unit.Code)
		}
		text(4, leading, "n Plots")
		for i, label := range labels {
			text(5, i+1, label)
		}
	}
	text(1, 1, input.Title)
	text(2, 1, caption)
	if len(preview.Rows) > 32768-first {
		return fail(errors.New("Summary workbook body exceeds the qualified source Integer row frame"))
	}
	less := func(left, right ProjectMetadataCell) bool {
		if left.Text == nil || right.Text == nil {
			return left.Text == nil && right.Text != nil
		}
		return *left.Text < *right.Text
	}
	for i, row := range preview.Rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if len(row.Covers) != len(preview.Columns) || len(row.Witnesses) == 0 ||
			(row.CommonName != nil) != common {
			return fail(errors.New("Summary workbook body requires distinct source-ordered groups and complete pivot values"))
		}
		values := []ProjectMetadataCell{row.Layer, row.Species}
		if common {
			values = append(values, *row.CommonName)
		}
		values = append(values, row.Order)
		for column, cell := range append(values, row.Covers...) {
			if cell.Storage != "text" && cell.Storage != "null" {
				return fail(fmt.Errorf("Summary workbook body row%d requires original text/NULL output, not scalar conversion", i+1))
			}
			if _, err := metadataCellValue(cell); err != nil {
				return fail(fmt.Errorf("Summary workbook body row%d: %w", i+1, err))
			}
			add(first+i, column+1, cell)
		}
		if i > 0 {
			prior := preview.Rows[i-1]
			ordered := less(prior.Layer, row.Layer) || reflect.DeepEqual(prior.Layer, row.Layer) &&
				(less(prior.Species, row.Species) || reflect.DeepEqual(prior.Species, row.Species) &&
					common && less(*prior.CommonName, *row.CommonName))
			if !ordered {
				return fail(errors.New("Summary workbook body groups are duplicated or outside the source literal ordering"))
			}
		}
	}
	data, err := renderSummaryWorkbookWithSheets(ctx, provenance, summaryWorkbookLayout{
		Sheet: sheet, Title: input.Title, LastRow: first + len(preview.Rows) - 1, LastColumn: leading + len(preview.Columns),
		SeparatorRow: separator, ItalicFromRow: first, HeaderRows: []int{1, headerRow}, Cells: cells,
	}, writeSheets)
	if err != nil {
		return fail(err)
	}
	return summaryVegetationWorkbook{Bytes: data, Sheet: sheet, FirstVegetationRow: first}, nil
}
