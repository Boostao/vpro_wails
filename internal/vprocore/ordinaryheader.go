package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"fmt"
	"reflect"
	"unicode/utf8"
)

func OrdinaryTextFields(h FS882Header) []BecHeaderField {
	return []BecHeaderField{
		{"AirPhotoNum", "airPhotoNum", "Env", 20, h.AirPhotoNum},
		{"EnteredBy", "enteredBy", "Admin", 50, h.EnteredBy},
		{"SnowCoverregime", "snowCoverRegime", "Env", 1, h.SnowCoverRegime},
		{"SoilSurveyor", "soilSurveyor", "Env", 30, h.SoilSurveyor},
		{"VegSurveyor", "vegSurveyor", "Env", 30, h.VegSurveyor},
		{"SoilNotes", "soilNotes", "Env", 0, h.SoilNotes},
		{"VegNotes", "vegNotes", "Env", 0, h.VegNotes},
		{"Photo", "photo", "Env", 50, h.Photo},
	}
}

type OrdinaryNumberField struct {
	Name, property, Table string `json:"-" yaml:"-"`
	integer               *int
	single                *float64
	IsInteger             bool `json:"-" yaml:"-"`
}

func OrdinaryNumberFields(h FS882Header) []OrdinaryNumberField {
	return []OrdinaryNumberField{
		{"RootRestrictingDepth", "rootRestrictingDepth", "Env", h.RootRestrictingDepth, nil, true},
		{"RootingDepth", "rootingDepth", "Env", h.RootingDepth, nil, true},
		{"SeepageDepth", "seepageDepth", "Env", h.SeepageDepth, nil, true},
		{"ActiveLayerDepth", "activeLayerDepth", "Env", nil, h.ActiveLayerDepth, false},
		{"HumusThickness", "humusThickness", "Admin", nil, h.HumusThickness, false},
		{"StrataCoverHerb", "strataCoverHerb", "Env", nil, h.StrataCoverHerb, false},
		{"StrataCoverMoss", "strataCoverMoss", "Env", nil, h.StrataCoverMoss, false},
		{"StrataCoverShrub", "strataCoverShrub", "Env", nil, h.StrataCoverShrub, false},
		{"StrataCoverTree", "strataCoverTree", "Env", nil, h.StrataCoverTree, false},
		{"XCoord", "xCoord", "Env", nil, h.XCoord, false},
		{"YCoord", "yCoord", "Env", nil, h.YCoord, false},
	}
}

func IsOrdinaryProperty(property string) bool {
	for _, field := range OrdinaryTextFields(FS882Header{}) {
		if field.Property == property {
			return true
		}
	}
	for _, field := range OrdinaryNumberFields(FS882Header{}) {
		if field.property == property {
			return true
		}
	}
	return false
}

func ValidateOrdinaryText(field BecHeaderField) error {
	if field.Maximum > 0 {
		return sqlutil.ValidateSiteCodeText(field.Name, field.Value, field.Maximum)
	}
	if field.Value != nil && (!utf8.ValidString(*field.Value) || *field.Value == "") {
		return fmt.Errorf("%s must be NULL or nonempty valid Unicode text", field.Name)
	}
	return nil
}

func ValidateOrdinaryInteger(name string, value *int) error {
	if value != nil && (*value < -32768 || *value > 32767) {
		return fmt.Errorf("%s exceeds the physical Access Integer range (-32768 to 32767)", name)
	}
	return nil
}

var strataCoverHeaderColumns = map[string]bool{
	"StrataCoverHerb":  true,
	"StrataCoverMoss":  true,
	"StrataCoverShrub": true,
	"StrataCoverTree":  true,
}

func validateOrdinaryHeaderValuesWithCoverMode(h FS882Header, old *FS882Header, enforceCoverBounds bool) error {
	previous := FS882Header{}
	if old != nil {
		previous = *old
	}
	texts := OrdinaryTextFields(previous)
	for index, field := range OrdinaryTextFields(h) {
		if old != nil && SameSiteCode(texts[index].Value, field.Value) {
			continue
		}
		if err := ValidateOrdinaryText(field); err != nil {
			return err
		}
	}
	numbers := OrdinaryNumberFields(previous)
	for index, field := range OrdinaryNumberFields(h) {
		if field.IsInteger {
			if old != nil && reflect.DeepEqual(field.integer, numbers[index].integer) {
				continue
			}
			if err := ValidateOrdinaryInteger(field.Name, field.integer); err != nil {
				return err
			}
		} else if enforceCoverBounds && strataCoverHeaderColumns[field.Name] {
			if err := ValidateSingleCoverChange(field.Name, numbers[index].single, field.single); err != nil {
				return err
			}
		} else if err := ValidateSingleRangeChange(field.Name, numbers[index].single, field.single); err != nil {
			return err
		}
	}
	return ValidateSourceParentBooleanNull("SpeciesListComplete", previous.SpeciesListComplete == nil, h.SpeciesListComplete == nil)
}

func validateOrdinaryHeaderValues(h FS882Header, old *FS882Header) error {
	return validateOrdinaryHeaderValuesWithCoverMode(h, old, true)
}

func (s *PlotService) validateOrdinaryHeaderValues(h FS882Header, old *FS882Header) error {
	return validateOrdinaryHeaderValuesWithCoverMode(h, old, s.ContextScoped)
}

func (s *PlotService) validateOrdinaryHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, s.validateOrdinaryHeaderValues)
}

func ValidateOrdinaryRestoreValue(table, column string, value any) error {
	for _, field := range OrdinaryTextFields(FS882Header{}) {
		if field.Table != table || field.Name != column || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s audit restore requires nullable text", column)
		}
		field.Value = &text
		return ValidateOrdinaryText(field)
	}
	for _, field := range OrdinaryNumberFields(FS882Header{}) {
		if field.Table != table || field.Name != column || value == nil {
			continue
		}
		if field.IsInteger {
			number, ok := value.(int)
			if !ok {
				return fmt.Errorf("%s audit restore requires a nullable integer", column)
			}
			return ValidateOrdinaryInteger(column, &number)
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable number", column)
		}
		return ValidateSingleRangeChange(column, nil, &number)
	}
	return nil
}
