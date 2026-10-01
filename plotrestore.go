package main

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

type AuditRestoreAction string

const (
	AuditRestoreCancel AuditRestoreAction = "cancel"
	AuditRestoreRetain AuditRestoreAction = "retain"
	AuditRestorePrune  AuditRestoreAction = "prune"
)

type AuditRestoreResult struct {
	Cancelled       bool `json:"cancelled"`
	RestoredRows    int  `json:"restoredRows"`
	PrunedAuditRows int  `json:"prunedAuditRows"`
	CleanedVegRows  int  `json:"cleanedVegRows"` // Compatibility only; field restoration never deletes Veg rows.
}

type auditRestorePlan struct {
	entry         AuditEntry
	rowID         int64
	table, column string
	member        reflect.Value
	before, after any
	when          time.Time
}

func auditRowIDs(rowIDs []string) ([]int64, error) {
	ids := make([]int64, 0, len(rowIDs))
	seen := map[int64]bool{}
	for _, raw := range rowIDs {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || strconv.FormatInt(id, 10) != raw {
			return nil, fmt.Errorf("invalid exact audit rowId %q", raw)
		}
		if seen[id] {
			return nil, fmt.Errorf("duplicate audit rowId %q", raw)
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids, nil
}

func auditRestoreValue(raw *string, member reflect.Value) (any, error) {
	kind := member.Kind()
	nullable := kind == reflect.Pointer
	if nullable {
		kind = member.Type().Elem().Kind()
	}
	if raw == nil {
		if !nullable {
			return nil, errors.New("required field cannot be restored to NULL")
		}
		return nil, nil
	}
	switch kind {
	case reflect.String:
		if !nullable && strings.TrimSpace(*raw) == "" {
			return nil, errors.New("required field cannot be blank")
		}
		return *raw, nil
	case reflect.Int:
		value, err := strconv.ParseInt(*raw, 10, strconv.IntSize)
		if err != nil {
			return nil, fmt.Errorf("invalid integer audit value %q", *raw)
		}
		return int(value), nil
	case reflect.Float64:
		value, err := strconv.ParseFloat(*raw, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("invalid finite numeric audit value %q", *raw)
		}
		return value, nil
	case reflect.Bool:
		switch *raw {
		case "-1", "1":
			return true, nil
		case "0":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid Access boolean audit value %q", *raw)
		}
	}
	return nil, errors.New("unsupported audit value type")
}

func readAuditEntries(db headerDB, project, predicate string, args ...any) ([]AuditEntry, error) {
	if !projectNamePattern.MatchString(project) {
		return nil, errors.New("invalid project name")
	}
	schema, err := db.Query(`PRAGMA table_info(` + quoteHeaderIdentifier(project+"_Audit") + `)`)
	if err != nil {
		return nil, err
	}
	columns := map[string]bool{}
	for schema.Next() {
		var cid, required, pk int
		var name, typ string
		var defaultValue sql.NullString
		if err := schema.Scan(&cid, &name, &typ, &required, &defaultValue, &pk); err != nil {
			schema.Close()
			return nil, err
		}
		columns[strings.ToLower(name)] = true
	}
	err = schema.Err()
	schema.Close()
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"Project", "User", "PlotNumber", "Table", "EditField", "EditWhen", "BeforeEdit", "AfterEdit", "Restore", "Flag", "ID"} {
		if !columns[strings.ToLower(name)] {
			return nil, fmt.Errorf("audit schema is missing verified column %q", name)
		}
	}
	flag := reflect.ValueOf(false)
	query := `SELECT rowid, "Project", "User", "PlotNumber", "Table", "EditField",
		CAST("EditWhen" AS TEXT), "BeforeEdit", "AfterEdit", ` +
		headerReadColumn(`"Restore"`, flag) + `, ` + headerReadColumn(`"Flag"`, flag) + `, "ID"
		FROM ` + quoteHeaderIdentifier(project+"_Audit") + ` WHERE ` + predicate +
		` ORDER BY "EditWhen" DESC, rowid DESC`
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	entries := []AuditEntry{}
	for rows.Next() {
		var entry AuditEntry
		var id int64
		if err := rows.Scan(&id, &entry.Project, &entry.User, &entry.PlotNumber,
			&entry.Table, &entry.EditField, &entry.EditWhen, &entry.BeforeEdit,
			&entry.AfterEdit, &entry.Restore, &entry.Flag, &entry.ID); err != nil {
			return nil, fmt.Errorf("invalid audit row: %w", err)
		}
		entry.RowID = strconv.FormatInt(id, 10)
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func selectedAuditEntries(tx *sql.Tx, project, plot string, ids []int64) ([]AuditEntry, error) {
	entries := make([]AuditEntry, 0, len(ids))
	for _, id := range ids {
		rows, err := readAuditEntries(tx, project, `rowid = ?`, id)
		if err != nil {
			return nil, err
		}
		if len(rows) != 1 {
			return nil, fmt.Errorf("audit rowId %d no longer exists", id)
		}
		entry := rows[0]
		if entry.Project != project || entry.PlotNumber != plot {
			return nil, fmt.Errorf("audit rowId %d does not belong to project %q plot %q", id, project, plot)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *PlotService) restorePlans(tx *sql.Tx, project, plot string, entries []AuditEntry) ([]auditRestorePlan, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	caps, err := headerCapabilities(tx, project)
	if err != nil {
		return nil, err
	}
	if !caps["plotNumber"] {
		return nil, errors.New("audit restore requires verified Env/Admin identity columns")
	}
	if err := childParent(tx, project, plot); err != nil {
		return nil, err
	}
	plans := make([]auditRestorePlan, 0, len(entries))
	childSchemas := map[string]map[string]bool{}
	for _, entry := range entries {
		if entry.Project != project || entry.PlotNumber != plot {
			return nil, fmt.Errorf("audit rowId %s has foreign project/plot ownership", entry.RowID)
		}
		plan := auditRestorePlan{entry: entry}
		plan.rowID, err = strconv.ParseInt(entry.RowID, 10, 64)
		if err != nil {
			return nil, err
		}
		for _, table := range []string{"Admin", "Env", "Humus", "Mineral", "Other", "Veg"} {
			if strings.EqualFold(entry.Table, "_"+table) || strings.EqualFold(entry.Table, project+"_"+table) {
				plan.table = table
				break
			}
		}
		if plan.table == "Env" || plan.table == "Admin" {
			if entry.ID != nil {
				return nil, fmt.Errorf("parent audit rowId %s must not have a child ID", entry.RowID)
			}
			for _, field := range headerFields {
				if field.table == plan.table && field.property != "plotNumber" &&
					strings.EqualFold(field.column, entry.EditField) && caps[field.property] {
					plan.column = field.column
					plan.member = reflect.ValueOf(FS882Header{}).FieldByName(field.member)
					break
				}
			}
		} else if fields, exists := childFields[plan.table]; exists {
			if entry.ID == nil || *entry.ID < math.MinInt32 || *entry.ID > math.MaxInt32 {
				return nil, fmt.Errorf("child audit rowId %s requires an exact signed32 child ID", entry.RowID)
			}
			schema := childSchemas[plan.table]
			if schema == nil {
				schema, err = childCapabilities(tx, project, plan.table)
				if err != nil {
					return nil, err
				}
				childSchemas[plan.table] = schema
			}
			var record any
			switch plan.table {
			case "Veg":
				record = VegRecord{}
			case "Humus":
				record = HumusRecord{}
			case "Mineral":
				record = MineralRecord{}
			case "Other":
				record = OtherRecord{}
			}
			for _, field := range fields {
				if strings.EqualFold(field.column, entry.EditField) && schema[strings.ToLower(field.column)] {
					plan.column = field.column
					plan.member = reflect.ValueOf(record).FieldByName(field.member)
					break
				}
			}
			if plan.table == "Veg" {
				// V7mdlAudit.RestoreAuditRecords jumps over every Cover* audit.
				// Reject the selection instead of silently pruning skipped history.
				if strings.HasPrefix(strings.ToLower(entry.EditField), "cover") {
					return nil, fmt.Errorf("Veg Cover* audit restoration is unsupported (rowId %s)", entry.RowID)
				}
			}
		}
		if plan.column == "" {
			return nil, fmt.Errorf("unsupported audit restore field %q.%q (rowId %s)", entry.Table, entry.EditField, entry.RowID)
		}
		plan.when, err = time.Parse("2006-01-02 15:04:05", entry.EditWhen)
		if err != nil {
			plan.when, err = time.Parse(time.RFC3339Nano, entry.EditWhen)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid audit EditWhen %q (rowId %s)", entry.EditWhen, entry.RowID)
		}
		plan.before, err = auditRestoreValue(entry.BeforeEdit, plan.member)
		if err == nil {
			plan.after, err = auditRestoreValue(entry.AfterEdit, plan.member)
		}
		if err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if reflect.DeepEqual(plan.before, plan.after) {
			return nil, fmt.Errorf("audit rowId %s does not describe a value change", entry.RowID)
		}
		if err := validateQualityRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateSubstrateRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := s.validateSiteCodeRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateRegionRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateSoilRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateGeologyRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateParentCodeRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if err := validateOrdinaryRestoreValue(plan.table, plan.column, plan.before); err != nil {
			return nil, fmt.Errorf("audit rowId %s: %w", entry.RowID, err)
		}
		if _, err := restoreCurrentValue(tx, project, plot, plan); err != nil {
			return nil, err
		}
		plans = append(plans, plan)
	}
	// Source: ORDER BY Table, EditWhen DESC. Normalize accepted table aliases
	// and use descending rowid as the stable reverse-edit tie breaker.
	sort.Slice(plans, func(i, j int) bool {
		if plans[i].table != plans[j].table {
			return plans[i].table < plans[j].table
		}
		if !plans[i].when.Equal(plans[j].when) {
			return plans[i].when.After(plans[j].when)
		}
		return plans[i].rowID > plans[j].rowID
	})
	type cell struct {
		table, column string
		id            int64
	}
	states := map[cell]any{}
	for _, plan := range plans {
		key := cell{table: plan.table, column: plan.column}
		if plan.entry.ID != nil {
			key.id = *plan.entry.ID
		}
		current, exists := states[key]
		if !exists {
			current, err = restoreCurrentValue(tx, project, plot, plan)
			if err != nil {
				return nil, err
			}
		}
		if !reflect.DeepEqual(current, plan.after) {
			return nil, fmt.Errorf("stale audit rowId %s: selected history does not match current %s.%s", plan.entry.RowID, plan.table, plan.column)
		}
		states[key] = plan.before
	}
	return plans, nil
}

func restoreTarget(plan auditRestorePlan, plot string) (string, []any) {
	if plan.table == "Admin" {
		return `"Plot" = ?`, []any{plot}
	}
	if plan.table == "Env" {
		return `"PlotNumber" = ?`, []any{plot}
	}
	return `"PlotNumber" = ? AND "ID" = ?`, []any{plot, *plan.entry.ID}
}

func restoreCurrentValue(tx *sql.Tx, project, plot string, plan auditRestorePlan) (any, error) {
	predicate, args := restoreTarget(plan, plot)
	table := quoteHeaderIdentifier(project + "_" + plan.table)
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+predicate, args...).Scan(&count); err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, fmt.Errorf("audit rowId %s target requires one existing %s identity, found %d", plan.entry.RowID, plan.table, count)
	}
	column := headerReadColumn(quoteHeaderIdentifier(plan.column), plan.member)
	if plan.member.Kind() == reflect.Pointer && plan.member.Type().Elem().Kind() == reflect.Bool {
		var raw *string
		if err := tx.QueryRow(`SELECT CAST(`+quoteHeaderIdentifier(plan.column)+` AS TEXT) FROM `+
			table+` WHERE `+predicate, args...).Scan(&raw); err != nil {
			return nil, err
		}
		if _, err := auditRestoreValue(raw, plan.member); err != nil {
			return nil, fmt.Errorf("invalid stored boolean restore target: %w", err)
		}
	}
	destination := reflect.New(plan.member.Type())
	if err := tx.QueryRow(`SELECT `+column+` FROM `+table+` WHERE `+predicate, args...).Scan(destination.Interface()); err != nil {
		return nil, fmt.Errorf("invalid stored restore target: %w", err)
	}
	value := destination.Elem()
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, nil
		}
		value = value.Elem()
	}
	return value.Interface(), nil
}

// SetAuditRestoreSelection atomically replaces this plot's stored Restore flags.
// Empty selection clears flags; foreign, missing, duplicate or unsupported rows
// fail without altering the existing selection. rowIds are exact decimal strings.
func (s *PlotService) SetAuditRestoreSelection(plotNumber string, rowIDs []string) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	if strings.TrimSpace(plotNumber) == "" {
		return errors.New("PlotNumber is required")
	}
	ids, err := auditRowIDs(rowIDs)
	if err != nil {
		return err
	}
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	entries, err := selectedAuditEntries(tx, project, plotNumber, ids)
	if err != nil {
		return err
	}
	if _, err := s.restorePlans(tx, project, plotNumber, entries); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE `+quoteHeaderIdentifier(project+"_Audit")+
		` SET "Restore" = 0 WHERE "Project" = ? AND "PlotNumber" = ?`, project, plotNumber); err != nil {
		return err
	}
	for _, id := range ids {
		result, err := tx.Exec(`UPDATE `+quoteHeaderIdentifier(project+"_Audit")+
			` SET "Restore" = -1 WHERE rowid = ? AND "Project" = ? AND "PlotNumber" = ?`, id, project, plotNumber)
		if err != nil {
			return err
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return fmt.Errorf("audit selection lost rowId %d", id)
		}
	}
	return tx.Commit()
}

// RestoreSelectedAuditRecords performs an explicit selection, independently of
// persisted Restore flags. Cancellation and an empty selection never mutate data.
// Unlike Access CleanVegPlot, field restoration never deletes vegetation records.
func (s *PlotService) RestoreSelectedAuditRecords(plotNumber string, rowIDs []string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	return s.restoreAuditSelection(plotNumber, rowIDs, action, false)
}

func (s *PlotService) restoreAuditSelection(plot string, rowIDs []string, action AuditRestoreAction, marked bool) (*AuditRestoreResult, error) {
	if err := s.requireContextEdit(); err != nil {
		return nil, err
	}
	switch action {
	case AuditRestoreCancel:
		return &AuditRestoreResult{Cancelled: true}, nil
	case AuditRestoreRetain, AuditRestorePrune:
	default:
		return nil, fmt.Errorf("invalid audit restore action %q", action)
	}
	if strings.TrimSpace(plot) == "" {
		return nil, errors.New("PlotNumber is required")
	}
	ids, err := auditRowIDs(rowIDs)
	if err != nil {
		return nil, err
	}
	if !marked && len(ids) == 0 {
		return &AuditRestoreResult{}, nil
	}
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var entries []AuditEntry
	if marked {
		entries, err = readAuditEntries(tx, project, `"PlotNumber" = ? AND "Restore" <> 0`, plot)
	} else {
		entries, err = selectedAuditEntries(tx, project, plot, ids)
	}
	if err != nil {
		return nil, err
	}
	plans, err := s.restorePlans(tx, project, plot, entries)
	if err != nil {
		return nil, err
	}
	result := &AuditRestoreResult{}
	for _, plan := range plans {
		current, err := restoreCurrentValue(tx, project, plot, plan)
		if err != nil {
			return nil, err
		}
		if !reflect.DeepEqual(current, plan.after) {
			return nil, fmt.Errorf("stale audit rowId %s: current %s.%s differs from AfterEdit", plan.entry.RowID, plan.table, plan.column)
		}
		predicate, args := restoreTarget(plan, plot)
		args = append([]any{headerStorageValue(plan.before)}, args...)
		update, err := tx.Exec(`UPDATE `+quoteHeaderIdentifier(project+"_"+plan.table)+
			` SET `+quoteHeaderIdentifier(plan.column)+` = ? WHERE `+predicate, args...)
		if err != nil {
			return nil, err
		}
		if count, err := update.RowsAffected(); err != nil || count != 1 {
			return nil, fmt.Errorf("audit restore lost target for rowId %s", plan.entry.RowID)
		}
		result.RestoredRows++
	}
	if action == AuditRestorePrune {
		for _, plan := range plans {
			deleted, err := tx.Exec(`DELETE FROM `+quoteHeaderIdentifier(project+"_Audit")+
				` WHERE rowid = ? AND "Project" = ? AND "PlotNumber" = ?`, plan.rowID, project, plot)
			if err != nil {
				return nil, err
			}
			if count, err := deleted.RowsAffected(); err != nil || count != 1 {
				return nil, fmt.Errorf("audit prune lost rowId %s", plan.entry.RowID)
			}
			result.PrunedAuditRows++
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}
