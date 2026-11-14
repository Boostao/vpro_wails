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

type EnvironmentWorkbookReview struct {
	Preview        LongEnvironmentPreview     `json:"preview"`
	Sheets         []EnvironmentWorkbookSheet `json:"sheets"`
	ApprovalHash   string                     `json:"approvalHash"`
	WorkbookSHA256 string                     `json:"workbookSHA256"`
	Bytes          int                        `json:"bytes"`
}

type EnvironmentWorkbookExportRequest struct {
	Title        string `json:"title"`
	ApprovalHash string `json:"approvalHash"`
	Destination  string `json:"destination"`
}

func (request *EnvironmentWorkbookExportRequest) UnmarshalJSON(data []byte) error {
	type plain EnvironmentWorkbookExportRequest
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "title", "approvalHash", "destination"); err != nil {
		return err
	}
	*request = EnvironmentWorkbookExportRequest(decoded)
	return nil
}

type EnvironmentWorkbookOutcome struct {
	Status               string `json:"status"`
	RequestedDestination string `json:"requestedDestination"`
	Path                 string `json:"path"`
	SHA256               string `json:"sha256"`
	ErrorMessage         string `json:"errorMessage"`
}

type EnvironmentWorkbookService struct {
	contexts *vprocore.ContextService
}

type environmentWorkbookSource struct {
	Preview LongEnvironmentPreview
	Tables  []vprocore.ProjectMetadataTable
}

type environmentWorkbookHooks struct {
	snapshot    vprocore.PublicationReadSnapshotHooks
	publication vprocore.ArtifactPublicationHooks
}

func NewEnvironmentWorkbookService(contexts *vprocore.ContextService) (*EnvironmentWorkbookService, error) {
	return &EnvironmentWorkbookService{contexts: contexts}, nil
}

func (s *EnvironmentWorkbookService) authorize(ctx context.Context) error {
	if ctx == nil {
		return errors.New("Long Environment workbook requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Long Environment workbook context service is unavailable")
	}
	return nil
}

func readEnvironmentWorkbookSource(ctx context.Context, owner *vprocore.SqliteContext, tx *sql.Tx, contextID, title string) (environmentWorkbookSource, error) {
	fail := func(err error) (environmentWorkbookSource, error) { return environmentWorkbookSource{}, err }
	if owner.Selection.SU == "None" || owner.Selection.SU == "USysSuTableDynamic" {
		return fail(errors.New("Long Environment workbook requires a selected normal SU; dynamic hierarchy remains unavailable"))
	}
	tables := make([]vprocore.ProjectMetadataTable, 4)
	for i, source := range []struct{ role, table string }{
		{"project", owner.Selection.Project + "_Env"}, {"project", owner.Selection.Project + "_Admin"},
		{"su", owner.Selection.SU + "_SU"}, {"VLists", "MasterSiteUnitList"},
	} {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.role)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return fail(err)
		}
		if count != 1 {
			return fail(fmt.Errorf("Long Environment workbook requires original physical table %s.%s", source.role, source.table))
		}
		var err error
		tables[i], err = vprocore.ReadSQLiteStorageRows(ctx, tx, source.role, source.table, "", nil, "")
		if err != nil {
			return fail(err)
		}
	}
	report, err := reportcore.PlanLongEnvironment(ctx, owner.Selection.Project, owner.Selection.SU, title, tables[0], tables[1], tables[2], tables[3])
	if err != nil {
		return fail(err)
	}
	return environmentWorkbookSource{LongEnvironmentPreview{contextID, owner.Selection.ProjectPath, owner.Selection.SUPath, report}, tables}, nil
}

func environmentWorkbookApproval(source environmentWorkbookSource, workbook environmentWorkbook) (string, error) {
	data, err := json.Marshal(struct {
		Source   environmentWorkbookSource
		Sheets   []EnvironmentWorkbookSheet
		Workbook []byte
	}{source, workbook.Sheets, workbook.Bytes})
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(append([]byte("VPRO Long Environment workbook approval v1\x00"), data...))
	return hex.EncodeToString(hash[:]), nil
}

