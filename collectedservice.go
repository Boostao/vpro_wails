package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"
)

type CollectedRecordUpdate struct {
	ID       int     `json:"id"`
	Expected *string `json:"expected"`
	Clicks   int     `json:"clicks"`
}

func (update *CollectedRecordUpdate) UnmarshalJSON(data []byte) error {
	type plain CollectedRecordUpdate
	if err := validateJSONTextProperties(data, map[string]string{"expected": "Collected expected"}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, property := range []string{"id", "expected", "clicks"} {
		raw, present := properties[property]
		if !present || (property != "expected" && strings.TrimSpace(string(raw)) == "null") {
			return fmt.Errorf("Collected draft requires explicit %s (expected may be NULL)", property)
		}
	}
	return json.Unmarshal(data, (*plain)(update))
}

func collectedAfterClicks(expected *string, clicks int) *string {
	value := expected
	for range clicks {
		if value == nil {
			next := "C"
			value = &next
		} else if *value == "C" || *value == "c" || *value == "\uFF23" || *value == "\uFF43" {
			next := "V"
			value = &next
		} else if *value == "V" || *value == "v" || *value == "\uFF36" || *value == "\uFF56" {
			value = nil
		}
	}
	return value
}

func (s *PlotService) UpdateCollectedRecords(plot string, updates []CollectedRecordUpdate) error {
	patches := make([]childRecordPatch, 0, len(updates))
	for _, update := range updates {
		if update.Clicks < 1 || update.Clicks > 3 {
			return errors.New("Collected draft requires one to three cycle clicks")
		}
		if update.Expected != nil && !utf8.ValidString(*update.Expected) {
			return errors.New("Collected expected contains malformed UTF-8")
		}
		patches = append(patches, childRecordPatch{kind: "Veg", id: update.ID, cells: map[string]childExpectedValue{
			"collected": {otherNullable(update.Expected), otherNullable(collectedAfterClicks(update.Expected, update.Clicks)), reflect.String},
		}})
	}
	return s.updateChildPatches(plot, patches)
}

func (s *ContextService) UpdateCollectedRecords(contextID, plot string, updates []CollectedRecordUpdate) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.UpdateCollectedRecords(plot, updates) })
}
