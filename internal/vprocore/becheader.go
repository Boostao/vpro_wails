package vprocore

import (
	"github.com/boostao/vpro-wails/internal/references"
)

import (
	"database/sql"
	"fmt"
)

type BecHeaderField struct {
	Name, Property, Table string  `json:"-" yaml:"-"`
	Maximum               int     `json:"-" yaml:"-"`
	Value                 *string `json:"-" yaml:"-"`
}

func BecHeaderFields(h FS882Header) []BecHeaderField {
	return []BecHeaderField{
		{"Zone", "zone", "Env", 4, h.Zone},
		{"SubZone", "subZone", "Env", 8, h.SubZone},
		{"SiteSeries", "siteSeries", "Env", 5, h.SiteSeries},
		{"UserSiteUnit", "userSiteUnit", "Admin", 100, h.UserSiteUnit},
	}
}

func validateBECHeaderValues(h FS882Header, old *FS882Header) error {
	var previous []BecHeaderField
	if old != nil {
		previous = BecHeaderFields(*old)
	}
	for i, field := range BecHeaderFields(h) {
		if err := references.ValidateBECCode(field.Name, field.Value, field.Maximum); err != nil {
			if old != nil && previous[i].Value != nil && field.Value != nil && *previous[i].Value == *field.Value {
				continue
			}
			return err
		}
	}
	return nil
}

func validateBECHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if err := validateBECHeaderValues(h, nil); err == nil {
		return nil
	} else if mode == headerCreate {
		return err
	}
	caps, err := HeaderCapabilities(db, project)
	if err != nil {
		return err
	}
	old := FS882Header{}
	previous := []*(*string){&old.Zone, &old.SubZone, &old.SiteSeries, &old.UserSiteUnit}
	for i, field := range BecHeaderFields(h) {
		if references.ValidateBECCode(field.Name, field.Value, field.Maximum) == nil {
			continue
		}
		if !caps[field.Property] {
			return fmt.Errorf("unsupported header property %q in active project", field.Property)
		}
		key := "PlotNumber"
		if field.Table == "Admin" {
			key = "Plot"
		}
		err := db.QueryRow(`SELECT `+QuoteHeaderIdentifier(field.Name)+` FROM `+
			QuoteHeaderIdentifier(project+"_"+field.Table)+` WHERE `+QuoteHeaderIdentifier(key)+`=?`, h.PlotNumber).Scan(previous[i])
		if err == sql.ErrNoRows {
			return validateBECHeaderValues(h, nil)
		}
		if err != nil {
			return err
		}
	}
	return validateBECHeaderValues(h, &old)
}
