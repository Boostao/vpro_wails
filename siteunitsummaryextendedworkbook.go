package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func siteUnitSummaryWorkbookFields(grouping int) ([]SiteUnitSummaryField, error) {
	fields := siteUnitSummaryFields()
	if grouping == 1 {
		return fields, nil
	}
	if grouping != 2 {
		return nil, errors.New("Summary workbook strata requires separate physical LayerCode preparation")
	}
	result := append([]SiteUnitSummaryField{}, fields[:23]...)
	for index, caption := range siteUnitLifeformCaptions {
		result = append(result, SiteUnitSummaryField{"Lifeform", fmt.Sprintf("Lifeform%d", index), caption, "VEGETATION", "lifeform-cover"})
	}
	return append(result, fields[27:]...), nil
}

func prepareSiteUnitSummaryLifeformWorkbook(ctx context.Context, input siteUnitQuickVegetationInput) (siteUnitSummaryWorkbook, error) {
	if ctx == nil {
		return siteUnitSummaryWorkbook{}, errors.New("Summary lifeform workbook requires a context")
	}
	preview, err := siteUnitSummaryLifeformPreview(ctx, input)
	if err != nil {
		return siteUnitSummaryWorkbook{}, err
	}
	return prepareSiteUnitSummaryWorkbookProjection(ctx, preview, 2, nil)
}

func prepareSiteUnitSummarySpeciesWorkbook(ctx context.Context, input siteUnitQuickVegetationInput,
	request SiteUnitSpeciesListRequest) (siteUnitSummaryWorkbook, error) {
	preview, err := siteUnitSummarySpeciesPreview(ctx, input, request)
	if err != nil {
		return siteUnitSummaryWorkbook{}, err
	}
	return prepareSiteUnitSummaryWorkbookProjection(ctx, preview.Environment, request.OrderBy, &preview)
}

func siteUnitSummarySpeciesPreview(ctx context.Context, input siteUnitQuickVegetationInput,
	request SiteUnitSpeciesListRequest) (SiteUnitSpeciesListPreview, error) {
	if ctx == nil {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species projection requires a context")
	}
	if request.Method != input.Environment.Report.Method || request.Method != 1 && request.Method != 2 {
		return SiteUnitSpeciesListPreview{}, errors.New("Summary species method and owned Environment differ")
	}
	options := siteUnitSpeciesListOptions{request.OrderBy, request.CoverCalculation, request.AndOr,
		request.PresenceGreaterThan, request.CoverGreaterThan}
	units, err := planSiteUnitSpeciesList(ctx, input, options)
	if err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	result := SiteUnitSpeciesListPreview{Environment: input.Environment, Options: request,
		Units: []SiteUnitSpeciesListUnit{}}
	if request.OrderBy == 2 {
		result.Environment, err = siteUnitSummaryLifeformPreview(ctx, input)
		if err != nil {
			return SiteUnitSpeciesListPreview{}, err
		}
	}
	for _, unit := range units {
		if err := ctx.Err(); err != nil {
			return SiteUnitSpeciesListPreview{}, err
		}
		next := SiteUnitSpeciesListUnit{Code: unit.Code, NPlots: unit.NPlots, Groups: []SiteUnitSpeciesListGroup{}}
		for _, group := range unit.Groups {
			rows := []SiteUnitSpeciesListRow{}
			for _, row := range group.Rows {
				if err := ctx.Err(); err != nil {
					return SiteUnitSpeciesListPreview{}, err
				}
				rows = append(rows, SiteUnitSpeciesListRow{row.Species, row.ScientificName, row.EnglishName,
					row.CodeType, row.Cover, row.Presence, row.PhysicalValues, row.Included, row.ReferenceRowIDs})
			}
			next.Groups = append(next.Groups, SiteUnitSpeciesListGroup{group.Index, group.Caption, rows})
		}
		result.Units = append(result.Units, next)
	}
	if err := ctx.Err(); err != nil {
		return SiteUnitSpeciesListPreview{}, err
	}
	return result, nil
}

func writeSiteUnitSummarySpeciesCells(ctx context.Context, book *excelize.File, sheet string, species SiteUnitSpeciesListUnit,
	environment SiteUnitSummaryUnit, grouping, bold int, text func(string, string, string) error) error {
	if species.Code != environment.Code || species.NPlots != len(environment.Plots) {
		return errors.New("Summary species worksheet unit and physical denominator differ")
	}
	for index, heading := range []string{"Scientific Name", "Common Name", "%Cover", "%Presence"} {
		cell := string(rune('A'+index)) + "4"
		if err := text(sheet, cell, heading); err != nil {
			return err
		}
	}
	if err := book.SetCellStyle(sheet, "A4", "D4", bold); err != nil {
		return err
	}
	position := 5
	for _, group := range species.Groups {
		if err := ctx.Err(); err != nil {
			return err
		}
		if position > 1048576 {
			return errors.New("Summary species worksheet exceeds Excel's row limit")
		}
		cell := "A" + strconv.Itoa(position)
		if err := text(sheet, cell, group.Caption); err != nil {
			return err
		}
		if grouping == 2 {
			if err := book.SetCellStyle(sheet, cell, cell, bold); err != nil {
				return err
			}
		}
		position++
		for _, row := range group.Rows {
			if !row.Included {
				continue
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if position > 1048576 {
				return errors.New("Summary species worksheet exceeds Excel's row limit")
			}
			for index, value := range []*string{row.ScientificName.Text, row.EnglishName.Text, &row.Cover, &row.Presence} {
				if value == nil {
					continue
				}
				if index >= 2 {
					if _, ok := new(big.Rat).SetString(*value); !ok {
						return errors.New("Summary species worksheet requires source-formatted decimal values")
					}
				}
				if err := text(sheet, string(rune('A'+index))+strconv.Itoa(position), *value); err != nil {
					return err
				}
			}
			position++
		}
		position++
	}
	for _, column := range []struct {
		name  string
		width float64
	}{{"A", 45}, {"B", 35}, {"C", 12}, {"D", 12}} {
		if err := book.SetColWidth(sheet, column.name, column.name, column.width); err != nil {
			return err
		}
	}
	return ctx.Err()
}
