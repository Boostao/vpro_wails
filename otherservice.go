package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var otherTextMaximum = map[string]int{
	"DataName": 50, "DataItem": 255, "UserItem1": 255, "UserItem2": 255, "UserItem3": 255,
}

var otherTextJSONNames = map[string]string{
	"dataname": "Other.DataName", "dataitem": "Other.DataItem",
	"useritem1": "Other.UserItem1", "useritem2": "Other.UserItem2", "useritem3": "Other.UserItem3",
}

func validateOtherField(column string, value any) error {
	if value == nil {
		return nil
	}
	if maximum, text := otherTextMaximum[column]; text {
		return validateChildPhysicalText("Other."+column, value, maximum)
	}
	switch column {
	case "UserFlag1", "UserFlag2", "UserFlag3":
		if _, ok := value.(bool); ok {
			return nil
		}
	}
	return fmt.Errorf("unsupported Other value for %q", column)
}

type OtherTextUpdate struct {
	Value    *string `json:"value"`
	Expected *string `json:"expected"`
}

func (change *OtherTextUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherTextUpdate
	if err := validateJSONTextProperties(data, map[string]string{"value": "Other value", "expected": "Other expected"}); err != nil {
		return err
	}
	return decodeChildChange(data, (*plain)(change))
}

type OtherFlagUpdate struct {
	Value    *bool `json:"value"`
	Expected *bool `json:"expected"`
}

func (change *OtherFlagUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherFlagUpdate
	return decodeChildChange(data, (*plain)(change))
}

type OtherRecordUpdate struct {
	ID    int                        `json:"id"`
	Text  map[string]OtherTextUpdate `json:"text"`
	Flags map[string]OtherFlagUpdate `json:"flags"`
}

func (update *OtherRecordUpdate) UnmarshalJSON(data []byte) error {
	type plain OtherRecordUpdate
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	raw, present := properties["id"]
	if !present || string(raw) == "null" {
		return errors.New("Other draft requires an explicit signed32 identity")
	}
	return json.Unmarshal(data, (*plain)(update))
}

func otherNullable[T string | bool](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

// Patch only explicit cells; expected values prevent stale full-row replacement.
func (s *PlotService) UpdateOtherRecords(plot string, updates []OtherRecordUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		patch := childRecordPatch{kind: "Other", id: update.ID, cells: map[string]childExpectedValue{}}
		for property, change := range update.Text {
			patch.cells[property] = childExpectedValue{otherNullable(change.Expected), otherNullable(change.Value), reflect.String}
		}
		for property, change := range update.Flags {
			if _, present := patch.cells[property]; present {
				return fmt.Errorf("Other property %q has conflicting change types", property)
			}
			patch.cells[property] = childExpectedValue{otherNullable(change.Expected), otherNullable(change.Value), reflect.Bool}
		}
		patches = append(patches, patch)
	}
	return s.updateChildPatches(plot, patches)
}
