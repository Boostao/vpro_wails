package cloudsync

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const (
	MergeResolutionKeepTarget   = "keep_target"
	MergeResolutionAcceptSource = "accept_source"
)

var mergeTableSuffixes = []string{"Env", "Admin", "Veg", "Humus", "Mineral", "Other"}

// ListSubmissionsFilter filters staging.submissions rows.
type ListSubmissionsFilter struct {
	Status      string `json:"status,omitempty"`
	SubmittedBy string `json:"submittedBy,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

func (r *ListSubmissionsFilter) UnmarshalJSON(data []byte) error {
	type plain ListSubmissionsFilter
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// StagingSubmissionSummary represents one queue item in staging.submissions.
type StagingSubmissionSummary struct {
	SubmissionID     string `json:"submissionId"`
	ProjectName      string `json:"projectName"`
	ProjectID        string `json:"projectId,omitempty"`
	SubmittedBy      string `json:"submittedBy"`
	DBRole           string `json:"dbRole"`
	SubmittedAt      string `json:"submittedAt"`
	Status           string `json:"status"`
	PlotCount        int    `json:"plotCount"`
	VegRowCount      int    `json:"vegRowCount"`
	SourceDBSHA256   string `json:"sourceDbSha256"`
	CompliancePassed bool   `json:"compliancePassed"`
	UserSpeciesCount int    `json:"userSpeciesCount"`
	LumpRowCount     int    `json:"lumpRowCount"`
	SURowCount       int    `json:"suRowCount"`
	SubmitterNotes   string `json:"submitterNotes,omitempty"`
	BaseSnapshotTag  string `json:"baseSnapshotTag,omitempty"`
	ReviewerNotes    string `json:"reviewerNotes,omitempty"`
	ReviewedBy       string `json:"reviewedBy,omitempty"`
	ReviewedAt       string `json:"reviewedAt,omitempty"`
	MergedTxID       string `json:"mergedTxId,omitempty"`
}

// ProvisionalSpeciesDiff compares a personal species in user_species_manifest against standards.usys_all_specs.
type ProvisionalSpeciesDiff struct {
	Code               string  `json:"code"`
	ScientificName     *string `json:"scientificName,omitempty"`
	LifeForm           *int    `json:"lifeForm,omitempty"`
	Status             string  `json:"status"` // "new_provisional" | "matches_official" | "matches_synonym"
	OfficialCode       string  `json:"officialCode,omitempty"`
	OfficialScientific string  `json:"officialScientific,omitempty"`
}

// StagingSubmissionDetail provides full custodian review payloads for a submission.
type StagingSubmissionDetail struct {
	Summary                 StagingSubmissionSummary `json:"summary"`
	ComplianceReport        json.RawMessage          `json:"complianceReport"`
	UserSpeciesManifest     CloudUserSpeciesManifest `json:"userSpeciesManifest"`
	ProvisionalSpeciesDiffs []ProvisionalSpeciesDiff `json:"provisionalSpeciesDiffs"`
	LumpRows                []StagingLumpRow         `json:"lumpRows"`
	SURows                  []StagingSURow           `json:"suRows"`
}

// CloudMergeConflictItem mirrors ThreeWayMergePreview's MergeConflictItem with custodian protection metadata.
type CloudMergeConflictItem struct {
	ConflictID         string  `json:"conflictId"`
	PlotNumber         string  `json:"plotNumber"`
	Table              string  `json:"table"`
	Key                string  `json:"key"`
	Field              string  `json:"field"`
	BaseVal            *string `json:"baseVal,omitempty"`
	SourceVal          *string `json:"sourceVal"`
	TargetVal          *string `json:"targetVal"`
	Kind               string  `json:"kind"`              // "FastForward" | "Conflict" | "TargetModifiedOnly"
	DefaultResolution  string  `json:"defaultResolution"` // "accept_source" | "keep_target"
	CustodianProtected bool    `json:"custodianProtected"`
}

// CloudMergePlotPreview summarizes per-plot merge status and field/child conflicts.
type CloudMergePlotPreview struct {
	PlotNumber string                   `json:"plotNumber"`
	Category   string                   `json:"category"` // "Added" | "Identical" | "FastForward" | "Conflict" | "TargetOnly"
	Items      []CloudMergeConflictItem `json:"items"`
}

// StagingMergePreviewRequest configures 3-way diffing of staging.<submission> vs becmaster.
type StagingMergePreviewRequest struct {
	SubmissionID string `json:"submissionId"`
	BaseDBPath   string `json:"baseDbPath,omitempty"`
	BaseProject  string `json:"baseProject,omitempty"`
}

func (r *StagingMergePreviewRequest) UnmarshalJSON(data []byte) error {
	type plain StagingMergePreviewRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// StagingMergePreview is ThreeWayMergePreview-compatible for Custodian staging -> becmaster review.
type StagingMergePreview struct {
	SubmissionID            string                   `json:"submissionId"`
	SourceProject           string                   `json:"sourceProject"`
	TargetProject           string                   `json:"targetProject"`
	BaseDBPath              string                   `json:"baseDbPath,omitempty"`
	BaseProject             string                   `json:"baseProject,omitempty"`
	PreviewDigest           string                   `json:"previewDigest"`
	AddedCount              int                      `json:"addedCount"`
	IdenticalCount          int                      `json:"identicalCount"`
	FastForwardCount        int                      `json:"fastForwardCount"`
	ConflictCount           int                      `json:"conflictCount"`
	TargetOnlyCount         int                      `json:"targetOnlyCount"`
	ElapsedMs               int64                    `json:"elapsedMs"`
	Plots                   []CloudMergePlotPreview  `json:"plots"`
	Conflicts               []CloudMergeConflictItem `json:"conflicts"`
	CustodianProtectedDiffs []CloudMergeConflictItem `json:"custodianProtectedDiffs"`
}

// StagingMergeExecuteRequest commits selective plot/field resolutions into becmaster atomically.
type StagingMergeExecuteRequest struct {
	SubmissionID              string            `json:"submissionId"`
	BaseDBPath                string            `json:"baseDbPath,omitempty"`
	BaseProject               string            `json:"baseProject,omitempty"`
	ExpectedPreviewDigest     string            `json:"expectedPreviewDigest"`
	DefaultConflictResolution string            `json:"defaultConflictResolution,omitempty"`
	PlotResolutions           map[string]string `json:"plotResolutions,omitempty"`
	FieldResolutions          map[string]string `json:"fieldResolutions,omitempty"`
	ReviewerNotes             string            `json:"reviewerNotes,omitempty"`
	Actor                     string            `json:"actor,omitempty"`
}

func (r *StagingMergeExecuteRequest) UnmarshalJSON(data []byte) error {
	type plain StagingMergeExecuteRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// StagingMergeResult summarizes atomic Custodian merge provenance and child ID allocations.
type StagingMergeResult struct {
	MergeTxID          string  `json:"mergeTxId"`
	SubmissionID       string  `json:"submissionId"`
	SourceProject      string  `json:"sourceProject"`
	AddedPlots         int     `json:"addedPlots"`
	UpdatedPlots       int     `json:"updatedPlots"`
	SkippedPlots       int     `json:"skippedPlots"`
	AllocatedChildIDs  []int64 `json:"allocatedChildIds"`
	ReservedDeletedIDs []int64 `json:"reservedDeletedIds"`
	AuditRowsWritten   int     `json:"auditRowsWritten"`
	TailAuditHash      string  `json:"tailAuditHash"`
	MergedAt           string  `json:"mergedAt"`
}

// RejectSubmissionRequest marks a pending staging submission as rejected with reviewer notes.
type RejectSubmissionRequest struct {
	SubmissionID  string `json:"submissionId"`
	ReviewerNotes string `json:"reviewerNotes"`
	Actor         string `json:"actor,omitempty"`
}

func (r *RejectSubmissionRequest) UnmarshalJSON(data []byte) error {
	type plain RejectSubmissionRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// CreateSnapshotRequest cuts a dated BECMaster snapshot record and optional local export bundle.
type CreateSnapshotRequest struct {
	SnapshotTag     string `json:"snapshotTag"`
	Notes           string `json:"notes,omitempty"`
	ExportFormat    string `json:"exportFormat,omitempty"` // "" | "sqlite" | "rds" | "gpkg"
	DestinationPath string `json:"destinationPath,omitempty"`
	ZoneFilter      string `json:"zoneFilter,omitempty"`
}

func (r *CreateSnapshotRequest) UnmarshalJSON(data []byte) error {
	type plain CreateSnapshotRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// SnapshotBundleExporterFunc allows Layer 3 (main) to export optional .rds / .gpkg snapshot bundles
// from a staged VP08 SQLite file without Layer-2 sibling imports.
type SnapshotBundleExporterFunc func(ctx context.Context, sqlitePath, projectName, format, destinationPath string) (exportedPath string, sha256Hex string, err error)

// SnapshotRecord represents a row in becmaster.snapshots (Schedule A 1.iii).
type SnapshotRecord struct {
	SnapshotTag     string `json:"snapshotTag"`
	CreatedAt       string `json:"createdAt"`
	CreatedBy       string `json:"createdBy"`
	PlotCount       int    `json:"plotCount"`
	VegCount        int64  `json:"vegCount"`
	BECVersion      string `json:"becVersion"`
	TaxonomyVersion string `json:"taxonomyVersion"`
	ManifestSHA256  string `json:"manifestSha256"`
	Notes           string `json:"notes,omitempty"`
	ExportedPath    string `json:"exportedPath,omitempty"`
}

// DownloadLogEntry represents a row in becmaster.download_log (Schedule A 1.v).
type DownloadLogEntry struct {
	ID           int64           `json:"id"`
	DownloadedAt string          `json:"downloadedAt"`
	Actor        string          `json:"actor"`
	RoleTier     string          `json:"roleTier"`
	FilterJSON   json.RawMessage `json:"filterJson"`
	Format       string          `json:"format"`
	PlotCount    int             `json:"plotCount"`
}

// AuditChainVerification reports cryptographic verification of becmaster.audit hash chaining.
type AuditChainVerification struct {
	VerifiedRows  int64  `json:"verifiedRows"`
	Valid         bool   `json:"valid"`
	PartialWindow bool   `json:"partialWindow"`
	TailHash      string `json:"tailHash"`
}

type pgQueryer interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

// ListStagingSubmissions lists staging queue entries ordered newest-first.
func (m *Manager) ListStagingSubmissions(ctx context.Context, filter ListSubmissionsFilter) ([]StagingSubmissionSummary, error) {
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return nil, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var clauses []string
	var args []any
	if st := strings.TrimSpace(filter.Status); st != "" {
		args = append(args, st)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if sb := strings.TrimSpace(filter.SubmittedBy); sb != "" {
		args = append(args, sb)
		clauses = append(clauses, fmt.Sprintf("submitted_by = $%d", len(args)))
	}
	whereSQL := ""
	if len(clauses) > 0 {
		whereSQL = "WHERE " + strings.Join(clauses, " AND ")
	}
	args = append(args, limit)
	query := fmt.Sprintf(`
		SELECT
			submission_id::text,
			project_name,
			COALESCE(project_id, ''),
			submitted_by,
			db_role,
			submitted_at,
			status,
			plot_count,
			veg_row_count,
			source_db_sha256,
			compliance_passed,
			COALESCE((user_species_manifest->>'rowCount')::int, 0),
			COALESCE(jsonb_array_length(lump_rows), 0),
			COALESCE(jsonb_array_length(su_rows), 0),
			COALESCE(submitter_notes, ''),
			COALESCE(base_snapshot_tag, ''),
			COALESCE(reviewer_notes, ''),
			COALESCE(reviewed_by, ''),
			reviewed_at,
			COALESCE(merged_tx_id::text, '')
		FROM staging.submissions
		%s
		ORDER BY submitted_at DESC
		LIMIT $%d
	`, whereSQL, len(args))

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, wrapRedactedError("list staging submissions", err, creds)
	}
	defer rows.Close()

	out := []StagingSubmissionSummary{}
	for rows.Next() {
		var item StagingSubmissionSummary
		var subAt time.Time
		var revAt *time.Time
		if err := rows.Scan(
			&item.SubmissionID,
			&item.ProjectName,
			&item.ProjectID,
			&item.SubmittedBy,
			&item.DBRole,
			&subAt,
			&item.Status,
			&item.PlotCount,
			&item.VegRowCount,
			&item.SourceDBSHA256,
			&item.CompliancePassed,
			&item.UserSpeciesCount,
			&item.LumpRowCount,
			&item.SURowCount,
			&item.SubmitterNotes,
			&item.BaseSnapshotTag,
			&item.ReviewerNotes,
			&item.ReviewedBy,
			&revAt,
			&item.MergedTxID,
		); err != nil {
			return nil, wrapRedactedError("scan staging submission row", err, creds)
		}
		item.SubmittedAt = subAt.UTC().Format(time.RFC3339)
		if revAt != nil {
			item.ReviewedAt = revAt.UTC().Format(time.RFC3339)
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapRedactedError("iterate staging submissions", err, creds)
	}
	return out, nil
}

// GetStagingSubmissionDetail returns full compliance report and provisional species diffs for a submission.
func (m *Manager) GetStagingSubmissionDetail(ctx context.Context, submissionID string) (StagingSubmissionDetail, error) {
	if _, err := uuid.Parse(strings.TrimSpace(submissionID)); err != nil {
		return StagingSubmissionDetail{}, errors.New("submissionId must be a valid UUID")
	}
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return StagingSubmissionDetail{}, err
	}

	var detail StagingSubmissionDetail
	var subAt time.Time
	var revAt *time.Time
	var compRaw, manifestRaw, lumpRaw, suRaw []byte
	err = pool.QueryRow(ctx, `
		SELECT
			submission_id::text,
			project_name,
			COALESCE(project_id, ''),
			submitted_by,
			db_role,
			submitted_at,
			status,
			plot_count,
			veg_row_count,
			source_db_sha256,
			compliance_passed,
			COALESCE(submitter_notes, ''),
			COALESCE(base_snapshot_tag, ''),
			COALESCE(reviewer_notes, ''),
			COALESCE(reviewed_by, ''),
			reviewed_at,
			COALESCE(merged_tx_id::text, ''),
			compliance_report::text,
			user_species_manifest::text,
			lump_rows::text,
			su_rows::text
		FROM staging.submissions
		WHERE submission_id = $1
	`, strings.TrimSpace(submissionID)).Scan(
		&detail.Summary.SubmissionID,
		&detail.Summary.ProjectName,
		&detail.Summary.ProjectID,
		&detail.Summary.SubmittedBy,
		&detail.Summary.DBRole,
		&subAt,
		&detail.Summary.Status,
		&detail.Summary.PlotCount,
		&detail.Summary.VegRowCount,
		&detail.Summary.SourceDBSHA256,
		&detail.Summary.CompliancePassed,
		&detail.Summary.SubmitterNotes,
		&detail.Summary.BaseSnapshotTag,
		&detail.Summary.ReviewerNotes,
		&detail.Summary.ReviewedBy,
		&revAt,
		&detail.Summary.MergedTxID,
		&compRaw,
		&manifestRaw,
		&lumpRaw,
		&suRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StagingSubmissionDetail{}, fmt.Errorf("staging submission %s not found", submissionID)
		}
		return StagingSubmissionDetail{}, wrapRedactedError("get staging submission detail", err, creds)
	}
	detail.Summary.SubmittedAt = subAt.UTC().Format(time.RFC3339)
	if revAt != nil {
		detail.Summary.ReviewedAt = revAt.UTC().Format(time.RFC3339)
	}
	detail.ComplianceReport = json.RawMessage(compRaw)
	_ = json.Unmarshal(manifestRaw, &detail.UserSpeciesManifest)
	if detail.UserSpeciesManifest.Species == nil {
		detail.UserSpeciesManifest.Species = []CloudUserSpeciesRecord{}
	}
	_ = json.Unmarshal(lumpRaw, &detail.LumpRows)
	if detail.LumpRows == nil {
		detail.LumpRows = []StagingLumpRow{}
	}
	_ = json.Unmarshal(suRaw, &detail.SURows)
	if detail.SURows == nil {
		detail.SURows = []StagingSURow{}
	}
	detail.Summary.UserSpeciesCount = len(detail.UserSpeciesManifest.Species)
	detail.Summary.LumpRowCount = len(detail.LumpRows)
	detail.Summary.SURowCount = len(detail.SURows)

	detail.ProvisionalSpeciesDiffs = []ProvisionalSpeciesDiff{}
	for _, sp := range detail.UserSpeciesManifest.Species {
		diff := ProvisionalSpeciesDiff{
			Code:           sp.Code,
			ScientificName: sp.ScientificName,
			LifeForm:       sp.LifeForm,
			Status:         "new_provisional",
		}
		var offCode, offSci string
		if qErr := pool.QueryRow(ctx, `SELECT COALESCE("Code",''), COALESCE("ScientificName",'') FROM standards.usys_all_specs WHERE UPPER("Code") = UPPER($1) LIMIT 1`, sp.Code).Scan(&offCode, &offSci); qErr == nil && offCode != "" {
			diff.Status = "matches_official"
			diff.OfficialCode = offCode
			diff.OfficialScientific = offSci
		} else if qErr := pool.QueryRow(ctx, `SELECT COALESCE("Code",''), COALESCE("ScientificName",'') FROM standards.usys_all_specs WHERE UPPER("OldCode") = UPPER($1) LIMIT 1`, sp.Code).Scan(&offCode, &offSci); qErr == nil && offCode != "" {
			diff.Status = "matches_synonym"
			diff.OfficialCode = offCode
			diff.OfficialScientific = offSci
		}
		detail.ProvisionalSpeciesDiffs = append(detail.ProvisionalSpeciesDiffs, diff)
	}

	return detail, nil
}

// cloudCell holds a typed value for byte-exact 3-way diffing and PostgreSQL parameterized writes.
type cloudCell struct {
	kind    string // "null" | "int" | "float" | "text"
	intVal  int64
	fltVal  float64
	textVal string
}

func pgValueToCloudCell(col ColumnSpec, raw any) cloudCell {
	if raw == nil {
		return cloudCell{kind: "null"}
	}
	if col.IsAccessBool {
		switch v := raw.(type) {
		case int16:
			if v != 0 {
				return cloudCell{kind: "int", intVal: -1}
			}
			return cloudCell{kind: "int", intVal: 0}
		case int32:
			if v != 0 {
				return cloudCell{kind: "int", intVal: -1}
			}
			return cloudCell{kind: "int", intVal: 0}
		case int64:
			if v != 0 {
				return cloudCell{kind: "int", intVal: -1}
			}
			return cloudCell{kind: "int", intVal: 0}
		case bool:
			if v {
				return cloudCell{kind: "int", intVal: -1}
			}
			return cloudCell{kind: "int", intVal: 0}
		}
	}
	switch v := raw.(type) {
	case int16:
		return cloudCell{kind: "int", intVal: int64(v)}
	case int32:
		return cloudCell{kind: "int", intVal: int64(v)}
	case int64:
		return cloudCell{kind: "int", intVal: v}
	case float32:
		return cloudCell{kind: "float", fltVal: float64(v)}
	case float64:
		return cloudCell{kind: "float", fltVal: v}
	case string:
		return cloudCell{kind: "text", textVal: v}
	case []byte:
		return cloudCell{kind: "text", textVal: string(v)}
	default:
		return cloudCell{kind: "text", textVal: fmt.Sprint(v)}
	}
}

func (c cloudCell) comparableString() *string {
	switch c.kind {
	case "null":
		return nil
	case "int":
		s := strconv.FormatInt(c.intVal, 10)
		return &s
	case "float":
		s := vprocore.SourceDoubleDecimalString(c.fltVal)
		return &s
	case "text":
		s := c.textVal
		return &s
	default:
		return nil
	}
}

func (c cloudCell) pgParam(col ColumnSpec) any {
	switch c.kind {
	case "null":
		return nil
	case "int":
		if col.IsAccessBool {
			if c.intVal != 0 {
				return int16(-1)
			}
			return int16(0)
		}
		if col.PGType == "SMALLINT" {
			return int16(c.intVal)
		}
		return c.intVal
	case "float":
		return c.fltVal
	case "text":
		return c.textVal
	default:
		return nil
	}
}

type cloudSnapshot struct {
	label string
	plots map[string]bool
	// suffix -> plotNumber -> rowKey -> colName -> *string
	cells map[string]map[string]map[string]*string
	// suffix -> plotNumber -> rowKey -> colName -> cloudCell
	rawCells map[string]map[string]map[string]cloudCell
}

func newCloudSnapshot(label string) *cloudSnapshot {
	s := &cloudSnapshot{
		label:    label,
		plots:    map[string]bool{},
		cells:    map[string]map[string]map[string]*string{},
		rawCells: map[string]map[string]map[string]cloudCell{},
	}
	for _, suffix := range mergeTableSuffixes {
		s.cells[suffix] = map[string]map[string]*string{}
		s.rawCells[suffix] = map[string]map[string]cloudCell{}
	}
	return s
}

type childRowRecord struct {
	cells map[string]cloudCell
	comps map[string]*string
}

func buildLogicalRowsForPlot(suffix string, cols []ColumnSpec, rows []childRowRecord) (map[string]map[string]*string, map[string]map[string]cloudCell) {
	compOut := map[string]map[string]*string{}
	rawOut := map[string]map[string]cloudCell{}

	baseKey := func(r childRowRecord) string {
		switch suffix {
		case "Env", "Admin":
			return suffix
		case "Veg":
			spp := ""
			if r.cells["Species"].kind == "text" {
				spp = r.cells["Species"].textVal
			}
			layer := ""
			if r.cells["Layer"].kind == "text" {
				layer = r.cells["Layer"].textVal
			}
			if layer != "" {
				return fmt.Sprintf("Species=%s|Layer=%s", spp, layer)
			}
			return fmt.Sprintf("Species=%s", spp)
		case "Humus", "Mineral":
			hz := ""
			if r.cells["Horizon"].kind == "text" {
				hz = r.cells["Horizon"].textVal
			}
			return fmt.Sprintf("Horizon=%s", hz)
		case "Other":
			dn := ""
			if r.cells["DataName"].kind == "text" {
				dn = r.cells["DataName"].textVal
			}
			return fmt.Sprintf("DataName=%s", dn)
		default:
			return suffix
		}
	}

	rowSig := func(r childRowRecord) string {
		var b strings.Builder
		b.WriteString(baseKey(r))
		for _, c := range cols {
			if c.Name == "ID" || c.Name == "PlotNumber" || c.Name == "Plot" {
				continue
			}
			b.WriteByte('|')
			if v := r.comps[c.Name]; v != nil {
				b.WriteString(*v)
			} else {
				b.WriteString("<NULL>")
			}
		}
		return b.String()
	}

	sorted := make([]childRowRecord, len(rows))
	copy(sorted, rows)
	slices.SortFunc(sorted, func(a, b childRowRecord) int {
		return strings.Compare(rowSig(a), rowSig(b))
	})

	counts := map[string]int{}
	for _, r := range sorted {
		bk := baseKey(r)
		counts[bk]++
		key := bk
		if counts[bk] > 1 {
			key = fmt.Sprintf("%s#%d", bk, counts[bk])
		}
		compOut[key] = r.comps
		rawOut[key] = r.cells
	}
	return compOut, rawOut
}

func scanSnapshotTableRows(snap *cloudSnapshot, suffix string, cols []ColumnSpec, rows pgx.Rows) error {
	plotCol := "PlotNumber"
	if suffix == "Admin" {
		plotCol = "Plot"
	}
	byPlot := map[string][]childRowRecord{}
	for rows.Next() {
		vals, vErr := rows.Values()
		if vErr != nil {
			rows.Close()
			return vErr
		}
		rec := childRowRecord{
			cells: make(map[string]cloudCell, len(cols)),
			comps: make(map[string]*string, len(cols)),
		}
		plotStr := ""
		for i, col := range cols {
			cc := pgValueToCloudCell(col, vals[i])
			rec.cells[col.Name] = cc
			rec.comps[col.Name] = cc.comparableString()
			if col.Name == plotCol && cc.kind == "text" {
				plotStr = cc.textVal
			}
		}
		if plotStr == "" {
			continue
		}
		snap.plots[plotStr] = true
		byPlot[plotStr] = append(byPlot[plotStr], rec)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for plot, list := range byPlot {
		cMap, rMap := buildLogicalRowsForPlot(suffix, cols, list)
		for rk, cm := range cMap {
			snap.cells[suffix][plot+"\x00"+rk] = cm
			snap.rawCells[suffix][plot+"\x00"+rk] = rMap[rk]
		}
	}
	return nil
}

func loadPGSubsetSnapshot(ctx context.Context, q pgQueryer, schema, label string, submissionID string, plotFilter []string) (*cloudSnapshot, error) {
	snap := newCloudSnapshot(label)
	if schema == "becmaster" && submissionID == "" && len(plotFilter) == 0 {
		return snap, nil
	}

	pgTables := map[string]string{
		"Env": "env", "Admin": "admin", "Veg": "veg",
		"Humus": "humus", "Mineral": "mineral", "Other": "other",
	}

	batch := &pgx.Batch{}
	for _, suffix := range mergeTableSuffixes {
		cols := VP08ColumnsBySuffix(suffix)
		plotCol := "PlotNumber"
		if suffix == "Admin" {
			plotCol = "Plot"
		}
		if schema == "staging" {
			batch.Queue(fmt.Sprintf(`SELECT %s FROM staging.%s WHERE submission_id = $1`, quotedColumnCSV(cols), quotePGIdent(pgTables[suffix])), submissionID)
		} else if submissionID != "" && len(plotFilter) == 0 {
			batch.Queue(fmt.Sprintf(`SELECT %s FROM becmaster.%s WHERE %s IN (SELECT "PlotNumber" FROM staging.env WHERE submission_id = $1)`, quotedColumnCSV(cols), quotePGIdent(pgTables[suffix]), quotePGIdent(plotCol)), submissionID)
		} else {
			batch.Queue(fmt.Sprintf(`SELECT %s FROM becmaster.%s WHERE %s = ANY($1)`, quotedColumnCSV(cols), quotePGIdent(pgTables[suffix]), quotePGIdent(plotCol)), plotFilter)
		}
	}

	br := q.SendBatch(ctx, batch)
	defer br.Close()

	for _, suffix := range mergeTableSuffixes {
		cols := VP08ColumnsBySuffix(suffix)
		rows, err := br.Query()
		if err != nil {
			return nil, fmt.Errorf("load %s.%s: %w", schema, pgTables[suffix], err)
		}
		if err := scanSnapshotTableRows(snap, suffix, cols, rows); err != nil {
			return nil, fmt.Errorf("scan %s.%s: %w", schema, pgTables[suffix], err)
		}
	}
	return snap, br.Close()
}

func loadSQLiteBaseSubsetSnapshot(ctx context.Context, dbPath, reqProject string, plotFilter []string) (*cloudSnapshot, string, error) {
	resolved, proj, err := resolveProjectInSQLiteFile(ctx, dbPath, reqProject)
	if err != nil {
		return nil, "", err
	}
	snap := newCloudSnapshot(proj)
	if len(plotFilter) == 0 {
		return snap, proj, nil
	}
	allowed := make(map[string]bool, len(plotFilter))
	for _, p := range plotFilter {
		allowed[p] = true
	}

	db, err := sqlutil.OpenReadOnlyContext(ctx, resolved)
	if err != nil {
		return nil, "", err
	}
	defer db.Close()

	for _, suffix := range mergeTableSuffixes {
		cols := VP08ColumnsBySuffix(suffix)
		plotCol := "PlotNumber"
		if suffix == "Admin" {
			plotCol = "Plot"
		}
		rows, qErr := db.QueryContext(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE typeof(%s) = 'text'`,
			sqliteSelectColumnsCSV(cols), quotePGIdent(proj+"_"+suffix), quotePGIdent(plotCol)))
		if qErr != nil {
			return nil, "", qErr
		}
		rawVals := make([]any, len(cols))
		dest := make([]any, len(cols))
		for i := range rawVals {
			dest[i] = &rawVals[i]
		}
		byPlot := map[string][]childRowRecord{}
		for rows.Next() {
			if sErr := rows.Scan(dest...); sErr != nil {
				rows.Close()
				return nil, "", sErr
			}
			rec := childRowRecord{
				cells: make(map[string]cloudCell, len(cols)),
				comps: make(map[string]*string, len(cols)),
			}
			plotStr := ""
			for i, col := range cols {
				pgVal, _ := CanonicalizeSQLiteToPG(col, rawVals[i])
				cc := pgValueToCloudCell(col, pgVal)
				rec.cells[col.Name] = cc
				rec.comps[col.Name] = cc.comparableString()
				if col.Name == plotCol && cc.kind == "text" {
					plotStr = cc.textVal
				}
			}
			if plotStr == "" || !allowed[plotStr] {
				continue
			}
			snap.plots[plotStr] = true
			byPlot[plotStr] = append(byPlot[plotStr], rec)
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, "", err
		}
		for plot, list := range byPlot {
			cMap, rMap := buildLogicalRowsForPlot(suffix, cols, list)
			for rk, cm := range cMap {
				snap.cells[suffix][plot+"\x00"+rk] = cm
				snap.rawCells[suffix][plot+"\x00"+rk] = rMap[rk]
			}
		}
	}
	return snap, proj, nil
}

