package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf16"
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

func validateQualityValue(name string, value *string) error {
	if value == nil {
		return nil
	}
	if !utf8.ValidString(*value) {
		return fmt.Errorf("%s must contain valid Unicode (invalid UTF-8)", name)
	}
	if *value == "" || len(utf16.Encode([]rune(*value))) > 15 {
		return fmt.Errorf("%s must be NULL or a nonempty string of at most 15 UTF-16 units", name)
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
	var syntax json.RawMessage
	if err := json.Unmarshal(data, &syntax); err != nil {
		return err
	}
	if len(syntax) == 0 || syntax[0] != '{' {
		return json.Unmarshal(data, (*headerAlias)(h))
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return nil
	}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok {
			continue
		}
		fields := append(qualityHeaderFields(FS882Header{}), siteCodeHeaderFields(FS882Header{})...)
		fields = append(fields, regionHeaderFields(FS882Header{})...)
		fields = append(fields, soilHeaderFields(FS882Header{})...)
		fields = append(fields, geologyHeaderFields(FS882Header{})...)
		fields = append(fields, parentCodeHeaderFields(FS882Header{})...)
		for _, field := range fields {
			if strings.EqualFold(name, field.property) {
				if err := validateQualityJSONToken(raw); err != nil {
					return fmt.Errorf("%s: %w", field.name, err)
				}
			}
		}
	}
	return json.Unmarshal(data, (*headerAlias)(h))
}

func validateQualityJSONToken(raw []byte) error {
	if len(raw) == 0 || raw[0] != '"' {
		return nil // The standard decoder retains its type/syntax errors.
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8 in quality JSON string")
	}
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if raw[i] != 'u' {
			continue
		}
		unit := qualityHexUnit(raw[i+1 : i+5])
		i += 4
		if unit >= 0xDC00 && unit <= 0xDFFF {
			return fmt.Errorf("unpaired low UTF-16 surrogate in quality JSON string")
		}
		if unit < 0xD800 || unit > 0xDBFF {
			continue
		}
		if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return fmt.Errorf("unpaired high UTF-16 surrogate in quality JSON string")
		}
		low := qualityHexUnit(raw[i+3 : i+7])
		if low < 0xDC00 || low > 0xDFFF {
			return fmt.Errorf("mismatched UTF-16 surrogate pair in quality JSON string")
		}
		i += 6
	}
	return nil
}

func qualityHexUnit(data []byte) uint16 {
	var value uint16
	for _, digit := range data {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		}
	}
	return value
}
