package main

import (
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

func validateCodeHeaderValues(h FS882Header, old *FS882Header, fields func(FS882Header) []becHeaderField, rejectUnchangedMalformedText bool) error {
	var previous []becHeaderField
	if old != nil {
		previous = fields(*old)
	}
	for i, field := range fields(h) {
		if rejectUnchangedMalformedText && field.value != nil && !utf8.ValidString(*field.value) {
			return validateSiteCodeText(field.name, field.value, field.maximum)
		}
		if old != nil && sameSiteCode(previous[i].value, field.value) {
			continue
		}
		if err := validateSiteCodeText(field.name, field.value, field.maximum); err != nil {
			return err
		}
	}
	return nil
}

func validateCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode, validate func(FS882Header, *FS882Header) error) error {
	if mode == headerCreate {
		return validate(h, nil)
	}
	caps, err := headerCapabilities(db, project)
	if err != nil {
		return err
	}
	old, err := readHeader(db, project, h.PlotNumber, caps)
	if errors.Is(err, sql.ErrNoRows) {
		old = nil
	} else if err != nil {
		return err
	}
	return validate(h, old)
}

func validateCodeRestoreValue(table, column string, value any, fields []becHeaderField) error {
	for _, field := range fields {
		if field.table != table || field.name != column {
			continue
		}
		if value == nil {
			return nil
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable string", column)
		}
		return validateSiteCodeText(column, &text, field.maximum)
	}
	return nil
}
