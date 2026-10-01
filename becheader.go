package main

import (
	"database/sql"
	"fmt"
)

type becHeaderField struct {
	name, property, table string
	maximum               int
	value                 *string
}

func becHeaderFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"Zone", "zone", "Env", 4, h.Zone},
		{"SubZone", "subZone", "Env", 8, h.SubZone},
		{"SiteSeries", "siteSeries", "Env", 5, h.SiteSeries},
		{"UserSiteUnit", "userSiteUnit", "Admin", 100, h.UserSiteUnit},
	}
}

func validateBECHeaderValues(h FS882Header, old *FS882Header) error {
	var previous []becHeaderField
	if old != nil {
		previous = becHeaderFields(*old)
	}
	for i, field := range becHeaderFields(h) {
		if err := validateBECCode(field.name, field.value, field.maximum); err != nil {
			if old != nil && previous[i].value != nil && field.value != nil && *previous[i].value == *field.value {
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
	caps, err := headerCapabilities(db, project)
	if err != nil {
		return err
	}
	old := FS882Header{}
	previous := []*(*string){&old.Zone, &old.SubZone, &old.SiteSeries, &old.UserSiteUnit}
	for i, field := range becHeaderFields(h) {
		if validateBECCode(field.name, field.value, field.maximum) == nil {
			continue
		}
		if !caps[field.property] {
			return fmt.Errorf("unsupported header property %q in active project", field.property)
		}
		key := "PlotNumber"
		if field.table == "Admin" {
			key = "Plot"
		}
		err := db.QueryRow(`SELECT `+quoteHeaderIdentifier(field.name)+` FROM `+
			quoteHeaderIdentifier(project+"_"+field.table)+` WHERE `+quoteHeaderIdentifier(key)+`=?`, h.PlotNumber).Scan(previous[i])
		if err == sql.ErrNoRows {
			return validateBECHeaderValues(h, nil)
		}
		if err != nil {
			return err
		}
	}
	return validateBECHeaderValues(h, &old)
}
