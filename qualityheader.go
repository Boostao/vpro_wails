package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"unicode/utf8"
)

func qualityHeaderFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"SitePlotQuality", "sitePlotQuality", "Admin", 15, h.SitePlotQuality},
		{"VegPlotQuality", "vegPlotQuality", "Admin", 15, h.VegPlotQuality},
		{"SoilPlotQuality", "soilPlotQuality", "Admin", 15, h.SoilPlotQuality},
	}
}

func isQualityProperty(property string) bool {
	return property == "sitePlotQuality" || property == "vegPlotQuality" || property == "soilPlotQuality"
}

func validateQualityRestoreValue(table, column string, value any) error {
	if table != "Admin" {
		return nil
	}
	for _, field := range qualityHeaderFields(FS882Header{}) {
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
		return validateQualityValue(column, &text)
	}
	return nil
}

func validateQualityHeaderValues(h FS882Header, old *FS882Header) error {
	var previous []becHeaderField
	if old != nil {
		previous = qualityHeaderFields(*old)
	}
	for i, field := range qualityHeaderFields(h) {
		// Unicode is never grandfathered: Go/JSON must not repair malformed input.
		if field.value != nil && !utf8.ValidString(*field.value) {
			return validateQualityValue(field.name, field.value)
		}
		if err := validateQualityValue(field.name, field.value); err != nil {
			if old != nil && previous[i].value != nil && field.value != nil && *previous[i].value == *field.value {
				continue
			}
			return err
		}
	}
	return nil
}

func validateQualityHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	if err := validateQualityHeaderValues(h, nil); err == nil {
		return nil
	} else if mode == headerCreate {
		return err
	}
	caps, err := headerCapabilities(db, project)
	if err != nil {
		return err
	}
	old := FS882Header{}
	previous := []*(*string){&old.SitePlotQuality, &old.VegPlotQuality, &old.SoilPlotQuality}
	for i, field := range qualityHeaderFields(h) {
		if validateQualityValue(field.name, field.value) == nil {
			continue
		}
		if field.value != nil && !utf8.ValidString(*field.value) {
			return validateQualityValue(field.name, field.value)
		}
		if !caps[field.property] {
			return fmt.Errorf("unsupported header property %q in active project", field.property)
		}
		err := db.QueryRow(`SELECT `+quoteHeaderIdentifier(field.name)+` FROM `+
			quoteHeaderIdentifier(project+"_Admin")+` WHERE "Plot"=?`, h.PlotNumber).Scan(previous[i])
		if err == sql.ErrNoRows {
			return validateQualityHeaderValues(h, nil)
		}
		if err != nil {
			return err
		}
	}
	return validateQualityHeaderValues(h, &old)
}

// A token pass preserves each occurrence, including duplicate/case-folded keys,
// before encoding/json replaces malformed Unicode in string values.
func (h *FS882Header) UnmarshalJSON(data []byte) error {
	type headerAlias FS882Header
	fields := append(qualityHeaderFields(FS882Header{}), siteCodeHeaderFields(FS882Header{})...)
	fields = append(fields, regionHeaderFields(FS882Header{})...)
	fields = append(fields, soilHeaderFields(FS882Header{})...)
	fields = append(fields, geologyHeaderFields(FS882Header{})...)
	fields = append(fields, parentCodeHeaderFields(FS882Header{})...)
	fields = append(fields, ordinaryTextFields(FS882Header{})...)
	fields = append(fields, soilDrainageHeaderField(FS882Header{}))
	fields = append(fields, masterBECHeaderField(FS882Header{}))
	names := make(map[string]string, len(fields))
	for _, field := range fields {
		names[strings.ToLower(field.property)] = field.name
	}
	if err := validateJSONTextProperties(data, names); err != nil {
		return err
	}
	return json.Unmarshal(data, (*headerAlias)(h))
}

// The standard decoder retains its type/syntax errors.
