package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

const twoPageVegetationTableEnvironment = "VPRO_TWO_PAGE_VEGETATION_TABLE_REVIEW"
const twoPageVegetationTableScanMaximum = 100000
const twoPageVegetationTablePageMaximum = 200
const twoPageVegetationTableOrdering = "controlled-English-ignore-case; numeric-descending-NULL-last; physical-row-ties"

type TwoPageVegetationTableColumn struct {
	Name    string `json:"name"`
	Caption string `json:"caption"`
}

type TwoPageVegetationTableRow struct {
	VegRowID string                `json:"vegRowId"`
	SURowID  *string               `json:"suRowId"`
	Cells    []ProjectMetadataCell `json:"cells"`
}

type TwoPageVegetationTableReview struct {
	ContextID             string                         `json:"contextId"`
	Project               string                         `json:"project"`
	ProjectPath           string                         `json:"projectPath"`
	SU                    string                         `json:"su"`
	SUPath                string                         `json:"suPath"`
	Plot                  string                         `json:"plot"`
	Form                  string                         `json:"form"`
	Table                 string                         `json:"table"`
	SUTable               string                         `json:"suTable"`
	Columns               []TwoPageVegetationTableColumn `json:"columns"`
	Rows                  []TwoPageVegetationTableRow    `json:"rows"`
	Original              ProjectMetadataTable           `json:"original"`
	SelectedOriginal      *ProjectMetadataTable          `json:"selectedOriginal"`
	Offset                int                            `json:"offset"`
	Limit                 int                            `json:"limit"`
	TotalRows             int64                          `json:"totalRows"`
	OrderingPolicy        string                         `json:"orderingPolicy"`
	GeneralLocaleVerified bool                           `json:"generalLocaleVerified"`
}

type TwoPageVegetationTableService struct {
	contexts *ContextService
	review   bool
}

func NewTwoPageVegetationTableService(contexts *ContextService, lookup func(string) (string, bool)) (*TwoPageVegetationTableService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("two-page vegetation table requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(twoPageVegetationTableEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &TwoPageVegetationTableService{contexts, enabled}, nil
}

func twoPageVegetationTableSource(form string) ([]TwoPageVegetationTableColumn, []string, error) {
	columns := []TwoPageVegetationTableColumn{
		{"PlotNumber", "PlotNumber"}, {"Species", "Species"}, {"Cover1", "A1"}, {"Cover2", "A2"},
		{"Cover3", "A3"}, {"TotalA", "Total A"}, {"Cover4", "B1"}, {"Cover5", "B2"},
		{"Cover5a", "CHARS shrubs a"}, {"Cover5b", "CHARS shrubs b"}, {"Cover5c", "CHARS shrubs c"},
		{"TotalB", "Total B"}, {"Cover6", "C"}, {"Cover7", "D"},
	}
	order := []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB", "Cover6", "Cover7"}
	switch form {
	case "FS882-8x6XL":
		columns = append(columns, TwoPageVegetationTableColumn{"Cover8", "Dr/Dw"},
			TwoPageVegetationTableColumn{"Cover9", "Ep"}, TwoPageVegetationTableColumn{"Cover10", "Cover10"})
		order = append(order, "Cover8", "Cover9", "Cover10")
	case "FS882-8x6XL-CHARS":
	default:
		return nil, nil, fmt.Errorf("two-page vegetation table does not support source form %q", form)
	}
	return columns, order, nil
}

func (s *TwoPageVegetationTableService) GetReview(ctx context.Context, contextID, plot, form string, offset, limit int) (*TwoPageVegetationTableReview, error) {
	if ctx == nil {
		return nil, errors.New("two-page vegetation table requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.review {
		return nil, errors.New("two-page vegetation table requires independently enabled saved-data review")
	}
	if plot == "" || !utf8.ValidString(plot) || offset < 0 || limit < 1 || limit > twoPageVegetationTablePageMaximum {
		return nil, errors.New("two-page vegetation table requires a valid literal opening parent, nonnegative offset and limit1..200")
	}
	columns, order, err := twoPageVegetationTableSource(form)
	if err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (*TwoPageVegetationTableReview, error) {
		return withOwnedSIVISnapshot(ctx, plots, func(owner *sqliteContext, tx *sql.Tx) (*TwoPageVegetationTableReview, error) {
			project, table := owner.selection.Project, owner.selection.Project+"_Veg"
			if err := requireTwoPageChildReadScope(ctx, tx, project, table, plot, "vegetation table"); err != nil {
				return nil, err
			}
			read := func(alias, name string) (ProjectMetadataTable, error) {
				if err := validateSIVIPhysicalSchema(ctx, tx, alias, name, "two-page vegetation table"); err != nil {
					return ProjectMetadataTable{}, err
				}
				var count int64
				if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quoteHeaderIdentifier(alias)+"."+quoteHeaderIdentifier(name)).Scan(&count); err != nil {
					return ProjectMetadataTable{}, err
				}
				if count < 0 || count > twoPageVegetationTableScanMaximum {
					return ProjectMetadataTable{}, errors.New("two-page vegetation table source exceeds the100000-row physical snapshot bound; no partial review")
				}
				return readPhysicalLocationTable(ctx, tx, alias, name)
			}
			veg, err := read("project", table)
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation table original: %w", err)
			}
			var selected *ProjectMetadataTable
			suTable := ""
			if owner.selection.SU != "None" {
				suTable = owner.selection.SU + "_SU"
				source, err := read("su", suTable)
				if err != nil {
					return nil, fmt.Errorf("two-page vegetation table selected source: %w", err)
				}
				selected = &source
			}
			result, err := planTwoPageVegetationTable(ctx, veg, selected, columns, order, offset, limit)
			if err != nil {
				return nil, err
			}
			result.ContextID, result.Project, result.ProjectPath = contextID, project, owner.selection.ProjectPath
			result.SU, result.SUPath, result.Plot, result.Form = owner.selection.SU, owner.selection.SUPath, plot, form
			result.Table, result.SUTable = table, suTable
			return result, nil
		})
	})
}

