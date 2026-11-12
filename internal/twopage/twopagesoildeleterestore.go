package twopage

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type DeleteTwoPageSoilHorizonRequest struct {
	RequestID  string `json:"requestId,omitempty"`
	PlotNumber string `json:"plotNumber"`
	Form       string `json:"form"`
	ID         int64  `json:"id"`
	RowID      string `json:"rowId,omitempty"`
}

type DeleteTwoPageSoilHorizonResult struct {
	RequestID        string `json:"requestId"`
	ContextID        string `json:"contextId"`
	Project          string `json:"project"`
	PlotNumber       string `json:"plotNumber"`
	Form             string `json:"form"`
	Kind             string `json:"kind"`
	DeletedID        int64  `json:"deletedId"`
	Horizon          string `json:"horizon"`
	ReservedInLedger bool   `json:"reservedInLedger"`
	AuditRowCount    int    `json:"auditRowCount"`
	EditWhen         string `json:"editWhen"`
	Replayed         bool   `json:"replayed"`
}

type DeletedTwoPageSoilHorizonEntry struct {
	DeleteRequestID string   `json:"deleteRequestId"`
	PlotNumber      string   `json:"plotNumber"`
	Form            string   `json:"form"`
	Kind            string   `json:"kind"`
	DeletedID       int64    `json:"deletedId"`
	Horizon         string   `json:"horizon"`
	UpperDepth      *float64 `json:"upperDepth,omitempty"`
	LowerDepth      *float64 `json:"lowerDepth,omitempty"`
	DeletedBy       string   `json:"deletedBy"`
	DeletedAt       string   `json:"deletedAt"`
	Restored        bool     `json:"restored"`
}

type RestoreTwoPageSoilHorizonRequest struct {
	RequestID       string `json:"requestId,omitempty"`
	PlotNumber      string `json:"plotNumber"`
	Form            string `json:"form"`
	DeleteRequestID string `json:"deleteRequestId"`
}

type RestoreTwoPageSoilHorizonResult struct {
	RequestID       string `json:"requestId"`
	DeleteRequestID string `json:"deleteRequestId"`
	ContextID       string `json:"contextId"`
	Project         string `json:"project"`
	PlotNumber      string `json:"plotNumber"`
	Form            string `json:"form"`
	Kind            string `json:"kind"`
	RestoredID      int64  `json:"restoredId"`
	RestoredRowID   string `json:"restoredRowId"`
	Horizon         string `json:"horizon"`
	AuditRowCount   int    `json:"auditRowCount"`
	EditWhen        string `json:"editWhen"`
	Replayed        bool   `json:"replayed"`
}

func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

func normalizeTwoPageSoilForm(raw string) (form, kind string, err error) {
	switch strings.TrimSpace(raw) {
	case "SoilHumusXL", "Humus":
		return "SoilHumusXL", "Humus", nil
	case "SoilMineralXL", "Mineral":
		return "SoilMineralXL", "Mineral", nil
	default:
		return "", "", fmt.Errorf("unsupported two-page soil form %q (expected SoilHumusXL or SoilMineralXL)", raw)
	}
}

func validateTwoPageSoilDeleteRestoreHistory(ctx context.Context, history twoPageSoilHistory) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !vprocore.SiviDeletionUUID.MatchString(history.Request.RequestID) {
		return errors.New("two-page soil delete/restore history requires a lowercase UUIDv4 requestId")
	}
	if history.Request.Original == nil || history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 {
		return errors.New("two-page soil delete/restore history lacks required original or actor provenance")
	}
	if _, err := time.Parse("2006-01-02 15:04:05", history.EditWhen); err != nil {
		return fmt.Errorf("two-page soil delete/restore history timestamp: %w", err)
	}
	switch history.Action {
	case "delete":
		if history.DeletedID == nil || history.DeletedRow == nil || len(history.DeletedRow.Cells) != len(history.Committed.Columns) {
			return errors.New("two-page soil delete history lacks complete deleted row or ID")
		}
	case "restore":
		if history.DeletedID == nil || history.RestoreOfRequestID == nil || !vprocore.SiviDeletionUUID.MatchString(*history.RestoreOfRequestID) || history.RestoredRowID == nil {
			return errors.New("two-page soil restore history lacks source deletion UUID or restored row ID")
		}
	default:
		return fmt.Errorf("unsupported two-page soil history action %q", history.Action)
	}
	return nil
}

