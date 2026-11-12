package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"unicode/utf8"
)

func QualityHeaderFields(h FS882Header) []BecHeaderField {
	return []BecHeaderField{
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
	for _, field := range QualityHeaderFields(FS882Header{}) {
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
		return sqlutil.ValidateQualityValue(column, &text)
	}
	return nil
}

func validateQualityHeaderValues(h FS882Header, old *FS882Header) error {
	var previous []BecHeaderField
	if old != nil {
		previous = QualityHeaderFields(*old)
	}
	for i, field := range QualityHeaderFields(h) {
		// Unicode is never grandfathered: Go/JSON must not repair malformed input.
		if field.Value != nil && !utf8.ValidString(*field.Value) {
			return sqlutil.ValidateQualityValue(field.Name, field.Value)
		}
		if err := sqlutil.ValidateQualityValue(field.Name, field.Value); err != nil {
			if old != nil && previous[i].Value != nil && field.Value != nil && *previous[i].Value == *field.Value {
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
	caps, err := HeaderCapabilities(db, project)
	if err != nil {
		return err
	}
	old := FS882Header{}
	previous := []*(*string){&old.SitePlotQuality, &old.VegPlotQuality, &old.SoilPlotQuality}
	for i, field := range QualityHeaderFields(h) {
		if sqlutil.ValidateQualityValue(field.Name, field.Value) == nil {
			continue
		}
		if field.Value != nil && !utf8.ValidString(*field.Value) {
			return sqlutil.ValidateQualityValue(field.Name, field.Value)
		}
		if !caps[field.Property] {
			return fmt.Errorf("unsupported header property %q in active project", field.Property)
		}
		err := db.QueryRow(`SELECT `+QuoteHeaderIdentifier(field.Name)+` FROM `+
			QuoteHeaderIdentifier(project+"_Admin")+` WHERE "Plot"=?`, h.PlotNumber).Scan(previous[i])
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
	fields := append(QualityHeaderFields(FS882Header{}), SiteCodeHeaderFields(FS882Header{})...)
	fields = append(fields, RegionHeaderFields(FS882Header{})...)
	fields = append(fields, SoilHeaderFields(FS882Header{})...)
	fields = append(fields, GeologyHeaderFields(FS882Header{})...)
	fields = append(fields, ParentCodeHeaderFields(FS882Header{})...)
	fields = append(fields, OrdinaryTextFields(FS882Header{})...)
	fields = append(fields, soilDrainageHeaderField(FS882Header{}))
	fields = append(fields, masterBECHeaderField(FS882Header{}))
	names := make(map[string]string, len(fields))
	for _, field := range fields {
		names[strings.ToLower(field.Property)] = field.Name
	}
	if err := sqlutil.ValidateJSONTextProperties(data, names); err != nil {
		return err
	}
	return json.Unmarshal(data, (*headerAlias)(h))
}

// The standard decoder retains its type/syntax errors.
