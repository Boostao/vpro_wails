package sivi

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const SiviCreationPlotHistoryTable = "__VPRO_SIVICreationPlotHistory"

const siviCreationPlotHistoryDDL = `CREATE TABLE IF NOT EXISTS "__VPRO_SIVICreationPlotHistory" (
	"RequestID" TEXT NOT NULL PRIMARY KEY,
	"PlotNumber" TEXT NOT NULL,
	"PlotType" TEXT NOT NULL,
	"Created" TEXT NOT NULL,
	"Actor" TEXT NOT NULL,
	"Undone" INTEGER NOT NULL DEFAULT 0,
	"UndoneAt" TEXT,
	"UndoRequestID" TEXT,
	"Proposal" TEXT NOT NULL
)`

type CreateSIVIPlotRequest struct {
	RequestID       string `json:"requestId,omitempty"`
	PlotNumber      string `json:"plotNumber"`
	PlotType        string `json:"plotType"` // "Ground" | "Visual" | "Note" | "Other"
	ProjectID       string `json:"projectId,omitempty"`
	FieldNumber     string `json:"fieldNumber,omitempty"`
	Date            string `json:"date,omitempty"`
	SiteSurveyor    string `json:"siteSurveyor,omitempty"`
	SVPolygonNumber string `json:"svPolygonNumber,omitempty"`
	Zone            string `json:"zone,omitempty"`
	SubZone         string `json:"subZone,omitempty"`
	SiteSeries      string `json:"siteSeries,omitempty"`
	UserSiteUnit    string `json:"userSiteUnit,omitempty"`
	AddToActiveSU   bool   `json:"addToActiveSu,omitempty"`
}

type CreateSIVIPlotResult struct {
	RequestID     string `json:"requestId"`
	ContextID     string `json:"contextId"`
	Project       string `json:"project"`
	PlotNumber    string `json:"plotNumber"`
	PlotType      string `json:"plotType"`
	AddedToSU     bool   `json:"addedToSu"`
	AuditRowCount int    `json:"auditRowCount"`
	CreatedAt     string `json:"createdAt"`
	Actor         string `json:"actor"`
	Replayed      bool   `json:"replayed"`
}

type UndoCreateSIVIPlotRequest struct {
	RequestID         string `json:"requestId,omitempty"`
	CreationRequestID string `json:"creationRequestId"`
	PlotNumber        string `json:"plotNumber"`
}

type UndoCreateSIVIPlotResult struct {
	RequestID         string `json:"requestId"`
	CreationRequestID string `json:"creationRequestId"`
	ContextID         string `json:"contextId"`
	Project           string `json:"project"`
	PlotNumber        string `json:"plotNumber"`
	RemovedFromSU     bool   `json:"removedFromSu"`
	UndoneAt          string `json:"undoneAt"`
	Actor             string `json:"actor"`
	Replayed          bool   `json:"replayed"`
}

type SIVICreatedPlotEntry struct {
	RequestID       string `json:"requestId"`
	PlotNumber      string `json:"plotNumber"`
	PlotType        string `json:"plotType"`
	ProjectID       string `json:"projectId,omitempty"`
	SVPolygonNumber string `json:"svPolygonNumber,omitempty"`
	CreatedAt       string `json:"createdAt"`
	Actor           string `json:"actor"`
	Undone          bool   `json:"undone"`
	UndoneAt        string `json:"undoneAt,omitempty"`
	CanUndo         bool   `json:"canUndo"`
}

type siviPlotCreationProposal struct {
	Request   CreateSIVIPlotRequest `json:"request"`
	Project   string                `json:"project"`
	Actor     string                `json:"actor"`
	CreatedAt string                `json:"createdAt"`
	AddedToSU bool                  `json:"addedToSu"`
	SUTable   string                `json:"suTable,omitempty"`
}

func newSIVIPlotUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func normalizeSIVIPlotType(raw string) (string, error) {
	switch strings.TrimSpace(raw) {
	case "Ground":
		return "Ground", nil
	case "Visual":
		return "Visual", nil
	case "Note":
		return "Note", nil
	case "Other":
		return "Other", nil
	default:
		return "", fmt.Errorf("invalid SIVI PlotType %q (expected Ground, Visual, Note, or Other)", raw)
	}
}

func validateSIVIUTF16(label, val string, maxUnits int) error {
	if !utf8.ValidString(val) || strings.ContainsRune(val, 0) {
		return fmt.Errorf("%s contains malformed UTF-8 or NUL bytes", label)
	}
	if maxUnits > 0 && len(utf16.Encode([]rune(val))) > maxUnits {
		return fmt.Errorf("%s exceeds %d UTF-16 code units", label, maxUnits)
	}
	return nil
}

func parseYearFromDate(raw string) *int64 {
	trimmed := strings.TrimSpace(raw)
	if len(trimmed) >= 4 {
		if yr, err := strconv.ParseInt(trimmed[:4], 10, 64); err == nil && yr >= 1900 && yr <= 2500 {
			return &yr
		}
	}
	return nil
}

// CreateSIVIPlot creates a new FS1333 SIVI parent plot atomically across <Project>_Env,
// <Project>_Admin, __VPRO_SIVICreationPlotHistory, and <Project>_Audit inside a single
// BEGIN IMMEDIATE transaction.
func CreateSIVIPlot(s *vprocore.ContextService, ctx context.Context, contextID string, req CreateSIVIPlotRequest) (CreateSIVIPlotResult, error) {
	if s == nil {
		return CreateSIVIPlotResult{}, errors.New("SIVI plot context service is unavailable")
	}
	if req.PlotNumber == "" || strings.TrimSpace(req.PlotNumber) == "" {
		return CreateSIVIPlotResult{}, errors.New("PlotNumber is required and must not be blank")
	}
	if err := validateSIVIUTF16("PlotNumber", req.PlotNumber, 7); err != nil {
		return CreateSIVIPlotResult{}, err
	}
	plotType, err := normalizeSIVIPlotType(req.PlotType)
	if err != nil {
		return CreateSIVIPlotResult{}, err
	}
	for _, fld := range []struct {
		name string
		val  string
		max  int
	}{
		{"ProjectID", req.ProjectID, 30},
		{"FieldNumber", req.FieldNumber, 50},
		{"Date", req.Date, 32},
		{"SiteSurveyor", req.SiteSurveyor, 30},
		{"SV_PolygonNumber", req.SVPolygonNumber, 25},
		{"Zone", req.Zone, 4},
		{"SubZone", req.SubZone, 8},
		{"SiteSeries", req.SiteSeries, 5},
		{"UserSiteUnit", req.UserSiteUnit, 100},
	} {
		if err := validateSIVIUTF16(fld.name, fld.val, fld.max); err != nil {
			return CreateSIVIPlotResult{}, err
		}
	}

	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID, err = newSIVIPlotUUIDv4()
		if err != nil {
			return CreateSIVIPlotResult{}, err
		}
	} else if !vprocore.SiviDeletionUUID.MatchString(requestID) {
		return CreateSIVIPlotResult{}, errors.New("requestId must be a lowercase UUIDv4")
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (CreateSIVIPlotResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return CreateSIVIPlotResult{}, err
		}
		owner := plots.Projects.Sqlite
		actor := strings.TrimSpace(plots.CurrentUser)
		if actor == "" {
			actor = "VProUser"
		}

		var out CreateSIVIPlotResult
		wErr := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			project := owner.Selection.Project
			envTbl := vprocore.QuoteHeaderIdentifier(project + "_Env")
			adminTbl := vprocore.QuoteHeaderIdentifier(project + "_Admin")
			auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")

			if _, err := tx.ExecContext(ctx, siviCreationPlotHistoryDDL); err != nil {
				return err
			}

			// Idempotent replay check.
			var existingPlot, existingType, existingCreated, existingActor, existingProposal string
			var existingUndone int64
			rErr := tx.QueryRowContext(ctx, `SELECT "PlotNumber","PlotType","Created","Actor","Undone","Proposal"
				FROM "__VPRO_SIVICreationPlotHistory" WHERE "RequestID" = ?`, requestID).
				Scan(&existingPlot, &existingType, &existingCreated, &existingActor, &existingUndone, &existingProposal)
			if rErr == nil {
				if existingPlot != req.PlotNumber || existingType != plotType {
					return errors.New("requestId already belongs to a different SIVI plot creation")
				}
				var prop siviPlotCreationProposal
				_ = json.Unmarshal([]byte(existingProposal), &prop)
				out = CreateSIVIPlotResult{
					RequestID:     requestID,
					ContextID:     contextID,
					Project:       project,
					PlotNumber:    existingPlot,
					PlotType:      existingType,
					AddedToSU:     prop.AddedToSU,
					AuditRowCount: 0,
					CreatedAt:     existingCreated,
					Actor:         existingActor,
					Replayed:      true,
				}
				return nil
			} else if !errors.Is(rErr, sql.ErrNoRows) {
				return rErr
			}

			// Guard duplicate plot numbers (both SQLite PK equality and literal BLOB check).
			var envExists, adminExists int
			if err := tx.QueryRowContext(ctx, `SELECT
				(SELECT COUNT(*) FROM `+envTbl+` WHERE "PlotNumber" = ? OR (typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB))),
				(SELECT COUNT(*) FROM `+adminTbl+` WHERE "Plot" = ? OR (typeof("Plot")='text' AND CAST("Plot" AS BLOB)=CAST(? AS BLOB)))`,
				req.PlotNumber, req.PlotNumber, req.PlotNumber, req.PlotNumber).Scan(&envExists, &adminExists); err != nil {
				return err
			}
			if envExists > 0 || adminExists > 0 {
				return fmt.Errorf("plot %q already exists in project %s", req.PlotNumber, project)
			}

			now := time.Now().Format("2006-01-02 15:04:05")

			// Build _Env insert (Access BOOLEAN false = 0).
			envCols := []string{`"PlotNumber"`, `"SpeciesListComplete"`, `"SV_FloodPlain"`, `"Flag"`}
			envArgs := []any{req.PlotNumber, int64(0), int64(0), int64(0)}
			envAudits := [][2]string{{"PlotNumber", req.PlotNumber}}

			effectivePID := strings.TrimSpace(req.ProjectID)
			if effectivePID == "" && len(utf16.Encode([]rune(project))) <= 30 {
				effectivePID = project
			}
			for _, opt := range []struct {
				col string
				val string
			}{
				{"ProjectID", effectivePID},
				{"FieldNumber", req.FieldNumber},
				{"Date", req.Date},
				{"SiteSurveyor", req.SiteSurveyor},
				{"SV_PolygonNumber", req.SVPolygonNumber},
				{"Zone", req.Zone},
				{"SubZone", req.SubZone},
				{"SiteSeries", req.SiteSeries},
			} {
				if opt.val != "" {
					envCols = append(envCols, vprocore.QuoteHeaderIdentifier(opt.col))
					envArgs = append(envArgs, opt.val)
					envAudits = append(envAudits, [2]string{opt.col, opt.val})
				}
			}

			envMarks := make([]string, len(envCols))
			for i := range envMarks {
				envMarks[i] = "?"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+envTbl+` (`+strings.Join(envCols, ", ")+`) VALUES (`+strings.Join(envMarks, ", ")+`)`, envArgs...); err != nil {
				return fmt.Errorf("insert SIVI _Env row: %w", err)
			}

			// Build _Admin insert (Access BOOLEAN UpdatedFromCards = 0).
			adminCols := []string{`"Plot"`, `"PlotType"`, `"UpdatedFromCards"`, `"EnteredBy"`}
			adminArgs := []any{req.PlotNumber, plotType, int64(0), actor}
			adminAudits := [][2]string{
				{"Plot", req.PlotNumber},
				{"PlotType", plotType},
				{"EnteredBy", actor},
			}
			if startYr := parseYearFromDate(req.Date); startYr != nil {
				adminCols = append(adminCols, `"StartDate"`)
				adminArgs = append(adminArgs, *startYr)
				adminAudits = append(adminAudits, [2]string{"StartDate", strconv.FormatInt(*startYr, 10)})
			}
			if req.UserSiteUnit != "" {
				adminCols = append(adminCols, `"UserSiteUnit"`)
				adminArgs = append(adminArgs, req.UserSiteUnit)
				adminAudits = append(adminAudits, [2]string{"UserSiteUnit", req.UserSiteUnit})
			}
			adminMarks := make([]string, len(adminCols))
			for i := range adminMarks {
				adminMarks[i] = "?"
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+adminTbl+` (`+strings.Join(adminCols, ", ")+`) VALUES (`+strings.Join(adminMarks, ", ")+`)`, adminArgs...); err != nil {
				return fmt.Errorf("insert SIVI _Admin row: %w", err)
			}

			addedSU := false
			suTableName := ""
			if req.AddToActiveSU && owner.Selection.SU != "None" &&
				vprocore.SameDesktopPath(owner.Selection.SUPath, owner.Selection.ProjectPath) {
				suTableName = owner.Selection.SU + "_SU"
				quotedSU := vprocore.QuoteHeaderIdentifier(suTableName)
				var unitVal any
				if req.UserSiteUnit != "" {
					unitVal = req.UserSiteUnit
				}
				if _, err := tx.ExecContext(ctx, `INSERT INTO `+quotedSU+` ("PlotNumber", "SiteUnit") VALUES (?, ?)`, req.PlotNumber, unitVal); err != nil {
					return fmt.Errorf("insert SIVI plot into active SU %s: %w", suTableName, err)
				}
				addedSU = true
			}

			// Record <Project>_Audit rows (Restore = 0, Flag = 0).
			auditStmt, err := tx.PrepareContext(ctx, `INSERT INTO `+auditTbl+
				` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,NULL,?,0,0,NULL)`)
			if err != nil {
				return err
			}
			defer auditStmt.Close()

			auditCount := 0
			for _, ea := range envAudits {
				if _, err := auditStmt.ExecContext(ctx, project, actor, req.PlotNumber, "_Env", ea[0], now, ea[1]); err != nil {
					return err
				}
				auditCount++
			}
			for _, aa := range adminAudits {
				if _, err := auditStmt.ExecContext(ctx, project, actor, req.PlotNumber, "_Admin", aa[0], now, aa[1]); err != nil {
					return err
				}
				auditCount++
			}

			reqCopy := req
			reqCopy.RequestID = requestID
			reqCopy.PlotType = plotType
			reqCopy.ProjectID = effectivePID
			propBytes, err := json.Marshal(siviPlotCreationProposal{
				Request:   reqCopy,
				Project:   project,
				Actor:     actor,
				CreatedAt: now,
				AddedToSU: addedSU,
				SUTable:   suTableName,
			})
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_SIVICreationPlotHistory"
				("RequestID","PlotNumber","PlotType","Created","Actor","Undone","Proposal")
				VALUES (?,?,?,?,?,0,?)`,
				requestID, req.PlotNumber, plotType, now, actor, string(propBytes)); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			out = CreateSIVIPlotResult{
				RequestID:     requestID,
				ContextID:     contextID,
				Project:       project,
				PlotNumber:    req.PlotNumber,
				PlotType:      plotType,
				AddedToSU:     addedSU,
				AuditRowCount: auditCount,
				CreatedAt:     now,
				Actor:         actor,
				Replayed:      false,
			}
			return nil
		})
		if wErr != nil {
			return CreateSIVIPlotResult{}, wErr
		}
		return out, nil
	})
}

// UndoCreateSIVIPlot atomically reverses a SIVI parent plot creation recorded in
// __VPRO_SIVICreationPlotHistory (provided no child records have been added to the plot),
// removing the _Admin and _Env rows, marking Undone = -1 in __VPRO_SIVICreationPlotHistory,
// and writing a permanent undo receipt to <Project>_Audit.
func UndoCreateSIVIPlot(s *vprocore.ContextService, ctx context.Context, contextID string, req UndoCreateSIVIPlotRequest) (UndoCreateSIVIPlotResult, error) {
	if s == nil {
		return UndoCreateSIVIPlotResult{}, errors.New("SIVI plot context service is unavailable")
	}
	creationReqID := strings.TrimSpace(req.CreationRequestID)
	if !vprocore.SiviDeletionUUID.MatchString(creationReqID) {
		return UndoCreateSIVIPlotResult{}, errors.New("creationRequestId must be a lowercase UUIDv4")
	}
	if req.PlotNumber == "" || !utf8.ValidString(req.PlotNumber) || strings.ContainsRune(req.PlotNumber, 0) {
		return UndoCreateSIVIPlotResult{}, errors.New("plotNumber is required")
	}
	undoReqID := strings.TrimSpace(req.RequestID)
	var err error
	if undoReqID == "" {
		undoReqID, err = newSIVIPlotUUIDv4()
		if err != nil {
			return UndoCreateSIVIPlotResult{}, err
		}
	} else if !vprocore.SiviDeletionUUID.MatchString(undoReqID) {
		return UndoCreateSIVIPlotResult{}, errors.New("requestId must be a lowercase UUIDv4")
	}
	if undoReqID == creationReqID {
		return UndoCreateSIVIPlotResult{}, errors.New("undo requestId must differ from creationRequestId")
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (UndoCreateSIVIPlotResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return UndoCreateSIVIPlotResult{}, err
		}
		owner := plots.Projects.Sqlite
		actor := strings.TrimSpace(plots.CurrentUser)
		if actor == "" {
			actor = "VProUser"
		}

		var out UndoCreateSIVIPlotResult
		wErr := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			project := owner.Selection.Project
			var hasHistory bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, SiviCreationPlotHistoryTable).Scan(&hasHistory); err != nil {
				return err
			}
			if !hasHistory {
				return errors.New("no SIVI plot creation history exists in this project")
			}

			var storedPlot, storedProposal string
			var undone int64
			var undoneAt, prevUndoID sql.NullString
			if err := tx.QueryRowContext(ctx, `SELECT "PlotNumber","Undone","UndoneAt","UndoRequestID","Proposal"
				FROM "__VPRO_SIVICreationPlotHistory" WHERE "RequestID" = ?`, creationReqID).
				Scan(&storedPlot, &undone, &undoneAt, &prevUndoID, &storedProposal); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("SIVI plot creation receipt %s not found", creationReqID)
				}
				return err
			}
			if storedPlot != req.PlotNumber {
				return fmt.Errorf("SIVI plot creation receipt %s belongs to plot %q, not %q", creationReqID, storedPlot, req.PlotNumber)
			}

			var prop siviPlotCreationProposal
			_ = json.Unmarshal([]byte(storedProposal), &prop)

			if undone != 0 {
				if prevUndoID.Valid && prevUndoID.String == undoReqID {
					out = UndoCreateSIVIPlotResult{
						RequestID:         undoReqID,
						CreationRequestID: creationReqID,
						ContextID:         contextID,
						Project:           project,
						PlotNumber:        storedPlot,
						RemovedFromSU:     prop.AddedToSU,
						UndoneAt:          undoneAt.String,
						Actor:             actor,
						Replayed:          true,
					}
					return nil
				}
				return fmt.Errorf("SIVI plot creation %s for plot %q was already undone", creationReqID, storedPlot)
			}

			// Guard: refuse to undo plot creation if any child observations exist on this plot.
			for _, suffix := range []string{"Veg", "Humus", "Mineral", "Other"} {
				childTbl := vprocore.QuoteHeaderIdentifier(project + "_" + suffix)
				var cnt int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+childTbl+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`, storedPlot).Scan(&cnt); err != nil {
					return err
				}
				if cnt > 0 {
					return fmt.Errorf("cannot undo creation of SIVI plot %q because %d %s record(s) exist on this plot", storedPlot, cnt, suffix)
				}
			}

			envTbl := vprocore.QuoteHeaderIdentifier(project + "_Env")
			adminTbl := vprocore.QuoteHeaderIdentifier(project + "_Admin")
			auditTbl := vprocore.QuoteHeaderIdentifier(project + "_Audit")

			removedSU := false
			if prop.AddedToSU && prop.SUTable != "" {
				var suExists bool
				_ = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, prop.SUTable).Scan(&suExists)
				if suExists {
					if _, err := tx.ExecContext(ctx, `DELETE FROM `+vprocore.QuoteHeaderIdentifier(prop.SUTable)+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`, storedPlot); err != nil {
						return err
					}
					removedSU = true
				}
			}

			// Delete plot-scoped audit rows before deleting _Admin and _Env, then append permanent NULL-plot undo receipt.
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+auditTbl+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`, storedPlot); err != nil {
				return err
			}
			aRes, err := tx.ExecContext(ctx, `DELETE FROM `+adminTbl+` WHERE typeof("Plot")='text' AND CAST("Plot" AS BLOB)=CAST(? AS BLOB)`, storedPlot)
			if err != nil {
				return err
			}
			if n, _ := aRes.RowsAffected(); n != 1 {
				return fmt.Errorf("undo SIVI plot %q expected 1 _Admin row, found %d", storedPlot, n)
			}
			eRes, err := tx.ExecContext(ctx, `DELETE FROM `+envTbl+` WHERE typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB)`, storedPlot)
			if err != nil {
				return err
			}
			if n, _ := eRes.RowsAffected(); n != 1 {
				return fmt.Errorf("undo SIVI plot %q expected 1 _Env row, found %d", storedPlot, n)
			}

			now := time.Now().Format("2006-01-02 15:04:05")
			// Access BOOLEAN true = -1!
			if _, err := tx.ExecContext(ctx, `UPDATE "__VPRO_SIVICreationPlotHistory"
				SET "Undone" = -1, "UndoneAt" = ?, "UndoRequestID" = ?
				WHERE "RequestID" = ?`, now, undoReqID, creationReqID); err != nil {
				return err
			}

			undoReceiptJSON, _ := json.Marshal(map[string]any{
				"undoRequestId":     undoReqID,
				"creationRequestId": creationReqID,
				"plotNumber":        storedPlot,
				"plotType":          prop.Request.PlotType,
			})
			// PlotNumber is NULL so <Project>_Audit foreign key to <Project>_Env(PlotNumber) is satisfied after deletion.
			if _, err := tx.ExecContext(ctx, `INSERT INTO `+auditTbl+
				` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,NULL,'_Env','UndoCreateSIVIPlot',?,?,?,0,0,NULL)`,
				project, actor, now, storedPlot, string(undoReceiptJSON)); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			out = UndoCreateSIVIPlotResult{
				RequestID:         undoReqID,
				CreationRequestID: creationReqID,
				ContextID:         contextID,
				Project:           project,
				PlotNumber:        storedPlot,
				RemovedFromSU:     removedSU,
				UndoneAt:          now,
				Actor:             actor,
				Replayed:          false,
			}
			return nil
		})
		if wErr != nil {
			return UndoCreateSIVIPlotResult{}, wErr
		}
		return out, nil
	})
}

