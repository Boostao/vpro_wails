package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"
)

type childField struct {
	member string
	column string
}

// Verified against packaged XL ControlSource bindings and Sample SQLite/Access
// definitions. PH aliases HumusFormpH; VonPost matches stored lowercase vonPost.
// Cultural/Other and abundance flags in Veg are integers, not booleans.
var childFields = map[string][]childField{
	"Veg": {
		{"Species", "Species"}, {"Layer", "Layer"},
		{"Cover1", "Cover1"}, {"Cover2", "Cover2"}, {"Cover3", "Cover3"}, {"TotalA", "TotalA"},
		{"Cover4", "Cover4"}, {"Cover5", "Cover5"}, {"Cover5a", "Cover5a"},
		{"Cover5b", "Cover5b"}, {"Cover5c", "Cover5c"}, {"TotalB", "TotalB"},
		{"Cover6", "Cover6"}, {"Cover7", "Cover7"}, {"Cover8", "Cover8"},
		{"Cover9", "Cover9"}, {"Cover10", "Cover10"}, {"Collected", "Collected"},
		{"Height1", "Height1"}, {"Height2", "Height2"}, {"Height3", "Height3"},
		{"Height4", "Height4"}, {"Height5", "Height5"}, {"Height6", "Height6"},
		{"LL", "LL"}, {"AF", "AF"}, {"DC", "DC"}, {"UT", "UT"}, {"VI", "VI"},
		{"PV", "PV"}, {"PG", "PG"}, {"FFA", "FFA"},
		{"Cultural1", "Cultural1"}, {"Cultural2", "Cultural2"},
		{"Other1", "Other1"}, {"Other2", "Other2"},
	},
	"Humus": {
		{"Horizon", "Horizon"}, {"UpperDepth", "UpperDepth"}, {"LowerDepth", "LowerDepth"},
		{"PH", "HumusFormpH"}, {"Comment", "Comment"},
		{"HumusStructureDegree", "HumusStructureDegree"}, {"HumusStructureKind", "HumusStructureKind"},
		{"MycelAbundance", "MycelAbundance"}, {"FecalAbundance", "FecalAbundance"},
		{"RootsAbundance", "RootsAbundance"}, {"RootsSize", "RootsSize"}, {"VonPost", "vonPost"},
	},
	"Mineral": {
		{"Horizon", "Horizon"}, {"UpperDepth", "UpperDepth"}, {"LowerDepth", "LowerDepth"},
		{"Texture", "Texture"}, {"Colour", "Colour"}, {"Comments", "Comments"},
		{"PitDepthLimit", "PitDepthLimit"}, {"ASP", "ASP"},
		{"PercentCoarseFragsGravel", "PercentCoarseFragsGravel"},
		{"PercentCoarseFragsCobbles", "PercentCoarseFragsCobbles"},
		{"PercentCoarseFragsStones", "PercentCoarseFragsStones"},
		{"PercentCoarseFragsTotal", "PercentCoarseFragsTotal"},
		{"PercentCoarseFragsShape", "PercentCoarseFragsShape"},
		{"RootsAbundance", "RootsAbundance"}, {"RootsSize", "RootsSize"},
		{"MineralStructureClass", "MineralStructureClass"}, {"MineralStructureKind", "MineralStructureKind"},
		{"MineralFormpH", "MineralFormpH"},
	},
	"Other": {
		{"DataName", "DataName"}, {"DataItem", "DataItem"},
		{"UserItem1", "UserItem1"}, {"UserItem2", "UserItem2"}, {"UserItem3", "UserItem3"},
		{"UserFlag1", "UserFlag1"}, {"UserFlag2", "UserFlag2"}, {"UserFlag3", "UserFlag3"},
	},
}

var childLegacyFields = map[string]int{"Veg": 18, "Humus": 5, "Mineral": 6, "Other": 2}

