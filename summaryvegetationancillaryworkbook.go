package main

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type summaryVegetationAncillaryWorkbookInput struct {
	Source        summaryVegetationWorkbookSource
	Options       summaryVegetationWorkbookOptions
	Tables        summaryVegetationAncillaryTables
	CreatedAt     string
	CreateSummary bool
}

func prepareSummaryVegetationAncillaryWorkbook(ctx context.Context,
	input summaryVegetationAncillaryWorkbookInput) (summaryVegetationWorkbook, error) {
	fail := func(err error) (summaryVegetationWorkbook, error) { return summaryVegetationWorkbook{}, err }
	if ctx == nil {
		return fail(errors.New("Summary ancillary workbook requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	created, err := time.Parse(time.RFC3339, input.CreatedAt)
	if err != nil || created.Year() < 100 || created.Format(time.RFC3339) != input.CreatedAt {
		return fail(errors.New("Summary ancillary workbook requires an explicit canonical reviewed RFC3339 time"))
	}
	preview := input.Source.Input.Vegetation
	options := input.Options
	if options.Ancillary != nil && options.Ancillary.CreateSummary != input.CreateSummary {
		return fail(errors.New("Summary ancillary workbook summary selection differs from its retained options"))
	}
	if options.Title != input.Source.Input.Title || options.Average != preview.Average ||
		options.Display != preview.Display || options.IncludeEnv != (input.Source.Input.Environment != nil) ||
		options.ShowClass != 0 && options.ShowClass != 1 && options.ShowClass != 10 && options.ShowClass != 20 {
		return fail(errors.New("Summary ancillary workbook requires its complete matching retained inspection options"))
	}
	calculationClass := options.ShowClass
	if options.Display != 4 && options.Display != 5 {
		calculationClass = 0
	}
	if calculationClass != preview.ShowClass {
		return fail(errors.New("Summary ancillary workbook retained ShowClass differs from its calculation policy"))
	}
	if options.IncludeEnv && len(input.Source.Environment.Env.Columns) == 0 {
		return fail(errors.New("Summary ancillary workbook requires the complete environmental header source"))
	}
	if !reflect.DeepEqual(preview, input.Source.Raw.Preview) {
		return fail(errors.New("Summary ancillary workbook requires the same retained raw/body preview"))
	}
	if err := validateSummaryAncillaryWorksheetRows(len(input.Source.Raw.SU.Rows), 0, false); err != nil {
		return fail(err)
	}
	members, _, err := prepareSummaryAncillaryMemberships(ctx, input.Source.Raw.SU)
	if err != nil {
		return fail(err)
	}
	if len(members) != len(preview.Memberships) {
		return fail(errors.New("Summary ancillary workbook requires complete physical/body SU memberships"))
	}
	for i, member := range members {
		actual := preview.Memberships[i]
		if member.SourceRowID != actual.RowID || !metadataCellEqual(member.PlotNumber, actual.PlotNumber) ||
			!metadataCellEqual(member.SiteUnit, actual.SiteUnit) {
			return fail(errors.New("Summary ancillary workbook physical/body SU memberships differ"))
		}
	}
	names := []string{preview.Project, "SU Table", "_VPRO_Source"}
	var plan summaryVegetationAncillaryPlan
	if input.CreateSummary {
		names = append(names, "ReportSummary")
		if len(input.Source.Environment.Env.Columns) != 0 && !reflect.DeepEqual(input.Source.Environment.Env, input.Tables.Env) {
			return fail(errors.New("Summary ancillary workbook requires the same complete header/summary Env source"))
		}
		plan, err = prepareSummaryVegetationAncillary(ctx, summaryVegetationAncillaryInput{
			CreatedAt: input.CreatedAt, SU: input.Source.Raw.SU, Veg: input.Source.Raw.Veg, Tables: input.Tables,
		})
		if err != nil {
			return fail(err)
		}
		if err := validateSummaryAncillaryWorksheetRows(len(plan.Memberships), len(plan.Units), true); err != nil {
			return fail(err)
		}
	}
	for i, name := range names {
		for _, other := range names[:i] {
			if strings.EqualFold(name, other) {
				return fail(errors.New("Summary ancillary workbook worksheet identities collide; no partial workbook"))
			}
		}
	}
	provenance := struct {
		Scope string
		Input summaryVegetationAncillaryWorkbookInput
	}{
		"Read-only literal normal-SU Layer inspection with ancillary sheets; zero value limit, no quality/lump/common-name/guide/constancy callbacks, Excel coercion or original Report/printing parity",
		input,
	}
	result, err := prepareSummaryVegetationWorkbookWithSheets(ctx, input.Source.Input, provenance, func(book *excelize.File) error {
		if _, err := book.NewSheet("SU Table"); err != nil {
			return err
		}
		for _, cell := range []struct{ address, value string }{{"A1", "Plotnumber"}, {"B1", "Site Unit"}} {
			if err := setReportWorkbookText(book, "SU Table", cell.address, cell.value); err != nil {
				return err
			}
		}
		if err := writeSummaryAncillaryMemberships(ctx, book, "SU Table", members, 2, 1); err != nil {
			return err
		}
		if err := book.SetColWidth("SU Table", "A", "B", 24); err != nil {
			return err
		}
		if err := book.SetColVisible("SU Table", "D:G", false); err != nil {
			return err
		}
		if !input.CreateSummary {
			return nil
		}
		if _, err := book.NewSheet("ReportSummary"); err != nil {
			return err
		}
		average := "n Plots"
		if preview.Average == 2 {
			average = "Characteristic"
		}
		for row, value := range []string{"Vegetation table:", "Site unit table:", "Lumping table:", "Data grouped by:",
			"Data ordered by:", "Values > than:", "Number of plots in database", "Number of plots in site unit table",
			"Average type:", "AllSpecs version:"} {
			if err := setReportWorkbookText(book, "ReportSummary", fmt.Sprintf("A%d", row+4), value); err != nil {
				return err
			}
		}
		texts := []struct{ address, value string }{
			{"A1", input.Source.Input.Title}, {"A2", "Date created: " + plan.CreatedAt},
			{"B4", preview.Project}, {"B5", preview.SU + "_SU"}, {"B6", "None_Lump"}, {"B7", "Layer"},
			{"B8", "Species"}, {"B12", average}, {"D4", "Short name"}, {"E4", "Units"}, {"F4", "Long name"},
			{"H4", "Plotnumber"}, {"I4", "Site Unit"},
		}
		for _, cell := range texts {
			if err := setReportWorkbookText(book, "ReportSummary", cell.address, cell.value); err != nil {
				return err
			}
		}
		integers := []struct {
			address string
			value   int64
		}{{"B9", 0}, {"B11", int64(plan.SelectedPlotRows)}}
		if plan.EnvironmentCountAvailable {
			integers = append(integers, struct {
				address string
				value   int64
			}{"B10", int64(plan.EnvironmentRows)})
		} else if err := setReportWorkbookText(book, "ReportSummary", "B10", "N/A"); err != nil {
			return err
		}
		version := "Unknown"
		if plan.SpeciesVersion.Text != nil {
			version = *plan.SpeciesVersion.Text
		}
		if err := setReportWorkbookText(book, "ReportSummary", "B13", version); err != nil {
			return err
		}
		for i, unit := range plan.Units {
			if err := ctx.Err(); err != nil {
				return err
			}
			row := i + 5
			if unit.NumPlots > 999999999999999 {
				return errors.New("Summary ancillary joined count exceeds Excel's fifteen-digit precision")
			}
			if err := setReportWorkbookText(book, "ReportSummary", fmt.Sprintf("D%d", row), unit.Code); err != nil {
				return err
			}
			if err := book.SetCellInt("ReportSummary", fmt.Sprintf("E%d", row), unit.NumPlots); err != nil {
				return err
			}
			if unit.LongName.Text != nil {
				if err := setReportWorkbookText(book, "ReportSummary", fmt.Sprintf("F%d", row), *unit.LongName.Text); err != nil {
					return err
				}
			}
		}
		for _, cell := range integers {
			if err := book.SetCellInt("ReportSummary", cell.address, cell.value); err != nil {
				return err
			}
		}
		if err := writeSummaryAncillaryMemberships(ctx, book, "ReportSummary", members, 5, 8); err != nil {
			return err
		}
		if err := book.SetColWidth("ReportSummary", "A", "A", 29); err != nil {
			return err
		}
		return book.SetColWidth("ReportSummary", "B", "K", 24)
	})
	if err != nil {
		return fail(err)
	}
	if input.CreateSummary {
		result.Ancillary = &plan
	}
	return result, nil
}

func validateSummaryAncillaryWorksheetRows(memberships, units int, summary bool) error {
	if memberships < 0 || memberships > 1048575 || units < 0 {
		return errors.New("Summary ancillary SU worksheet exceeds its row frame")
	}
	// VBA increments its Integer row counter after the final unit write.
	if summary && (memberships > 1048572 || units > 32762) {
		return errors.New("Summary ancillary ReportSummary exceeds its worksheet or source Integer row frame")
	}
	return nil
}

func writeSummaryAncillaryMemberships(ctx context.Context, book *excelize.File, sheet string,
	members []summaryVegetationAncillaryMembership, firstRow, firstColumn int) error {
	for i, member := range members {
		if err := ctx.Err(); err != nil {
			return err
		}
		for j, cell := range []ProjectMetadataCell{member.PlotNumber, member.SiteUnit} {
			if cell.Text == nil {
				continue
			}
			address, err := excelize.CoordinatesToCellName(firstColumn+j, firstRow+i)
			if err != nil {
				return err
			}
			if err := setReportWorkbookText(book, sheet, address, *cell.Text); err != nil {
				return err
			}
		}
	}
	return nil
}
