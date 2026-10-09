package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"
)

type workingUnitSourceCopy struct {
	Path string `json:"Path"`
	Hash string `json:"Hash"`
}

type workingUnitSourceSchema struct {
	Kind            string   `json:"kind"`
	Table           string   `json:"table"`
	Name            string   `json:"name"`
	Connect         string   `json:"connect"`
	SourceTable     string   `json:"sourceTable"`
	Type            int      `json:"type"`
	Size            int      `json:"size"`
	Required        bool     `json:"required"`
	AllowZeroLength bool     `json:"allowZeroLength"`
	Default         string   `json:"default"`
	Unique          bool     `json:"unique"`
	Primary         bool     `json:"primary"`
	Fields          []string `json:"fields"`
}

type workingUnitSourceRow struct {
	SourceOrdinal            string  `json:"sourceOrdinal"`
	ID                       *string `json:"id"`
	SiteSeries               *string `json:"siteSeries"`
	SiteSeriesLongName       *string `json:"siteSeriesLongName"`
	SiteSeriesScientificName *string `json:"siteSeriesScientificName"`
	Level                    *int    `json:"level"`
}

type workingUnitSnapshot struct {
	Version                       int                                  `json:"version"`
	Exporter                      string                               `json:"exporter"`
	ReadOnlyNativeReferenceAccess bool                                 `json:"readOnlyNativeReferenceAccess"`
	SourceCopies                  []workingUnitSourceCopy              `json:"sourceCopies"`
	Schema                        map[string][]workingUnitSourceSchema `json:"schema"`
	Tables                        map[string][]workingUnitSourceRow    `json:"tables"`
	IdentityEncoding              string                               `json:"identityEncoding"`
	SourceContext                 string                               `json:"sourceContext"`
}

type workingUnitProvenance struct {
	Version          int                                  `json:"version"`
	Exporter         string                               `json:"exporter"`
	SnapshotSHA256   string                               `json:"snapshotSha256"`
	DatabaseSHA256   string                               `json:"databaseSha256"`
	MasterSHA256     string                               `json:"masterSha256"`
	UserSHA256       string                               `json:"userSha256"`
	MasterRows       int                                  `json:"masterRows"`
	UserRows         int                                  `json:"userRows"`
	MasterDuplicates int                                  `json:"masterDuplicates"`
	UserDuplicates   int                                  `json:"userDuplicates"`
	TypedCellsSHA256 string                               `json:"typedCellsSha256"`
	SourceCopies     []workingUnitSourceCopy              `json:"sourceCopies"`
	SourceSchema     map[string][]workingUnitSourceSchema `json:"sourceSchema"`
	IdentityEncoding string                               `json:"identityEncoding"`
	SourceContext    string                               `json:"sourceContext"`
}

func decodeWorkingUnitJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("unexpected trailing Working Unit JSON data")
	}
	return nil
}

