package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type VegetationCreationRequest struct {
	Form     string              `json:"form"`
	Species  string              `json:"species"`
	Values   map[string]*float64 `json:"values"`
	Decision string              `json:"decision,omitempty"`
	Entered  *string             `json:"entered,omitempty"`
	Selected *string             `json:"selected,omitempty"`
}

func (request *VegetationCreationRequest) UnmarshalJSON(data []byte) error {
	if err := validateJSONTextProperties(data, map[string]string{
		"form": "Vegetation creation form", "species": "Vegetation creation species",
		"decision": "Species decision", "entered": "Species entered code", "selected": "Species selected code",
	}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, property := range []string{"form", "species", "values"} {
		value, present := properties[property]
		if !present || strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("vegetation creation requires explicit non-NULL %s", property)
		}
	}
	if value, present := properties["decision"]; present && strings.TrimSpace(string(value)) == "null" {
		return errors.New("vegetation creation species decision must not be NULL")
	}
	for property := range properties {
		switch property {
		case "form", "species", "values", "decision", "entered", "selected":
		default:
			return fmt.Errorf("vegetation creation property %q is unavailable; identity and parent are assigned by the source context", property)
		}
	}
	type plain VegetationCreationRequest
	return json.Unmarshal(data, (*plain)(request))
}

func (request VegetationCreationRequest) speciesUpdate() VegetationSpeciesUpdate {
	return VegetationSpeciesUpdate{
		Form: request.Form, Value: request.Species, Decision: request.Decision,
		Entered: request.Entered, Selected: request.Selected,
	}
}

func prepareVegetationCreation(plot string, request VegetationCreationRequest) (VegRecord, string, string, error) {
	var empty VegRecord
	predicate, err := vegetationSpeciesPredicate(request.Form)
	if err != nil {
		return empty, "", "", err
	}
	rowPredicate, err := vegetationSpeciesRowPredicate(request.Form)
	if err != nil {
		return empty, "", "", err
	}
	if err := validateVegetationSpeciesLookup(request.Species); err != nil {
		return empty, "", "", err
	}
	if err := validateVegetationSpeciesDecision(request.speciesUpdate()); err != nil {
		return empty, "", "", err
	}
	if len(request.Values) == 0 {
		return empty, "", "", errors.New("vegetation creation requires explicit source numeric values; no zero covers are inferred")
	}
	for property, value := range request.Values {
		if !vegetationNumberForms[request.Form][property] {
			return empty, "", "", fmt.Errorf("vegetation creation property %q is unavailable in source form %q", property, request.Form)
		}
		if err := validateHeightNumericChange(property, nil, value); err != nil {
			return empty, "", "", err
		}
	}
	hasSourceValue := false
	for _, value := range request.Values {
		hasSourceValue = hasSourceValue || value != nil
	}
	if request.Form == "SubVegCXL" || request.Form == "SubVegChtXL" {
		hasSourceValue = request.Values["cover6"] != nil
	}
	if !hasSourceValue {
		return empty, "", "", fmt.Errorf("vegetation creation needs an explicit non-NULL value belonging to %s; no source cover is inferred", request.Form)
	}
	record := VegRecord{
		PlotNumber: plot, Species: request.Species,
		Cover1: request.Values["cover1"], Cover2: request.Values["cover2"],
		Cover3: request.Values["cover3"], TotalA: request.Values["totalA"],
		Cover4: request.Values["cover4"], Cover5: request.Values["cover5"],
		TotalB: request.Values["totalB"], Cover6: request.Values["cover6"],
		Cover7: request.Values["cover7"], Cover8: request.Values["cover8"],
		Cover9: request.Values["cover9"], Height1: request.Values["height1"],
		Height2: request.Values["height2"], Height3: request.Values["height3"],
		Height4: request.Values["height4"], Height5: request.Values["height5"],
		Height6: request.Values["height6"],
	}
	return record, predicate, rowPredicate, nil
}

func (s *PlotService) CreateSourceVegetation(plot string, request VegetationCreationRequest) (int64, error) {
	record, predicate, rowPredicate, err := prepareVegetationCreation(plot, request)
	if err != nil {
		return 0, err
	}
	var identity int64
	err = s.saveChildWithChecks("Veg", record, childSave, nil,
		func(tx *sql.Tx, table, parent string, id int64) error {
			if err := s.validateVegetationSpeciesReference(tx, request.speciesUpdate(), predicate); err != nil {
				return err
			}
			condition := rowPredicate + ` AND Species COLLATE BINARY=?`
			args := []any{parent, id, request.Species}
			for property, value := range request.Values {
				column := strings.ToUpper(property[:1]) + property[1:]
				condition += ` AND ` + quoteHeaderIdentifier(column) + ` IS ?`
				args = append(args, heightNumber(value))
			}
			var visible bool
			if err := tx.QueryRowContext(s.operationContext(), `SELECT EXISTS(SELECT 1 FROM `+table+
				` WHERE PlotNumber=? AND ID=? AND `+condition+`)`, args...).Scan(&visible); err != nil {
				return fmt.Errorf("created vegetation source row could not be checked: %w", err)
			}
			if !visible {
				return fmt.Errorf("created vegetation row differs from the planned species/numeric values or source membership in %s; no source value is inferred", request.Form)
			}
			identity = id
			return nil
		})
	if err != nil {
		return 0, err
	}
	return identity, nil
}

func (s *ContextService) CreateSourceVegetation(contextID, plot string, request VegetationCreationRequest) (int64, error) {
	var identity int64
	err := s.edit(contextID, func(plots *PlotService) error {
		var err error
		identity, err = plots.CreateSourceVegetation(plot, request)
		return err
	})
	return identity, err
}
