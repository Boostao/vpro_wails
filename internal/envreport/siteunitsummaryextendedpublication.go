package envreport

import (
	"github.com/boostao/vpro-wails/internal/reports/reportcore"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
)

type SiteUnitSummaryExtendedWorkbookOptions struct {
	Method              int `json:"method"`
	OrderBy             int `json:"orderBy"`
	IncludeSpecies      int `json:"includeSpecies"`
	CoverCalculation    int `json:"coverCalculation"`
	AndOr               int `json:"andOr"`
	PresenceGreaterThan int `json:"presenceGreaterThan"`
	CoverGreaterThan    int `json:"coverGreaterThan"`
}

func (request *SiteUnitSummaryExtendedWorkbookOptions) UnmarshalJSON(data []byte) error {
	type plain SiteUnitSummaryExtendedWorkbookOptions
	var value plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &value, "Extended Summary workbook",
		"method", "orderBy", "includeSpecies", "coverCalculation", "andOr", "presenceGreaterThan", "coverGreaterThan"); err != nil {
		return err
	}
	*request = SiteUnitSummaryExtendedWorkbookOptions(value)
	return nil
}

type siteUnitSummaryExtendedExportRequest struct {
	Method              int    `json:"method"`
	OrderBy             int    `json:"orderBy"`
	IncludeSpecies      int    `json:"includeSpecies"`
	CoverCalculation    int    `json:"coverCalculation"`
	AndOr               int    `json:"andOr"`
	PresenceGreaterThan int    `json:"presenceGreaterThan"`
	CoverGreaterThan    int    `json:"coverGreaterThan"`
	ApprovalHash        string `json:"approvalHash"`
	Destination         string `json:"destination"`
}

func (request *siteUnitSummaryExtendedExportRequest) UnmarshalJSON(data []byte) error {
	type plain siteUnitSummaryExtendedExportRequest
	var value plain
	if err := vprocore.DecodeStrictRequiredJSON(data, &value, "Extended Summary workbook export",
		"method", "orderBy", "includeSpecies", "coverCalculation", "andOr", "presenceGreaterThan", "coverGreaterThan",
		"approvalHash", "destination"); err != nil {
		return err
	}
	*request = siteUnitSummaryExtendedExportRequest(value)
	return nil
}

func (request SiteUnitSummaryExtendedWorkbookOptions) speciesRequest() SiteUnitSpeciesListRequest {
	return SiteUnitSpeciesListRequest{request.Method, request.OrderBy, request.CoverCalculation,
		request.AndOr, request.PresenceGreaterThan, request.CoverGreaterThan}
}