// Existing properties retain replacement semantics. For newly mapped fields,
// omitted legacy JSON/Go payloads preserve stored values; explicit JSON null or
// nil on a loaded record clears them. Non-nil Go values are always supplied.
func childPresence(record any) map[string]bool {
	switch r := record.(type) {
	case VegRecord:
		return r.childPresent
	case HumusRecord:
		return r.childPresent
	case MineralRecord:
		return r.childPresent
	case OtherRecord:
		return r.childPresent
	}
	return nil
}

func setChildPresence(record any, present map[string]bool) {
	switch r := record.(type) {
	case *VegRecord:
		r.childPresent = present
	case *HumusRecord:
		r.childPresent = present
	case *MineralRecord:
		r.childPresent = present
	case *OtherRecord:
		r.childPresent = present
	}
}

func childJSONKey(value reflect.Value, field childField) string {
	member, _ := value.Type().FieldByName(field.member)
	return strings.Split(member.Tag.Get("json"), ",")[0]
}

func decodeChildJSON(data []byte, record any) (map[string]bool, error) {
	if err := json.Unmarshal(data, record); err != nil {
		return nil, err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return nil, err
	}
	present := make(map[string]bool, len(properties))
	for key := range properties {
		present[strings.ToLower(key)] = true
	}
	return present, nil
}

func (r *VegRecord) UnmarshalJSON(data []byte) error {
	type plain VegRecord
	var decoded plain
	present, err := decodeChildJSON(data, &decoded)
	if err != nil {
		return err
	}
	*r = VegRecord(decoded)
	r.childPresent = present
	return nil
}

func (r *HumusRecord) UnmarshalJSON(data []byte) error {
	type plain HumusRecord
	var decoded plain
	present, err := decodeChildJSON(data, &decoded)
	if err != nil {
		return err
	}
	*r = HumusRecord(decoded)
	r.childPresent = present
	return nil
}

func (r *MineralRecord) UnmarshalJSON(data []byte) error {
	type plain MineralRecord
	var decoded plain
	present, err := decodeChildJSON(data, &decoded)
	if err != nil {
		return err
	}
	*r = MineralRecord(decoded)
	r.childPresent = present
	return nil
}

func (r *OtherRecord) UnmarshalJSON(data []byte) error {
	type plain OtherRecord
	if err := validateJSONTextProperties(data, otherTextJSONNames); err != nil {
		return err
	}
	var decoded plain
	present, err := decodeChildJSON(data, &decoded)
	if err != nil {
		return err
	}
	*r = OtherRecord(decoded)
	r.childPresent = present
	return nil
}