func validateWorkingUnitSnapshot(data []byte, p workingUnitProvenance) (*workingUnitSnapshot, error) {
	if p.Version != 1 || p.Exporter != "native-access-dao-working-unit-v1" || becHash(data) != p.SnapshotSHA256 {
		return nil, errors.New("Working Unit snapshot checksum/provenance mismatch")
	}
	var snapshot workingUnitSnapshot
	if err := decodeWorkingUnitJSON(data, &snapshot); err != nil {
		return nil, fmt.Errorf("Working Unit snapshot: %w", err)
	}
	if snapshot.Version != p.Version || snapshot.Exporter != p.Exporter || !snapshot.ReadOnlyNativeReferenceAccess ||
		len(snapshot.Tables) != 2 || len(snapshot.Schema) != 2 ||
		snapshot.IdentityEncoding != "exact signed32 string, not float; frozen exact sourceOrdinal string retained" ||
		snapshot.SourceContext != "USysMasterSiteUnitList direct link to owned VLists.MasterSiteUnitList; USysUserSiteUnitList direct link to owned VUser.UserSiteUnitList" {
		return nil, errors.New("Working Unit snapshot source boundary mismatch")
	}
	found := map[string]bool{}
	for _, source := range snapshot.SourceCopies {
		name := strings.ToLower(source.Path[strings.LastIndexAny(source.Path, `\/`)+1:])
		expected := ""
		switch name {
		case "vlists.accda":
			expected = p.MasterSHA256
		case "vuser.accda":
			expected = p.UserSHA256
		default:
			continue
		}
		if found[name] || len(expected) != 64 || strings.ToLower(source.Hash) != expected {
			return nil, errors.New("Working Unit native reference provenance mismatch")
		}
		found[name] = true
	}
	if len(found) != 2 {
		return nil, errors.New("Working Unit native reference provenance missing")
	}
	for _, table := range []string{"MasterSiteUnitList", "UserSiteUnitList"} {
		schema := snapshot.Schema[table]
		if len(schema) != 8 || schema[0].Kind != "schema-table" || schema[0].Table != table ||
			schema[0].Connect != "" || schema[0].SourceTable != "" {
			return nil, fmt.Errorf("Working Unit source schema mismatch: %s", table)
		}
		names := []string{"ID", "SiteSeries", "SiteSeriesLongName", "SiteSeriesScientificName", "Level"}
		types, sizes := []int{4, 10, 10, 10, 3}, []int{4, 50, 120, 255, 2}
		defaults := []string{"GenUniqueID()", "", "", "", "11"}
		for i, name := range names {
			field := schema[i+1]
			if field.Kind != "schema-field" || field.Table != table || field.Name != name || field.Type != types[i] ||
				field.Size != sizes[i] || field.Required || field.AllowZeroLength != (types[i] == 10) || field.Default != defaults[i] {
				return nil, fmt.Errorf("Working Unit source field schema mismatch: %s.%s", table, name)
			}
		}
		for i := 6; i < 8; i++ {
			index := schema[i]
			if index.Kind != "schema-index" || index.Table != table || !reflect.DeepEqual(index.Fields, []string{"ID"}) ||
				(i == 7 && (!index.Primary || !index.Unique || index.Name != "PrimaryKey")) ||
				(i == 6 && (index.Primary || index.Name != "ID" || index.Unique != (table == "MasterSiteUnitList"))) {
				return nil, fmt.Errorf("Working Unit source identity index mismatch: %s", table)
			}
		}
		expected := p.MasterRows
		if table == "UserSiteUnitList" {
			expected = p.UserRows
		}
		rows := snapshot.Tables[table]
		if len(rows) != expected {
			return nil, fmt.Errorf("Working Unit source row count mismatch: %s", table)
		}
		ids := map[string]bool{}
		for i, row := range rows {
			if row.SourceOrdinal != strconv.Itoa(i+1) || row.ID == nil {
				return nil, fmt.Errorf("Working Unit exact source identity missing at %s/%d", table, i+1)
			}
			id, err := strconv.ParseInt(*row.ID, 10, 32)
			if err != nil || strconv.FormatInt(id, 10) != *row.ID || ids[*row.ID] {
				return nil, fmt.Errorf("Working Unit signed32 identity invalid/duplicate at %s/%s", table, row.SourceOrdinal)
			}
			ids[*row.ID] = true
			for j, text := range []*string{row.SiteSeries, row.SiteSeriesLongName, row.SiteSeriesScientificName} {
				if text != nil && len(utf16.Encode([]rune(*text))) > sizes[j+1] {
					return nil, fmt.Errorf("Working Unit source string exceeds schema at %s/%s", table, row.SourceOrdinal)
				}
			}
			if row.Level != nil && (*row.Level < -32768 || *row.Level > 32767) {
				return nil, errors.New("Working Unit source Level is not signed16")
			}
		}
	}
	return &snapshot, nil
}

const workingUnitSchema = `
CREATE TABLE WorkingUnits(
 Origin TEXT NOT NULL CHECK(Origin IN ('master','user')),
 RowID TEXT NOT NULL,
 SourceID TEXT,
 Code TEXT,
 Description TEXT,
 ScientificName TEXT,
 Level INTEGER CHECK(Level IS NULL OR Level BETWEEN -32768 AND 32767),
 PRIMARY KEY(Origin,RowID), UNIQUE(Origin,SourceID));
CREATE INDEX WorkingUnitChoices ON WorkingUnits(Origin,Level,Code COLLATE NOCASE);
CREATE TABLE WorkingUnitProvenance(Version INTEGER NOT NULL, Exporter TEXT NOT NULL,
 SnapshotSHA256 TEXT NOT NULL, MasterSHA256 TEXT NOT NULL, UserSHA256 TEXT NOT NULL);
`

type workingUnitDB interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