func authorizeSiteUnitSummaryExtendedWorkbook(ctx context.Context, service *vprocore.ContextService, request SiteUnitSummaryExtendedWorkbookOptions) error {
	if ctx == nil {
		return errors.New("Extended Summary workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if service == nil || service.Projects == nil || service.Projects.Sqlite == nil ||
		service.Plots == nil || service.Projects.Preferences == nil {
		return errors.New("Extended Summary workbook owned context/configuration is unavailable")
	}
	if request.Method != 1 && request.Method != 2 || request.IncludeSpecies != 0 && request.IncludeSpecies != 1 {
		return errors.New("Extended Summary workbook requires explicit method and species inclusion")
	}
	if err := validateSiteUnitSpeciesListOptions(siteUnitSpeciesListOptions{request.OrderBy, request.CoverCalculation,
		request.AndOr, request.PresenceGreaterThan, request.CoverGreaterThan}); err != nil {
		return err
	}
	if request.IncludeSpecies == 0 && (request.OrderBy != 2 || request.CoverCalculation != 1 ||
		request.AndOr != 1 || request.PresenceGreaterThan != 0 || request.CoverGreaterThan != 0) {
		return errors.New("No-species extension requires lifeform grouping and explicit inactive criteria defaults; original layer workbook is separate")
	}
	return nil
}

type siteUnitSummaryExtendedSource struct {
	Input   siteUnitQuickVegetationInput
	Scope   SiteUnitSummaryWorkbookScope
	Options SiteUnitSummaryExtendedWorkbookOptions
}

type SiteUnitSummaryExtendedWorkbookReview struct {
	Environment    SiteUnitSummaryPreview                 `json:"environment"`
	Species        *SiteUnitSpeciesListPreview            `json:"species"`
	Scope          SiteUnitSummaryWorkbookScope           `json:"scope"`
	Options        SiteUnitSummaryExtendedWorkbookOptions `json:"options"`
	Sheets         []reportcore.VegetationWorkbookSheet   `json:"sheets"`
	ApprovalHash   string                                 `json:"approvalHash"`
	WorkbookSHA256 string                                 `json:"workbookSHA256"`
	Bytes          int                                    `json:"bytes"`
}

func readSiteUnitSummaryExtendedSource(ctx context.Context, plots *vprocore.PlotService, owner *vprocore.SqliteContext,
	tx *sql.Tx, contextID string, request SiteUnitSummaryExtendedWorkbookOptions) (siteUnitSummaryExtendedSource, error) {
	scope, err := readSiteUnitSummaryPublicationScope(ctx, plots.Projects.Preferences, true)
	if err != nil {
		return siteUnitSummaryExtendedSource{}, err
	}
	input, err := readSiteUnitQuickVegetationSource(ctx, owner, tx, contextID, request.Method)
	if err != nil {
		return siteUnitSummaryExtendedSource{}, err
	}
	return siteUnitSummaryExtendedSource{input, scope, request}, nil
}

func prepareSiteUnitSummaryExtendedSource(ctx context.Context, source siteUnitSummaryExtendedSource) (siteUnitSummaryWorkbook,
	SiteUnitSummaryPreview, *SiteUnitSpeciesListPreview, error) {
	if source.Options.IncludeSpecies == 1 {
		species, err := siteUnitSummarySpeciesPreview(ctx, source.Input, source.Options.speciesRequest())
		if err != nil {
			return siteUnitSummaryWorkbook{}, SiteUnitSummaryPreview{}, nil, err
		}
		workbook, err := prepareSiteUnitSummaryWorkbookProjection(ctx, species.Environment, source.Options.OrderBy, &species)
		if err != nil {
			return siteUnitSummaryWorkbook{}, SiteUnitSummaryPreview{}, nil, err
		}
		return workbook, species.Environment, &species, nil
	}
	environment, err := siteUnitSummaryLifeformPreview(ctx, source.Input)
	if err != nil {
		return siteUnitSummaryWorkbook{}, SiteUnitSummaryPreview{}, nil, err
	}
	workbook, err := prepareSiteUnitSummaryWorkbookProjection(ctx, environment, 2, nil)
	if err != nil {
		return siteUnitSummaryWorkbook{}, SiteUnitSummaryPreview{}, nil, err
	}
	return workbook, environment, nil, nil
}

func siteUnitSummaryExtendedApproval(source siteUnitSummaryExtendedSource, workbook siteUnitSummaryWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		Source   siteUnitSummaryExtendedSource
		Sheets   []reportcore.VegetationWorkbookSheetInternal
		Workbook []byte
	}{source, workbook.Sheets, workbook.Bytes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO extended Summary workbook approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func reviewSiteUnitSummaryExtendedWorkbook(ctx context.Context, service *vprocore.ContextService, contextID, requestJSON string) (*SiteUnitSummaryExtendedWorkbookReview, error) {
	var request SiteUnitSummaryExtendedWorkbookOptions
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := authorizeSiteUnitSummaryExtendedWorkbook(ctx, service, request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, service, contextID, func(plots *vprocore.PlotService) (*SiteUnitSummaryExtendedWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SiteUnitSummaryExtendedWorkbookReview, error) {
			source, err := readSiteUnitSummaryExtendedSource(ctx, plots, owner, tx, contextID, request)
			if err != nil {
				return nil, err
			}
			workbook, environment, species, err := prepareSiteUnitSummaryExtendedSource(ctx, source)
			if err != nil {
				return nil, err
			}
			scope, err := readSiteUnitSummaryPublicationScope(ctx, plots.Projects.Preferences, true)
			if err != nil || scope != source.Scope {
				return nil, errors.Join(err, errors.New("Extended Summary saved scope changed during preparation; reload review"))
			}
			approval, err := siteUnitSummaryExtendedApproval(source, workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			sheets := []reportcore.VegetationWorkbookSheet{}
			for _, sheet := range workbook.Sheets {
				sheets = append(sheets, reportcore.VegetationWorkbookSheet{Unit: vprocore.CloneSiteUnitCell(sheet.Unit), Name: sheet.Name})
			}
			return &SiteUnitSummaryExtendedWorkbookReview{environment, species, scope, request, sheets, approval, hex.EncodeToString(hash[:]), len(workbook.Bytes)}, nil
		})
	})
}

func exportSiteUnitSummaryExtendedWorkbook(ctx context.Context, service *vprocore.ContextService, contextID, requestJSON string,
	hooks siteUnitSummaryWorkbookHooks) (*SiteUnitSummaryWorkbookOutcome, error) {
	var request siteUnitSummaryExtendedExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	options := SiteUnitSummaryExtendedWorkbookOptions{request.Method, request.OrderBy, request.IncludeSpecies,
		request.CoverCalculation, request.AndOr, request.PresenceGreaterThan, request.CoverGreaterThan}
	if err := authorizeSiteUnitSummaryExtendedWorkbook(ctx, service, options); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Destination); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Extended Summary workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Extended Summary workbook requires the exact reviewed approval hash")
	}
	return vprocore.WithContextPlotRequest(ctx, service, contextID, func(plots *vprocore.PlotService) (*SiteUnitSummaryWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (siteUnitSummaryExtendedSource, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (siteUnitSummaryExtendedSource, error) {
				return readSiteUnitSummaryExtendedSource(ctx, plots, owner, tx, contextID, options)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, _, _, err := prepareSiteUnitSummaryExtendedSource(ctx, expected)
		if err != nil {
			return nil, err
		}
		approval, err := siteUnitSummaryExtendedApproval(expected, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Extended Summary raw source/options/saved scope differ; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Extended Summary raw source/options/saved scope changed; reload review")
			}
			return nil
		}
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "Extended Summary workbook", Artifact: "XLSX", CommitLabel: "workbook",
			StagePattern: ".vpro-extended-summary-workbook-*",
			Encode: func(ctx context.Context) ([]byte, error) {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
				return bytes.Clone(workbook.Bytes), nil
			},
			Validate: func(ctx context.Context, actual []byte) error {
				if err := ctx.Err(); err != nil {
					return err
				}
				if !bytes.Equal(actual, workbook.Bytes) {
					return errors.New("staged Extended Summary workbook differs from reviewed bytes")
				}
				return nil
			},
		}, validateSource, hooks.publication)
		if result.Published {
			publishErr = errors.Join(publishErr, validateSource())
			if publishErr != nil {
				publishErr = fmt.Errorf("Workbook published; do not replay: %w", publishErr)
			}
		}
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "Extended Summary workbook")
		return &SiteUnitSummaryWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
