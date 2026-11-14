package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"encoding/json"
	"errors"

	"strings"
)

type GoogleEarthKMLService struct {
	contexts *vprocore.ContextService
}

func NewGoogleEarthKMLService(contexts *vprocore.ContextService) *GoogleEarthKMLService {
	return &GoogleEarthKMLService{contexts: contexts}
}

type GoogleEarthKMLRequest struct {
	DescriptionField string `json:"descriptionField"`
	Title            string `json:"title"`
}

func (request *GoogleEarthKMLRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthKMLRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth KML preview", "descriptionField", "title"); err != nil {
		return err
	}
	*request = GoogleEarthKMLRequest(decoded)
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
	if s == nil || s.contexts == nil {
		return nil, errors.New("Google Earth KML context service is unavailable")
	}
	var request GoogleEarthKMLRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.DescriptionField, request.Title); err != nil {
		return nil, err
	}
	if request.DescriptionField == "" || strings.ContainsRune(request.DescriptionField, 0) {
		return nil, errors.New("Google Earth KML preview requires a literal physical description field")
	}
	source, err := ReadGoogleEarthKML(s.contexts, ctx, contextID, request.DescriptionField, request.Title)
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
	if err := vprocore.ValidateGoogleEarthReviewStrings(source.ContextID, source.Project, source.ProjectPath, source.SU,
		source.SUPath, source.DescriptionField, source.Title, string(source.Bytes)); err != nil {
		return nil, err
	}
	return &GoogleEarthKMLReview{ContextID: source.ContextID, Project: source.Project, ProjectPath: source.ProjectPath,
		SU: source.SU, SUPath: source.SUPath, DescriptionField: source.DescriptionField, Title: source.Title,
		PlacemarkCount: source.PlacemarkCount, ByteCount: len(source.Bytes), KML: string(source.Bytes)}, nil
}