func (s *EnvironmentWorkbookService) GetReview(ctx context.Context, contextID, requestJSON string) (*EnvironmentWorkbookReview, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request LongEnvironmentRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Title); err != nil {
		return nil, err
	}
	if err := validateLongEnvironmentTitle(request.Title); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*EnvironmentWorkbookReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, vprocore.PublicationReadSnapshotHooks{}, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*EnvironmentWorkbookReview, error) {
			source, err := readEnvironmentWorkbookSource(ctx, owner, tx, contextID, request.Title)
			if err != nil {
				return nil, err
			}
			workbook, err := prepareLongEnvironmentWorkbook(ctx, source.Preview.Report)
			if err != nil {
				return nil, err
			}
			approval, err := environmentWorkbookApproval(source, workbook)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(workbook.Bytes)
			return &EnvironmentWorkbookReview{source.Preview, workbook.Sheets, approval, hex.EncodeToString(hash[:]), len(workbook.Bytes)}, nil
		})
	})
}

func (s *EnvironmentWorkbookService) ExportReviewed(ctx context.Context, contextID, requestJSON string) (*EnvironmentWorkbookOutcome, error) {
	return s.exportReviewed(ctx, contextID, requestJSON, environmentWorkbookHooks{})
}

func (s *EnvironmentWorkbookService) exportReviewed(ctx context.Context, contextID, requestJSON string, hooks environmentWorkbookHooks) (*EnvironmentWorkbookOutcome, error) {
	if err := s.authorize(ctx); err != nil {
		return nil, err
	}
	var request EnvironmentWorkbookExportRequest
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if err := vprocore.ValidateGoogleEarthReviewStrings(contextID, request.Title, request.Destination); err != nil {
		return nil, err
	}
	if err := validateLongEnvironmentTitle(request.Title); err != nil {
		return nil, err
	}
	if !strings.EqualFold(filepath.Ext(request.Destination), ".xlsx") {
		return nil, errors.New("Long Environment workbook requires an explicit .xlsx destination")
	}
	hash, err := hex.DecodeString(request.ApprovalHash)
	if err != nil || len(hash) != sha256.Size || hex.EncodeToString(hash) != request.ApprovalHash {
		return nil, errors.New("Long Environment workbook requires the exact reviewed approval hash")
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*EnvironmentWorkbookOutcome, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		read := func() (environmentWorkbookSource, error) {
			return vprocore.WithPublicationReadSnapshot(ctx, owner, hooks.snapshot, func(tx *sql.Tx) (environmentWorkbookSource, error) {
				return readEnvironmentWorkbookSource(ctx, owner, tx, contextID, request.Title)
			})
		}
		expected, err := read()
		if err != nil {
			return nil, err
		}
		workbook, err := prepareLongEnvironmentWorkbook(ctx, expected.Preview.Report)
		if err != nil {
			return nil, err
		}
		approval, err := environmentWorkbookApproval(expected, workbook)
		if err != nil {
			return nil, err
		}
		if approval != request.ApprovalHash {
			return nil, errors.New("Long Environment workbook source/scope/physical rows/title differs; reload review")
		}
		validateSource := func() error {
			actual, err := read()
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(actual, expected) {
				return errors.New("Long Environment workbook raw source/physical rows/metadata differs; reload review")
			}
			return nil
		}
		result, publishErr := vprocore.PublishArtifactChecked(ctx, request.Destination, vprocore.ArtifactPublicationFormat{
			Label: "Long Environment workbook", Artifact: "XLSX", CommitLabel: "workbook",
			StagePattern: ".vpro-environment-workbook-*",
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
					return errors.New("staged workbook differs from reviewed bytes")
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
		receipt := vprocore.ArtifactPublicationOutcome(request.Destination, result, publishErr, "Long Environment workbook")
		return &EnvironmentWorkbookOutcome{receipt.Status, receipt.RequestedDestination, receipt.Path, receipt.SHA256, receipt.ErrorMessage}, nil
	})
}