func unmarshalAnyTwoPageSoilHistory(raw string) (twoPageSoilHistory, error) {
	// Note: twoPageSoilRequest has custom UnmarshalJSON requiring non-empty edits when Horizon is nil;
	// bypass twoPageSoilRequest.UnmarshalJSON for delete/restore entries where Edits is empty.
	var probe struct {
		Action string `json:"Action"`
	}
	if err := json.Unmarshal([]byte(raw), &probe); err != nil {
		return twoPageSoilHistory{}, err
	}
	if probe.Action == "delete" || probe.Action == "restore" {
		type plainReq struct {
			RequestID string               `json:"requestId"`
			Original  *twoPageSoilOriginal `json:"original"`
			Edits     []twoPageSoilEdit    `json:"edits"`
			Horizon   *string              `json:"horizon,omitempty"`
		}
		var envelope struct {
			Request            plainReq                     `json:"Request"`
			Committed          vprocore.ProjectMetadataTable `json:"Committed"`
			Actor              string                       `json:"Actor"`
			AuditStrength      int                          `json:"AuditStrength"`
			EditWhen           string                       `json:"EditWhen"`
			Audits             []vprocore.AuditEntry        `json:"Audits"`
			CreatedRowID       *string                      `json:"CreatedRowID,omitempty"`
			Action             string                       `json:"Action,omitempty"`
			DeletedID          *int64                       `json:"DeletedID,omitempty"`
			DeletedRow         *vprocore.ProjectMetadataRow `json:"DeletedRow,omitempty"`
			RestoreOfRequestID *string                      `json:"RestoreOfRequestID,omitempty"`
			RestoredRowID      *string                      `json:"RestoredRowID,omitempty"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			return twoPageSoilHistory{}, err
		}
		return twoPageSoilHistory{
			Request: twoPageSoilRequest{
				RequestID: envelope.Request.RequestID,
				Original:  envelope.Request.Original,
				Edits:     envelope.Request.Edits,
				Horizon:   envelope.Request.Horizon,
			},
			Committed:          envelope.Committed,
			Actor:              envelope.Actor,
			AuditStrength:      envelope.AuditStrength,
			EditWhen:           envelope.EditWhen,
			Audits:             envelope.Audits,
			CreatedRowID:       envelope.CreatedRowID,
			Action:             envelope.Action,
			DeletedID:          envelope.DeletedID,
			DeletedRow:         envelope.DeletedRow,
			RestoreOfRequestID: envelope.RestoreOfRequestID,
			RestoredRowID:      envelope.RestoredRowID,
		}, nil
	}
	var h twoPageSoilHistory
	if err := json.Unmarshal([]byte(raw), &h); err != nil {
		return twoPageSoilHistory{}, err
	}
	return h, nil
}

func marshalDeleteRestoreHistory(h twoPageSoilHistory) (string, error) {
	data, err := json.Marshal(h)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteTwoPageSoilHorizon deletes an owned <Project>_Humus or <Project>_Mineral row
// inside a single BEGIN IMMEDIATE transaction, permanently reserving its ID in
// __VPRO_ChildIdentity so it is never reused by new horizon creations, and recording
// full <Project>_Audit and __VPRO_TwoPageSoilHistory provenance.
func DeleteTwoPageSoilHorizon(s *vprocore.ContextService, ctx context.Context, contextID string, req DeleteTwoPageSoilHorizonRequest) (DeleteTwoPageSoilHorizonResult, error) {
	if s == nil {
		return DeleteTwoPageSoilHorizonResult{}, errors.New("two-page soil context service is unavailable")
	}
	form, kind, err := normalizeTwoPageSoilForm(req.Form)
	if err != nil {
		return DeleteTwoPageSoilHorizonResult{}, err
	}
	if req.PlotNumber == "" || !utf8.ValidString(req.PlotNumber) || strings.ContainsRune(req.PlotNumber, 0) {
		return DeleteTwoPageSoilHorizonResult{}, errors.New("soil horizon deletion requires a literal valid plotNumber")
	}
	if err := vprocore.ValidateChildPhysicalText("PlotNumber", req.PlotNumber, 7); err != nil {
		return DeleteTwoPageSoilHorizonResult{}, err
	}
	if req.ID == 0 || req.ID < -2147483648 || req.ID > 2147483647 {
		return DeleteTwoPageSoilHorizonResult{}, errors.New("soil horizon deletion requires a non-zero signed32 child ID")
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID, err = newUUIDv4()
		if err != nil {
			return DeleteTwoPageSoilHorizonResult{}, err
		}
	} else if !vprocore.SiviDeletionUUID.MatchString(requestID) {
		return DeleteTwoPageSoilHorizonResult{}, errors.New("requestId must be a lowercase UUIDv4")
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (DeleteTwoPageSoilHorizonResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return DeleteTwoPageSoilHorizonResult{}, err
		}
		owner := plots.Projects.Sqlite
		actor := strings.TrimSpace(plots.CurrentUser)
		if actor == "" {
			actor = "VProUser"
		}
		strength := plots.AuditStrength

		var out DeleteTwoPageSoilHorizonResult
		wErr := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			suAlias, err := sourceform.SiviParentWriterSU(ctx, conn, owner)
			if err != nil {
				return err
			}
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			if suAlias == "sivi_su" {
				if err := vprocore.RequireRollbackJournalReadLock(ctx, tx, suAlias, "two-page soil deletion SU check"); err != nil {
					return err
				}
			}
			if err := twoPageSoilWriterSelectedParent(ctx, tx, owner, suAlias, req.PlotNumber); err != nil {
				return err
			}

			project := owner.Selection.Project
			tableName := project + "_" + kind
			quotedTbl := vprocore.QuoteHeaderIdentifier(tableName)
			quotedAudit := vprocore.QuoteHeaderIdentifier(project + "_Audit")

			// Check idempotent replay in __VPRO_TwoPageSoilHistory if table exists.
			var hasHistory bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, twoPageSoilHistoryTable).Scan(&hasHistory); err != nil {
				return err
			}
			if hasHistory {
				var rawProposal string
				rErr := tx.QueryRowContext(ctx, `SELECT Proposal FROM "__VPRO_TwoPageSoilHistory" WHERE RequestID = ?`, requestID).Scan(&rawProposal)
				if rErr == nil {
					prev, uErr := unmarshalAnyTwoPageSoilHistory(rawProposal)
					if uErr != nil {
						return uErr
					}
					if prev.Action != "delete" || prev.DeletedID == nil || *prev.DeletedID != req.ID ||
						prev.Request.Original == nil || prev.Request.Original.Plot != req.PlotNumber || prev.Request.Original.Form != form {
						return errors.New("requestId already belongs to a different two-page soil history entry")
					}
					hz := extractHorizonFromRow(prev.Committed.Columns, *prev.DeletedRow)
					out = DeleteTwoPageSoilHorizonResult{
						RequestID:        requestID,
						ContextID:        contextID,
						Project:          project,
						PlotNumber:       req.PlotNumber,
						Form:             form,
						Kind:             kind,
						DeletedID:        req.ID,
						Horizon:          hz,
						ReservedInLedger: true,
						AuditRowCount:    len(prev.Audits),
						EditWhen:         prev.EditWhen,
						Replayed:         true,
					}
					return nil
				} else if !errors.Is(rErr, sql.ErrNoRows) {
					return rErr
				}
			}

			// Verify 1:1 parent in Env and Admin.
			if err := vprocore.ChildParent(tx, project, req.PlotNumber); err != nil {
				return err
			}

			// Read plot rows before deletion.
			beforeRecords, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "main", tableName, "PlotNumber", req.PlotNumber, "")
			if err != nil {
				return err
			}
			colMap, err := vprocore.SiteUnitTransferColumns(beforeRecords, "ID", "PlotNumber", "Horizon")
			if err != nil {
				return err
			}

			targetIdx := -1
			idText := strconv.FormatInt(req.ID, 10)
			for i, r := range beforeRecords.Rows {
				if sourceform.SiviIdentityCellMatches(r.Cells[colMap["ID"]], idText) {
					if targetIdx >= 0 {
						return fmt.Errorf("ambiguous %s ID %d in plot %s; deletion refused", kind, req.ID, req.PlotNumber)
					}
					targetIdx = i
				}
			}
			if targetIdx < 0 {
				return fmt.Errorf("%s horizon ID %d not found on plot %s", kind, req.ID, req.PlotNumber)
			}
			deletedRow := beforeRecords.Rows[targetIdx]
			if strings.TrimSpace(req.RowID) != "" && deletedRow.RowID != strings.TrimSpace(req.RowID) {
				return fmt.Errorf("%s horizon physical rowId changed (expected %s, found %s); reload before deleting", kind, req.RowID, deletedRow.RowID)
			}

			// Ensure __VPRO_ChildIdentity exists and permanently reserve all existing + deleted IDs!
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
				"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID"))`); err != nil {
				return err
			}
			idScan, err := tx.QueryContext(ctx, `SELECT "ID" FROM `+quotedTbl+` WHERE typeof("ID")='integer' AND "ID" > 0`)
			if err != nil {
				return err
			}
			var idsToReserve []int64
			for idScan.Next() {
				var v int64
				if err := idScan.Scan(&v); err != nil {
					idScan.Close()
					return err
				}
				idsToReserve = append(idsToReserve, v)
			}
			if err := errors.Join(idScan.Err(), idScan.Close()); err != nil {
				return err
			}
			if req.ID > 0 && !slices.Contains(idsToReserve, req.ID) {
				idsToReserve = append(idsToReserve, req.ID)
			}
			for _, v := range idsToReserve {
				if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_ChildIdentity" ("ChildTable","ID") VALUES (?,?) ON CONFLICT DO NOTHING`,
					quotedTbl, v); err != nil {
					return err
				}
			}

			// Delete the exact physical row.
			physRowID, err := strconv.ParseInt(deletedRow.RowID, 10, 64)
			if err != nil {
				return err
			}
			dRes, err := tx.ExecContext(ctx, `DELETE FROM `+quotedTbl+` WHERE rowid = ? AND typeof("PlotNumber")='text' AND CAST("PlotNumber" AS BLOB)=CAST(? AS BLOB) AND "ID" = ?`,
				physRowID, req.PlotNumber, req.ID)
			if err != nil {
				return err
			}
			if n, _ := dRes.RowsAffected(); n != 1 {
				return errors.New("two-page soil deletion did not affect exactly 1 row")
			}

			now := time.Now().Format("2006-01-02 15:04:05")
			var auditEntries []vprocore.AuditEntry
			for cIdx, col := range beforeRecords.Columns {
				if col.Name == "ID" || col.Name == "PlotNumber" {
					continue
				}
				cell := deletedRow.Cells[cIdx]
				if cell.Storage == "null" || (col.Name == "Flag" && cell.Storage == "integer" && cell.Integer != nil && *cell.Integer == "0") {
					continue
				}
				oldVal, mErr := vprocore.MetadataCellValue(cell)
				if mErr != nil {
					return mErr
				}
				// Always record deletion audit for non-null cells so full audit trail exists.
				effectiveStrength := strength
				if effectiveStrength < 3 {
					effectiveStrength = 3
				}
				tracked, aErr := vprocore.AuditChildFieldsTracked(tx, project, kind, req.PlotNumber, req.ID,
					[]vprocore.ChildField{{Column: col.Name}}, []any{oldVal}, []any{nil}, actor, effectiveStrength, now)
				if aErr != nil {
					return aErr
				}
				for _, tr := range tracked {
					ids, _ := vprocore.AuditRowIDs([]string{tr.RowID})
					recs, sErr := vprocore.SelectedAuditEntries(tx, project, req.PlotNumber, ids)
					if sErr != nil || len(recs) != 1 {
						return errors.Join(sErr, errors.New("deleted soil audit lookup failed"))
					}
					auditEntries = append(auditEntries, recs[0])
				}
			}
			// If every non-identity cell was NULL, record at least Horizon/ID deletion audit row.
			if len(auditEntries) == 0 {
				ins, aErr := tx.ExecContext(ctx, `INSERT INTO `+quotedAudit+
					` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
					project, actor, req.PlotNumber, "_"+kind, "Horizon", now, strconv.FormatInt(req.ID, 10), nil, req.ID)
				if aErr != nil {
					return aErr
				}
				aRowID, _ := ins.LastInsertId()
				ids, _ := vprocore.AuditRowIDs([]string{strconv.FormatInt(aRowID, 10)})
				recs, _ := vprocore.SelectedAuditEntries(tx, project, req.PlotNumber, ids)
				if len(recs) == 1 {
					auditEntries = append(auditEntries, recs[0])
				}
			}

			afterRecords, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "main", tableName, "PlotNumber", req.PlotNumber, "")
			if err != nil {
				return err
			}

			delIDCopy := req.ID
			delRowCopy := deletedRow
			history := twoPageSoilHistory{
				Request: twoPageSoilRequest{
					RequestID: requestID,
					Original: &twoPageSoilOriginal{
						ContextID:    contextID,
						Project:      project,
						Plot:         req.PlotNumber,
						Form:         form,
						RecordSource: "Usys" + kind,
						Table:        tableName,
						Records:      beforeRecords,
					},
					Edits: []twoPageSoilEdit{},
				},
				Committed:     afterRecords,
				Actor:         actor,
				AuditStrength: strength,
				EditWhen:      now,
				Audits:        auditEntries,
				Action:        "delete",
				DeletedID:     &delIDCopy,
				DeletedRow:    &delRowCopy,
			}
			proposalJSON, err := marshalDeleteRestoreHistory(history)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS "__VPRO_TwoPageSoilHistory" (
				RequestID TEXT NOT NULL PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL)`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_TwoPageSoilHistory" VALUES(?,?,?)`,
				requestID, now, proposalJSON); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			hz := extractHorizonFromRow(beforeRecords.Columns, deletedRow)
			out = DeleteTwoPageSoilHorizonResult{
				RequestID:        requestID,
				ContextID:        contextID,
				Project:          project,
				PlotNumber:       req.PlotNumber,
				Form:             form,
				Kind:             kind,
				DeletedID:        req.ID,
				Horizon:          hz,
				ReservedInLedger: req.ID > 0,
				AuditRowCount:    len(auditEntries),
				EditWhen:         now,
				Replayed:         false,
			}
			return nil
		})
		if wErr != nil {
			return DeleteTwoPageSoilHorizonResult{}, wErr
		}
		return out, nil
	})
}

