package spatial

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type GoogleEarthKMLExportService struct {
	contexts *vprocore.ContextService
}

func NewGoogleEarthKMLExportService(contexts *vprocore.ContextService) *GoogleEarthKMLExportService {
	return &GoogleEarthKMLExportService{contexts: contexts}
}

func (s *GoogleEarthKMLExportService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Google Earth KML export requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil {
		return errors.New("Google Earth KML export context service is unavailable")
	}
	return nil
}

type GoogleEarthKMLExportReviewRequest struct {
	DescriptionField string `json:"descriptionField"`
	Title            string `json:"title"`
}

func (request *GoogleEarthKMLExportReviewRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthKMLExportReviewRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth KML export review", "descriptionField", "title"); err != nil {
		return err
	}
	*request = GoogleEarthKMLExportReviewRequest(decoded)
	return nil
}

type GoogleEarthKMLExportRequest struct {
	DescriptionField string `json:"descriptionField"`
	Title            string `json:"title"`
	ApprovalHash     string `json:"approvalHash"`
	Destination      string `json:"destination"`
}

func (request *GoogleEarthKMLExportRequest) UnmarshalJSON(data []byte) error {
	type plain GoogleEarthKMLExportRequest
	var decoded plain
	if err := vprocore.DecodeGoogleEarthKMLJSON(data, &decoded, "Google Earth KML export", "descriptionField", "title", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = GoogleEarthKMLExportRequest(decoded)
	return nil
}

type GoogleEarthKMLExportReview struct {
	Review       GoogleEarthKMLReview `json:"review"`
	ApprovalHash string               `json:"approvalHash"`
	KMLSHA256    string               `json:"kmlSHA256"`
}

type GoogleEarthKMLExportOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

func googleEarthKMLApprovalHash(source ownedGoogleEarthKMLPreparation) (string, error) {
	source = snapshotGoogleEarthKMLPreparation(source)
	data, err := json.Marshal(source)
	if err != nil {
		return "", fmt.Errorf("Google Earth KML source approval: %w", err)
	}
	digest := sha256.Sum256(append([]byte("VPRO KML source approval v1\x00"), data...))
	return hex.EncodeToString(digest[:]), nil
}

func (s *GoogleEarthKMLExportService) GetGoogleEarthKMLExportReview(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLExportReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request GoogleEarthKMLExportReviewRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.DescriptionField, request.Title); err != nil {
		return nil, err
	}
	source, err := ReadGoogleEarthKMLPublicationReview(s.contexts, ctx, contextID, request.DescriptionField, request.Title)
	if err != nil {
		return nil, err
	}
	review, err := googleEarthKMLReviewFromOwned(&source.KML)
	if err != nil {
		return nil, err
	}
	approval, err := googleEarthKMLApprovalHash(*source)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	digest := sha256.Sum256(source.KML.Bytes)
	return &GoogleEarthKMLExportReview{Review: *review, ApprovalHash: approval, KMLSHA256: hex.EncodeToString(digest[:])}, nil
}

func (s *GoogleEarthKMLExportService) ExportReviewedGoogleEarthKML(ctx context.Context, contextID, requestJSON string) (*GoogleEarthKMLExportOutcome, error) {
	return s.exportReviewedGoogleEarthKML(ctx, contextID, requestJSON, googleEarthKMLOwnedPublicationHooks{})
}

func (s *GoogleEarthKMLExportService) exportReviewedGoogleEarthKML(ctx context.Context, contextID, requestJSON string, hooks googleEarthKMLOwnedPublicationHooks) (*GoogleEarthKMLExportOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request GoogleEarthKMLExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.DescriptionField, request.Title, request.Destination); err != nil {
		return nil, err
	}
	if len(request.ApprovalHash) != 64 || strings.ToLower(request.ApprovalHash) != request.ApprovalHash {
		return nil, errors.New("Google Earth KML export requires the exact reviewed source approval hash")
	}
	if _, err := hex.DecodeString(request.ApprovalHash); err != nil {
		return nil, errors.New("Google Earth KML export requires a hexadecimal source approval hash")
	}
	source, err := ReadGoogleEarthKMLPublicationReview(s.contexts, ctx, contextID, request.DescriptionField, request.Title)
	if err != nil {
		return nil, err
	}
	approval, err := googleEarthKMLApprovalHash(*source)
	if err != nil {
		return nil, err
	}
	if approval != request.ApprovalHash {
		return nil, errors.New("Google Earth KML export raw source/scope/title differs; prepare a new review before publishing")
	}
	result, publishErr := PublishOwnedGoogleEarthKMLWithHooks(s.contexts, ctx, contextID, *source, request.Destination, hooks)
	return googleEarthKMLExportOutcome(request.Destination, result, publishErr), nil
}

func googleEarthKMLExportOutcome(requested string, result vprocore.ArtifactPublication, err error) *GoogleEarthKMLExportOutcome {
	receipt := vprocore.ArtifactPublicationOutcome(requested, result, err, "Google Earth KML")
	return &GoogleEarthKMLExportOutcome{
		Status: receipt.Status, RequestedDestination: receipt.RequestedDestination,
		Path: receipt.Path, SHA256: receipt.SHA256, ErrorMessage: receipt.ErrorMessage,
	}
}
