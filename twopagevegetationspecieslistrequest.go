package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
)

func (references *twoPageVegetationSpeciesReferences) UnmarshalJSON(data []byte) error {
	required := []string{"contextId", "project", "plot", "form", "source", "master", "personal"}
	properties, err := sourceChildJSONObject(data, "two-page Species list references", required, nil)
	if err != nil {
		return err
	}
	if err := sourceChildJSONNonNull(properties, "two-page Species list references", required...); err != nil {
		return err
	}
	for _, name := range []string{"master", "personal"} {
		if err := siviSpeciesTableJSON(properties[name]); err != nil {
			return err
		}
	}
	type plain twoPageVegetationSpeciesReferences
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Source != twoPageVegetationSpeciesReferenceSource {
		return errors.New("two-page Species references require their exact physical source provenance")
	}
	for _, source := range []struct {
		table    ProjectMetadataTable
		personal bool
	}{{decoded.Master, false}, {decoded.Personal, true}} {
		projected, err := projectTwoPageVegetationSpeciesDefinitions(context.Background(), source.table, source.personal)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(projected, source.table) {
			return errors.New("two-page Species references require exactly their original projected physical columns")
		}
	}
	*references = twoPageVegetationSpeciesReferences(decoded)
	return nil
}

func planTwoPageVegetationRequest(ctx context.Context, request twoPageVegetationRequest, canonicalSingle bool) ([]twoPageVegetationAssignment, error) {
	plan, err := planTwoPageVegetationQualifiedEdits(ctx, request.Original, request.Edits, canonicalSingle, request.SpeciesList)
	if err != nil {
		return nil, err
	}
	if request.SpeciesList != nil {
		changedSpecies := false
		for _, assignment := range plan {
			changedSpecies = changedSpecies || assignment.Column == "Species"
		}
		if !changedSpecies {
			return nil, errors.New("two-page Species list request requires a changed qualified Species assignment")
		}
	}
	return plan, nil
}