func plotRowsFromSnap(snap *cloudSnapshot, suffix, plot string) map[string]map[string]*string {
	out := map[string]map[string]*string{}
	if snap == nil {
		return out
	}
	prefix := plot + "\x00"
	for k, v := range snap.cells[suffix] {
		if strings.HasPrefix(k, prefix) {
			out[strings.TrimPrefix(k, prefix)] = v
		}
	}
	return out
}

func plotRawRowsFromSnap(snap *cloudSnapshot, suffix, plot string) map[string]map[string]cloudCell {
	out := map[string]map[string]cloudCell{}
	if snap == nil {
		return out
	}
	prefix := plot + "\x00"
	for k, v := range snap.rawCells[suffix] {
		if strings.HasPrefix(k, prefix) {
			out[strings.TrimPrefix(k, prefix)] = v
		}
	}
	return out
}

func writeDigestItem(h hash.Hash, s string) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(len(s)))
	_, _ = h.Write(b[:])
	_, _ = h.Write([]byte(s))
}

func hashCloudSnapshot(snap *cloudSnapshot) string {
	if snap == nil {
		return ""
	}
	h := sha256.New()
	writeDigestItem(h, snap.label)
	for _, suffix := range mergeTableSuffixes {
		writeDigestItem(h, suffix)
		cols := VP08ColumnsBySuffix(suffix)
		keys := make([]string, 0, len(snap.rawCells[suffix]))
		for k := range snap.rawCells[suffix] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		writeDigestItem(h, strconv.Itoa(len(keys)))
		for _, k := range keys {
			writeDigestItem(h, k)
			rm := snap.rawCells[suffix][k]
			for _, col := range cols {
				c := rm[col.Name]
				writeDigestItem(h, c.kind)
				switch c.kind {
				case "int":
					writeDigestItem(h, strconv.FormatInt(c.intVal, 10))
				case "float":
					writeDigestItem(h, strconv.FormatUint(math.Float64bits(c.fltVal), 16))
				case "text":
					writeDigestItem(h, c.textVal)
				}
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func summarizeCloudRow(cols []ColumnSpec, row map[string]*string) string {
	var parts []string
	for _, col := range cols {
		if col.Name == "ID" || col.Name == "PlotNumber" || col.Name == "Plot" {
			continue
		}
		if v := row[col.Name]; v != nil && *v != "" && !(col.Name == "Flag" && *v == "0") {
			parts = append(parts, fmt.Sprintf("%s=%s", col.Name, *v))
		}
	}
	return strings.Join(parts, ", ")
}

func sameNullableStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func makeCloudConflictID(plot, table, key string) string {
	return plot + "::" + table + "::" + key
}

func buildStagingMergePreviewOnQueryer(
	ctx context.Context,
	q pgQueryer,
	submissionID string,
	baseDBPath string,
	baseProjectReq string,
) (StagingMergePreview, *cloudSnapshot, *cloudSnapshot, string, error) {
	t0 := time.Now()
	var projName, srcSHA string
	if err := q.QueryRow(ctx, `SELECT project_name, source_db_sha256 FROM staging.submissions WHERE submission_id = $1`, submissionID).Scan(&projName, &srcSHA); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StagingMergePreview{}, nil, nil, "", fmt.Errorf("staging submission %s not found", submissionID)
		}
		return StagingMergePreview{}, nil, nil, "", err
	}

	srcSnap, err := loadPGSubsetSnapshot(ctx, q, "staging", projName, submissionID, nil)
	if err != nil {
		return StagingMergePreview{}, nil, nil, "", err
	}

	plotSet := map[string]bool{}
	for p := range srcSnap.plots {
		plotSet[p] = true
	}

	var baseSnap *cloudSnapshot
	var baseProj string
	if strings.TrimSpace(baseDBPath) != "" {
		// Load base plots matching source plots (or any base plots so target-deleted plots are detected)
		resolvedBase, bp, bErr := resolveProjectInSQLiteFile(ctx, baseDBPath, baseProjectReq)
		if bErr != nil {
			return StagingMergePreview{}, nil, nil, "", fmt.Errorf("base snapshot database: %w", bErr)
		}
		baseProj = bp
		bdb, oErr := sqlutil.OpenReadOnlyContext(ctx, resolvedBase)
		if oErr != nil {
			return StagingMergePreview{}, nil, nil, "", oErr
		}
		pRows, pErr := bdb.QueryContext(ctx, fmt.Sprintf(`SELECT CAST("PlotNumber" AS TEXT) FROM %s WHERE typeof("PlotNumber")='text'`, quotePGIdent(bp+"_Env")))
		if pErr == nil {
			for pRows.Next() {
				var p string
				if pRows.Scan(&p) == nil && p != "" {
					plotSet[p] = true
				}
			}
			pRows.Close()
		}
		_ = bdb.Close()
	}

	overlapPlots := make([]string, 0, len(plotSet))
	for p := range plotSet {
		overlapPlots = append(overlapPlots, p)
	}
	slices.Sort(overlapPlots)

	if strings.TrimSpace(baseDBPath) != "" {
		baseSnap, baseProj, err = loadSQLiteBaseSubsetSnapshot(ctx, baseDBPath, baseProjectReq, overlapPlots)
		if err != nil {
			return StagingMergePreview{}, nil, nil, "", err
		}
	}

	tgtSnap, err := loadPGSubsetSnapshot(ctx, q, "becmaster", "BECMaster", "", overlapPlots)
	if err != nil {
		return StagingMergePreview{}, nil, nil, "", err
	}

	dh := sha256.New()
	writeDigestItem(dh, submissionID)
	writeDigestItem(dh, hashCloudSnapshot(srcSnap))
	writeDigestItem(dh, hashCloudSnapshot(tgtSnap))
	writeDigestItem(dh, hashCloudSnapshot(baseSnap))
	digest := hex.EncodeToString(dh.Sum(nil))

	preview := StagingMergePreview{
		SubmissionID:            submissionID,
		SourceProject:           projName,
		TargetProject:           "BECMaster",
		BaseDBPath:              strings.TrimSpace(baseDBPath),
		BaseProject:             baseProj,
		PreviewDigest:           digest,
		Plots:                   make([]CloudMergePlotPreview, 0, len(overlapPlots)),
		Conflicts:               []CloudMergeConflictItem{},
		CustodianProtectedDiffs: []CloudMergeConflictItem{},
	}

	lookupBase := func(suffix, plot, rowKey, field string) (*string, bool) {
		if baseSnap == nil || !baseSnap.plots[plot] {
			return nil, false
		}
		rm, has := baseSnap.cells[suffix][plot+"\x00"+rowKey]
		if !has {
			return nil, true
		}
		if field == "*" {
			s := summarizeCloudRow(VP08ColumnsBySuffix(suffix), rm)
			return &s, true
		}
		return rm[field], true
	}

	for _, plot := range overlapPlots {
		inSrc := srcSnap.plots[plot]
		inTgt := tgtSnap.plots[plot]

		if inSrc && !inTgt {
			if baseSnap != nil && baseSnap.plots[plot] {
				cid := makeCloudConflictID(plot, "Env", "Plot")
				present := "Present"
				item := CloudMergeConflictItem{
					ConflictID:        cid,
					PlotNumber:        plot,
					Table:             "Env",
					Key:               "Plot",
					Field:             "PlotNumber",
					BaseVal:           &present,
					SourceVal:         &present,
					TargetVal:         nil,
					Kind:              "Conflict",
					DefaultResolution: MergeResolutionKeepTarget,
				}
				preview.ConflictCount++
				preview.Plots = append(preview.Plots, CloudMergePlotPreview{
					PlotNumber: plot,
					Category:   "Conflict",
					Items:      []CloudMergeConflictItem{item},
				})
				preview.Conflicts = append(preview.Conflicts, item)
			} else {
				preview.AddedCount++
				preview.Plots = append(preview.Plots, CloudMergePlotPreview{
					PlotNumber: plot,
					Category:   "Added",
					Items:      []CloudMergeConflictItem{},
				})
			}
			continue
		}

		if !inSrc && inTgt {
			preview.TargetOnlyCount++
			preview.Plots = append(preview.Plots, CloudMergePlotPreview{
				PlotNumber: plot,
				Category:   "TargetOnly",
				Items:      []CloudMergeConflictItem{},
			})
			continue
		}
		if !inSrc && !inTgt {
			continue
		}

		// Both staging and becmaster contain this PlotNumber — diff all 6 plot tables!
		var items []CloudMergeConflictItem
		hasRealConflict := false
		hasFastForward := false

		for _, suffix := range mergeTableSuffixes {
			cols := VP08ColumnsBySuffix(suffix)
			srcRows := plotRowsFromSnap(srcSnap, suffix, plot)
			tgtRows := plotRowsFromSnap(tgtSnap, suffix, plot)

			rowKeySet := map[string]bool{}
			for rk := range srcRows {
				rowKeySet[rk] = true
			}
			for rk := range tgtRows {
				rowKeySet[rk] = true
			}
			rowKeys := make([]string, 0, len(rowKeySet))
			for rk := range rowKeySet {
				rowKeys = append(rowKeys, rk)
			}
			slices.Sort(rowKeys)

			isAmbiguousKey := func(rowKey string) bool {
				baseK := rowKey
				if hashIdx := strings.IndexByte(rowKey, '#'); hashIdx >= 0 {
					baseK = rowKey[:hashIdx]
				}
				return strings.Contains(rowKey, "#") || srcRows[baseK+"#2"] != nil || tgtRows[baseK+"#2"] != nil
			}

			for _, rk := range rowKeys {
				sRow, hasS := srcRows[rk]
				tRow, hasT := tgtRows[rk]
				ambiguous := suffix != "Env" && suffix != "Admin" && isAmbiguousKey(rk)
				if hasS && !hasT {
					sSum := summarizeCloudRow(cols, sRow)
					bVal, inBase := lookupBase(suffix, plot, rk, "*")
					kind, defRes := "Conflict", MergeResolutionKeepTarget
					if !ambiguous && baseSnap != nil && inBase && bVal == nil {
						kind = "FastForward"
						defRes = MergeResolutionAcceptSource
						hasFastForward = true
					} else {
						hasRealConflict = true
					}
					item := CloudMergeConflictItem{
						ConflictID:        makeCloudConflictID(plot, suffix, rk),
						PlotNumber:        plot,
						Table:             suffix,
						Key:               rk,
						Field:             "*",
						BaseVal:           bVal,
						SourceVal:         &sSum,
						TargetVal:         nil,
						Kind:              kind,
						DefaultResolution: defRes,
					}
					items = append(items, item)
					if kind == "Conflict" {
						preview.Conflicts = append(preview.Conflicts, item)
					}
					continue
				}
				if !hasS && hasT {
					tSum := summarizeCloudRow(cols, tRow)
					bVal, inBase := lookupBase(suffix, plot, rk, "*")
					kind, defRes := "Conflict", MergeResolutionKeepTarget
					if !ambiguous && baseSnap != nil && inBase {
						if sameNullableStr(&tSum, bVal) {
							kind = "FastForward"
							defRes = MergeResolutionAcceptSource
							hasFastForward = true
						} else {
							hasRealConflict = true
						}
					} else {
						hasRealConflict = true
					}
					item := CloudMergeConflictItem{
						ConflictID:        makeCloudConflictID(plot, suffix, rk),
						PlotNumber:        plot,
						Table:             suffix,
						Key:               rk,
						Field:             "*",
						BaseVal:           bVal,
						SourceVal:         nil,
						TargetVal:         &tSum,
						Kind:              kind,
						DefaultResolution: defRes,
					}
					items = append(items, item)
					if kind == "Conflict" {
						preview.Conflicts = append(preview.Conflicts, item)
					}
					continue
				}

				for _, col := range cols {
					if col.Name == "ID" || col.Name == "PlotNumber" || col.Name == "Plot" {
						continue
					}
					sv := sRow[col.Name]
					tv := tRow[col.Name]
					if sameNullableStr(sv, tv) {
						continue
					}
					fieldKey := col.Name
					if suffix != "Env" && suffix != "Admin" {
						fieldKey = rk + "." + col.Name
					}
					bVal, inBase := lookupBase(suffix, plot, rk, col.Name)
					isProtected := suffix == "Admin" && CustodianProtectedAdminFields[col.Name]
					kind, defRes := "Conflict", MergeResolutionKeepTarget
					if !isProtected && !ambiguous && baseSnap != nil && inBase {
						if sameNullableStr(tv, bVal) && !sameNullableStr(sv, bVal) {
							kind = "FastForward"
							defRes = MergeResolutionAcceptSource
							hasFastForward = true
						} else if sameNullableStr(sv, bVal) && !sameNullableStr(tv, bVal) {
							kind = "TargetModifiedOnly"
							defRes = MergeResolutionKeepTarget
						} else {
							hasRealConflict = true
						}
					} else {
						hasRealConflict = true
					}
					item := CloudMergeConflictItem{
						ConflictID:         makeCloudConflictID(plot, suffix, fieldKey),
						PlotNumber:         plot,
						Table:              suffix,
						Key:                fieldKey,
						Field:              col.Name,
						BaseVal:            bVal,
						SourceVal:          sv,
						TargetVal:          tv,
						Kind:               kind,
						DefaultResolution:  defRes,
						CustodianProtected: isProtected,
					}
					items = append(items, item)
					if kind == "Conflict" {
						preview.Conflicts = append(preview.Conflicts, item)
					}
					if isProtected {
						preview.CustodianProtectedDiffs = append(preview.CustodianProtectedDiffs, item)
					}
				}
			}
		}

		if len(items) == 0 {
			preview.IdenticalCount++
			preview.Plots = append(preview.Plots, CloudMergePlotPreview{
				PlotNumber: plot,
				Category:   "Identical",
				Items:      []CloudMergeConflictItem{},
			})
			continue
		}

		category := "Conflict"
		if !hasRealConflict && hasFastForward {
			category = "FastForward"
			preview.FastForwardCount++
		} else if !hasRealConflict {
			category = "TargetOnly"
			preview.TargetOnlyCount++
		} else {
			preview.ConflictCount++
		}
		preview.Plots = append(preview.Plots, CloudMergePlotPreview{
			PlotNumber: plot,
			Category:   category,
			Items:      items,
		})
	}

	preview.ElapsedMs = time.Since(t0).Milliseconds()
	return preview, srcSnap, tgtSnap, srcSHA, nil
}

// PreviewStagingToMasterMerge computes a fast (<500ms) indexed 3-way merge preview
// of staging.<submission> against overlapping plots in becmaster.
func (m *Manager) PreviewStagingToMasterMerge(ctx context.Context, req StagingMergePreviewRequest) (StagingMergePreview, error) {
	subID := strings.TrimSpace(req.SubmissionID)
	if _, err := uuid.Parse(subID); err != nil {
		return StagingMergePreview{}, errors.New("submissionId must be a valid UUID")
	}
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return StagingMergePreview{}, err
	}
	preview, _, _, _, err := buildStagingMergePreviewOnQueryer(ctx, pool, subID, req.BaseDBPath, req.BaseProject)
	if err != nil {
		return StagingMergePreview{}, wrapRedactedError("preview staging-to-BECMaster merge", err, creds)
	}
	return preview, nil
}

func validateCloudMergeCell(suffix string, col ColumnSpec, cell cloudCell) error {
	if cell.kind == "null" {
		return nil
	}
	if cell.kind == "text" {
		if err := ValidateUTF16Bound(suffix+"."+col.Name, cell.textVal, col.UTF16MaxLength); err != nil {
			return err
		}
	}
	if col.IsSingleCover && (cell.kind == "float" || cell.kind == "int") {
		v := cell.fltVal
		if cell.kind == "int" {
			v = float64(cell.intVal)
		}
		if _, ok := NarrowSingleCover(v); !ok {
			return fmt.Errorf("%s.%s cover=%v must be >= 0 and < 100 in IEEE-754 SINGLE", suffix, col.Name, v)
		}
	}
	return nil
}

func validateHorizonRowDepth(suffix string, row map[string]cloudCell) error {
	if suffix != "Humus" && suffix != "Mineral" {
		return nil
	}
	hz := ""
	if row["Horizon"].kind == "text" {
		hz = row["Horizon"].textVal
	}
	var uPtr, lPtr *float64
	if uc := row["UpperDepth"]; uc.kind == "float" {
		v := uc.fltVal
		uPtr = &v
	} else if uc.kind == "int" {
		v := float64(uc.intVal)
		uPtr = &v
	}
	if lc := row["LowerDepth"]; lc.kind == "float" {
		v := lc.fltVal
		lPtr = &v
	} else if lc.kind == "int" {
		v := float64(lc.intVal)
		lPtr = &v
	}
	return ValidateLMH25HorizonDepth(suffix, hz, uPtr, lPtr)
}

func allocateMasterChildID(ctx context.Context, tx pgx.Tx, childTable string) (int64, error) {
	var nextID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO becmaster.child_identity_watermark (child_table, max_id)
		VALUES ($1, COALESCE((SELECT GREATEST(MAX(id), 0) FROM becmaster.child_identity WHERE child_table = $1), 0) + 1)
		ON CONFLICT (child_table) DO UPDATE SET
			max_id = GREATEST(
				becmaster.child_identity_watermark.max_id + 1,
				COALESCE((SELECT GREATEST(MAX(id), 0) FROM becmaster.child_identity WHERE child_table = $1), 0) + 1
			)
		RETURNING max_id
	`, childTable).Scan(&nextID); err != nil {
		return 0, err
	}
	for {
		tag, err := tx.Exec(ctx, `INSERT INTO becmaster.child_identity (child_table, id) VALUES ($1, $2) ON CONFLICT (child_table, id) DO NOTHING`, childTable, nextID)
		if err != nil {
			return 0, err
		}
		if tag.RowsAffected() == 1 {
			_, _ = tx.Exec(ctx, `UPDATE becmaster.child_identity_watermark SET max_id = GREATEST(max_id, $2) WHERE child_table = $1`, childTable, nextID)
			return nextID, nil
		}
		nextID++
	}
}

func reserveMasterChildID(ctx context.Context, tx pgx.Tx, childTable string, id int64) error {
	if _, err := tx.Exec(ctx, `INSERT INTO becmaster.child_identity (child_table, id) VALUES ($1, $2) ON CONFLICT (child_table, id) DO NOTHING`, childTable, id); err != nil {
		return err
	}
	if id > 0 {
		_, _ = tx.Exec(ctx, `
			INSERT INTO becmaster.child_identity_watermark (child_table, max_id)
			VALUES ($1, $2)
			ON CONFLICT (child_table) DO UPDATE SET max_id = GREATEST(becmaster.child_identity_watermark.max_id, EXCLUDED.max_id)
		`, childTable, id)
	}
	return nil
}

// ExecuteStagingToMasterMerge commits selective staging -> becmaster changes inside a single
// SERIALIZABLE PostgreSQL transaction with optimistic preview digest lock and hash-chained audit provenance.
func (m *Manager) ExecuteStagingToMasterMerge(ctx context.Context, req StagingMergeExecuteRequest) (StagingMergeResult, error) {
	subID := strings.TrimSpace(req.SubmissionID)
	if _, err := uuid.Parse(subID); err != nil {
		return StagingMergeResult{}, errors.New("submissionId must be a valid UUID")
	}
	if strings.TrimSpace(req.ExpectedPreviewDigest) == "" {
		return StagingMergeResult{}, errors.New("expectedPreviewDigest is required; run PreviewStagingToMasterMerge first")
	}
	defaultRes := strings.TrimSpace(req.DefaultConflictResolution)
	if defaultRes == "" {
		defaultRes = MergeResolutionKeepTarget
	}
	checkChoice := func(c string) error {
		if c != MergeResolutionKeepTarget && c != MergeResolutionAcceptSource {
			return fmt.Errorf("invalid merge resolution %q (expected %q or %q)", c, MergeResolutionKeepTarget, MergeResolutionAcceptSource)
		}
		return nil
	}
	if err := checkChoice(defaultRes); err != nil {
		return StagingMergeResult{}, err
	}
	for k, v := range req.PlotResolutions {
		if k == "" {
			return StagingMergeResult{}, errors.New("plotResolutions key must not be empty")
		}
		if err := checkChoice(v); err != nil {
			return StagingMergeResult{}, err
		}
	}
	for k, v := range req.FieldResolutions {
		if k == "" {
			return StagingMergeResult{}, errors.New("fieldResolutions key must not be empty")
		}
		if err := checkChoice(v); err != nil {
			return StagingMergeResult{}, err
		}
	}

	resolveChoice := func(item CloudMergeConflictItem) string {
		if c, ok := req.FieldResolutions[item.ConflictID]; ok {
			return c
		}
		// Custodian-protected fields (BECSiteUnit, BEC_Use, GIS_BGC) require explicit per-field acceptance!
		if item.CustodianProtected {
			return MergeResolutionKeepTarget
		}
		if c, ok := req.PlotResolutions[item.PlotNumber]; ok {
			return c
		}
		if item.Kind == "FastForward" || item.Kind == "TargetModifiedOnly" {
			return item.DefaultResolution
		}
		return defaultRes
	}

	conn, creds, err := m.OpenDedicatedConn(ctx, RoleModeCustodian)
	if err != nil {
		return StagingMergeResult{}, err
	}
	defer conn.Close(ctx)

	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return StagingMergeResult{}, wrapRedactedError("begin SERIALIZABLE merge transaction", err, creds)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var curStatus, currentDBUser string
	if err := tx.QueryRow(ctx, `SELECT status, current_user FROM staging.submissions WHERE submission_id = $1 FOR UPDATE`, subID).Scan(&curStatus, &currentDBUser); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return StagingMergeResult{}, fmt.Errorf("staging submission %s not found", subID)
		}
		return StagingMergeResult{}, wrapRedactedError("lock staging submission", err, creds)
	}
	if curStatus != "pending" {
		return StagingMergeResult{}, fmt.Errorf("staging submission %s has status %q (only 'pending' submissions can be merged)", subID, curStatus)
	}

	actor := strings.TrimSpace(req.Actor)
	if actor == "" {
		actor = currentDBUser
	}

	preview, srcSnap, tgtSnap, srcSHA, err := buildStagingMergePreviewOnQueryer(ctx, tx, subID, req.BaseDBPath, req.BaseProject)
	if err != nil {
		return StagingMergeResult{}, wrapRedactedError("re-verify merge preview inside SERIALIZABLE transaction", err, creds)
	}
	if preview.PreviewDigest != strings.TrimSpace(req.ExpectedPreviewDigest) {
		return StagingMergeResult{}, errors.New("staging or BECMaster plots changed since preview; re-run PreviewStagingToMasterMerge before merging")
	}

	mergeTxID := uuid.NewString()
	nowEditStr := time.Now().UTC().Format("2006-01-02 15:04:05")
	nowUTC := time.Now().UTC()

	var pendingAudits [][]any // each entry is 11 VP08AuditColumns values
	recordAudit := func(plot, tableSuffix, field string, beforeVal, afterVal *string, childID *int64) {
		var bAny, aAny, idAny any
		if beforeVal != nil {
			bAny = *beforeVal
		}
		if afterVal != nil {
			aAny = *afterVal
		}
		if childID != nil {
			idAny = *childID
		}
		pendingAudits = append(pendingAudits, []any{
			"BECMaster", actor, plot, "_" + tableSuffix, field, nowEditStr,
			bAny, aAny, int16(0), int16(0), idAny,
		})
	}

	allocatedIDs := []int64{}
	reservedDeletedIDs := []int64{}
	addedPlots, updatedPlots, skippedPlots := 0, 0, 0

	pgTblName := map[string]string{
		"Env": "env", "Admin": "admin", "Veg": "veg",
		"Humus": "humus", "Mineral": "mineral", "Other": "other",
	}

	insertChildRow := func(suffix, plot string, srcRow map[string]cloudCell) error {
		if err := validateHorizonRowDepth(suffix, srcRow); err != nil {
			return fmt.Errorf("plot %s %s: %w", plot, suffix, err)
		}
		cols := VP08ColumnsBySuffix(suffix)
		childTbl := pgTblName[suffix]
		newID, aErr := allocateMasterChildID(ctx, tx, childTbl)
		if aErr != nil {
			return aErr
		}
		allocatedIDs = append(allocatedIDs, newID)

		placeholders := make([]string, len(cols))
		args := make([]any, len(cols))
		for i, col := range cols {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			switch col.Name {
			case "PlotNumber":
				args[i] = plot
			case "ID":
				args[i] = newID
			default:
				c := srcRow[col.Name]
				if vErr := validateCloudMergeCell(suffix, col, c); vErr != nil {
					return fmt.Errorf("plot %s: %w", plot, vErr)
				}
				args[i] = c.pgParam(col)
			}
		}
		insSQL := fmt.Sprintf("INSERT INTO becmaster.%s (%s) VALUES (%s)",
			quotePGIdent(childTbl), quotedColumnCSV(cols), strings.Join(placeholders, ", "))
		if _, err := tx.Exec(ctx, insSQL, args...); err != nil {
			return err
		}
		for _, col := range cols {
			if col.Name == "PlotNumber" || col.Name == "ID" {
				continue
			}
			c := srcRow[col.Name]
			if c.kind == "null" || (col.Name == "Flag" && c.kind == "int" && c.intVal == 0) {
				continue
			}
			recordAudit(plot, suffix, col.Name, nil, c.comparableString(), &newID)
		}
		return nil
	}

	insertWholePlot := func(plot string) error {
		if err := ValidateUTF16Bound("PlotNumber", plot, 7); err != nil {
			return err
		}
		for _, suffix := range []string{"Env", "Admin"} {
			srcMap := plotRawRowsFromSnap(srcSnap, suffix, plot)
			srcRow, ok := srcMap[suffix]
			if !ok {
				return fmt.Errorf("staging plot %s is missing required %s row", plot, suffix)
			}
			cols := VP08ColumnsBySuffix(suffix)
			keyCol := "PlotNumber"
			if suffix == "Admin" {
				keyCol = "Plot"
			}
			placeholders := make([]string, len(cols))
			args := make([]any, len(cols))
			for i, col := range cols {
				placeholders[i] = fmt.Sprintf("$%d", i+1)
				if col.Name == keyCol {
					args[i] = plot
				} else {
					c := srcRow[col.Name]
					if vErr := validateCloudMergeCell(suffix, col, c); vErr != nil {
						return fmt.Errorf("plot %s: %w", plot, vErr)
					}
					args[i] = c.pgParam(col)
				}
			}
			insSQL := fmt.Sprintf("INSERT INTO becmaster.%s (%s) VALUES (%s)",
				quotePGIdent(pgTblName[suffix]), quotedColumnCSV(cols), strings.Join(placeholders, ", "))
			if _, err := tx.Exec(ctx, insSQL, args...); err != nil {
				return err
			}
			pCopy := plot
			recordAudit(plot, suffix, keyCol, nil, &pCopy, nil)
			for _, col := range cols {
				if col.Name == keyCol {
					continue
				}
				c := srcRow[col.Name]
				if c.kind == "null" {
					continue
				}
				recordAudit(plot, suffix, col.Name, nil, c.comparableString(), nil)
			}
		}
		for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other"} {
			childMap := plotRawRowsFromSnap(srcSnap, suffix, plot)
			keys := make([]string, 0, len(childMap))
			for k := range childMap {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, rk := range keys {
				if err := insertChildRow(suffix, plot, childMap[rk]); err != nil {
					return err
				}
			}
		}
		return nil
	}

	colSpecLookup := func(suffix, field string) (ColumnSpec, bool) {
		for _, c := range VP08ColumnsBySuffix(suffix) {
			if c.Name == field {
				return c, true
			}
		}
		return ColumnSpec{}, false
	}

	for _, plotPlan := range preview.Plots {
		if err := ctx.Err(); err != nil {
			return StagingMergeResult{}, err
		}
		plot := plotPlan.PlotNumber
		switch plotPlan.Category {
		case "Identical", "TargetOnly":
			skippedPlots++
		case "Added":
			if choice, ok := req.PlotResolutions[plot]; ok && choice == MergeResolutionKeepTarget {
				skippedPlots++
				continue
			}
			if err := insertWholePlot(plot); err != nil {
				return StagingMergeResult{}, wrapRedactedError("insert added plot "+plot, err, creds)
			}
			addedPlots++
		case "FastForward", "Conflict":
			if len(plotPlan.Items) == 1 && plotPlan.Items[0].Key == "Plot" && !tgtSnap.plots[plot] && srcSnap.plots[plot] {
				if resolveChoice(plotPlan.Items[0]) == MergeResolutionAcceptSource {
					if err := insertWholePlot(plot); err != nil {
						return StagingMergeResult{}, wrapRedactedError("re-insert plot "+plot, err, creds)
					}
					addedPlots++
				} else {
					skippedPlots++
				}
				continue
			}

			plotModified := false
			touchedChildRows := map[string]struct{ suffix, plot, rowKey string }{}

			for _, item := range plotPlan.Items {
				if resolveChoice(item) != MergeResolutionAcceptSource {
					continue
				}
				suffix := item.Table
				pgTbl := pgTblName[suffix]

				if suffix == "Env" || suffix == "Admin" {
					keyCol := "PlotNumber"
					if suffix == "Admin" {
						keyCol = "Plot"
					}
					if item.Field == "*" {
						if item.SourceVal != nil && item.TargetVal == nil {
							srcRow := srcSnap.rawCells[suffix][plot+"\x00"+suffix]
							cols := VP08ColumnsBySuffix(suffix)
							placeholders := make([]string, len(cols))
							args := make([]any, len(cols))
							for i, col := range cols {
								placeholders[i] = fmt.Sprintf("$%d", i+1)
								if col.Name == keyCol {
									args[i] = plot
								} else {
									c := srcRow[col.Name]
									if vErr := validateCloudMergeCell(suffix, col, c); vErr != nil {
										return StagingMergeResult{}, fmt.Errorf("plot %s: %w", plot, vErr)
									}
									args[i] = c.pgParam(col)
								}
							}
							if _, iErr := tx.Exec(ctx, fmt.Sprintf("INSERT INTO becmaster.%s (%s) VALUES (%s)",
								quotePGIdent(pgTbl), quotedColumnCSV(cols), strings.Join(placeholders, ", ")), args...); iErr != nil {
								return StagingMergeResult{}, wrapRedactedError("insert missing "+suffix, iErr, creds)
							}
							pCopy := plot
							recordAudit(plot, suffix, keyCol, nil, &pCopy, nil)
							plotModified = true
						}
						continue
					}
					colSpec, ok := colSpecLookup(suffix, item.Field)
					if !ok {
						return StagingMergeResult{}, fmt.Errorf("unknown column %s.%s", suffix, item.Field)
					}
					srcCell := srcSnap.rawCells[suffix][plot+"\x00"+suffix][item.Field]
					if err := validateCloudMergeCell(suffix, colSpec, srcCell); err != nil {
						return StagingMergeResult{}, fmt.Errorf("plot %s: %w", plot, err)
					}
					tag, uErr := tx.Exec(ctx, fmt.Sprintf(
						`UPDATE becmaster.%s SET %s = $1 WHERE %s = $2`,
						quotePGIdent(pgTbl), quotePGIdent(item.Field), quotePGIdent(keyCol),
					), srcCell.pgParam(colSpec), plot)
					if uErr != nil {
						return StagingMergeResult{}, wrapRedactedError("update "+suffix+"."+item.Field, uErr, creds)
					}
					if tag.RowsAffected() != 1 {
						return StagingMergeResult{}, fmt.Errorf("merge update on %s.%s for plot %s matched %d rows", suffix, item.Field, plot, tag.RowsAffected())
					}
					recordAudit(plot, suffix, item.Field, item.TargetVal, item.SourceVal, nil)
					plotModified = true
					continue
				}

				// Child table addition / deletion
				if item.Field == "*" {
					if item.SourceVal != nil && item.TargetVal == nil {
						srcRow := srcSnap.rawCells[suffix][plot+"\x00"+item.Key]
						if err := insertChildRow(suffix, plot, srcRow); err != nil {
							return StagingMergeResult{}, wrapRedactedError("insert child row", err, creds)
						}
						plotModified = true
					} else if item.SourceVal == nil && item.TargetVal != nil {
						tgtRow := tgtSnap.rawCells[suffix][plot+"\x00"+item.Key]
						idCell := tgtRow["ID"]
						if idCell.kind != "int" {
							return StagingMergeResult{}, fmt.Errorf("target %s row in plot %s lacks integer ID", suffix, plot)
						}
						tgtID := idCell.intVal
						if err := reserveMasterChildID(ctx, tx, pgTbl, tgtID); err != nil {
							return StagingMergeResult{}, wrapRedactedError("reserve deleted child ID", err, creds)
						}
						reservedDeletedIDs = append(reservedDeletedIDs, tgtID)

						tag, dErr := tx.Exec(ctx, fmt.Sprintf(
							`DELETE FROM becmaster.%s WHERE "PlotNumber" = $1 AND "ID" = $2`,
							quotePGIdent(pgTbl),
						), plot, tgtID)
						if dErr != nil {
							return StagingMergeResult{}, wrapRedactedError("delete child row", dErr, creds)
						}
						if tag.RowsAffected() != 1 {
							return StagingMergeResult{}, fmt.Errorf("child delete on %s (plot %s, ID %d) matched %d rows", suffix, plot, tgtID, tag.RowsAffected())
						}
						for _, col := range VP08ColumnsBySuffix(suffix) {
							if col.Name == "PlotNumber" || col.Name == "ID" {
								continue
							}
							oldCell := tgtRow[col.Name]
							if oldCell.kind == "null" || (col.Name == "Flag" && oldCell.kind == "int" && oldCell.intVal == 0) {
								continue
							}
							recordAudit(plot, suffix, col.Name, oldCell.comparableString(), nil, &tgtID)
						}
						plotModified = true
					}
					continue
				}

				// Field-level update on an existing child row
				dotIdx := strings.LastIndex(item.Key, ".")
				if dotIdx <= 0 {
					return StagingMergeResult{}, fmt.Errorf("malformed child conflict key %q", item.Key)
				}
				rowKey := item.Key[:dotIdx]
				colSpec, ok := colSpecLookup(suffix, item.Field)
				if !ok {
					return StagingMergeResult{}, fmt.Errorf("unknown child column %s.%s", suffix, item.Field)
				}
				srcCell := srcSnap.rawCells[suffix][plot+"\x00"+rowKey][item.Field]
				if err := validateCloudMergeCell(suffix, colSpec, srcCell); err != nil {
					return StagingMergeResult{}, fmt.Errorf("plot %s: %w", plot, err)
				}
				tgtRow := tgtSnap.rawCells[suffix][plot+"\x00"+rowKey]
				idCell := tgtRow["ID"]
				if idCell.kind != "int" {
					return StagingMergeResult{}, fmt.Errorf("target %s row in plot %s lacks integer ID", suffix, plot)
				}
				tgtID := idCell.intVal
				tag, uErr := tx.Exec(ctx, fmt.Sprintf(
					`UPDATE becmaster.%s SET %s = $1 WHERE "PlotNumber" = $2 AND "ID" = $3`,
					quotePGIdent(pgTbl), quotePGIdent(item.Field),
				), srcCell.pgParam(colSpec), plot, tgtID)
				if uErr != nil {
					return StagingMergeResult{}, wrapRedactedError("update child field", uErr, creds)
				}
				if tag.RowsAffected() != 1 {
					return StagingMergeResult{}, fmt.Errorf("child update on %s.%s (plot %s, ID %d) matched %d rows", suffix, item.Field, plot, tgtID, tag.RowsAffected())
				}
				recordAudit(plot, suffix, item.Field, item.TargetVal, item.SourceVal, &tgtID)
				if suffix == "Humus" || suffix == "Mineral" {
					touchedChildRows[suffix+"\x00"+plot+"\x00"+rowKey] = struct{ suffix, plot, rowKey string }{suffix, plot, rowKey}
				}
				plotModified = true
			}

			// Verify LMH25 horizon depth sign invariant on any updated Humus/Mineral rows!
			for _, tr := range touchedChildRows {
				tgtID := tgtSnap.rawCells[tr.suffix][tr.plot+"\x00"+tr.rowKey]["ID"].intVal
				var hz *string
				var up, lo *float64
				if err := tx.QueryRow(ctx, fmt.Sprintf(
					`SELECT "Horizon", "UpperDepth", "LowerDepth" FROM becmaster.%s WHERE "PlotNumber" = $1 AND "ID" = $2`,
					quotePGIdent(pgTblName[tr.suffix]),
				), tr.plot, tgtID).Scan(&hz, &up, &lo); err != nil {
					return StagingMergeResult{}, wrapRedactedError("verify merged horizon depth", err, creds)
				}
				hzStr := ""
				if hz != nil {
					hzStr = *hz
				}
				if err := ValidateLMH25HorizonDepth(tr.suffix, hzStr, up, lo); err != nil {
					return StagingMergeResult{}, fmt.Errorf("plot %s: %w", tr.plot, err)
				}
			}

			if plotModified {
				updatedPlots++
			} else {
				skippedPlots++
			}
		}
	}

	// Insert genuinely new ProjectID metadata rows into becmaster.metadata (never overwriting existing curated master metadata without review)
	if addedPlots > 0 || updatedPlots > 0 {
		newMetaRows, mErr := tx.Query(ctx, fmt.Sprintf(`
			SELECT %s FROM staging.metadata s
			WHERE s.submission_id = $1
			  AND NOT EXISTS (SELECT 1 FROM becmaster.metadata m WHERE m."ProjectID" = s."ProjectID")
		`, quotedColumnCSV(VP08MetadataColumns)), subID)
		if mErr != nil {
			return StagingMergeResult{}, wrapRedactedError("query new staging.metadata rows", mErr, creds)
		}
		var metaPayloads [][]any
		for newMetaRows.Next() {
			vals, vErr := newMetaRows.Values()
			if vErr != nil {
				newMetaRows.Close()
				return StagingMergeResult{}, wrapRedactedError("scan staging.metadata row", vErr, creds)
			}
			cp := make([]any, len(vals))
			copy(cp, vals)
			metaPayloads = append(metaPayloads, cp)
		}
		newMetaRows.Close()
		if err := newMetaRows.Err(); err != nil {
			return StagingMergeResult{}, wrapRedactedError("iterate staging.metadata rows", err, creds)
		}

		metaIDColIdx := len(VP08MetadataColumns) - 1
		placeholders := make([]string, len(VP08MetadataColumns))
		for i := range placeholders {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
		insMetaSQL := fmt.Sprintf("INSERT INTO becmaster.metadata (%s) VALUES (%s) ON CONFLICT (\"ProjectID\") DO NOTHING",
			quotedColumnCSV(VP08MetadataColumns), strings.Join(placeholders, ", "))
		for _, rowVals := range metaPayloads {
			newMetaID, aErr := allocateMasterChildID(ctx, tx, "metadata")
			if aErr != nil {
				return StagingMergeResult{}, wrapRedactedError("allocate metadata ID", aErr, creds)
			}
			rowVals[metaIDColIdx] = newMetaID
			tag, iErr := tx.Exec(ctx, insMetaSQL, rowVals...)
			if iErr != nil {
				return StagingMergeResult{}, wrapRedactedError("insert becmaster.metadata", iErr, creds)
			}
			if tag.RowsAffected() == 1 {
				if pid, ok := rowVals[0].(string); ok {
					pidCopy := pid
					recordAudit("", "Metadata", "ProjectID", nil, &pidCopy, &newMetaID)
				}
			}
		}
	}

	// Append hash-chained audit trail rows into becmaster.audit
	prevHash := GenesisAuditHash
	_ = tx.QueryRow(ctx, `SELECT row_hash FROM becmaster.audit ORDER BY audit_seq DESC LIMIT 1`).Scan(&prevHash)
	if len(pendingAudits) > 0 {
		copyCols := append(columnNames(VP08AuditColumns), "merge_tx_id", "prev_hash", "row_hash")
		idx := 0
		_, cErr := tx.CopyFrom(
			ctx,
			pgx.Identifier{"becmaster", "audit"},
			copyCols,
			pgx.CopyFromFunc(func() ([]any, error) {
				if idx >= len(pendingAudits) {
					return nil, nil
				}
				vp08Row := pendingAudits[idx]
				idx++
				rowHash := ComputeAuditRowHash(prevHash, mergeTxID, vp08Row)
				out := make([]any, len(copyCols))
				copy(out, vp08Row)
				out[len(VP08AuditColumns)] = mergeTxID
				out[len(VP08AuditColumns)+1] = prevHash
				out[len(VP08AuditColumns)+2] = rowHash
				prevHash = rowHash
				return out, nil
			}),
		)
		if cErr != nil {
			return StagingMergeResult{}, wrapRedactedError("append hash-chained becmaster.audit rows", cErr, creds)
		}
	}

	summaryPayload, _ := json.Marshal(map[string]any{
		"sourceProject":      preview.SourceProject,
		"previewDigest":      preview.PreviewDigest,
		"addedPlots":         addedPlots,
		"updatedPlots":       updatedPlots,
		"skippedPlots":       skippedPlots,
		"auditRowsWritten":   len(pendingAudits),
		"allocatedChildIds":  len(allocatedIDs),
		"reservedDeletedIds": len(reservedDeletedIDs),
		"tailAuditHash":      prevHash,
	})

	if _, err := tx.Exec(ctx, `
		INSERT INTO becmaster.merge_ledger (
			merge_tx_id, submission_id, actor, merged_at, source_sha256,
			added_plots, updated_plots, summary_json
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)
	`, mergeTxID, subID, actor, nowUTC, srcSHA, addedPlots, updatedPlots, string(summaryPayload)); err != nil {
		return StagingMergeResult{}, wrapRedactedError("insert becmaster.merge_ledger", err, creds)
	}

	var revNotesPtr *string
	if rn := strings.TrimSpace(req.ReviewerNotes); rn != "" {
		revNotesPtr = &rn
	}
	if _, err := tx.Exec(ctx, `
		UPDATE staging.submissions
		SET status = 'merged',
		    reviewed_by = $2,
		    reviewed_at = $3,
		    reviewer_notes = COALESCE($4, reviewer_notes),
		    merged_tx_id = $5
		WHERE submission_id = $1
	`, subID, actor, nowUTC, revNotesPtr, mergeTxID); err != nil {
		return StagingMergeResult{}, wrapRedactedError("mark staging submission merged", err, creds)
	}

	if err := tx.Commit(ctx); err != nil {
		return StagingMergeResult{}, wrapRedactedError("commit SERIALIZABLE merge transaction", err, creds)
	}

	slices.Sort(allocatedIDs)
	slices.Sort(reservedDeletedIDs)
	reservedDeletedIDs = slices.Compact(reservedDeletedIDs)

	return StagingMergeResult{
		MergeTxID:          mergeTxID,
		SubmissionID:       subID,
		SourceProject:      preview.SourceProject,
		AddedPlots:         addedPlots,
		UpdatedPlots:       updatedPlots,
		SkippedPlots:       skippedPlots,
		AllocatedChildIDs:  allocatedIDs,
		ReservedDeletedIDs: reservedDeletedIDs,
		AuditRowsWritten:   len(pendingAudits),
		TailAuditHash:      prevHash,
		MergedAt:           nowUTC.Format(time.RFC3339),
	}, nil
}

// RejectStagingSubmission marks a pending staging submission as rejected with reviewer notes.
func (m *Manager) RejectStagingSubmission(ctx context.Context, req RejectSubmissionRequest) (StagingSubmissionSummary, error) {
	subID := strings.TrimSpace(req.SubmissionID)
	if _, err := uuid.Parse(subID); err != nil {
		return StagingSubmissionSummary{}, errors.New("submissionId must be a valid UUID")
	}
	notes := strings.TrimSpace(req.ReviewerNotes)
	if notes == "" {
		return StagingSubmissionSummary{}, errors.New("reviewerNotes are required when rejecting a staging submission")
	}

	pool, creds, err := m.AcquirePool(ctx, RoleModeCustodian)
	if err != nil {
		return StagingSubmissionSummary{}, err
	}
	actor := strings.TrimSpace(req.Actor)
	nowUTC := time.Now().UTC()
	tag, err := pool.Exec(ctx, `
		UPDATE staging.submissions
		SET status = 'rejected',
		    reviewer_notes = $2,
		    reviewed_by = COALESCE(NULLIF($3, ''), current_user),
		    reviewed_at = $4
		WHERE submission_id = $1 AND status = 'pending'
	`, subID, notes, actor, nowUTC)
	if err != nil {
		return StagingSubmissionSummary{}, wrapRedactedError("reject staging submission", err, creds)
	}
	if tag.RowsAffected() == 0 {
		return StagingSubmissionSummary{}, fmt.Errorf("staging submission %s is not in 'pending' status or does not exist", subID)
	}

	list, err := m.ListStagingSubmissions(ctx, ListSubmissionsFilter{Limit: 200})
	if err == nil {
		for _, item := range list {
			if item.SubmissionID == subID {
				return item, nil
			}
		}
	}
	return StagingSubmissionSummary{
		SubmissionID:  subID,
		Status:        "rejected",
		ReviewerNotes: notes,
		ReviewedAt:    nowUTC.Format(time.RFC3339),
	}, nil
}

// CreateBECMasterSnapshot cuts a named read-only snapshot record in becmaster.snapshots (Schedule A 1.iii)
// and optionally exports a local .db / .rds / .gpkg snapshot bundle.
func (m *Manager) CreateBECMasterSnapshot(
	ctx context.Context,
	s *vprocore.ContextService,
	contextID string,
	req CreateSnapshotRequest,
	bundleExporter SnapshotBundleExporterFunc,
) (SnapshotRecord, error) {
	tag := strings.TrimSpace(req.SnapshotTag)
	if tag == "" || len(tag) > 80 {
		return SnapshotRecord{}, errors.New("snapshotTag must be 1..80 characters")
	}
	for _, r := range tag {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.') {
			return SnapshotRecord{}, errors.New("snapshotTag may contain only ASCII letters, digits, '_', '-', or '.'")
		}
	}
	format := strings.ToLower(strings.TrimSpace(req.ExportFormat))
	if format != "" && format != "sqlite" && format != "rds" && format != "gpkg" {
		return SnapshotRecord{}, fmt.Errorf("unsupported snapshot exportFormat %q (expected sqlite, rds, or gpkg)", format)
	}

	pool, creds, err := m.AcquirePool(ctx, RoleModeCustodian)
	if err != nil {
		return SnapshotRecord{}, err
	}

	var plotCount int
	var vegCount int64
	var tailHash string
	if err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM becmaster.env),
			(SELECT COUNT(*) FROM becmaster.veg),
			COALESCE((SELECT row_hash FROM becmaster.audit ORDER BY audit_seq DESC LIMIT 1), $1)
	`, GenesisAuditHash).Scan(&plotCount, &vegCount, &tailHash); err != nil {
		return SnapshotRecord{}, wrapRedactedError("read BECMaster snapshot counts", err, creds)
	}

	becVer, taxVer := readBundledStandardsVersions()
	_ = pool.QueryRow(ctx, `
		SELECT bec_version, taxonomy_version FROM standards.library_releases
		WHERE is_current = TRUE ORDER BY published_at DESC LIMIT 1
	`).Scan(&becVer, &taxVer)

	var exportedPath, manifestSHA string
	if format != "" {
		if s == nil || contextID == "" {
			return SnapshotRecord{}, errors.New("active context is required when exporting a local snapshot file")
		}
		destPath := strings.TrimSpace(req.DestinationPath)
		if destPath == "" {
			return SnapshotRecord{}, errors.New("destinationPath is required when exportFormat is specified")
		}
		filter := SubsetFilter{AllowFullCheckout: true}
		if z := strings.TrimSpace(req.ZoneFilter); z != "" {
			filter = SubsetFilter{Zones: []string{z}}
		}
		stageSQLitePath := destPath
		var cleanupTemp func()
		if format != "sqlite" {
			tmpDir, tErr := os.MkdirTemp("", "vpro-snap-bundle-*")
			if tErr != nil {
				return SnapshotRecord{}, tErr
			}
			cleanupTemp = func() { _ = os.RemoveAll(tmpDir) }
			stageSQLitePath = filepath.Join(tmpDir, "SnapStage.db")
		}
		if cleanupTemp != nil {
			defer cleanupTemp()
		}

		subRes, pErr := m.PullBECMasterSubset(ctx, s, contextID, PullSubsetRequest{
			Filter:            filter,
			TargetProject:     "SnapExport",
			DestinationPath:   stageSQLitePath,
			ActivateOnSuccess: false,
		})
		if pErr != nil {
			return SnapshotRecord{}, pErr
		}
		if format == "sqlite" {
			exportedPath = subRes.DestinationPath
			manifestSHA = subRes.DestinationSHA
		} else {
			if bundleExporter == nil {
				return SnapshotRecord{}, fmt.Errorf("snapshot exporter for format %q is not wired", format)
			}
			expP, expSHA, bErr := bundleExporter(ctx, subRes.DestinationPath, "SnapExport", format, destPath)
			if bErr != nil {
				return SnapshotRecord{}, bErr
			}
			exportedPath = expP
			manifestSHA = expSHA
		}
	}

	if manifestSHA == "" {
		sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s|%s|%s", tag, plotCount, vegCount, becVer, taxVer, tailHash)))
		manifestSHA = hex.EncodeToString(sum[:])
	}

	nowUTC := time.Now().UTC()
	var notesPtr *string
	if n := strings.TrimSpace(req.Notes); n != "" {
		notesPtr = &n
	}
	var createdBy string
	if err := pool.QueryRow(ctx, `
		INSERT INTO becmaster.snapshots (
			snapshot_tag, created_at, created_by, plot_count, veg_count,
			bec_version, taxonomy_version, manifest_sha256, notes
		) VALUES ($1, $2, current_user, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (snapshot_tag) DO UPDATE SET
			created_at = EXCLUDED.created_at,
			created_by = EXCLUDED.created_by,
			plot_count = EXCLUDED.plot_count,
			veg_count = EXCLUDED.veg_count,
			bec_version = EXCLUDED.bec_version,
			taxonomy_version = EXCLUDED.taxonomy_version,
			manifest_sha256 = EXCLUDED.manifest_sha256,
			notes = EXCLUDED.notes
		RETURNING created_by
	`, tag, nowUTC, plotCount, vegCount, becVer, taxVer, manifestSHA, notesPtr).Scan(&createdBy); err != nil {
		return SnapshotRecord{}, wrapRedactedError("insert becmaster.snapshots", err, creds)
	}

	return SnapshotRecord{
		SnapshotTag:     tag,
		CreatedAt:       nowUTC.Format(time.RFC3339),
		CreatedBy:       createdBy,
		PlotCount:       plotCount,
		VegCount:        vegCount,
		BECVersion:      becVer,
		TaxonomyVersion: taxVer,
		ManifestSHA256:  manifestSHA,
		Notes:           strings.TrimSpace(req.Notes),
		ExportedPath:    exportedPath,
	}, nil
}

