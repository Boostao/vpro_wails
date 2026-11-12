package vprocore

import (
	"database/sql"
	"fmt"
	"math"
)

// Retain finite imported overflow only when it equals the actual stored value.
// New values preserve float64 precision within the native SINGLE range.
func ValidateSingleRangeChange(property string, before, after *float64) error {
	if after == nil {
		return nil
	}
	if math.IsNaN(*after) || math.IsInf(*after, 0) {
		return fmt.Errorf("property %q must be finite", property)
	}
	if SameHeightValue(before, after) {
		return nil
	}
	if math.Abs(*after) > math.MaxFloat32 {
		return fmt.Errorf("property %q exceeds physical Single range", property)
	}
	return nil
}

type substrateHeaderField struct {
	Property string   `json:"-" yaml:"-"`
	Column   string   `json:"-" yaml:"-"`
	Value    *float64 `json:"-" yaml:"-"`
}

func SubstrateHeaderFields(h FS882Header) []substrateHeaderField {
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
	for _, field := range SubstrateHeaderFields(FS882Header{}) {
		if property == field.Property {
			return true
		}
	}
	return false
}

func ValidateSubstrateHeaderValues(h FS882Header, old *FS882Header) error {
	previous := SubstrateHeaderFields(FS882Header{})
	if old != nil {
		previous = SubstrateHeaderFields(*old)
	}
	for i, field := range SubstrateHeaderFields(h) {
		if err := ValidateSingleRangeChange(field.Property, previous[i].Value, field.Value); err != nil {
			return err
		}
	}
	return nil
}

func ValidateSubstrateHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if err := ValidateSubstrateHeaderValues(h, nil); err == nil {
		return nil
	} else if mode == headerCreate {
		return err
	}
	caps, err := HeaderCapabilities(db, project)
	if err != nil {
		return err
	}
	for _, field := range SubstrateHeaderFields(h) {
		if err := ValidateSingleRangeChange(field.Property, nil, field.Value); err == nil {
			continue
		}
		if math.IsNaN(*field.Value) || math.IsInf(*field.Value, 0) {
			return ValidateSingleRangeChange(field.Property, nil, field.Value)
		}
		if !caps[field.Property] {
			return fmt.Errorf("unsupported header property %q in active project", field.Property)
		}
		var previous *float64
		err := db.QueryRow(`SELECT `+QuoteHeaderIdentifier(field.Column)+` FROM `+
			QuoteHeaderIdentifier(project+"_Env")+` WHERE "PlotNumber"=?`, h.PlotNumber).Scan(&previous)
		if err == sql.ErrNoRows {
			return ValidateSingleRangeChange(field.Property, nil, field.Value)
		}
		if err != nil {
			return err
		}
		if err := ValidateSingleRangeChange(field.Property, previous, field.Value); err != nil {
			return err
		}
	}
	return nil
}

func validateSubstrateRestoreValue(table, column string, value any) error {
	if table != "Env" {
		return nil
	}
	for _, field := range SubstrateHeaderFields(FS882Header{}) {
		if column != field.Column {
			continue
		}
		if value == nil {
			return nil
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable number", column)
		}
		return ValidateSingleRangeChange(field.Property, nil, &number)
	}
	return nil
}
