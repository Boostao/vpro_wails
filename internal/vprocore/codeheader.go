package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

func validateCodeHeaderValues(h FS882Header, old *FS882Header, fields func(FS882Header) []BecHeaderField, rejectUnchangedMalformedText bool) error {
	var previous []BecHeaderField
	if old != nil {
		previous = fields(*old)
	}
	for i, field := range fields(h) {
		if rejectUnchangedMalformedText && field.Value != nil && !utf8.ValidString(*field.Value) {
			return sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum)
		}
		if old != nil && SameSiteCode(previous[i].Value, field.Value) {
			continue
		}
		if err := sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum); err != nil {
			return err
		}
	}
	return nil
}

func validateCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode, validate func(FS882Header, *FS882Header) error) error {
	if mode == headerCreate {
		return validate(h, nil)
	}
	caps, err := HeaderCapabilities(db, project)
	if err != nil {
		return err
	}
	old, err := ReadHeader(db, project, h.PlotNumber, caps)
	if errors.Is(err, sql.ErrNoRows) {
		old = nil
	} else if err != nil {
		return err
	}
	return validate(h, old)
}

func validateCodeRestoreValue(table, column string, value any, fields []BecHeaderField) error {
	for _, field := range fields {
		if field.Table != table || field.Name != column {
			continue
		}
		if value == nil {
			return nil
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable string", column)
		}
		return sqlutil.ValidateSiteCodeText(column, &text, field.Maximum)
	}
	return nil
}
