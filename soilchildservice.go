package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
)

type SoilTextUpdate struct {
	Value    *string `json:"value"`
	Expected *string `json:"expected"`
}

func decodeChildChange(data []byte, target any) error {
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	if _, present := properties["value"]; !present {
		return errors.New("child change requires explicit value and expected")
	}
	if _, present := properties["expected"]; !present {
		return errors.New("child change requires explicit value and expected")
	}
	return json.Unmarshal(data, target)
}

func (change *SoilTextUpdate) UnmarshalJSON(data []byte) error {
	type plain SoilTextUpdate
	if err := validateJSONTextProperties(data, map[string]string{"value": "Soil value", "expected": "Soil expected"}); err != nil {
		return err
	}
	return decodeChildChange(data, (*plain)(change))
}

type SoilNumberUpdate struct {
	Value    *float64 `json:"value"`
	Expected *float64 `json:"expected"`
}

func (change *SoilNumberUpdate) UnmarshalJSON(data []byte) error {
	type plain SoilNumberUpdate
	return decodeChildChange(data, (*plain)(change))
}

type SoilRecordUpdate struct {
	Kind    string                      `json:"kind"`
	ID      int                         `json:"id"`
	Text    map[string]SoilTextUpdate   `json:"text"`
	Numbers map[string]SoilNumberUpdate `json:"numbers"`
}

func (update *SoilRecordUpdate) UnmarshalJSON(data []byte) error {
	type plain SoilRecordUpdate
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range []string{"id", "kind"} {
		if value, present := properties[name]; !present || string(value) == "null" {
			return errors.New("soil draft requires an explicit kind and signed32 identity")
		}
	}
	return json.Unmarshal(data, (*plain)(update))
}

func soilPatchNumber(kind, column string, number *float64) (any, reflect.Kind, error) {
	scalar := reflect.Float64
	if soilChildIntegerColumns[kind][column] {
		scalar = reflect.Int
	}
	if number == nil {
		return nil, scalar, nil
	}
	if math.IsNaN(*number) || math.IsInf(*number, 0) {
		return nil, scalar, fmt.Errorf("%s.%s requires a finite number", kind, column)
	}
	if scalar == reflect.Int {
		if math.Trunc(*number) != *number || math.Abs(*number) > 9007199254740991 {
			return nil, scalar, fmt.Errorf("%s.%s requires an exact integer", kind, column)
		}
		return int(*number), scalar, nil
	}
	return *number, scalar, nil
}

func (s *PlotService) UpdateSoilRecords(plot string, updates []SoilRecordUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		if update.Kind != "Humus" && update.Kind != "Mineral" {
			return fmt.Errorf("unsupported soil draft kind %q", update.Kind)
		}
		patch := childRecordPatch{kind: update.Kind, id: update.ID, cells: map[string]childExpectedValue{}}
		for property, change := range update.Text {
			patch.cells[property] = childExpectedValue{otherNullable(change.Expected), otherNullable(change.Value), reflect.String}
		}
		for property, change := range update.Numbers {
			if _, present := patch.cells[property]; present {
				return fmt.Errorf("soil property %q has conflicting change types", property)
			}
			var column string
			for _, field := range childFields[update.Kind] {
				if childJSONKey(childPatchShape(update.Kind), field) == property {
					column = field.column
					break
				}
			}
			expected, scalar, err := soilPatchNumber(update.Kind, column, change.Expected)
			if err != nil {
				return err
			}
			value, _, err := soilPatchNumber(update.Kind, column, change.Value)
			if err != nil {
				return err
			}
			patch.cells[property] = childExpectedValue{expected, value, scalar}
		}
		patches = append(patches, patch)
	}
	return s.updateChildPatches(plot, patches)
}
