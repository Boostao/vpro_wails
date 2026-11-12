package reportcore

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"

	"errors"
	"fmt"

	"math"
	"math/big"
	"reflect"
	"sort"

	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type ReportUnitNames struct {
	LongName   *string
	Status     string
	Candidates []vprocore.EnvironmentReportName
}

func EnvironmentWorkbookText(value string) error {
	if !utf8.ValidString(value) || len(utf16.Encode([]rune(value))) > 32767 {
		return errors.New("workbook text requires valid Unicode within Excel's 32767 UTF-16 units; no truncation")
	}
	for _, r := range value {
		if r != '\t' && r != '\n' && r != '\r' && (r < 0x20 || r == 0xfffe || r == 0xffff) {
			return errors.New("workbook text contains an unsupported XML character; no replacement")
		}
	}
	return nil
}

func EnvironmentWorkbookSheetName(code string, index int) (string, error) {
	if err := EnvironmentWorkbookText(code); err != nil {
		return "", err
	}
	name := code
	if name == "" {
		name = fmt.Sprintf("NoName%d", index)
	}
	units := utf16.Encode([]rune(name))
	if len(units) > 31 {
		if units[30] >= 0xd800 && units[30] <= 0xdbff {
			return "", errors.New("source 31-unit worksheet truncation would split Unicode; choose a valid unit name explicitly")
		}
		name = string(utf16.Decode(units[:31]))
	}
	name = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`:/\[]*?`, r) {
			return '-'
		}
		return r
	}, name)
	if strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") || strings.EqualFold(name, "_VPRO_Source") {
		return "", errors.New("worksheet name conflicts with Excel/provenance rules; no implicit identity change")
	}
	return name, nil
}

func LifeformSummarySingle(value *big.Rat) (float64, error) {
	precision := uint(value.Num().BitLen() + value.Denom().BitLen() + 32)
	single, _ := new(big.Float).SetPrec(precision).SetMode(big.ToNearestEven).SetRat(value).Float32()
	result := float64(single)
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return 0, errors.New("cannot assign a finite SINGLE; no clamping applied")
	}
	return result, nil
}

func LifeformSummaryText(cell vprocore.ProjectMetadataCell, limit int) error {
	value, err := vprocore.VegetationReportIdentity(cell)
	if err != nil {
		return err
	}
	if value != nil && len(utf16.Encode([]rune(*value))) > limit {
		return fmt.Errorf("historical text exceeds source TEXT(%d) UTF-16 units; no repair or truncation applied", limit)
	}
	return nil
}

// Validate provenance before JSON can replace malformed Unicode or typed cells.
func LifeformWorkbookSourceValue(value reflect.Value) error {
	return ReportWorkbookSourceValue(value, func(cell vprocore.ProjectMetadataCell) error {
		_, err := vprocore.VegetationReportIdentity(cell)
		return err
	}, "Lifeform")
}

func ResolveReportUnitNames(ctx context.Context, rows []vprocore.ProjectMetadataRow, column int, label string) (ReportUnitNames, error) {
	result := ReportUnitNames{Status: "missing", Candidates: []vprocore.EnvironmentReportName{}}
	names, unsupported := map[string]bool{}, false
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return ReportUnitNames{}, err
		}
		if column < 0 || column >= len(row.Cells) {
			return ReportUnitNames{}, errors.New("report unit name column is outside the original reference row")
		}
		cell := row.Cells[column]
		if _, err := vprocore.MetadataCellValue(cell); err != nil {
			return ReportUnitNames{}, fmt.Errorf("%s master name value: %w", label, err)
		}
		result.Candidates = append(result.Candidates, vprocore.EnvironmentReportName{RowID: row.RowID, Value: vprocore.CloneSiteUnitCell(cell)})
		switch cell.Storage {
		case "text":
			names[*cell.Text] = true
		case "null":
		default:
			unsupported = true
		}
	}
	sort.Slice(result.Candidates, func(i, j int) bool { return result.Candidates[i].RowID < result.Candidates[j].RowID })
	switch {
	case unsupported:
		result.Status = "unsupported_storage"
	case len(names) > 1:
		result.Status = "conflicting"
	case len(names) == 1:
		result.Status = "unique"
		name := vprocore.SortedEnvironmentReportKeys(names)[0]
		result.LongName = &name
	}
	if err := ctx.Err(); err != nil {
		return ReportUnitNames{}, err
	}
	return result, nil
}

func VegetationTextKey(cell vprocore.ProjectMetadataCell) string {
	if cell.Text == nil {
		return "null"
	}
	return "text:" + *cell.Text
}

type VegetationReportMembership struct {
	RowID      string                       `json:"rowId"`
	PlotNumber vprocore.ProjectMetadataCell `json:"plotNumber"`
	SiteUnit   vprocore.ProjectMetadataCell `json:"siteUnit"`
}

type VegetationWorkbookSheetInternal struct {
	Unit vprocore.ProjectMetadataCell
	Name string
}

type VegetationWorkbookSheet struct {
	Unit vprocore.ProjectMetadataCell `json:"unit"`
	Name string                       `json:"name"`
}