// ListCreatedSIVIPlots returns SIVI parent plots created via CreateSIVIPlot in the active project,
// ordered newest-first, indicating whether each plot can still be cleanly undone.
func ListCreatedSIVIPlots(s *vprocore.ContextService, ctx context.Context, contextID string) ([]SIVICreatedPlotEntry, error) {
	if s == nil {
		return nil, errors.New("SIVI plot context service is unavailable")
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (entries []SIVICreatedPlotEntry, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		tx, err := owner.BeginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, rb)
			}
		}()

		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project.sqlite_master WHERE type='table' AND name=?)`, SiviCreationPlotHistoryTable).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return []SIVICreatedPlotEntry{}, nil
		}

		project := owner.Selection.Project
		envTbl := `project.` + vprocore.QuoteHeaderIdentifier(project+"_Env")
		vegTbl := `project.` + vprocore.QuoteHeaderIdentifier(project+"_Veg")
		humusTbl := `project.` + vprocore.QuoteHeaderIdentifier(project+"_Humus")
		minTbl := `project.` + vprocore.QuoteHeaderIdentifier(project+"_Mineral")
		otherTbl := `project.` + vprocore.QuoteHeaderIdentifier(project+"_Other")

		rows, err := tx.QueryContext(ctx, `SELECT h."RequestID", h."PlotNumber", h."PlotType", h."Created", h."Actor", h."Undone", COALESCE(h."UndoneAt", ''), h."Proposal",
			EXISTS(SELECT 1 FROM `+envTbl+` e WHERE typeof(e."PlotNumber")='text' AND CAST(e."PlotNumber" AS BLOB)=CAST(h."PlotNumber" AS BLOB)),
			(EXISTS(SELECT 1 FROM `+vegTbl+` v WHERE typeof(v."PlotNumber")='text' AND CAST(v."PlotNumber" AS BLOB)=CAST(h."PlotNumber" AS BLOB))
			 OR EXISTS(SELECT 1 FROM `+humusTbl+` hm WHERE typeof(hm."PlotNumber")='text' AND CAST(hm."PlotNumber" AS BLOB)=CAST(h."PlotNumber" AS BLOB))
			 OR EXISTS(SELECT 1 FROM `+minTbl+` m WHERE typeof(m."PlotNumber")='text' AND CAST(m."PlotNumber" AS BLOB)=CAST(h."PlotNumber" AS BLOB))
			 OR EXISTS(SELECT 1 FROM `+otherTbl+` o WHERE typeof(o."PlotNumber")='text' AND CAST(o."PlotNumber" AS BLOB)=CAST(h."PlotNumber" AS BLOB)))
			FROM project."__VPRO_SIVICreationPlotHistory" h
			ORDER BY h.rowid DESC`)
		if err != nil {
			return nil, err
		}
		entries = []SIVICreatedPlotEntry{}
		for rows.Next() {
			var rID, plot, pType, created, actor, undoneAt, rawProp string
			var undone int64
			var inEnv, hasChildren bool
			if err := rows.Scan(&rID, &plot, &pType, &created, &actor, &undone, &undoneAt, &rawProp, &inEnv, &hasChildren); err != nil {
				rows.Close()
				return nil, err
			}
			var prop siviPlotCreationProposal
			_ = json.Unmarshal([]byte(rawProp), &prop)
			isUndone := undone != 0
			entries = append(entries, SIVICreatedPlotEntry{
				RequestID:       rID,
				PlotNumber:      plot,
				PlotType:        pType,
				ProjectID:       prop.Request.ProjectID,
				SVPolygonNumber: prop.Request.SVPolygonNumber,
				CreatedAt:       created,
				Actor:           actor,
				Undone:          isUndone,
				UndoneAt:        undoneAt,
				CanUndo:         !isUndone && inEnv && !hasChildren,
			})
		}
		if err := errors.Join(rows.Err(), rows.Close()); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return entries, nil
	})
}