func childCapabilities(db headerDB, project, kind string) (map[string]bool, error) {
	if !projectNamePattern.MatchString(project) {
		return nil, errors.New("invalid project name")
	}
	rows, err := db.Query(`PRAGMA table_info(` + quoteHeaderIdentifier(project+"_"+kind) + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := map[string]bool{}
	for rows.Next() {
		var cid, notNull, pk int
		var name, typ string
		var def sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &notNull, &def, &pk); err != nil {
			return nil, err
		}
		columns[strings.ToLower(name)] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if !columns["id"] || !columns["plotnumber"] {
		return nil, fmt.Errorf("%s is missing verified ID/PlotNumber identity columns", kind)
	}
	return columns, nil
}

// GetChildCapabilities reports exact DTO property support, without schema alias
// guessing. Unbound stored columns are not advertised as editable properties.
func (s *PlotService) GetChildCapabilities(kind string) (map[string]bool, error) {
	var record any
	switch kind {
	case "Veg":
		record = VegRecord{}
	case "Humus":
		record = HumusRecord{}
	case "Mineral":
		record = MineralRecord{}
	case "Other":
		record = OtherRecord{}
	default:
		return nil, fmt.Errorf("unsupported child kind %q", kind)
	}
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer release()
	caps, err := childCapabilities(s.readDB(db), project, kind)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{"id": true, "plotNumber": true}
	value := reflect.ValueOf(record)
	for _, field := range childFields[kind] {
		result[childJSONKey(value, field)] = caps[strings.ToLower(field.column)]
	}
	return result, nil
}

func listChildRecords[T any](s *PlotService, kind, plot string) ([]T, error) {
	db, project, release, err := s.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer release()
	caps, err := childCapabilities(s.readDB(db), project, kind)
	if err != nil {
		return nil, err
	}
	columns := []string{`"ID"`, `"PlotNumber"`}
	recordType := reflect.ValueOf(new(T)).Elem()
	var fields []childField
	for _, field := range childFields[kind] {
		if caps[strings.ToLower(field.column)] {
			fields = append(fields, field)
			columns = append(columns, headerReadColumn(quoteHeaderIdentifier(field.column), recordType.FieldByName(field.member)))
		}
	}
	order := `"ID"`
	if kind == "Veg" && caps["species"] {
		order = `"Species"`
	} else if caps["upperdepth"] {
		switch kind {
		case "Humus":
			order = `"UpperDepth" DESC, "ID"`
		case "Mineral":
			order = `"UpperDepth", "ID"`
		}
	}
	rows, err := s.readDB(db).Query(`SELECT `+strings.Join(columns, ",")+` FROM `+
		quoteHeaderIdentifier(project+"_"+kind)+` WHERE "PlotNumber" = ? ORDER BY `+order, plot)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var records []T
	for rows.Next() {
		var record T
		value := reflect.ValueOf(&record).Elem()
		var id sql.NullInt64
		destinations := []any{&id, value.FieldByName("PlotNumber").Addr().Interface()}
		present := map[string]bool{}
		for _, field := range fields {
			destinations = append(destinations, value.FieldByName(field.member).Addr().Interface())
			present[strings.ToLower(childJSONKey(value, field))] = true
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		if !id.Valid {
			detail := ""
			if kind == "Veg" {
				detail = fmt.Sprintf(" species %q", value.FieldByName("Species").String())
			}
			return nil, fmt.Errorf("%s row for plot %q%s has unsupported NULL ID; repair its identity before editing", kind, plot, detail)
		}
		value.FieldByName("ID").SetInt(id.Int64)
		setChildPresence(&record, present)
		records = append(records, record)
	}
	return records, rows.Err()
}

func childValue(record reflect.Value, field childField) any {
	value := record.FieldByName(field.member)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return value.Elem().Interface()
	}
	return value.Interface()
}

func childParent(tx *sql.Tx, project, plot string) error {
	for _, identity := range []struct{ table, key string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		var count int
		err := tx.QueryRow(`SELECT COUNT(*) FROM `+quoteHeaderIdentifier(project+"_"+identity.table)+
			` WHERE `+quoteHeaderIdentifier(identity.key)+` = ?`, plot).Scan(&count)
		if err != nil {
			return err
		}

		if count != 1 {
			return fmt.Errorf("plot %q requires exactly one Env and Admin parent", plot)
		}
	}
	return nil
}

func requireChildIdentity(tx *sql.Tx, table, kind, plot string, id int64) error {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, id).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%s identity (%q, %d) requires one existing row, found %d; reload before saving", kind, plot, id, count)
	}
	return nil
}

func auditChildFields(tx *sql.Tx, project, kind, plot string, id int64, fields []childField, before, after []any, user string, strength int, when string) error {
	for i, field := range fields {
		if !auditHeaderChange(before[i], after[i], strength) {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO `+quoteHeaderIdentifier(project+"_Audit")+
			` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
			project, user, plot, "_"+kind, field.column, when, headerAuditValue(before[i]), headerAuditValue(after[i]), id); err != nil {
			return err
		}
	}
	return nil
}