// ListBECMasterSnapshots lists dated snapshots newest-first.
func (m *Manager) ListBECMasterSnapshots(ctx context.Context) ([]SnapshotRecord, error) {
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT snapshot_tag, created_at, created_by, plot_count, veg_count,
		       bec_version, taxonomy_version, manifest_sha256, COALESCE(notes, '')
		FROM becmaster.snapshots
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, wrapRedactedError("list becmaster.snapshots", err, creds)
	}
	defer rows.Close()

	out := []SnapshotRecord{}
	for rows.Next() {
		var r SnapshotRecord
		var ts time.Time
		if err := rows.Scan(&r.SnapshotTag, &ts, &r.CreatedBy, &r.PlotCount, &r.VegCount, &r.BECVersion, &r.TaxonomyVersion, &r.ManifestSHA256, &r.Notes); err != nil {
			return nil, wrapRedactedError("scan becmaster.snapshots row", err, creds)
		}
		r.CreatedAt = ts.UTC().Format(time.RFC3339)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapRedactedError("iterate becmaster.snapshots", err, creds)
	}
	return out, nil
}

// ListBECMasterDownloadLog lists download_log audit records (Schedule A 1.v).
func (m *Manager) ListBECMasterDownloadLog(ctx context.Context, limit int) ([]DownloadLogEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	pool, creds, err := m.AcquirePool(ctx, RoleModeCustodian)
	if err != nil {
		return nil, err
	}
	rows, err := pool.Query(ctx, `
		SELECT id, downloaded_at, actor, role_tier, filter_json::text, format, plot_count
		FROM becmaster.download_log
		ORDER BY id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, wrapRedactedError("list becmaster.download_log", err, creds)
	}
	defer rows.Close()

	out := []DownloadLogEntry{}
	for rows.Next() {
		var e DownloadLogEntry
		var ts time.Time
		var rawFilter string
		if err := rows.Scan(&e.ID, &ts, &e.Actor, &e.RoleTier, &rawFilter, &e.Format, &e.PlotCount); err != nil {
			return nil, wrapRedactedError("scan becmaster.download_log row", err, creds)
		}
		e.DownloadedAt = ts.UTC().Format(time.RFC3339)
		e.FilterJSON = json.RawMessage(rawFilter)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapRedactedError("iterate becmaster.download_log", err, creds)
	}
	return out, nil
}

// VerifyBECMasterAuditChain verifies the SHA-256 hash chain over the most recent limit rows in becmaster.audit,
// anchoring the window's first row against either GenesisAuditHash (at the start of the ledger) or its immediate predecessor.
func (m *Manager) VerifyBECMasterAuditChain(ctx context.Context, limit int) (AuditChainVerification, error) {
	if limit <= 0 {
		limit = 1000
	}
	pool, creds, err := m.AcquirePool(ctx, "")
	if err != nil {
		return AuditChainVerification{}, err
	}
	query := fmt.Sprintf(`
		SELECT * FROM (
			SELECT audit_seq, %s, COALESCE(merge_tx_id::text, ''), prev_hash, row_hash
			FROM becmaster.audit
			ORDER BY audit_seq DESC
			LIMIT $1
		) sub
		ORDER BY audit_seq ASC
	`, quotedColumnCSV(VP08AuditColumns))

	rows, err := pool.Query(ctx, query, limit)
	if err != nil {
		return AuditChainVerification{}, wrapRedactedError("verify becmaster.audit hash chain", err, creds)
	}
	defer rows.Close()

	var count int64
	var firstSeq int64
	var firstPrevHash string
	expectedPrev := ""
	tailHash := GenesisAuditHash
	for rows.Next() {
		vals, vErr := rows.Values()
		if vErr != nil {
			return AuditChainVerification{}, wrapRedactedError("scan becmaster.audit row", vErr, creds)
		}
		seq, _ := vals[0].(int64)
		vp08Row := make([]any, len(VP08AuditColumns))
		for i, col := range VP08AuditColumns {
			cc := pgValueToCloudCell(col, vals[1+i])
			vp08Row[i] = cc.pgParam(col)
		}
		mergeTx, _ := vals[12].(string)
		prevHash, _ := vals[13].(string)
		rowHash, _ := vals[14].(string)

		if count == 0 {
			firstSeq = seq
			firstPrevHash = prevHash
		} else if prevHash != expectedPrev {
			return AuditChainVerification{VerifiedRows: count, Valid: false, TailHash: tailHash}, fmt.Errorf("audit hash chain broken at seq=%d: prev_hash=%s != expected=%s", seq, prevHash, expectedPrev)
		}
		computed := ComputeAuditRowHash(prevHash, mergeTx, vp08Row)
		if computed != rowHash {
			return AuditChainVerification{VerifiedRows: count, Valid: false, TailHash: tailHash}, fmt.Errorf("audit row_hash tampered at seq=%d: computed=%s != stored=%s", seq, computed, rowHash)
		}
		expectedPrev = rowHash
		tailHash = rowHash
		count++
	}
	if err := rows.Err(); err != nil {
		return AuditChainVerification{}, wrapRedactedError("iterate becmaster.audit chain", err, creds)
	}
	rows.Close()

	partialWindow := false
	if count > 0 {
		var predHash string
		pErr := pool.QueryRow(ctx, `SELECT row_hash FROM becmaster.audit WHERE audit_seq < $1 ORDER BY audit_seq DESC LIMIT 1`, firstSeq).Scan(&predHash)
		if errors.Is(pErr, pgx.ErrNoRows) {
			// Window starts at the first surviving row in becmaster.audit -> must anchor to GenesisAuditHash!
			if firstPrevHash != GenesisAuditHash {
				return AuditChainVerification{VerifiedRows: count, Valid: false, TailHash: tailHash}, fmt.Errorf("initial audit row seq=%d does not anchor to GenesisAuditHash (deleted prefix detected)", firstSeq)
			}
		} else if pErr != nil {
			return AuditChainVerification{}, wrapRedactedError("verify audit chain predecessor anchor", pErr, creds)
		} else {
			partialWindow = true
			if firstPrevHash != predHash {
				return AuditChainVerification{VerifiedRows: count, Valid: false, PartialWindow: true, TailHash: tailHash}, fmt.Errorf("audit window boundary broken at seq=%d: prev_hash=%s != predecessor=%s", firstSeq, firstPrevHash, predHash)
			}
		}
	}

	return AuditChainVerification{
		VerifiedRows:  count,
		Valid:         true,
		PartialWindow: partialWindow,
		TailHash:      tailHash,
	}, nil
}