// RestoreTwoPageSoilHorizon restores a previously deleted <Project>_Humus or <Project>_Mineral
// horizon from __VPRO_TwoPageSoilHistory inside an atomic BEGIN IMMEDIATE transaction,
// verifying permanent reservation in __VPRO_ChildIdentity, marking deletion audits Restored (-1),
// and appending a restoration receipt to __VPRO_TwoPageSoilHistory.
func RestoreTwoPageSoilHorizon(s *vprocore.ContextService, ctx context.Context, contextID string, req RestoreTwoPageSoilHorizonRequest) (RestoreTwoPageSoilHorizonResult, error) {
	if s == nil {
		return RestoreTwoPageSoilHorizonResult{}, errors.New("two-page soil context service is unavailable")
	}
	form, kind, err := normalizeTwoPageSoilForm(req.Form)
	if err != nil {
		return RestoreTwoPageSoilHorizonResult{}, err
	}
	if req.PlotNumber == "" || !utf8.ValidString(req.PlotNumber) || strings.ContainsRune(req.PlotNumber, 0) {
		return RestoreTwoPageSoilHorizonResult{}, errors.New("soil horizon restoration requires a literal valid plotNumber")
	}
	deleteReqID := strings.TrimSpace(req.DeleteRequestID)
	if !vprocore.SiviDeletionUUID.MatchString(deleteReqID) {
		return RestoreTwoPageSoilHorizonResult{}, errors.New("deleteRequestId must be a lowercase UUIDv4")
	}
	requestID := strings.TrimSpace(req.RequestID)
	if requestID == "" {
		requestID, err = newUUIDv4()
		if err != nil {
			return RestoreTwoPageSoilHorizonResult{}, err
		}
	} else if !vprocore.SiviDeletionUUID.MatchString(requestID) {
		return RestoreTwoPageSoilHorizonResult{}, errors.New("requestId must be a lowercase UUIDv4")
	}
	if requestID == deleteReqID {
		return RestoreTwoPageSoilHorizonResult{}, errors.New("restoration requestId must differ from deleteRequestId")
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (RestoreTwoPageSoilHorizonResult, error) {
		if err := plots.RequireContextEdit(); err != nil {
			return RestoreTwoPageSoilHorizonResult{}, err
		}
		owner := plots.Projects.Sqlite
		actor := strings.TrimSpace(plots.CurrentUser)
		if actor == "" {
			actor = "VProUser"
		}
		strength := plots.AuditStrength

		var out RestoreTwoPageSoilHorizonResult
		wErr := owner.WithMetadataWriter(ctx, func(conn *sql.Conn) (txErr error) {
			suAlias, err := sourceform.SiviParentWriterSU(ctx, conn, owner)
			if err != nil {
				return err
			}
			tx, err := conn.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() {
				if rb := tx.Rollback(); rb != nil && !errors.Is(rb, sql.ErrTxDone) {
					txErr = errors.Join(txErr, rb)
				}
			}()

			if suAlias == "sivi_su" {
				if err := vprocore.RequireRollbackJournalReadLock(ctx, tx, suAlias, "two-page soil restoration SU check"); err != nil {
					return err
				}
			}
			if err := twoPageSoilWriterSelectedParent(ctx, tx, owner, suAlias, req.PlotNumber); err != nil {
				return err
			}

			project := owner.Selection.Project
			tableName := project + "_" + kind
			quotedTbl := vprocore.QuoteHeaderIdentifier(tableName)
			quotedAudit := vprocore.QuoteHeaderIdentifier(project + "_Audit")

			if err := vprocore.ChildParent(tx, project, req.PlotNumber); err != nil {
				return err
			}

			var hasHistory bool
			if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, twoPageSoilHistoryTable).Scan(&hasHistory); err != nil {
				return err
			}
			if !hasHistory {
				return errors.New("no two-page soil deletion history exists in this project")
			}

			// Read all history entries to check idempotent replay and ensure deletion wasn't already restored.
			hRows, err := tx.QueryContext(ctx, `SELECT RequestID, Proposal FROM "__VPRO_TwoPageSoilHistory" ORDER BY rowid`)
			if err != nil {
				return err
			}
			var targetDeletion *twoPageSoilHistory
			alreadyConsumed := false
			var replayRestore *twoPageSoilHistory

			for hRows.Next() {
				var rID, rawProp string
				if err := hRows.Scan(&rID, &rawProp); err != nil {
					hRows.Close()
					return err
				}
				parsed, pErr := unmarshalAnyTwoPageSoilHistory(rawProp)
				if pErr != nil {
					hRows.Close()
					return pErr
				}
				if rID == requestID {
					if parsed.Action != "restore" || parsed.RestoreOfRequestID == nil || *parsed.RestoreOfRequestID != deleteReqID {
						hRows.Close()
						return errors.New("requestId already belongs to another soil history operation")
					}
					cp := parsed
					replayRestore = &cp
				}
				if rID == deleteReqID && parsed.Action == "delete" {
					cp := parsed
					targetDeletion = &cp
				}
				if parsed.Action == "restore" && parsed.RestoreOfRequestID != nil && *parsed.RestoreOfRequestID == deleteReqID && rID != requestID {
					alreadyConsumed = true
				}
			}
			if err := errors.Join(hRows.Err(), hRows.Close()); err != nil {
				return err
			}

			if replayRestore != nil && targetDeletion != nil {
				hz := extractHorizonFromRow(targetDeletion.Request.Original.Records.Columns, *targetDeletion.DeletedRow)
				out = RestoreTwoPageSoilHorizonResult{
					RequestID:       requestID,
					DeleteRequestID: deleteReqID,
					ContextID:       contextID,
					Project:         project,
					PlotNumber:      req.PlotNumber,
					Form:            form,
					Kind:            kind,
					RestoredID:      *replayRestore.DeletedID,
					RestoredRowID:   *replayRestore.RestoredRowID,
					Horizon:         hz,
					AuditRowCount:   len(replayRestore.Audits),
					EditWhen:        replayRestore.EditWhen,
					Replayed:        true,
				}
				return nil
			}

			if targetDeletion == nil || targetDeletion.DeletedRow == nil || targetDeletion.DeletedID == nil {
				return fmt.Errorf("soil deletion receipt %s not found", deleteReqID)
			}
			if alreadyConsumed {
				return fmt.Errorf("soil deletion receipt %s has already been restored", deleteReqID)
			}
			if targetDeletion.Request.Original == nil ||
				targetDeletion.Request.Original.Project != project ||
				targetDeletion.Request.Original.Plot != req.PlotNumber ||
				targetDeletion.Request.Original.Form != form {
				return errors.New("soil deletion receipt belongs to another project, plot, or soil form")
			}

			restoredID := *targetDeletion.DeletedID
			// Verify ID is not currently occupied in <Project>_<Kind>.
			var occupied int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+quotedTbl+` WHERE "ID" = ?`, restoredID).Scan(&occupied); err != nil {
				return err
			}
			if occupied != 0 {
				return fmt.Errorf("cannot restore %s horizon ID %d because that ID is already occupied", kind, restoredID)
			}

			// Ensure permanent reservation in __VPRO_ChildIdentity remains intact!
			if restoredID > 0 {
				var reserved int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM "__VPRO_ChildIdentity" WHERE "ChildTable" = ? AND "ID" = ?`,
					quotedTbl, restoredID).Scan(&reserved); err != nil {
					return err
				}
				if reserved != 1 {
					return fmt.Errorf("%s horizon ID %d lost its permanent reservation in __VPRO_ChildIdentity", kind, restoredID)
				}
			}

			beforeRecords, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "main", tableName, "PlotNumber", req.PlotNumber, "")
			if err != nil {
				return err
			}
			origCols := targetDeletion.Request.Original.Records.Columns
			if len(beforeRecords.Columns) != len(origCols) {
				return errors.New("soil table schema changed since deletion; restoration refused")
			}

			quotedCols := make([]string, len(origCols))
			marks := make([]string, len(origCols))
			args := make([]any, len(origCols))
			for i, col := range origCols {
				quotedCols[i] = vprocore.QuoteHeaderIdentifier(col.Name)
				marks[i] = "?"
				val, mErr := vprocore.MetadataCellValue(targetDeletion.DeletedRow.Cells[i])
				if mErr != nil {
					return mErr
				}
				args[i] = val
			}
			insSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quotedTbl, strings.Join(quotedCols, ", "), strings.Join(marks, ", "))
			insRes, err := tx.ExecContext(ctx, insSQL, args...)
			if err != nil {
				return err
			}
			newRowID, err := insRes.LastInsertId()
			if err != nil {
				return err
			}

			// Mark original deletion audit rows with Restore = -1 (Access BOOLEAN true = -1) AND log restoration audit!
			for _, delAudit := range targetDeletion.Audits {
				if aRowID, pErr := strconv.ParseInt(delAudit.RowID, 10, 64); pErr == nil {
					_, _ = tx.ExecContext(ctx, `UPDATE `+quotedAudit+` SET "Restore" = -1 WHERE rowid = ? AND "PlotNumber" = ?`, aRowID, req.PlotNumber)
				}
			}

			now := time.Now().Format("2006-01-02 15:04:05")
			hz := extractHorizonFromRow(origCols, *targetDeletion.DeletedRow)
			var restoreAudits []vprocore.AuditEntry
			aIns, err := tx.ExecContext(ctx, `INSERT INTO `+quotedAudit+
				` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
				project, actor, req.PlotNumber, "_"+kind, "Horizon", now, nil, hz, restoredID)
			if err != nil {
				return err
			}
			aRowID, _ := aIns.LastInsertId()
			ids, _ := vprocore.AuditRowIDs([]string{strconv.FormatInt(aRowID, 10)})
			recs, _ := vprocore.SelectedAuditEntries(tx, project, req.PlotNumber, ids)
			if len(recs) == 1 {
				restoreAudits = append(restoreAudits, recs[0])
			}

			afterRecords, err := vprocore.ReadSQLiteLiteralTextRows(ctx, tx, "main", tableName, "PlotNumber", req.PlotNumber, "")
			if err != nil {
				return err
			}

			restoredRowIDStr := strconv.FormatInt(newRowID, 10)
			delIDCopy := restoredID
			delReqCopy := deleteReqID
			history := twoPageSoilHistory{
				Request: twoPageSoilRequest{
					RequestID: requestID,
					Original: &twoPageSoilOriginal{
						ContextID:    contextID,
						Project:      project,
						Plot:         req.PlotNumber,
						Form:         form,
						RecordSource: "Usys" + kind,
						Table:        tableName,
						Records:      beforeRecords,
					},
					Edits: []twoPageSoilEdit{},
				},
				Committed:          afterRecords,
				Actor:              actor,
				AuditStrength:      strength,
				EditWhen:           now,
				Audits:             restoreAudits,
				Action:             "restore",
				DeletedID:          &delIDCopy,
				RestoreOfRequestID: &delReqCopy,
				RestoredRowID:      &restoredRowIDStr,
			}
			proposalJSON, err := marshalDeleteRestoreHistory(history)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO "__VPRO_TwoPageSoilHistory" VALUES(?,?,?)`,
				requestID, now, proposalJSON); err != nil {
				return err
			}

			if err := ctx.Err(); err != nil {
				return err
			}
			if err := tx.Commit(); err != nil {
				return err
			}

			out = RestoreTwoPageSoilHorizonResult{
				RequestID:       requestID,
				DeleteRequestID: deleteReqID,
				ContextID:       contextID,
				Project:         project,
				PlotNumber:      req.PlotNumber,
				Form:            form,
				Kind:            kind,
				RestoredID:      restoredID,
				RestoredRowID:   restoredRowIDStr,
				Horizon:         hz,
				AuditRowCount:   len(restoreAudits),
				EditWhen:        now,
				Replayed:        false,
			}
			return nil
		})
		if wErr != nil {
			return RestoreTwoPageSoilHorizonResult{}, wErr
		}
		return out, nil
	})
}

// ListTwoPageSoilDeletedHorizons lists deleted horizons recorded in __VPRO_TwoPageSoilHistory
// for the specified plotNumber and soil form (SoilHumusXL or SoilMineralXL).
func ListTwoPageSoilDeletedHorizons(s *vprocore.ContextService, ctx context.Context, contextID, plotNumber, formReq string) ([]DeletedTwoPageSoilHorizonEntry, error) {
	if s == nil {
		return nil, errors.New("two-page soil context service is unavailable")
	}
	form, kind, err := normalizeTwoPageSoilForm(formReq)
	if err != nil {
		return nil, err
	}
	if plotNumber == "" || !utf8.ValidString(plotNumber) || strings.ContainsRune(plotNumber, 0) {
		return nil, errors.New("plotNumber must be a valid literal string")
	}

	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (entries []DeletedTwoPageSoilHorizonEntry, resultErr error) {
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

		if err := twoPageSoilSelectedParent(ctx, owner, plotNumber); err != nil {
			return nil, err
		}

		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project.sqlite_master WHERE type='table' AND name=?)`, twoPageSoilHistoryTable).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
			return []DeletedTwoPageSoilHorizonEntry{}, nil
		}

		rows, err := tx.QueryContext(ctx, `SELECT RequestID, Proposal FROM project."__VPRO_TwoPageSoilHistory" ORDER BY rowid DESC`)
		if err != nil {
			return nil, err
		}
		var parsedList []twoPageSoilHistory
		restoredSet := map[string]bool{}
		for rows.Next() {
			var reqID, raw string
			if err := rows.Scan(&reqID, &raw); err != nil {
				rows.Close()
				return nil, err
			}
			h, pErr := unmarshalAnyTwoPageSoilHistory(raw)
			if pErr != nil {
				rows.Close()
				return nil, pErr
			}
			if h.Action == "restore" && h.RestoreOfRequestID != nil {
				restoredSet[*h.RestoreOfRequestID] = true
			}
			if h.Action == "delete" && h.Request.Original != nil &&
				h.Request.Original.Project == owner.Selection.Project &&
				h.Request.Original.Plot == plotNumber &&
				h.Request.Original.Form == form {
				parsedList = append(parsedList, h)
			}
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

		entries = make([]DeletedTwoPageSoilHorizonEntry, 0, len(parsedList))
		for _, h := range parsedList {
			if h.DeletedID == nil || h.DeletedRow == nil {
				continue
			}
			cols := h.Request.Original.Records.Columns
			hz := extractHorizonFromRow(cols, *h.DeletedRow)
			upper := extractFloatFromRow(cols, *h.DeletedRow, "UpperDepth")
			lower := extractFloatFromRow(cols, *h.DeletedRow, "LowerDepth")
			entries = append(entries, DeletedTwoPageSoilHorizonEntry{
				DeleteRequestID: h.Request.RequestID,
				PlotNumber:      plotNumber,
				Form:            form,
				Kind:            kind,
				DeletedID:       *h.DeletedID,
				Horizon:         hz,
				UpperDepth:      upper,
				LowerDepth:      lower,
				DeletedBy:       h.Actor,
				DeletedAt:       h.EditWhen,
				Restored:        restoredSet[h.Request.RequestID],
			})
		}
		return entries, nil
	})
}

