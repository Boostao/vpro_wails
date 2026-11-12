package vegreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type summaryEnvironmentalHeaderCell struct {
	Row    int
	Column int
	Value  SummaryVegetationEnvironmentalValue
}

type summaryEnvironmentalHeaderPlan struct {
	FirstRow         int
	FirstValueColumn int
	LastValueColumn  int
	LastDataRow      int
	NextRow          int
	Fields           []vprocore.ProjectMetadataRow
	Cells            []summaryEnvironmentalHeaderCell
}

// Values retain their source tags; Excel's .Value coercion is not performed here.
func prepareSummaryEnvironmentalHeader(ctx context.Context, firstRow int, english bool,
	fieldList vprocore.ProjectMetadataTable, units []summaryVegetationUnit,
	rows []SummaryVegetationEnvironmentalNumericRow) (summaryEnvironmentalHeaderPlan, error) {
	fail := func(err error) (summaryEnvironmentalHeaderPlan, error) {
		return summaryEnvironmentalHeaderPlan{}, fmt.Errorf("Summary environmental header placement: %w", err)
	}
	if ctx == nil {
		return fail(errors.New("requires a context"))
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	if firstRow < 2 || firstRow > 32765 {
		return fail(errors.New("first row must preserve the original Integer header/return-row bounds"))
	}
	if len(units) == 0 || len(units) > 250 {
		return fail(errors.New("requires the original nonempty SV1 frame within the source250-group limit"))
	}
	seen := make(map[string]bool, len(units))
	for _, unit := range units {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if err := reportcore.EnvironmentWorkbookText(unit.Code); err != nil {
			return fail(err)
		}
		if seen[unit.Code] {
			return fail(errors.New("SV1 frame contains duplicate literal unit identities"))
		}
		seen[unit.Code] = true
	}
	if err := validateSummaryVegetationFieldListWitness(ctx, fieldList); err != nil {
		return fail(err)
	}
	index, err := indexSummaryVegetationFieldList(ctx, fieldList)
	if err != nil {
		return fail(err)
	}
	if len(rows) != len(index.Selected) || len(rows) > 32765-firstRow {
		return fail(errors.New("requires the complete selected matrix within original Integer return-row bounds"))
	}
	physical := make(map[string]vprocore.ProjectMetadataRow, len(fieldList.Rows))
	for _, row := range fieldList.Rows {
		physical[row.RowID] = row
	}
	result := summaryEnvironmentalHeaderPlan{
		FirstRow: firstRow, FirstValueColumn: 4, LastDataRow: firstRow + len(rows) - 1,
		NextRow: firstRow + len(rows) + 2, Fields: make([]vprocore.ProjectMetadataRow, 0, len(rows)),
		Cells: make([]summaryEnvironmentalHeaderCell, 0, len(units)+len(rows)*(len(units)+2)),
	}
	if english {
		result.FirstValueColumn = 5
	}
	result.LastValueColumn = result.FirstValueColumn + len(units) - 1
	appendCell := func(row, column int, value SummaryVegetationEnvironmentalValue) error {
		copy := SummaryVegetationEnvironmentalValue{Kind: value.Kind}
		switch value.Kind {
		case "empty":
			if value.Cell != nil {
				return errors.New("Empty value must not contain an ordinary cell")
			}
		case "cell":
			if value.Cell == nil {
				return errors.New("ordinary value requires its complete typed cell")
			}
			decoded, err := vprocore.MetadataCellValue(*value.Cell)
			if err != nil {
				return err
			}
			if text, ok := decoded.(string); ok {
				if err := reportcore.EnvironmentWorkbookText(text); err != nil {
					return err
				}
			}
			if value.Cell.Storage != "null" && value.Cell.Storage != "text" && value.Cell.Storage != "integer" {
				return errors.New("header placement has an unavailable ordinary storage type")
			}
			cell := vprocore.CloneSiteUnitCell(*value.Cell)
			copy.Cell = &cell
		default:
			return errors.New("header placement requires an exact Empty or ordinary cell tag")
		}
		result.Cells = append(result.Cells, summaryEnvironmentalHeaderCell{row, column, copy})
		return nil
	}
	for i, unit := range units {
		value := vprocore.ProjectMetadataCell{Storage: "text", Text: &unit.Code}
		if err := appendCell(firstRow-1, result.FirstValueColumn+i,
			SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &value}); err != nil {
			return fail(err)
		}
	}
	for i, row := range rows {
		if err := ctx.Err(); err != nil {
			return fail(err)
		}
		if !reflect.DeepEqual(row.Field, physical[index.Selected[i]]) || len(row.Values) != len(units) {
			return fail(errors.New("selected matrix order, complete field witness or unit cardinality differs"))
		}
		label := row.Field.Cells[2]
		if _, err := vprocore.VegetationReportIdentity(label); err != nil {
			return fail(err)
		}
		special := label.Text != nil && strings.EqualFold(*label.Text, "n Plots")
		if special {
			for _, r := range *label.Text {
				if r > 127 {
					return fail(errors.New("non-ASCII special-caption collation remains unavailable"))
				}
			}
		}
		captionColumn := 2
		if special {
			captionColumn = 3
		} else {
			source := row.Field.Cells[1]
			if _, err := vprocore.VegetationReportIdentity(source); err != nil {
				return fail(err)
			}
			if err := appendCell(firstRow+i, 1, SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &source}); err != nil {
				return fail(err)
			}
		}
		if err := appendCell(firstRow+i, captionColumn, SummaryVegetationEnvironmentalValue{Kind: "cell", Cell: &label}); err != nil {
			return fail(err)
		}
		result.Fields = append(result.Fields, cloneSummaryVegetationFieldListRow(row.Field))
		for j, value := range row.Values {
			if err := ctx.Err(); err != nil {
				return fail(err)
			}
			if err := appendCell(firstRow+i, result.FirstValueColumn+j, value); err != nil {
				return fail(fmt.Errorf("field %s unit %q: %w", row.Field.RowID, units[j].Code, err))
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return fail(err)
	}
	return result, nil
}
