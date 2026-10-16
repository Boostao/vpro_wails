package main

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/xuri/excelize/v2"
)

func setReportWorkbookText(book *excelize.File, sheet, cell, value string) error {
	if err := environmentWorkbookText(value); err != nil {
		return err
	}
	if strings.Contains(value, "_x") {
		// Plain shared strings do not preserve literal OOXML escapes.
		if err := book.SetCellRichText(sheet, cell, []excelize.RichTextRun{{Text: value}}); err != nil {
			return err
		}
		actual, err := book.GetCellValue(sheet, cell, excelize.Options{RawCellValue: true})
		if err != nil {
			return err
		}
		if actual != value {
			return errors.New("literal text cannot roundtrip through Excel's escape representation; no replacement or partial workbook")
		}
		return nil
	}
	return book.SetCellStr(sheet, cell, value)
}

func reportWorkbookSourceValue(value reflect.Value, validateCell func(ProjectMetadataCell) error, operation string) error {
	if value.Type() == reflect.TypeOf(ProjectMetadataCell{}) {
		if err := validateCell(value.Interface().(ProjectMetadataCell)); err != nil {
			return err
		}
	}
	switch value.Kind() {
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			if err := reportWorkbookSourceValue(value.Field(i), validateCell, operation); err != nil {
				return err
			}
		}
	case reflect.Array, reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			if err := reportWorkbookSourceValue(value.Index(i), validateCell, operation); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		if !value.IsNil() {
			return reportWorkbookSourceValue(value.Elem(), validateCell, operation)
		}
	case reflect.String:
		return environmentWorkbookText(value.String())
	case reflect.Float64:
		if math.IsNaN(value.Float()) || math.IsInf(value.Float(), 0) {
			return fmt.Errorf("%s workbook requires finite source numbers", operation)
		}
	}
	return nil
}