func extractHorizonFromRow(cols []vprocore.ProjectMetadataColumn, row vprocore.ProjectMetadataRow) string {
	for i, c := range cols {
		if c.Name == "Horizon" && i < len(row.Cells) && row.Cells[i].Text != nil {
			return *row.Cells[i].Text
		}
	}
	return ""
}

func extractFloatFromRow(cols []vprocore.ProjectMetadataColumn, row vprocore.ProjectMetadataRow, colName string) *float64 {
	for i, c := range cols {
		if c.Name == colName && i < len(row.Cells) && row.Cells[i].Real != nil {
			v := *row.Cells[i].Real
			return &v
		}
	}
	return nil
}

func (s *TwoPageSoilService) DeleteHorizon(ctx context.Context, contextID string, req DeleteTwoPageSoilHorizonRequest) (DeleteTwoPageSoilHorizonResult, error) {
	if err := s.require(ctx, true); err != nil {
		return DeleteTwoPageSoilHorizonResult{}, err
	}
	return DeleteTwoPageSoilHorizon(s.contexts, ctx, contextID, req)
}

func (s *TwoPageSoilService) RestoreHorizon(ctx context.Context, contextID string, req RestoreTwoPageSoilHorizonRequest) (RestoreTwoPageSoilHorizonResult, error) {
	if err := s.require(ctx, true); err != nil {
		return RestoreTwoPageSoilHorizonResult{}, err
	}
	return RestoreTwoPageSoilHorizon(s.contexts, ctx, contextID, req)
}

func (s *TwoPageSoilService) ListDeletedHorizons(ctx context.Context, contextID, plot, form string) ([]DeletedTwoPageSoilHorizonEntry, error) {
	if err := s.require(ctx, false); err != nil {
		return nil, err
	}
	return ListTwoPageSoilDeletedHorizons(s.contexts, ctx, contextID, plot, form)
}
