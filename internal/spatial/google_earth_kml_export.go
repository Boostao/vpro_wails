package spatial

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

func publishGoogleEarthKML(ctx context.Context, requested, title string, points []googleEarthKMLPoint) (vprocore.ArtifactPublication, error) {
	return publishGoogleEarthKMLWithHooks(ctx, requested, title, points, vprocore.ArtifactPublicationHooks{})
}

func publishGoogleEarthKMLWithHooks(ctx context.Context, requested, title string, points []googleEarthKMLPoint, hooks vprocore.ArtifactPublicationHooks) (vprocore.ArtifactPublication, error) {
	return publishGoogleEarthKMLChecked(ctx, requested, title, points, nil, hooks)
}

func publishGoogleEarthKMLChecked(ctx context.Context, requested, title string, points []googleEarthKMLPoint, validateSource func() error, hooks vprocore.ArtifactPublicationHooks) (vprocore.ArtifactPublication, error) {
	points = append([]googleEarthKMLPoint(nil), points...)
	return vprocore.PublishArtifactChecked(ctx, requested, vprocore.ArtifactPublicationFormat{
		Label:        "Google Earth KML publication",
		Artifact:     "KML",
		CommitLabel:  "Google Earth KML",
		StagePattern: ".vpro-google-earth-kml-*",
		Encode: func(ctx context.Context) ([]byte, error) {
			return prepareGoogleEarthKML(ctx, title, points)
		},
		Validate: func(ctx context.Context, staged []byte) error {
			expected, err := prepareGoogleEarthKML(ctx, title, points)
			if err != nil {
				return err
			}
			if !bytes.Equal(staged, expected) {
				return errors.New("staged KML differs from finalized source points")
			}
			return nil
		},
	}, validateSource, hooks)
}

func snapshotGoogleEarthKMLPreparation(expected ownedGoogleEarthKMLPreparation) ownedGoogleEarthKMLPreparation {
	rows := slices.Clone(expected.Source.Report.Rows)
	for i, row := range rows {
		row.PlotNumber = vprocore.CloneSiteUnitCell(row.PlotNumber)
		row.StoredLongitude = vprocore.CloneSiteUnitCell(row.StoredLongitude)
		row.Longitude = vprocore.CloneSiteUnitCell(row.Longitude)
		row.Latitude = vprocore.CloneSiteUnitCell(row.Latitude)
		row.Description = vprocore.CloneSiteUnitCell(row.Description)
		rows[i] = row
	}
	expected.Source.Report.Rows = rows
	expected.KML.Bytes = bytes.Clone(expected.KML.Bytes)
	return expected
}

func ReadGoogleEarthKMLPublicationReview(s *vprocore.ContextService, ctx context.Context, contextID, descriptionField, title string) (*ownedGoogleEarthKMLPreparation, error) {
	if err := validateGoogleEarthXMLText(title); err != nil {
		return nil, fmt.Errorf("Google Earth document name: %w", err)
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*ownedGoogleEarthKMLPreparation, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
			return readGoogleEarthKMLPreparationSnapshot(ctx, contextID, descriptionField, title, owner, tx)
		})
	})
}

type googleEarthKMLOwnedPublicationHooks struct {
	observe     func(string) error
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func PublishOwnedGoogleEarthKML(s *vprocore.ContextService, ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string) (vprocore.ArtifactPublication, error) {
	return PublishOwnedGoogleEarthKMLWithHooks(s, ctx, contextID, expected, requested, googleEarthKMLOwnedPublicationHooks{})
}

func PublishOwnedGoogleEarthKMLWithHooks(s *vprocore.ContextService, ctx context.Context, contextID string, expected ownedGoogleEarthKMLPreparation, requested string, hooks googleEarthKMLOwnedPublicationHooks) (vprocore.ArtifactPublication, error) {
	expected = snapshotGoogleEarthKMLPreparation(expected)
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (result vprocore.ArtifactPublication, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return result, err
		}
		defer owner.Mu.Unlock()
		defer func() {
			if result.Published && resultErr != nil {
				resultErr = fmt.Errorf("owned Google Earth KML published; do not replay publication: %w", resultErr)
			}
		}()
		observe := func(phase string) error {
			if hooks.observe != nil {
				return hooks.observe(phase)
			}
			return nil
		}
		if err := observe("snapshot"); err != nil {
			return result, err
		}
		if expected.KML.ContextID != contextID || expected.KML.Project != owner.Selection.Project ||
			expected.KML.ProjectPath != owner.Selection.ProjectPath ||
			expected.KML.SU != owner.Selection.SU || expected.KML.SUPath != owner.Selection.SUPath {
			return result, errors.New("owned Google Earth KML context/project/SU/path differs; reload source review")
		}
		validate := func() error {
			actual, err := vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (*ownedGoogleEarthKMLPreparation, error) {
				return readGoogleEarthKMLPreparationSnapshot(ctx, contextID, expected.KML.DescriptionField, expected.KML.Title, owner, tx)
			})
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(*actual, expected) {
				return errors.New("owned Google Earth KML raw source/physical rows/scope/title/bytes differs; reload source review")
			}
			return nil
		}
		if err := validate(); err != nil {
			return result, err
		}
		if err := observe("validated"); err != nil {
			return result, err
		}
		points, err := planGoogleEarthKMLPoints(ctx, expected.Source.Report.Rows)
		if err != nil {
			return result, err
		}
		result, resultErr = publishGoogleEarthKMLChecked(ctx, requested, expected.KML.Title, points, func() error {
			if err := observe("precommit"); err != nil {
				return err
			}
			return validate()
		}, hooks.publication)
		if result.Published {
			resultErr = errors.Join(resultErr, observe("published"), validate())
		}
		return result, resultErr
	})
}

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
