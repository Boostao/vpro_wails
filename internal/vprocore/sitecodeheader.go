package vprocore

import (
	"github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"errors"
	"fmt"

	"unicode/utf8"
)

func SiteCodeHeaderFields(h FS882Header) []BecHeaderField {
	return []BecHeaderField{
		{"SiteDisturbance1", "siteDisturbance1", "Env", 8, h.SiteDisturbance1},
		{"SiteDisturbance2", "siteDisturbance2", "Env", 8, h.SiteDisturbance2},
		{"SiteDisturbance3", "siteDisturbance3", "Env", 8, h.SiteDisturbance3},
		{"Exposure1", "exposure1", "Env", 2, h.Exposure1},
		{"Exposure2", "exposure2", "Env", 2, h.Exposure2},
	}
}

func isSiteCodeProperty(property string) bool {
	for _, field := range SiteCodeHeaderFields(FS882Header{}) {
		if field.Property == property {
			return true
		}
	}
	return false
}

func SameSiteCode(before, after *string) bool {
	return (before == nil && after == nil) || (before != nil && after != nil && *before == *after)
}

func (s *PlotService) validateSiteCodeHeaderValues(h FS882Header, old *FS882Header) error {
	previous := SiteCodeHeaderFields(FS882Header{})
	if old != nil {
		previous = SiteCodeHeaderFields(*old)
	}
	var membership map[string]bool
	for i, field := range SiteCodeHeaderFields(h) {
		if field.Value != nil && !utf8.ValidString(*field.Value) {
			return sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum)
		}
		if old != nil && SameSiteCode(previous[i].Value, field.Value) {
			continue
		}
		if err := sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum); err != nil {
			return err
		}
		if i < 3 || field.Value == nil {
			continue
		}
		if membership == nil {
			rows, err := s.exposureChoices()
			if err != nil {
				return fmt.Errorf("%s requires a verified Exposure catalogue: %w", field.Name, err)
			}
			membership = make(map[string]bool)
			for _, row := range rows {
				if row.Selectable && row.Code != nil {
					membership[*row.Code] = true
				}
			}
		}
		if err := CanonicalItemError(field.Name, "Exposure", *field.Value, membership); err != nil {
			return err
		}
	}

	return nil
}

func CanonicalItemError(name, list, value string, membership map[string]bool) error {
	if !membership[value] {
		return fmt.Errorf("%s must be NULL or an exact canonical %s Item; %q is not listed", name, list, value)
	}
	return nil
}

func (s *PlotService) validateSiteCodeHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if mode == headerCreate {
		return s.validateSiteCodeHeaderValues(h, nil)
	}
	caps, err := HeaderCapabilities(db, project)
	if err != nil {
		return err
	}
	if !caps["plotNumber"] {
		return errors.New("project is missing verified Env/Admin identity columns")
	}
	old, err := ReadHeader(db, project, h.PlotNumber, caps)
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
	for _, field := range SiteCodeHeaderFields(FS882Header{}) {
		if column != field.Name {
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

func (s *PlotService) exposureChoices() ([]references.SiteCodeChoice, error) {
	if s.SiteCodesError != nil {
		return nil, s.SiteCodesError
	}
	if s.SiteCodes != nil {
		return s.SiteCodes.ListExposureChoices(s.OperationContext())
	}

	catalogue, err := references.NewSiteCodeService(s.Projects.Root)
	if err != nil {
		return nil, err
	}
	rows, readErr := catalogue.ListExposureChoices(s.OperationContext())
	return rows, errors.Join(readErr, catalogue.Close())
}
