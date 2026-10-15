package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

const profileSpeciesPresenceEnvironment = "VPRO_PROFILE_SPECIES_PRESENCE_PREVIEW"
const profileSpeciesPresenceMode = "saved-species-only-readonly"

type ProjectProfileSpeciesPresencePreview struct {
	ContextID   string   `json:"contextId"`
	Mode        string   `json:"mode"`
	Project     string   `json:"project"`
	Table       string   `json:"table"`
	SU          string   `json:"su"`
	TotalPlots  int      `json:"totalPlots"`
	PlotNumbers []string `json:"plotNumbers"`
}

type ProjectProfileSpeciesPresenceService struct {
	contexts *ContextService
	enabled  bool
}

func NewProjectProfileSpeciesPresenceService(contexts *ContextService, lookup func(string) (string, bool)) (*ProjectProfileSpeciesPresenceService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("species-only preview requires owned contexts and explicit feature lookup")
	}
	enabled, err := siviFeature(profileSpeciesPresenceEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &ProjectProfileSpeciesPresenceService{contexts: contexts, enabled: enabled}, nil
}

func decodeSpeciesPresenceRequest(data []byte) (ProjectPlotProfileRunRequest, error) {
	var request ProjectPlotProfileRunRequest
	if err := validateTwoPageEntryJSONProperties(data); err != nil {
		return request, fmt.Errorf("species-only preview request: %w", err)
	}
	var wire any
	if err := json.Unmarshal(data, &wire); err != nil {
		return request, fmt.Errorf("species-only preview request: %w", err)
	}
	if err := validateSpeciesPresenceWireKeys(wire); err != nil {
		return request, err
	}
	if err := decodeProfileLifecycleJSON(data, &request, "originalRules", "subvarieties"); err != nil {
		return ProjectPlotProfileRunRequest{}, fmt.Errorf("species-only preview request: %w", err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return ProjectPlotProfileRunRequest{}, err
	}
	for name := range properties {
		switch name {
		case "originalRules", "projectLump", "subvarieties":
		default:
			return ProjectPlotProfileRunRequest{}, fmt.Errorf("species-only preview request has unknown property %q", name)
		}
	}
	return request, nil
}

func validateSpeciesPresenceWireKeys(value any) error {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "originalRules", "projectLump", "subvarieties",
				"columns", "rows", "name", "declaredType", "rowId", "cells",
				"storage", "text", "integer", "real", "blobHex":
			default:
				return fmt.Errorf("species-only preview request has unknown or noncanonical property %q", key)
			}
			if err := validateSpeciesPresenceWireKeys(child); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := validateSpeciesPresenceWireKeys(child); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ProjectProfileSpeciesPresenceService) Preview(ctx context.Context, contextID, requestJSON string) (*ProjectProfileSpeciesPresencePreview, error) {
	if ctx == nil {
		return nil, errors.New("species-only preview requires a request context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.contexts == nil || !s.enabled {
		return nil, errors.New("species-only preview requires independently enabled review; original form callback remains unavailable")
	}
	request, err := decodeSpeciesPresenceRequest([]byte(requestJSON))
	if err != nil {
		return nil, err
	}
	result, err := s.contexts.previewProjectPlotProfilePresence(ctx, contextID, request)
	if err != nil {
		return nil, err
	}
	return &ProjectProfileSpeciesPresencePreview{
		ContextID: contextID, Mode: profileSpeciesPresenceMode,
		Project: result.Project, Table: result.Table, SU: result.SU,
		TotalPlots: result.TotalPlots, PlotNumbers: result.PlotNumbers,
	}, nil
}
