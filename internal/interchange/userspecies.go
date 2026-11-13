package interchange

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const UserSpeciesManifestSchema = "vpro.userspecies.v1"

type UserSpeciesRecord struct {
	Code           string   `json:"code"`
	ScientificName *string  `json:"scientificName"`
	LifeForm       *int     `json:"lifeForm"`
	EnglishName    *string  `json:"englishName"`
	Report         *float64 `json:"report,omitempty"`
	SppNumber      *int     `json:"sppNumber,omitempty"`
	Codetype       *string  `json:"codetype,omitempty"`
}

type UserSpeciesManifest struct {
	SchemaVersion string              `json:"schemaVersion"`
	ExportedAt    string              `json:"exportedAt"`
	SourceProject string              `json:"sourceProject"`
	RowCount      int                 `json:"rowCount"`
	PayloadSHA256 string              `json:"payloadSha256"`
	Species       []UserSpeciesRecord `json:"species"`
}

type UserSpeciesExportRequest struct {
	Format          string `json:"format"` // "json" | "csv"
	DestinationPath string `json:"destinationPath,omitempty"`
}

func (r *UserSpeciesExportRequest) UnmarshalJSON(data []byte) error {
	type plain UserSpeciesExportRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type UserSpeciesExportResult struct {
	Format        string              `json:"format"`
	RowCount      int                 `json:"rowCount"`
	Path          string              `json:"path,omitempty"`
	FileSHA256    string              `json:"fileSha256"`
	PayloadSHA256 string              `json:"payloadSha256"`
	Content       string              `json:"content"`
	Species       []UserSpeciesRecord `json:"species"`
}

type UserSpeciesCollisionPolicy string

const (
	CollisionSkip      UserSpeciesCollisionPolicy = "skip"
	CollisionOverwrite UserSpeciesCollisionPolicy = "overwrite"
)

type UserSpeciesImportRequest struct {
	SourcePath      string                     `json:"sourcePath,omitempty"`
	Format          string                     `json:"format,omitempty"` // "json" | "csv"
	Content         string                     `json:"content,omitempty"`
	CollisionPolicy UserSpeciesCollisionPolicy `json:"collisionPolicy"`
}

func (r *UserSpeciesImportRequest) UnmarshalJSON(data []byte) error {
	type plain UserSpeciesImportRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

type UserSpeciesImportItemReview struct {
	Code           string `json:"code"`
	ScientificName string `json:"scientificName"`
	LifeForm       *int   `json:"lifeForm,omitempty"`
	Status         string `json:"status"` // "new" | "collision_user" | "collision_master"
	Action         string `json:"action"` // "insert" | "overwrite" | "skip"
	Reason         string `json:"reason,omitempty"`
}

type UserSpeciesImportPreview struct {
	TotalRows        int                           `json:"totalRows"`
	NewCount         int                           `json:"newCount"`
	UserCollisions   int                           `json:"userCollisions"`
	MasterCollisions int                           `json:"masterCollisions"`
	PayloadSHA256    string                        `json:"payloadSha256"`
	Items            []UserSpeciesImportItemReview `json:"items"`
}

type UserSpeciesImportResult struct {
	Preview          UserSpeciesImportPreview `json:"preview"`
	InsertedCount    int                      `json:"insertedCount"`
	OverwrittenCount int                      `json:"overwrittenCount"`
	SkippedCount     int                      `json:"skippedCount"`
}

func computeUserSpeciesPayloadHash(records []UserSpeciesRecord) (string, error) {
	canonical := make([]UserSpeciesRecord, len(records))
	copy(canonical, records)
	slices.SortFunc(canonical, func(a, b UserSpeciesRecord) int {
		return strings.Compare(strings.ToUpper(a.Code), strings.ToUpper(b.Code))
	})
	raw, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func readContextUserSpecies(ctx context.Context, tx *sql.Tx) ([]UserSpeciesRecord, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT Code, ScientificName, LifeForm, EnglishName, Report, SppNumber, Codetype
		FROM VUser.USysUserSpp
		ORDER BY Code COLLATE NOCASE, Code`)
	if err != nil {
		return nil, fmt.Errorf("read VUser.USysUserSpp: %w", err)
	}
	defer rows.Close()

	list := []UserSpeciesRecord{}
	for rows.Next() {
		var code string
		var sci, eng, ctype sql.NullString
		var lf, sppNum sql.NullInt64
		var rep sql.NullFloat64
		if err := rows.Scan(&code, &sci, &lf, &eng, &rep, &sppNum, &ctype); err != nil {
			return nil, err
		}
		rec := UserSpeciesRecord{Code: strings.ToUpper(strings.TrimSpace(code))}
		if sci.Valid {
			v := sci.String
			rec.ScientificName = &v
		}
		if lf.Valid {
			v := int(lf.Int64)
			rec.LifeForm = &v
		}
		if eng.Valid {
			v := eng.String
			rec.EnglishName = &v
		}
		if rep.Valid {
			v := rep.Float64
			rec.Report = &v
		}
		if sppNum.Valid {
			v := int(sppNum.Int64)
			rec.SppNumber = &v
		}
		if ctype.Valid {
			v := ctype.String
			rec.Codetype = &v
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}

func ExportUserSpeciesList(s *vprocore.ContextService, ctx context.Context, contextID string, request UserSpeciesExportRequest) (UserSpeciesExportResult, error) {
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if format == "" {
		if strings.HasSuffix(strings.ToLower(request.DestinationPath), ".csv") {
			format = "csv"
		} else {
			format = "json"
		}
	}
	if format != "json" && format != "csv" {
		return UserSpeciesExportResult{}, fmt.Errorf("unsupported user species export format %q (expected json or csv)", request.Format)
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (UserSpeciesExportResult, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return UserSpeciesExportResult{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return UserSpeciesExportResult{}, err
		}
		defer func() { _ = tx.Rollback() }()

		records, err := readContextUserSpecies(ctx, tx)
		if err != nil {
			return UserSpeciesExportResult{}, err
		}
		project := owner.Selection.Project
		if err := tx.Commit(); err != nil {
			return UserSpeciesExportResult{}, err
		}

		payloadSHA, err := computeUserSpeciesPayloadHash(records)
		if err != nil {
			return UserSpeciesExportResult{}, err
		}

		var encoded []byte
		if format == "json" {
			manifest := UserSpeciesManifest{
				SchemaVersion: UserSpeciesManifestSchema,
				ExportedAt:    time.Now().UTC().Format(time.RFC3339),
				SourceProject: project,
				RowCount:      len(records),
				PayloadSHA256: payloadSHA,
				Species:       records,
			}
			encoded, err = json.MarshalIndent(manifest, "", "  ")
			if err != nil {
				return UserSpeciesExportResult{}, err
			}
			encoded = append(encoded, '\n')
		} else {
			var buf bytes.Buffer
			w := csv.NewWriter(&buf)
			if err := w.Write([]string{"Code", "ScientificName", "LifeForm", "EnglishName", "Report", "SppNumber", "Codetype"}); err != nil {
				return UserSpeciesExportResult{}, err
			}
			for _, r := range records {
				row := []string{
					r.Code,
					optString(r.ScientificName),
					optInt(r.LifeForm),
					optString(r.EnglishName),
					optFloat(r.Report),
					optInt(r.SppNumber),
					optString(r.Codetype),
				}
				if err := w.Write(row); err != nil {
					return UserSpeciesExportResult{}, err
				}
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return UserSpeciesExportResult{}, err
			}
			encoded = buf.Bytes()
		}

		writtenPath, fileSHA, err := writeAtomicExportFile(request.DestinationPath, encoded)
		if err != nil {
			return UserSpeciesExportResult{}, err
		}
		return UserSpeciesExportResult{
			Format:        format,
			RowCount:      len(records),
			Path:          writtenPath,
			FileSHA256:    fileSHA,
			PayloadSHA256: payloadSHA,
			Content:       string(encoded),
			Species:       records,
		}, nil
	})
}

func optString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func optInt(v *int) string {
	if v == nil {
		return ""
	}
	return strconv.Itoa(*v)
}

func optFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func parseUserSpeciesInput(request UserSpeciesImportRequest) ([]UserSpeciesRecord, string, error) {
	rawContent := request.Content
	format := strings.ToLower(strings.TrimSpace(request.Format))
	if strings.TrimSpace(request.SourcePath) != "" {
		abs, err := filepath.Abs(filepath.Clean(request.SourcePath))
		if err != nil {
			return nil, "", err
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return nil, "", fmt.Errorf("read user species file: %w", err)
		}
		if !utf8.Valid(data) {
			return nil, "", errors.New("user species input file contains invalid UTF-8")
		}
		rawContent = string(data)
		if format == "" {
			if strings.EqualFold(filepath.Ext(abs), ".csv") {
				format = "csv"
			} else {
				format = "json"
			}
		}
	}
	if !utf8.ValidString(rawContent) {
		return nil, "", errors.New("user species input contains invalid UTF-8")
	}
	trimmed := strings.TrimSpace(rawContent)
	if trimmed == "" {
		return nil, "", errors.New("user species input is empty")
	}
	if format == "" {
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			format = "json"
		} else {
			format = "csv"
		}
	}

	var parsed []UserSpeciesRecord
	switch format {
	case "json":
		if strings.HasPrefix(trimmed, "[") {
			if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
				return nil, "", fmt.Errorf("parse user species JSON array: %w", err)
			}
		} else {
			var manifest UserSpeciesManifest
			if err := json.Unmarshal([]byte(trimmed), &manifest); err != nil {
				return nil, "", fmt.Errorf("parse user species JSON manifest: %w", err)
			}
			parsed = manifest.Species
			if manifest.PayloadSHA256 != "" {
				normalized, err := validateAndNormalizeUserSpeciesRecords(parsed)
				if err != nil {
					return nil, "", err
				}
				computed, err := computeUserSpeciesPayloadHash(normalized)
				if err != nil {
					return nil, "", err
				}
				if !strings.EqualFold(computed, manifest.PayloadSHA256) {
					return nil, "", fmt.Errorf("user species manifest SHA-256 mismatch (expected %s, computed %s)", manifest.PayloadSHA256, computed)
				}
				return normalized, computed, nil
			}
		}
	case "csv":
		r := csv.NewReader(strings.NewReader(rawContent))
		header, err := r.Read()
		if err != nil {
			return nil, "", fmt.Errorf("read user species CSV header: %w", err)
		}
		colMap := map[string]int{}
		for idx, h := range header {
			colMap[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "\ufeff")))] = idx
		}
		codeIdx, ok := colMap["code"]
		if !ok {
			return nil, "", errors.New("user species CSV header must include a Code column")
		}
		for {
			row, err := r.Read()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, "", fmt.Errorf("read user species CSV row: %w", err)
			}
			getCell := func(name string) string {
				i, exists := colMap[name]
				if !exists || i >= len(row) {
					return ""
				}
				return row[i]
			}
			if codeIdx >= len(row) || strings.TrimSpace(row[codeIdx]) == "" {
				continue
			}
			rec := UserSpeciesRecord{Code: row[codeIdx]}
			if v := strings.TrimSpace(getCell("scientificname")); v != "" {
				rec.ScientificName = &v
			}
			if v := strings.TrimSpace(getCell("englishname")); v != "" {
				rec.EnglishName = &v
			}
			if v := strings.TrimSpace(getCell("lifeform")); v != "" {
				lf, pErr := strconv.Atoi(v)
				if pErr != nil {
					return nil, "", fmt.Errorf("invalid LifeForm integer %q for species %s", v, rec.Code)
				}
				rec.LifeForm = &lf
			}
			if v := strings.TrimSpace(getCell("report")); v != "" {
				rep, pErr := strconv.ParseFloat(v, 64)
				if pErr != nil {
					return nil, "", fmt.Errorf("invalid Report float %q for species %s", v, rec.Code)
				}
				rec.Report = &rep
			}
			if v := strings.TrimSpace(getCell("sppnumber")); v != "" {
				sn, pErr := strconv.Atoi(v)
				if pErr != nil {
					return nil, "", fmt.Errorf("invalid SppNumber integer %q for species %s", v, rec.Code)
				}
				rec.SppNumber = &sn
			}
			if v := strings.TrimSpace(getCell("codetype")); v != "" {
				rec.Codetype = &v
			}
			parsed = append(parsed, rec)
		}
	default:
		return nil, "", fmt.Errorf("unsupported user species import format %q", format)
	}

	normalized, err := validateAndNormalizeUserSpeciesRecords(parsed)
	if err != nil {
		return nil, "", err
	}
	payloadSHA, err := computeUserSpeciesPayloadHash(normalized)
	if err != nil {
		return nil, "", err
	}
	return normalized, payloadSHA, nil
}

func validateAndNormalizeUserSpeciesRecords(records []UserSpeciesRecord) ([]UserSpeciesRecord, error) {
	seen := map[string]bool{}
	out := make([]UserSpeciesRecord, 0, len(records))
	for _, r := range records {
		code := strings.ToUpper(strings.TrimSpace(r.Code))
		if code == "" || !utf8.ValidString(code) || strings.ContainsRune(code, 0) {
			return nil, errors.New("every user species entry must have a non-empty valid UTF-8 Code")
		}
		if len(utf16.Encode([]rune(code))) > 8 {
			return nil, fmt.Errorf("user species code %q exceeds 8 UTF-16 code units", code)
		}
		if seen[code] {
			return nil, fmt.Errorf("duplicate species code %q in import payload", code)
		}
		seen[code] = true

		// USysUserSpp.LifeForm is INT16 (signed 16-bit INTEGER)
		if r.LifeForm != nil && (*r.LifeForm < -32768 || *r.LifeForm > 32767) {
			return nil, fmt.Errorf("user species %s LifeForm %d exceeds signed 16-bit INTEGER bounds", code, *r.LifeForm)
		}
		for label, ptr := range map[string]*string{"ScientificName": r.ScientificName, "EnglishName": r.EnglishName, "Codetype": r.Codetype} {
			if ptr != nil {
				if !utf8.ValidString(*ptr) || strings.ContainsRune(*ptr, 0) || len(utf16.Encode([]rune(*ptr))) > 255 {
					return nil, fmt.Errorf("user species %s %s must be valid UTF-8 of at most 255 UTF-16 units", code, label)
				}
			}
		}
		r.Code = code
		out = append(out, r)
	}
	return out, nil
}

func normalizeCollisionPolicy(policy UserSpeciesCollisionPolicy) (UserSpeciesCollisionPolicy, error) {
	p := strings.ToLower(strings.TrimSpace(string(policy)))
	if p == "" || p == "skip" {
		return CollisionSkip, nil
	}
	if p == "overwrite" {
		return CollisionOverwrite, nil
	}
	return "", fmt.Errorf("invalid collision policy %q (expected skip or overwrite)", policy)
}

func buildUserSpeciesPreview(ctx context.Context, tx *sql.Tx, records []UserSpeciesRecord, payloadSHA string, policy UserSpeciesCollisionPolicy) (UserSpeciesImportPreview, error) {
	masterCodes := map[string]string{}
	mRows, err := tx.QueryContext(ctx, `SELECT Code, OldCode, COALESCE(ScientificName,'') FROM VLists.USysAllSpecs`)
	if err != nil {
		return UserSpeciesImportPreview{}, err
	}
	for mRows.Next() {
		var code, oldCode sql.NullString
		var sci string
		if err := mRows.Scan(&code, &oldCode, &sci); err != nil {
			mRows.Close()
			return UserSpeciesImportPreview{}, err
		}
		if code.Valid && strings.TrimSpace(code.String) != "" {
			masterCodes[strings.ToUpper(strings.TrimSpace(code.String))] = sci
		}
		if oldCode.Valid && strings.TrimSpace(oldCode.String) != "" && code.Valid && strings.TrimSpace(code.String) != "" {
			u := strings.ToUpper(strings.TrimSpace(oldCode.String))
			if _, exists := masterCodes[u]; !exists {
				masterCodes[u] = "synonym of " + strings.ToUpper(strings.TrimSpace(code.String))
			}
		}
	}
	if err := mRows.Close(); err != nil {
		return UserSpeciesImportPreview{}, err
	}

	userCodes := map[string]string{}
	uRows, err := tx.QueryContext(ctx, `SELECT Code, COALESCE(ScientificName,'') FROM VUser.USysUserSpp`)
	if err != nil {
		return UserSpeciesImportPreview{}, err
	}
	for uRows.Next() {
		var code, sci string
		if err := uRows.Scan(&code, &sci); err != nil {
			uRows.Close()
			return UserSpeciesImportPreview{}, err
		}
		userCodes[strings.ToUpper(strings.TrimSpace(code))] = sci
	}
	if err := uRows.Close(); err != nil {
		return UserSpeciesImportPreview{}, err
	}

	preview := UserSpeciesImportPreview{
		TotalRows:     len(records),
		PayloadSHA256: payloadSHA,
		Items:         make([]UserSpeciesImportItemReview, 0, len(records)),
	}
	for _, r := range records {
		item := UserSpeciesImportItemReview{
			Code:           r.Code,
			ScientificName: optString(r.ScientificName),
			LifeForm:       r.LifeForm,
		}
		if mSci, inMaster := masterCodes[r.Code]; inMaster {
			item.Status = "collision_master"
			item.Action = "skip"
			item.Reason = fmt.Sprintf("Code already exists in provincial VLists.USysAllSpecs (%s)", mSci)
			preview.MasterCollisions++
		} else if uSci, inUser := userCodes[r.Code]; inUser {
			item.Status = "collision_user"
			preview.UserCollisions++
			if policy == CollisionOverwrite {
				item.Action = "overwrite"
				item.Reason = fmt.Sprintf("Overwrites existing VUser.USysUserSpp definition (%s)", uSci)
			} else {
				item.Action = "skip"
				item.Reason = fmt.Sprintf("Existing VUser.USysUserSpp definition (%s) preserved", uSci)
			}
		} else {
			item.Status = "new"
			item.Action = "insert"
			preview.NewCount++
		}
		preview.Items = append(preview.Items, item)
	}
	return preview, nil
}

func PreviewUserSpeciesImport(s *vprocore.ContextService, ctx context.Context, contextID string, request UserSpeciesImportRequest) (UserSpeciesImportPreview, error) {
	policy, err := normalizeCollisionPolicy(request.CollisionPolicy)
	if err != nil {
		return UserSpeciesImportPreview{}, err
	}
	records, payloadSHA, err := parseUserSpeciesInput(request)
	if err != nil {
		return UserSpeciesImportPreview{}, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (UserSpeciesImportPreview, error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return UserSpeciesImportPreview{}, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return UserSpeciesImportPreview{}, err
		}
		defer func() { _ = tx.Rollback() }()
		preview, err := buildUserSpeciesPreview(ctx, tx, records, payloadSHA, policy)
		if err != nil {
			return UserSpeciesImportPreview{}, err
		}
		if err := tx.Commit(); err != nil {
			return UserSpeciesImportPreview{}, err
		}
		return preview, nil
	})
}

func validateVUserOwnership(c *vprocore.SqliteContext) error {
	userInfo := c.AttachmentInfo["VUser"]
	if userInfo == nil || c.AttachmentInfo["VLists"] == nil {
		return errors.New("user species import requires identified VUser and VLists attachments")
	}
	for role, info := range c.AttachmentInfo {
		current, err := os.Stat(c.Attachments[role])
		if err != nil {
			return fmt.Errorf("verify %s attachment: %w", role, err)
		}
		if !os.SameFile(current, info) {
			return fmt.Errorf("%s database identity changed; reload context", role)
		}
		if role != "VUser" && os.SameFile(userInfo, info) {
			return fmt.Errorf("VUser database shares physical file with %s; refusing mutation", role)
		}
	}
	return nil
}

func ImportUserSpeciesList(s *vprocore.ContextService, ctx context.Context, contextID string, request UserSpeciesImportRequest) (UserSpeciesImportResult, error) {
	policy, err := normalizeCollisionPolicy(request.CollisionPolicy)
	if err != nil {
		return UserSpeciesImportResult{}, err
	}
	records, payloadSHA, err := parseUserSpeciesInput(request)
	if err != nil {
		return UserSpeciesImportResult{}, err
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (UserSpeciesImportResult, error) {
		owner := plots.Projects.Sqlite
		project := owner.Selection.Project
		user := plots.CurrentUser
		if strings.TrimSpace(user) == "" {
			user = "VPro"
		}

		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return UserSpeciesImportResult{}, err
		}
		defer owner.Mu.Unlock()
		if err := validateVUserOwnership(owner); err != nil {
			return UserSpeciesImportResult{}, err
		}

		readTx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return UserSpeciesImportResult{}, err
		}
		preview, pErr := buildUserSpeciesPreview(ctx, readTx, records, payloadSHA, policy)
		if pErr != nil {
			_ = readTx.Rollback()
			return UserSpeciesImportResult{}, pErr
		}
		if err := readTx.Commit(); err != nil {
			return UserSpeciesImportResult{}, err
		}

		db, err := sql.Open("sqlite3", sqlutil.SqliteFileURI(owner.Attachments["VUser"], "rw")+"&_foreign_keys=on&_busy_timeout=5000&_txlock=immediate")
		if err != nil {
			return UserSpeciesImportResult{}, err
		}
		defer db.Close()
		db.SetMaxOpenConns(1)
		conn, err := db.Conn(ctx)
		if err != nil {
			return UserSpeciesImportResult{}, err
		}
		defer conn.Close()

		if _, err := conn.ExecContext(ctx, "BEGIN IMMEDIATE"); err != nil {
			return UserSpeciesImportResult{}, err
		}
		committed := false
		defer func() {
			if !committed {
				_, _ = conn.ExecContext(context.WithoutCancel(ctx), "ROLLBACK")
			}
		}()

		recByCode := make(map[string]UserSpeciesRecord, len(records))
		for _, r := range records {
			recByCode[r.Code] = r
		}

		var inserted, overwritten, skipped int
		when := time.Now().Format("2006-01-02 15:04:05")

		for _, item := range preview.Items {
			rec := recByCode[item.Code]
			rep := 1.0
			if rec.Report != nil {
				rep = *rec.Report
			}
			ctype := rec.Codetype
			if ctype == nil {
				defaultU := "U"
				ctype = &defaultU
			}

			switch item.Action {
			case "insert":
				if _, err := conn.ExecContext(ctx, `
					INSERT INTO main.USysUserSpp (Code, ScientificName, LifeForm, EnglishName, Report, SppNumber, Codetype)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					rec.Code, rec.ScientificName, rec.LifeForm, rec.EnglishName, rep, rec.SppNumber, ctype); err != nil {
					return UserSpeciesImportResult{}, fmt.Errorf("insert user species %s: %w", rec.Code, err)
				}
				afterJSON, _ := json.Marshal(rec)
				_, _ = conn.ExecContext(ctx, `
					INSERT INTO main.USysAuditTrail ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit")
					VALUES (?, ?, NULL, 'USysUserSpp', 'ImportInsert', ?, NULL, ?)`,
					project, user, when, string(afterJSON))
				inserted++

			case "overwrite":
				if _, err := conn.ExecContext(ctx, `
					UPDATE main.USysUserSpp
					SET ScientificName=?, LifeForm=?, EnglishName=?, Report=?, SppNumber=?, Codetype=?
					WHERE Code COLLATE NOCASE=?`,
					rec.ScientificName, rec.LifeForm, rec.EnglishName, rep, rec.SppNumber, ctype, rec.Code); err != nil {
					return UserSpeciesImportResult{}, fmt.Errorf("overwrite user species %s: %w", rec.Code, err)
				}
				afterJSON, _ := json.Marshal(rec)
				_, _ = conn.ExecContext(ctx, `
					INSERT INTO main.USysAuditTrail ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit")
					VALUES (?, ?, NULL, 'USysUserSpp', 'ImportOverwrite', ?, ?, ?)`,
					project, user, when, item.Reason, string(afterJSON))
				overwritten++

			default:
				skipped++
			}
		}

		if err := ctx.Err(); err != nil {
			return UserSpeciesImportResult{}, err
		}
		if _, err := conn.ExecContext(ctx, "COMMIT"); err != nil {
			return UserSpeciesImportResult{}, err
		}
		committed = true

		return UserSpeciesImportResult{
			Preview:          preview,
			InsertedCount:    inserted,
			OverwrittenCount: overwritten,
			SkippedCount:     skipped,
		}, nil
	})
}
