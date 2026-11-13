package vprocore

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
)

type ChildField struct {
	Member string `json:"-" yaml:"-"`
	Column string `json:"-" yaml:"-"`
}

// Verified against packaged XL ControlSource bindings and Sample SQLite/Access
// definitions. PH aliases HumusFormpH; VonPost matches stored lowercase vonPost.
// Cultural/Other and abundance flags in Veg are integers, not booleans.
var ChildFields = map[string][]ChildField{
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

var ChildLegacyFields = map[string]int{"Veg": 18, "Humus": 5, "Mineral": 6, "Other": 2}

// Existing properties retain replacement semantics. For newly mapped fields,
// omitted legacy JSON/Go payloads preserve stored values; explicit JSON null or
// nil on a loaded record clears them. Non-nil Go values are always supplied.
func ChildPresence(record any) map[string]bool {
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

func SetChildPresence(record any, present map[string]bool) {
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

func ChildJSONKey(value reflect.Value, field ChildField) string {
	member, _ := value.Type().FieldByName(field.Member)
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
		name := key
		fields := reflect.TypeOf(record).Elem()
		for i := 0; i < fields.NumField(); i++ {
			tag := strings.Split(fields.Field(i).Tag.Get("json"), ",")[0]
			if tag != "" && strings.EqualFold(key, tag) {
				name = tag
				break
			}
		}
		present[strings.ToLower(name)] = true
	}
	return present, nil
}

func (r *VegRecord) UnmarshalJSON(data []byte) error {
	type plain VegRecord
	if err := sqlutil.ValidateJSONTextProperties(data, vegetationTextJSONNames); err != nil {
		return err
	}
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
	if err := sqlutil.ValidateJSONTextProperties(data, SoilChildJSONTextNames("Humus", HumusRecord{})); err != nil {
		return err
	}
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
	if err := sqlutil.ValidateJSONTextProperties(data, SoilChildJSONTextNames("Mineral", MineralRecord{})); err != nil {
		return err
	}
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
	if err := sqlutil.ValidateJSONTextProperties(data, otherTextJSONNames); err != nil {
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

func ChildCapabilities(db sqlutil.HeaderDB, project, kind string) (map[string]bool, error) {
	if !ProjectNamePattern.MatchString(project) {
		return nil, errors.New("invalid project name")
	}
	rows, err := db.Query(`PRAGMA table_info(` + QuoteHeaderIdentifier(project+"_"+kind) + `)`)
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
	db, project, release, err := s.GetActiveReadDB()
	if err != nil {
		return nil, err
	}
	defer release()
	caps, err := ChildCapabilities(s.ReadDB(db), project, kind)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{"id": true, "plotNumber": true}
	value := reflect.ValueOf(record)
	for _, field := range ChildFields[kind] {
		result[ChildJSONKey(value, field)] = caps[strings.ToLower(field.Column)]
	}
	return result, nil
}

func listChildRecords[T any](s *PlotService, kind, plot string) ([]T, error) {
	db, project, release, err := s.GetActiveReadDB()
	if err != nil {
		return nil, err
	}
	defer release()
	caps, err := ChildCapabilities(s.ReadDB(db), project, kind)
	if err != nil {
		return nil, err
	}
	columns := []string{`"ID"`, `"PlotNumber"`}
	recordType := reflect.ValueOf(new(T)).Elem()
	var fields []ChildField
	for _, field := range ChildFields[kind] {
		if caps[strings.ToLower(field.Column)] {
			fields = append(fields, field)
			columns = append(columns, headerReadColumn(QuoteHeaderIdentifier(field.Column), recordType.FieldByName(field.Member)))
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
	rows, err := s.ReadDB(db).Query(`SELECT `+strings.Join(columns, ",")+` FROM `+
		QuoteHeaderIdentifier(project+"_"+kind)+` WHERE "PlotNumber" = ? ORDER BY `+order, plot)
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
			destinations = append(destinations, value.FieldByName(field.Member).Addr().Interface())
			present[strings.ToLower(ChildJSONKey(value, field))] = true
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		if !id.Valid {
			detail := ""
			if kind == "Veg" {
				detail = fmt.Sprintf(" species %q", value.FieldByName("Species").String())
			}
			return nil, &childNullIdentityError{kind: kind, plot: plot, detail: detail}
		}
		value.FieldByName("ID").SetInt(id.Int64)
		SetChildPresence(&record, present)
		records = append(records, record)
	}
	return records, rows.Err()
}

func ChildValue(record reflect.Value, field ChildField) any {
	value := record.FieldByName(field.Member)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		return value.Elem().Interface()
	}
	return value.Interface()
}

func ChildParent(tx *sql.Tx, project, plot string) error {
	for _, identity := range []struct{ table, key string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		var count int
		err := tx.QueryRow(`SELECT COUNT(*) FROM `+QuoteHeaderIdentifier(project+"_"+identity.table)+
			` WHERE `+QuoteHeaderIdentifier(identity.key)+` = ?`, plot).Scan(&count)
		if err != nil {
			return err
		}

		if count != 1 {
			return fmt.Errorf("plot %q requires exactly one Env and Admin parent", plot)
		}
	}
	return nil
}

func RequireChildIdentity(tx *sql.Tx, table, kind, plot string, id int64) error {
	var count int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE "PlotNumber" = ? AND "ID" = ?`, plot, id).Scan(&count); err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("%s identity (%q, %d) requires one existing row, found %d; reload before saving", kind, plot, id, count)
	}
	return nil
}

func auditChildFields(tx *sql.Tx, project, kind, plot string, id int64, fields []ChildField, before, after []any, user string, strength int, when string) error {
	_, err := AuditChildFieldsTracked(tx, project, kind, plot, id, fields, before, after, user, strength, when)
	return err
}

type childAuditIdentity struct {
	RowID  string `json:"rowId"`
	Column string `json:"column"`
}

func AuditChildFieldsTracked(tx *sql.Tx, project, kind, plot string, id int64, fields []ChildField, before, after []any, user string, strength int, when string) ([]childAuditIdentity, error) {
	identities := []childAuditIdentity{}
	for i, field := range fields {
		if !AuditHeaderChange(before[i], after[i], strength) {
			continue
		}
		inserted, err := tx.Exec(`INSERT INTO `+QuoteHeaderIdentifier(project+"_Audit")+
			` ("Project","User","PlotNumber","Table","EditField","EditWhen","BeforeEdit","AfterEdit","Restore","Flag","ID") VALUES (?,?,?,?,?,?,?,?,0,0,?)`,
			project, user, plot, "_"+kind, field.Column, when, HeaderAuditValue(before[i]), HeaderAuditValue(after[i]), id)
		if err != nil {
			return nil, err
		}
		if count, err := inserted.RowsAffected(); err != nil || count != 1 {
			return nil, errors.Join(err, errors.New("child audit did not insert exactly one row"))
		}
		rowID, err := inserted.LastInsertId()
		if err != nil {
			return nil, err
		}
		identities = append(identities, childAuditIdentity{strconv.FormatInt(rowID, 10), field.Column})
	}
	return identities, nil
}

// Veg.ID is not a primary key. Allocate an unused positive ID without changing
// imported signed IDs or relying on SQLite rowid/autoincrement/MAX arithmetic.
// The reservation ledger also prevents a deleted identity being reassigned
// to a new row that a stale caller could subsequently overwrite.
// New IDs intentionally use positive Access LONG (signed32) range, so they
// remain exact in the unchanged numeric JS DTO. Imported signed IDs are kept.
func AllocateChildID(tx *sql.Tx, table string) (int64, error) {
	unquoted := strings.ReplaceAll(strings.Trim(table, `"`), `""`, `"`)
	query := `SELECT "ID" FROM ` + table + ` WHERE typeof("ID")='integer' AND "ID" > 0
		UNION SELECT "ID" FROM "__VPRO_ChildIdentity" WHERE "ChildTable" IN (?, ?) AND typeof("ID")='integer' AND "ID" > 0`
	args := []any{table, unquoted}

	if lastSep := strings.LastIndexByte(unquoted, '_'); lastSep > 0 {
		project := unquoted[:lastSep]
		suffix := unquoted[lastSep:]
		var hasAudit bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_master WHERE type='table' AND name=?)`, project+"_Audit").Scan(&hasAudit); err != nil {
			return 0, err
		}
		if hasAudit {
			auditTable := QuoteHeaderIdentifier(project + "_Audit")
			query += ` UNION SELECT "ID" FROM ` + auditTable + ` WHERE "Table" COLLATE NOCASE IN (?, ?) AND typeof("ID")='integer' AND "ID" > 0`
			args = append(args, suffix, unquoted)
		}
	}
	query += ` ORDER BY "ID"`
	rows, err := tx.Query(query, args...)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	return AvailableChildID(rows, math.MaxInt32)
}

func AvailableChildID(rows *sql.Rows, limit int64) (int64, error) {
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
	ChildDelete
)

// Legacy fields are replaced; newly mapped fields honor supplied presence.
// Loaded DTOs include supported fields, so their nil values explicitly clear.
// Identity is the plot/ID pair: imported Veg IDs may repeat across plots, but
// ambiguous duplicates within one plot must not be edited or deleted.
func (s *PlotService) saveChild(kind string, record any, mode childSaveMode) error {
	return s.SaveChildWithDeleteCheck(kind, record, mode, nil)
}

type childDeleteCheck func(*sql.Tx, string, string, int64) ([]ChildField, []any, error)
type childCreateCheck func(*sql.Tx, string, string, int64) error

func (s *PlotService) SaveChildWithDeleteCheck(kind string, record any, mode childSaveMode, check childDeleteCheck) error {
	return s.saveChildWithChecks(kind, record, mode, check, nil)
}

func (s *PlotService) saveChildWithChecks(kind string, record any, mode childSaveMode, check childDeleteCheck, createCheck childCreateCheck) error {
	if err := s.RequireContextEdit(); err != nil {
		return err
	}
	deleting := mode == ChildDelete
	value := reflect.ValueOf(record)
	plot := value.FieldByName("PlotNumber").String()
	id := value.FieldByName("ID").Int()
	if strings.TrimSpace(plot) == "" {
		return errors.New("PlotNumber is required")
	}
	for _, field := range ChildFields[kind] {
		if deleting {
			continue
		}
		if kind == "Veg" && HeightProperties[ChildJSONKey(value, field)] {
			continue
		}
		if kind == "Humus" || kind == "Mineral" {
			continue
		}
		if number, ok := ChildValue(value, field).(float64); ok && (math.IsNaN(number) || math.IsInf(number, 0)) {
			return fmt.Errorf("%s.%s must be finite", kind, field.Column)
		}
	}
	db, project, release, err := s.GetActiveDB()
	if err != nil {
		return err
	}
	defer release()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := ChildParent(tx, project, plot); err != nil {
		return err
	}
	caps, err := ChildCapabilities(tx, project, kind)
	if err != nil {
		return err
	}
	var fields []ChildField
	var columns []string
	var after []any
	present := ChildPresence(record)
	for i, field := range ChildFields[kind] {
		supplied := deleting || i < ChildLegacyFields[kind] ||
			ChildValue(value, field) != nil || present[strings.ToLower(ChildJSONKey(value, field))]
		if !caps[strings.ToLower(field.Column)] {
			if !deleting && ChildValue(value, field) != nil {
				return fmt.Errorf("unsupported %s property %q: active project is missing verified column %q", kind, ChildJSONKey(value, field), field.Column)
			}
			continue
		}
		if !supplied {
			continue
		}
		fields = append(fields, field)
		columns = append(columns, QuoteHeaderIdentifier(field.Column))
		if deleting {
			after = append(after, nil)
		} else {
			after = append(after, ChildValue(value, field))
		}
	}
	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS "__VPRO_ChildIdentity" (
		"ChildTable" TEXT NOT NULL, "ID" INTEGER NOT NULL, PRIMARY KEY ("ChildTable","ID")
	)`); err != nil {
		return err
	}
	table := QuoteHeaderIdentifier(project + "_" + kind)
	before := make([]any, len(fields))
	creating := mode == childSave && id == 0
	if createCheck != nil && !creating {
		return errors.New("child creation checks require a new automatically allocated identity")
	}
	if creating {
		id, err = AllocateChildID(tx, table)
		if err != nil {
			return err
		}
	} else {
		if err := RequireChildIdentity(tx, table, kind, plot, id); err != nil {
			return err
		}
		// Preserve SQL NULL and normalize scalar types for audit comparison.
		destinations := make([]any, len(fields))
		readColumns := make([]string, len(fields))
		for i, field := range fields {
			member := value.FieldByName(field.Member)
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
	if check != nil {
		if !deleting {
			return errors.New("child deletion checks require a deletion")
		}
		extraFields, extraBefore, err := check(tx, table, plot, id)
		if err != nil {
			return err
		}
		if len(extraFields) != len(extraBefore) {
			return errors.New("child deletion audit fields do not match their values")
		}
		fields = append(fields, extraFields...)
		before = append(before, extraBefore...)
		after = append(after, make([]any, len(extraFields))...)
	}
	if !deleting {
		var changedFields []ChildField
		var changedColumns []string
		var changedBefore, changedAfter []any
		for i, field := range fields {
			if !creating && reflect.DeepEqual(before[i], after[i]) {
				continue
			}
			if err := validateChildField(kind, field.Column, after[i]); err != nil {
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
			property := ChildJSONKey(value, field)
			if !HeightProperties[property] && field.Column != "Cover5a" && field.Column != "Cover5b" && field.Column != "Cover5c" {
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
			if err := ValidateHeightNumericChange(property, oldNumber, newNumber); err != nil {
				return fmt.Errorf("Veg identity (%q, %d): %w", plot, id, err)
			}
		}
	}
	if (kind == "Humus" || kind == "Mineral") && !deleting {
		depthOrHorizonModified := creating
		if !depthOrHorizonModified {
			for _, field := range fields {
				if field.Column == "Horizon" || field.Column == "UpperDepth" || field.Column == "LowerDepth" {
					depthOrHorizonModified = true
					break
				}
			}
		}
		if depthOrHorizonModified {
			var effHorizon *string
			var effUpper, effLower *float64
			if hVal := value.FieldByName("Horizon"); hVal.Kind() == reflect.Pointer && !hVal.IsNil() {
				s := hVal.Elem().String()
				effHorizon = &s
			}
			if uVal := value.FieldByName("UpperDepth"); uVal.Kind() == reflect.Pointer && !uVal.IsNil() {
				v := uVal.Elem().Float()
				effUpper = &v
			}
			if lVal := value.FieldByName("LowerDepth"); lVal.Kind() == reflect.Pointer && !lVal.IsNil() {
				v := lVal.Elem().Float()
				effLower = &v
			}
			if err := ValidateSoilHorizonDepthSign(kind, effHorizon, effUpper, effLower); err != nil {
				return fmt.Errorf("%s identity (%q, %d): %w", kind, plot, id, err)
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
		storedAfter[i] = HeaderStorageValue(entry)
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
	if createCheck != nil {
		if err := createCheck(tx, table, plot, id); err != nil {
			return err
		}
	}
	s.Mu.RLock()
	user, strength := s.CurrentUser, s.AuditStrength
	s.Mu.RUnlock()
	now := time.Now().Format("2006-01-02 15:04:05")
	if err := auditChildFields(tx, project, kind, plot, id, fields, before, after, user, strength, now); err != nil {
		return err
	}
	return tx.Commit()
}
