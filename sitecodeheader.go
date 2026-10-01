package main

import (
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf16"
	"unicode/utf8"
)

func siteCodeHeaderFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"SiteDisturbance1", "siteDisturbance1", "Env", 8, h.SiteDisturbance1},
		{"SiteDisturbance2", "siteDisturbance2", "Env", 8, h.SiteDisturbance2},
		{"SiteDisturbance3", "siteDisturbance3", "Env", 8, h.SiteDisturbance3},
		{"Exposure1", "exposure1", "Env", 2, h.Exposure1},
		{"Exposure2", "exposure2", "Env", 2, h.Exposure2},
	}
}

func isSiteCodeProperty(property string) bool {
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		if field.property == property {
			return true
		}
	}
	return false
}

func validateSiteCodeText(name string, value *string, limit int) error {
	if value == nil {
		return nil
	}
	if !utf8.ValidString(*value) {
		return fmt.Errorf("%s must contain valid Unicode (invalid UTF-8)", name)
	}
	if *value == "" || len(utf16.Encode([]rune(*value))) > limit {
		return fmt.Errorf("%s must be NULL or a nonempty string of at most %d UTF-16 units", name, limit)
	}
	return nil
}

func sameSiteCode(before, after *string) bool {
	return (before == nil && after == nil) || (before != nil && after != nil && *before == *after)
}

func (s *PlotService) validateSiteCodeHeaderValues(h FS882Header, old *FS882Header) error {
	previous := siteCodeHeaderFields(FS882Header{})
	if old != nil {
		previous = siteCodeHeaderFields(*old)
	}
	var membership map[string]bool
	for i, field := range siteCodeHeaderFields(h) {
		if field.value != nil && !utf8.ValidString(*field.value) {
			return validateSiteCodeText(field.name, field.value, field.maximum)
		}
		if old != nil && sameSiteCode(previous[i].value, field.value) {
			continue
		}
		if err := validateSiteCodeText(field.name, field.value, field.maximum); err != nil {
			return err
		}
		if i < 3 || field.value == nil {
			continue
		}
		if membership == nil {
			rows, err := s.exposureChoices()
			if err != nil {
				return fmt.Errorf("%s requires a verified Exposure catalogue: %w", field.name, err)
			}
			membership = make(map[string]bool)
			for _, row := range rows {
				if row.Selectable && row.Code != nil {
					membership[*row.Code] = true
				}
			}
		}
		if err := canonicalItemError(field.name, "Exposure", *field.value, membership); err != nil {
			return err
		}
	}

	return nil
}

func canonicalItemError(name, list, value string, membership map[string]bool) error {
	if !membership[value] {
		return fmt.Errorf("%s must be NULL or an exact canonical %s Item; %q is not listed", name, list, value)
	}
	return nil
}

func (s *PlotService) validateSiteCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if mode == headerCreate {
		return s.validateSiteCodeHeaderValues(h, nil)
	}
	caps, err := headerCapabilities(db, project)
	if err != nil {
		return err
	}
	if !caps["plotNumber"] {
		return errors.New("project is missing verified Env/Admin identity columns")
	}
	old, err := readHeader(db, project, h.PlotNumber, caps)
	if errors.Is(err, sql.ErrNoRows) {
		old = nil
	} else if err != nil {
		return err
	}
	return s.validateSiteCodeHeaderValues(h, old)
}

func (s *PlotService) validateSiteCodeRestoreValue(table, column string, value any) error {
	if table != "Env" {
		return nil
	}
	for _, field := range siteCodeHeaderFields(FS882Header{}) {
		if column != field.name {
			continue
		}
		if value == nil {
			return nil
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable string", column)
		}
		h := FS882Header{}
		switch column {
		case "SiteDisturbance1":
			h.SiteDisturbance1 = &text
		case "SiteDisturbance2":
			h.SiteDisturbance2 = &text
		case "SiteDisturbance3":
			h.SiteDisturbance3 = &text
		case "Exposure1":
			h.Exposure1 = &text
		case "Exposure2":
			h.Exposure2 = &text
		}
		return s.validateSiteCodeHeaderValues(h, nil)
	}
	return nil
}
