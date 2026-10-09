package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const googleEarthKMLFeatureEnvironment = "VPRO_GOOGLE_EARTH_KML_PREVIEW"

func googleEarthKMLFeature(lookup func(string) (string, bool)) (bool, error) {
	return siviFeature(googleEarthKMLFeatureEnvironment, lookup)
}

type GoogleEarthKMLService struct {
	contexts *ContextService
	enabled  bool
}

func NewGoogleEarthKMLService(contexts *ContextService, enabled bool) *GoogleEarthKMLService {
	return &GoogleEarthKMLService{contexts: contexts, enabled: enabled}
}

type GoogleEarthKMLRequest struct {
	DescriptionField string `json:"descriptionField"`
	Title            string `json:"title"`
}

func (request *GoogleEarthKMLRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthKMLRequest
	var decoded plain
	if err := decodeGoogleEarthKMLJSON(data, &decoded, "Google Earth KML preview", "descriptionField", "title"); err != nil {
		return err
	}
	*request = GoogleEarthKMLRequest(decoded)
	return nil
}

func decodeGoogleEarthKMLJSON(data []byte, target any, operation string, properties ...string) error {
	return decodeStrictRequiredJSON(data, target, operation, properties...)
}

func decodeStrictRequiredJSON(data []byte, target any, operation string, properties ...string) error {
	if err := decodeProfileLifecycleJSON(data, target, properties...); err != nil {
		return fmt.Errorf("%s request: %w", operation, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if _, err := decoder.Token(); err != nil {
		return err
	}
	seen := map[string]bool{}
	allowed := map[string]bool{}
	for _, property := range properties {
		allowed[property] = true
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok || !allowed[key] || seen[key] {
			return fmt.Errorf("%s has an unknown or duplicate property %q", operation, key)
		}
		seen[key] = true
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
	}
	return nil
}

type GoogleEarthKMLReview struct {
	ContextID        string `json:"contextId"`
	Project          string `json:"project"`
	ProjectPath      string `json:"projectPath"`
	SU               string `json:"su"`
	SUPath           string `json:"suPath"`
	DescriptionField string `json:"descriptionField"`
	Title            string `json:"title"`
	PlacemarkCount   int    `json:"placemarkCount"`
	ByteCount        int    `json:"byteCount"`
	KML              string `json:"kml"`
}

func (s *GoogleEarthKMLService) GetGoogleEarthKMLReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLReview, error) {
	if ctx == nil {
		return nil, errors.New("Google Earth KML preview requires a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || !s.enabled {
		return nil, errors.New("Google Earth KML byte preview is disabled in this session")
	}
	if s.contexts == nil {
		return nil, errors.New("Google Earth KML context service is unavailable")
	}
	var request GoogleEarthKMLRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := validateGoogleEarthReviewStrings(contextID, request.DescriptionField, request.Title); err != nil {
		return nil, err
	}
	if request.DescriptionField == "" || strings.ContainsRune(request.DescriptionField, 0) {
		return nil, errors.New("Google Earth KML preview requires a literal physical description field")
	}
	source, err := s.contexts.readGoogleEarthKML(ctx, contextID, request.DescriptionField, request.Title)
	if err != nil {
		return nil, err
	}
	review, err := googleEarthKMLReviewFromOwned(source)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return review, nil
}

func googleEarthKMLReviewFromOwned(source *ownedGoogleEarthKML) (*GoogleEarthKMLReview, error) {
	if err := validateGoogleEarthReviewStrings(source.ContextID, source.Project, source.ProjectPath, source.SU,
		source.SUPath, source.DescriptionField, source.Title, string(source.Bytes)); err != nil {
		return nil, err
	}
	return &GoogleEarthKMLReview{ContextID: source.ContextID, Project: source.Project, ProjectPath: source.ProjectPath,
		SU: source.SU, SUPath: source.SUPath, DescriptionField: source.DescriptionField, Title: source.Title,
		PlacemarkCount: source.PlacemarkCount, ByteCount: len(source.Bytes), KML: string(source.Bytes)}, nil
}
