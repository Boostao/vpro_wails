package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

const projectMetadataEditHistoryTable = "__VPRO_MetadataEditHistory"
const projectMetadataEditHistorySQL = `CREATE TABLE "__VPRO_MetadataEditHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

type projectMetadataEditHistory struct {
	Project    string                  `json:"project"`
	PlotNumber string                  `json:"plotNumber"`
	ProjectID  *string                 `json:"projectId"`
	ID         int64                   `json:"id"`
	User       string                  `json:"user"`
	EditWhen   string                  `json:"editWhen"`
	Columns    []ProjectMetadataColumn `json:"columns"`
	Original   ProjectMetadataRow      `json:"original"`
	Committed  ProjectMetadataRow      `json:"committed"`
	Audits     []childAuditIdentity    `json:"audits"`
	Records    []AuditEntry            `json:"records"`
}

func appendProjectMetadataEditHistory(ctx context.Context, tx *sql.Tx, history projectMetadataEditHistory) error {
	if len(history.Audits) == 0 {
		return nil
	}
	ids := make([]int64, len(history.Audits))
	for i, audit := range history.Audits {
		id, err := strconv.ParseInt(audit.RowID, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != audit.RowID {
			return errors.New("metadata provenance requires an exact inserted audit identity")
		}
		ids[i] = id
	}
	records, err := selectedAuditEntries(tx, history.Project, history.PlotNumber, ids)
	if err != nil {
		return err
	}
	index := map[string]int{}
	for i, column := range history.Columns {
		index[column.Name] = i
	}
	for i, record := range records {
		column, present := index[history.Audits[i].Column]
		if !present {
			return errors.New("metadata provenance audit column is absent from the observed schema")
		}
		before, err := metadataCellValue(history.Original.Cells[column])
		if err != nil {
			return err
		}
		after, err := metadataCellValue(history.Committed.Cells[column])
		if err != nil {
			return err
		}
		if record.RowID != history.Audits[i].RowID || record.Table != "_Metadata" ||
			record.EditField != history.Audits[i].Column || record.ID == nil || *record.ID != history.ID ||
			record.User != history.User || record.EditWhen != history.EditWhen || record.Restore || record.Flag ||
			!metadataAuditTextEqual(record.BeforeEdit, before) || !metadataAuditTextEqual(record.AfterEdit, after) {
			return fmt.Errorf("metadata inserted audit row %s differs from its typed mutation; all changes rolled back", record.RowID)
		}
	}
	history.Records = records
	proposal, err := json.Marshal(history)
	if err != nil {
		return err
	}
	return appendTechnicalProvenance(ctx, tx, projectMetadataEditHistoryTable, projectMetadataEditHistorySQL,
		string(proposal), "metadata edit")
}

func metadataAuditTextEqual(raw *string, value any) bool {
	expected := headerAuditValue(value)
	return raw == nil && expected == nil || raw != nil && expected == *raw
}
