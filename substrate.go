package main

import (
	"database/sql"
	"fmt"
	"math"
)

// Retain finite imported overflow only when it equals the actual stored value.
// New values preserve float64 precision within the native SINGLE range.
func validateSingleRangeChange(property string, before, after *float64) error {
	if after == nil {
		return nil
	}
	if math.IsNaN(*after) || math.IsInf(*after, 0) {
		return fmt.Errorf("property %q must be finite", property)
	}
	if sameHeightValue(before, after) {
		return nil
	}
	if math.Abs(*after) > math.MaxFloat32 {
		return fmt.Errorf("property %q exceeds physical Single range", property)
	}
	return nil
}

type substrateHeaderField struct {
	property string
	column   string
	value    *float64
}

func substrateHeaderFields(h FS882Header) []substrateHeaderField {
	return []substrateHeaderField{
		{"substrateBedRock", "SubstrateBedRock", h.SubstrateBedRock},
		{"substrateDecWood", "SubstrateDecWood", h.SubstrateDecWood},
		{"substrateMineralSoil", "SubstrateMineralSoil", h.SubstrateMineralSoil},
		{"substrateOrganicMatter", "SubstrateOrganicMatter", h.SubstrateOrganicMatter},
		{"substrateRocks", "SubstrateRocks", h.SubstrateRocks},
		{"substrateWater", "SubstrateWater", h.SubstrateWater},
	}
}

func isSubstrateProperty(property string) bool {
	for _, field := range substrateHeaderFields(FS882Header{}) {
		if property == field.property {
			return true
		}
	}
	return false
}

func validateSubstrateHeaderValues(h FS882Header, old *FS882Header) error {
	previous := substrateHeaderFields(FS882Header{})
	if old != nil {
		previous = substrateHeaderFields(*old)
	}
	for i, field := range substrateHeaderFields(h) {
		if err := validateSingleRangeChange(field.property, previous[i].value, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstrateHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if err := validateSubstrateHeaderValues(h, nil); err == nil {
		return nil
	} else if mode == headerCreate {
		return err
	}
	caps, err := headerCapabilities(db, project)
	if err != nil {
		return err
	}
	for _, field := range substrateHeaderFields(h) {
		if err := validateSingleRangeChange(field.property, nil, field.value); err == nil {
			continue
		}
		if math.IsNaN(*field.value) || math.IsInf(*field.value, 0) {
			return validateSingleRangeChange(field.property, nil, field.value)
		}
		if !caps[field.property] {
			return fmt.Errorf("unsupported header property %q in active project", field.property)
		}
		var previous *float64
		err := db.QueryRow(`SELECT `+quoteHeaderIdentifier(field.column)+` FROM `+
			quoteHeaderIdentifier(project+"_Env")+` WHERE "PlotNumber"=?`, h.PlotNumber).Scan(&previous)
		if err == sql.ErrNoRows {
			return validateSingleRangeChange(field.property, nil, field.value)
		}
		if err != nil {
			return err
		}
		if err := validateSingleRangeChange(field.property, previous, field.value); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstrateRestoreValue(table, column string, value any) error {
	if table != "Env" {
		return nil
	}
	for _, field := range substrateHeaderFields(FS882Header{}) {
		if column != field.column {
			continue
		}
		if value == nil {
			return nil
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable number", column)
		}
		return validateSingleRangeChange(field.property, nil, &number)
	}
	return nil
}
