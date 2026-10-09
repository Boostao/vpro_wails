package main

import (
	"database/sql"
	"fmt"
	"reflect"
	"unicode/utf8"
)

func ordinaryTextFields(h FS882Header) []becHeaderField {
	return []becHeaderField{
		{"AirPhotoNum", "airPhotoNum", "Env", 20, h.AirPhotoNum},
		{"EnteredBy", "enteredBy", "Admin", 50, h.EnteredBy},
		{"SoilSurveyor", "soilSurveyor", "Env", 30, h.SoilSurveyor},
		{"VegSurveyor", "vegSurveyor", "Env", 30, h.VegSurveyor},
		{"SoilNotes", "soilNotes", "Env", 0, h.SoilNotes},
		{"VegNotes", "vegNotes", "Env", 0, h.VegNotes},
		{"Photo", "photo", "Env", 50, h.Photo},
	}
}

type ordinaryNumberField struct {
	name, property, table string
	integer               *int
	single                *float64
	isInteger             bool
}

func ordinaryNumberFields(h FS882Header) []ordinaryNumberField {
	return []ordinaryNumberField{
		{"RootRestrictingDepth", "rootRestrictingDepth", "Env", h.RootRestrictingDepth, nil, true},
		{"RootingDepth", "rootingDepth", "Env", h.RootingDepth, nil, true},
		{"SeepageDepth", "seepageDepth", "Env", h.SeepageDepth, nil, true},
		{"HumusThickness", "humusThickness", "Admin", nil, h.HumusThickness, false},
		{"StrataCoverHerb", "strataCoverHerb", "Env", nil, h.StrataCoverHerb, false},
		{"StrataCoverMoss", "strataCoverMoss", "Env", nil, h.StrataCoverMoss, false},
		{"StrataCoverShrub", "strataCoverShrub", "Env", nil, h.StrataCoverShrub, false},
		{"StrataCoverTree", "strataCoverTree", "Env", nil, h.StrataCoverTree, false},
		{"XCoord", "xCoord", "Env", nil, h.XCoord, false},
		{"YCoord", "yCoord", "Env", nil, h.YCoord, false},
	}
}

func isOrdinaryProperty(property string) bool {
	for _, field := range ordinaryTextFields(FS882Header{}) {
		if field.property == property {
			return true
		}
	}
	for _, field := range ordinaryNumberFields(FS882Header{}) {
		if field.property == property {
			return true
		}
	}
	return false
}

func validateOrdinaryText(field becHeaderField) error {
	if field.maximum > 0 {
		return validateSiteCodeText(field.name, field.value, field.maximum)
	}
	if field.value != nil && (!utf8.ValidString(*field.value) || *field.value == "") {
		return fmt.Errorf("%s must be NULL or nonempty valid Unicode text", field.name)
	}
	return nil
}

func validateOrdinaryInteger(name string, value *int) error {
	if value != nil && (*value < -32768 || *value > 32767) {
		return fmt.Errorf("%s exceeds the physical Access Integer range (-32768 to 32767)", name)
	}
	return nil
}

func validateOrdinaryHeaderValues(h FS882Header, old *FS882Header) error {
	previous := FS882Header{}
	if old != nil {
		previous = *old
	}
	texts := ordinaryTextFields(previous)
	for index, field := range ordinaryTextFields(h) {
		if old != nil && sameSiteCode(texts[index].value, field.value) {
			continue
		}
		if err := validateOrdinaryText(field); err != nil {
			return err
		}
	}
	numbers := ordinaryNumberFields(previous)
	for index, field := range ordinaryNumberFields(h) {
		if field.isInteger {
			if old != nil && reflect.DeepEqual(field.integer, numbers[index].integer) {
				continue
			}
			if err := validateOrdinaryInteger(field.name, field.integer); err != nil {
				return err
			}
		} else if err := validateSingleRangeChange(field.name, numbers[index].single, field.single); err != nil {
			return err
		}
	}
	return nil
}

func validateOrdinaryHeaderBeforeTransaction(db *sql.DB, project string, h FS882Header, mode headerSaveMode) error {
	return validateCodeHeaderBeforeTransaction(db, project, h, mode, validateOrdinaryHeaderValues)
}

func validateOrdinaryRestoreValue(table, column string, value any) error {
	for _, field := range ordinaryTextFields(FS882Header{}) {
		if field.table != table || field.name != column || value == nil {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return fmt.Errorf("%s audit restore requires nullable text", column)
		}
		field.value = &text
		return validateOrdinaryText(field)
	}
	for _, field := range ordinaryNumberFields(FS882Header{}) {
		if field.table != table || field.name != column || value == nil {
			continue
		}
		if field.isInteger {
			number, ok := value.(int)
			if !ok {
				return fmt.Errorf("%s audit restore requires a nullable integer", column)
			}
			return validateOrdinaryInteger(column, &number)
		}
		number, ok := value.(float64)
		if !ok {
			return fmt.Errorf("%s audit restore requires a nullable number", column)
		}
		return validateSingleRangeChange(column, nil, &number)
	}
	return nil
}