func planTwoPageVegetationTable(ctx context.Context, veg ProjectMetadataTable, selected *ProjectMetadataTable,
	columns []TwoPageVegetationTableColumn, order []string, offset, limit int) (*TwoPageVegetationTableReview, error) {
	required := make([]string, len(columns))
	for i, column := range columns {
		required[i] = column.Name
	}
	indices, err := siteUnitTransferColumns(veg, required...)
	if err != nil {
		return nil, fmt.Errorf("two-page vegetation table schema: %w", err)
	}
	comparator := collate.New(language.English, collate.IgnoreCase)
	var buffer collate.Buffer
	key := func(cell ProjectMetadataCell) (string, bool, error) {
		value, err := metadataCellValue(cell)
		if err != nil || value == nil {
			return "", value == nil, err
		}
		text, ok := value.(string)
		if !ok {
			return "", false, errors.New("two-page vegetation table requires original TEXT/NULL PlotNumber storage; no identity coerced")
		}
		valueKey := string(comparator.KeyFromString(&buffer, text))
		buffer.Reset()
		return valueKey, false, nil
	}
	type preparedRow struct {
		index   int
		key     string
		null    bool
		numbers map[string]*big.Rat
	}
	prepared := make([]preparedRow, 0, len(veg.Rows))
	for i, row := range veg.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		valueKey, null, err := key(row.Cells[indices["PlotNumber"]])
		if err != nil {
			return nil, err
		}
		numbers := make(map[string]*big.Rat, len(columns)-2)
		for _, column := range columns[2:] {
			number, err := vegetationReportNumber(row.Cells[indices[column.Name]])
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation table row %s %s: %w", row.RowID, column.Name, err)
			}
			numbers[column.Name] = number
		}
		prepared = append(prepared, preparedRow{i, valueKey, null, numbers})
	}
	members := map[string][]int{}
	if selected != nil {
		suIndices, err := siteUnitTransferColumns(*selected, "PlotNumber")
		if err != nil {
			return nil, fmt.Errorf("two-page vegetation table membership schema: %w", err)
		}
		for i, row := range selected.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			valueKey, null, err := key(row.Cells[suIndices["PlotNumber"]])
			if err != nil {
				return nil, err
			}
			if !null {
				members[valueKey] = append(members[valueKey], i)
			}
		}
	}
	sort.SliceStable(prepared, func(i, j int) bool {
		a, b := prepared[i], prepared[j]
		if a.null != b.null {
			return a.null
		}
		if a.key != b.key {
			return a.key < b.key
		}
		for _, column := range order {
			x, y := a.numbers[column], b.numbers[column]
			if x == nil && y == nil {
				continue
			}
			if x == nil {
				return false
			}
			if y == nil {
				return true
			}
			if comparison := x.Cmp(y); comparison != 0 {
				return comparison > 0
			}
		}
		return false
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := &TwoPageVegetationTableReview{Columns: columns, Rows: []TwoPageVegetationTableRow{},
		Original: ProjectMetadataTable{Columns: veg.Columns, Rows: []ProjectMetadataRow{}},
		Offset:   offset, Limit: limit, OrderingPolicy: twoPageVegetationTableOrdering}
	if selected != nil {
		result.SelectedOriginal = &ProjectMetadataTable{Columns: selected.Columns, Rows: []ProjectMetadataRow{}}
	}
	seenVeg, seenSU := map[string]bool{}, map[string]bool{}
	for _, row := range prepared {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		membership := members[row.key]
		count := int64(1)
		if selected != nil {
			count = int64(len(membership))
			if row.null {
				count = 0
			}
		}
		position := result.TotalRows
		result.TotalRows += count
		start, end := int64(offset)-position, int64(offset)+int64(limit)-position
		if start < 0 {
			start = 0
		}
		if end > count {
			end = count
		}
		for i := start; i < end; i++ {
			original := veg.Rows[row.index]
			cells := make([]ProjectMetadataCell, len(columns))
			for j, column := range columns {
				cells[j] = original.Cells[indices[column.Name]]
			}
			projected := TwoPageVegetationTableRow{VegRowID: original.RowID, Cells: cells}
			if selected != nil {
				source := selected.Rows[membership[i]]
				id := source.RowID
				projected.SURowID = &id
				if !seenSU[id] {
					result.SelectedOriginal.Rows = append(result.SelectedOriginal.Rows, source)
					seenSU[id] = true
				}
			}
			result.Rows = append(result.Rows, projected)
			if !seenVeg[original.RowID] {
				result.Original.Rows = append(result.Original.Rows, original)
				seenVeg[original.RowID] = true
			}
		}
	}
	if int64(offset) > result.TotalRows {
		return nil, errors.New("two-page vegetation table offset exceeds the saved source row count")
	}
	return result, nil
}
