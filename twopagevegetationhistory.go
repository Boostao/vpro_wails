package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const twoPageVegetationHistoryTable = "__VPRO_TwoPageVegetationHistory"

type twoPageVegetationHistory struct {
	Request       twoPageVegetationRequest
	Committed     ProjectMetadataTable
	Actor         string
	AuditStrength int
	EditWhen      string
	Audits        []AuditEntry
}

type twoPageVegetationWriteResult struct {
	History   twoPageVegetationHistory `json:"history"`
	DidCommit bool                     `json:"didCommit"`
	Replayed  bool                     `json:"replayed"`
}

func twoPageVegetationPlannedTable(original ProjectMetadataTable, assignments []twoPageVegetationAssignment) ProjectMetadataTable {
	planned := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn{}, original.Columns...), Rows: []ProjectMetadataRow{}}
	for _, row := range original.Rows {
		copy := ProjectMetadataRow{RowID: row.RowID, Cells: make([]ProjectMetadataCell, len(row.Cells))}
		for i, cell := range row.Cells {
			copy.Cells[i] = cloneSiteUnitCell(cell)
			for _, assignment := range assignments {
				if row.RowID == assignment.RowID && strings.EqualFold(original.Columns[i].Name, assignment.Column) {
					copy.Cells[i] = cloneSiteUnitCell(assignment.After)
				}
			}
		}
		planned.Rows = append(planned.Rows, copy)
	}
	return planned
}

func validateTwoPageVegetationHistory(ctx context.Context, history twoPageVegetationHistory) error {
	plan, err := planTwoPageVegetationHistoricalEdits(ctx, history.Request.Original, history.Request.Edits)
	if err != nil || len(plan) == 0 || !siviDeletionUUID.MatchString(history.Request.RequestID) {
		return errors.Join(err, errors.New("two-page vegetation history requires a complete changed source plan"))
	}
	if history.Actor == "" || history.AuditStrength < 0 || history.AuditStrength > 3 {
		return errors.New("two-page vegetation history has unavailable audit provenance")
	}
	if err := validateChildPhysicalText("two-page vegetation actor", history.Actor, 100); err != nil {
		return err
	}
	if _, err := time.Parse("2006-01-02 15:04:05", history.EditWhen); err != nil {
		return fmt.Errorf("two-page vegetation history time: %w", err)
	}
	if !twoPageVegetationTableEqual(history.Committed, twoPageVegetationPlannedTable(history.Request.Original.Records, plan)) {
		return errors.New("two-page vegetation history changed unrelated source cells")
	}
	audited := 0
	seen := map[string]bool{}
	for _, assignment := range plan {
		before, err := metadataCellValue(assignment.Before)
		if err != nil {
			return err
		}
		after, err := metadataCellValue(assignment.After)
		if err != nil {
			return err
		}
		if !auditHeaderChange(before, after, history.AuditStrength) {
			continue
		}
		if audited >= len(history.Audits) {
			return errors.New("two-page vegetation history lost an audited change")
		}
		audit := history.Audits[audited]
		audited++
		rowID, err := strconv.ParseInt(audit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(rowID, 10) != audit.RowID || seen[audit.RowID] {
			return errors.New("two-page vegetation history requires distinct canonical audit rowids")
		}
		seen[audit.RowID] = true
		original := history.Request.Original
		if audit.Project != original.Project || audit.PlotNumber != original.Plot || audit.User != history.Actor ||
			audit.Table != "_Veg" || audit.EditField != assignment.Column || audit.EditWhen != history.EditWhen ||
			audit.ID == nil || *audit.ID != assignment.ID || audit.Restore || audit.Flag ||
			!reflect.DeepEqual(otherNullable(audit.BeforeEdit), headerAuditValue(before)) ||
			!reflect.DeepEqual(otherNullable(audit.AfterEdit), headerAuditValue(after)) {
			return errors.New("two-page vegetation history audit differs from its source assignment")
		}
	}
	if audited != len(history.Audits) {
		return errors.New("two-page vegetation history contains unplanned audits")
	}
	return ctx.Err()
}

func readTwoPageVegetationReceipt(ctx context.Context, request twoPageVegetationRequest, table ProjectMetadataTable) (*twoPageVegetationHistory, error) {
	if table.Columns == nil {
		return nil, ctx.Err()
	}
	columns, err := siteUnitTransferColumns(table, "RequestID", "Created", "Proposal")
	want := []ProjectMetadataColumn{{Name: "RequestID", DeclaredType: "TEXT"}, {Name: "Created", DeclaredType: "TEXT"}, {Name: "Proposal", DeclaredType: "TEXT"}}
	if err != nil || !reflect.DeepEqual(table.Columns, want) {
		return nil, errors.Join(err, errors.New("two-page vegetation receipt schema differs"))
	}
	var matched *twoPageVegetationHistory
	seen := map[string]bool{}
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		id, when, proposal := row.Cells[columns["RequestID"]], row.Cells[columns["Created"]], row.Cells[columns["Proposal"]]
		for _, cell := range []ProjectMetadataCell{id, when, proposal} {
			if _, err := metadataCellValue(cell); err != nil || cell.Storage != "text" {
				return nil, errors.Join(err, errors.New("two-page vegetation receipt requires canonical TEXT"))
			}
		}
		if seen[*id.Text] {
			return nil, errors.New("two-page vegetation receipt repeats a request")
		}
		seen[*id.Text] = true
		if _, err := sourceChildJSONObject([]byte(*proposal.Text), "two-page vegetation history",
			[]string{"Request", "Committed", "Actor", "AuditStrength", "EditWhen", "Audits"}, nil); err != nil {
			return nil, err
		}
		var history twoPageVegetationHistory
		if err := json.Unmarshal([]byte(*proposal.Text), &history); err != nil {
			return nil, err
		}
		canonical, err := json.Marshal(history)
		if err != nil || string(canonical) != *proposal.Text || *id.Text != history.Request.RequestID || *when.Text != history.EditWhen {
			return nil, errors.Join(err, errors.New("two-page vegetation receipt differs from canonical provenance"))
		}
		if err := validateTwoPageVegetationHistory(ctx, history); err != nil {
			return nil, err
		}
		if *id.Text == request.RequestID {
			expected, err := json.Marshal(request)
			if err != nil {
				return nil, err
			}
			committed, err := json.Marshal(history.Request)
			if err != nil {
				return nil, err
			}
			if string(committed) != string(expected) {
				return nil, errors.New("two-page vegetation request UUID belongs to another exact source plan")
			}
			matched = &history
		}
	}
	return matched, ctx.Err()
}