// The import boundary accepts the evidenced typed DAO snapshot, never an Access file.
func importWorkingUnitSnapshot(db *sql.DB, data []byte, p workingUnitProvenance) error {
	snapshot, err := validateWorkingUnitSnapshot(data, p)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(workingUnitSchema); err != nil {
		return err
	}
	statement, err := tx.Prepare(`INSERT INTO WorkingUnits VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer statement.Close()
	for _, table := range []string{"MasterSiteUnitList", "UserSiteUnitList"} {
		origin := "master"
		if table == "UserSiteUnitList" {
			origin = "user"
		}
		for _, row := range snapshot.Tables[table] {
			if _, err := statement.Exec(origin, row.SourceOrdinal, row.ID, row.SiteSeries, row.SiteSeriesLongName, row.SiteSeriesScientificName, row.Level); err != nil {
				return err
			}
		}
	}
	if _, err := tx.Exec(`INSERT INTO WorkingUnitProvenance VALUES(?,?,?,?,?)`, p.Version, p.Exporter, p.SnapshotSHA256, p.MasterSHA256, p.UserSHA256); err != nil {
		return err
	}
	if err := validateWorkingUnitDatabase(tx, p); err != nil {
		return err
	}
	for _, table := range []string{"MasterSiteUnitList", "UserSiteUnitList"} {
		origin := "master"
		if table == "UserSiteUnitList" {
			origin = "user"
		}
		rows, err := tx.Query(`SELECT RowID,SourceID,Code,Description,ScientificName,Level FROM WorkingUnits WHERE Origin=? ORDER BY CAST(RowID AS INTEGER)`, origin)
		if err != nil {
			return err
		}
		i := 0
		for rows.Next() {
			var actual workingUnitSourceRow
			if err := rows.Scan(&actual.SourceOrdinal, &actual.ID, &actual.SiteSeries, &actual.SiteSeriesLongName, &actual.SiteSeriesScientificName, &actual.Level); err != nil {
				rows.Close()
				return err
			}
			if i >= len(snapshot.Tables[table]) || !reflect.DeepEqual(actual, snapshot.Tables[table][i]) {
				rows.Close()
				return fmt.Errorf("Working Unit source cell preservation failure at %s/%d", table, i+1)
			}
			i++
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if i != len(snapshot.Tables[table]) {
			return errors.New("Working Unit import lost source rows")
		}
	}
	return tx.Commit()
}

func validateWorkingUnitDatabase(db workingUnitDB, p workingUnitProvenance) error {
	var count, version int
	var exporter, snapshot, master, user string
	if err := db.QueryRow(`SELECT COUNT(*) FROM WorkingUnitProvenance`).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("Working Unit provenance missing/ambiguous: %v", err)
	}
	if err := db.QueryRow(`SELECT Version,Exporter,SnapshotSHA256,MasterSHA256,UserSHA256 FROM WorkingUnitProvenance`).Scan(&version, &exporter, &snapshot, &master, &user); err != nil {
		return err
	}
	if version != p.Version || exporter != p.Exporter || snapshot != p.SnapshotSHA256 || master != p.MasterSHA256 || user != p.UserSHA256 {
		return errors.New("Working Unit database provenance mismatch")
	}
	for _, item := range []struct {
		origin           string
		rows, duplicates int
	}{{"master", p.MasterRows, p.MasterDuplicates}, {"user", p.UserRows, p.UserDuplicates}} {
		if err := db.QueryRow(`SELECT COUNT(*) FROM WorkingUnits WHERE Origin=?`, item.origin).Scan(&count); err != nil || count != item.rows {
			return fmt.Errorf("Working Unit %s catalogue count mismatch: %v", item.origin, err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM (SELECT Code FROM WorkingUnits WHERE Origin=? GROUP BY Code HAVING COUNT(*)>1)`, item.origin).Scan(&count); err != nil || count != item.duplicates {
			return fmt.Errorf("Working Unit %s duplicate count mismatch: %v", item.origin, err)
		}
	}
	if p.TypedCellsSHA256 != "" {
		tables, err := workingUnitSourceTables(db)
		if err != nil {
			return err
		}
		data, err := json.Marshal(tables)
		if err != nil {
			return err
		}
		if becHash(data) != p.TypedCellsSHA256 {
			return errors.New("Working Unit typed source cell integrity mismatch")
		}
	}
	return nil
}

func workingUnitSourceTables(db workingUnitDB) (map[string][]workingUnitSourceRow, error) {
	tables := map[string][]workingUnitSourceRow{}
	for _, item := range []struct{ table, origin string }{{"MasterSiteUnitList", "master"}, {"UserSiteUnitList", "user"}} {
		rows, err := db.Query(`SELECT RowID,SourceID,Code,Description,ScientificName,Level FROM WorkingUnits WHERE Origin=? ORDER BY CAST(RowID AS INTEGER)`, item.origin)
		if err != nil {
			return nil, err
		}
		table := []workingUnitSourceRow{}
		for rows.Next() {
			var row workingUnitSourceRow
			if err := rows.Scan(&row.SourceOrdinal, &row.ID, &row.SiteSeries, &row.SiteSeriesLongName, &row.SiteSeriesScientificName, &row.Level); err != nil {
				rows.Close()
				return nil, err
			}
			table = append(table, row)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		tables[item.table] = table
	}
	return tables, nil
}