// Veg.ID is not a primary key. Allocate an unused positive ID without changing
// imported signed IDs or relying on SQLite rowid/autoincrement/MAX arithmetic.
// The reservation ledger also prevents a deleted identity being reassigned
// to a new row that a stale caller could subsequently overwrite.
// New IDs intentionally use positive Access LONG (signed32) range, so they
// remain exact in the unchanged numeric JS DTO. Imported signed IDs are kept.
func allocateChildID(tx *sql.Tx, table string) (int64, error) {
	rows, err := tx.Query(`SELECT "ID" FROM `+table+` WHERE "ID" > 0
		UNION SELECT "ID" FROM "__VPRO_ChildIdentity" WHERE "ChildTable" = ? AND "ID" > 0 ORDER BY "ID"`, table)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return availableChildID(rows, math.MaxInt32)
}

func availableChildID(rows *sql.Rows, limit int64) (int64, error) {
	var candidate int64 = 1
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		if id > candidate {
			break
		}
		if candidate == limit {
			return 0, errors.New("child ID space exhausted")
		}
		candidate++
	}
	return candidate, rows.Err()
}

type childSaveMode int

const (
	childSave childSaveMode = iota
	childUpdate
	childDelete
)

// Legacy fields are replaced; newly mapped fields honor supplied presence.
// Loaded DTOs include supported fields, so their nil values explicitly clear.
// Identity is the plot/ID pair: imported Veg IDs may repeat across plots, but
// ambiguous duplicates within one plot must not be edited or deleted.
func (s *PlotService) saveChild(kind string, record any, mode childSaveMode) error {
	if err := s.requireContextEdit(); err != nil {
		return err
	}
	deleting := mode == childDelete
	value := reflect.ValueOf(record)
	plot := value.FieldByName("PlotNumber").String()
	id := value.FieldByName("ID").Int()
	if strings.TrimSpace(plot) == "" {
		return errors.New("PlotNumber is required")
	}
	for _, field := range childFields[kind] {
		if deleting {
			continue
		}
		if kind == "Veg" && heightProperties[childJSONKey(value, field)] {
			continue
		}
		if number, ok := childValue(value, field).(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
			return fmt.Errorf("%s.%s must be finite", kind, field.column)
		}
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
	if err := childParent(tx, project, plot); err != nil {
		return err
	}
	caps, err := childCapabilities(tx, project, kind)
	if err != nil {
		return err
	}
	var fields []childField
	var columns []string
	var after []any
	present := childPresence(record)
	for i, field := range childFields[kind] {
		supplied := deleting || i < childLegacyFields[kind] ||
			childValue(value, field) != nil || present[strings.ToLower(childJSONKey(value, field))]
		if !caps[strings.ToLower(field.column)] {
			if !deleting && childValue(value, field) != nil {
				return fmt.Errorf("unsupported %s property %q: active project is missing verified column %q", kind, childJSONKey(value, field), field.column)
			}
			continue
		}
		if !supplied {
			continue
		}
		fields = append(fields, field)
		columns = append(columns, quoteHeaderIdentifier(field.column))
		if deleting {
			after = append(after, nil)
		} else {
			after = append(after, childValue(value, field))
		}
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID")
	)`); err != nil {
		return err
	}
	table := quoteHeaderIdentifier(project + "_" + kind)
	before := make([]any, len(fields))
	creating := mode == childSave && id == 0
	if creating {
		id, err = allocateChildID(tx, table)
		if err != nil {
			return err
		}
	} else {
		if err := requireChildIdentity(tx, table, kind, plot, id); err != nil {
			return err
		}
		// Preserve SQL NULL and normalize scalar types for audit comparison.
		destinations := make([]any, len(fields))
		readColumns := make([]string, len(fields))
		for i, field := range fields {
			member := value.FieldByName(field.member)
			readColumns[i] = headerReadColumn(columns[i], member)
			kind := member.Kind()
			if kind == reflect.Pointer {
				kind = member.Type().Elem().Kind()
			}
			switch kind {
			case reflect.Float64:
				destinations[i] = new(sql.NullFloat64)
			case reflect.Int:
				destinations[i] = new(sql.NullInt64)
			case reflect.Bool:
				destinations[i] = new(sql.NullBool)
			default:
				destinations[i] = new(sql.NullString)
			}
		}
		if len(fields) > 0 {
			if err := tx.QueryRow(`SELECT `+strings.Join(readColumns, ",")+` FROM `+table+
				` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, id).Scan(destinations...); err != nil {
				return err
			}
		}
		for i, destination := range destinations {
			switch v := destination.(type) {
			case *sql.NullFloat64:
				if v.Valid {
					before[i] = v.Float64
				}
			case *sql.NullString:
				if v.Valid {
					before[i] = v.String
				}
			case *sql.NullInt64:
				if v.Valid {
					before[i] = int(v.Int64)
				}
			case *sql.NullBool:
				if v.Valid {
					before[i] = v.Bool
				}
			}
		}
	}
	if kind == "Other" && !deleting {
		var changedFields []childField
		var changedColumns []string
		var changedBefore, changedAfter []any
		for i, field := range fields {
			if !creating && reflect.DeepEqual(before[i], after[i]) {
				continue
			}
			if err := validateOtherField(field.column, after[i]); err != nil {
				return err
			}
			changedFields = append(changedFields, field)
			changedColumns = append(changedColumns, columns[i])
			changedBefore = append(changedBefore, before[i])
			changedAfter = append(changedAfter, after[i])
		}
		fields, columns, before, after = changedFields, changedColumns, changedBefore, changedAfter
	}
	if kind == "Veg" && !deleting {
		for i, field := range fields {
			property := childJSONKey(value, field)
			if !heightProperties[property] {
				continue
			}
			var oldNumber, newNumber *float64
			if before[i] != nil {
				number := before[i].(float64)
				oldNumber = &number
			}
			if after[i] != nil {
				number := after[i].(float64)
				newNumber = &number
			}
			if err := validateHeightNumericChange(property, oldNumber, newNumber); err != nil {
				return fmt.Errorf("Veg identity (%q, %d): %w", plot, id, err)
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO "__VPRO_ChildIdentity" ("ChildTable","ID") VALUES (?,?)
		ON CONFLICT ("ChildTable","ID") DO NOTHING`, table, id); err != nil {
		return err
	}
	if !creating && !deleting && len(fields) == 0 {
		return tx.Commit()
	}
	var query string
	var args []any
	storedAfter := make([]any, len(after))
	for i, entry := range after {
		storedAfter[i] = headerStorageValue(entry)
	}
	switch {
	case deleting:
		query = `DELETE FROM ` + table + ` WHERE "PlotNumber" = ? AND "ID" = ?`
		args = []any{plot, id}
	case creating:
		marks := strings.TrimSuffix(strings.Repeat("?,", len(fields)+2), ",")
		query = `INSERT INTO ` + table + ` (` + strings.Join(append([]string{`"PlotNumber"`, `"ID"`}, columns...), ",") + `) VALUES (` + marks + `)`
		args = append([]any{plot, id}, storedAfter...)
	default:
		assignments := make([]string, len(columns))
		for i, column := range columns {
			assignments[i] = column + ` = ?`
		}
		query = `UPDATE ` + table + ` SET ` + strings.Join(assignments, ",") + ` WHERE "PlotNumber" = ? AND "ID" = ?`
		args = append(append([]any{}, storedAfter...), plot, id)
	}
	result, err := tx.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%s mutation expected one row, found %d", kind, affected)
	}
	s.mu.RLock()
	user, strength := s.currentUser, s.auditStrength
	s.mu.RUnlock()
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := auditChildFields(tx, project, kind, plot, id, fields, before, after, user, strength, now); err != nil {
		return err
	}
	return tx.Commit()
}
