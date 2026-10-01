package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
)

type becSourceColumn struct {
	Name    string `json:"name"`
	DAOType int    `json:"daoType"`
	Size    int    `json:"size"`
}

type becSnapshot struct {
	Version            int               `json:"version"`
	Exporter           string            `json:"exporter"`
	SourceAccessSHA256 string            `json:"sourceAccessSha256"`
	ZoneSchema         []becSourceColumn `json:"zoneSchema"`
	SeriesSchema       []becSourceColumn `json:"seriesSchema"`
	Zones              []BECSubZone      `json:"zones"`
	SiteSeries         []BECSiteSeries   `json:"siteSeries"`
}

type becProvenance struct {
	Version        int    `json:"version"`
	Exporter       string `json:"exporter"`
	SourceSHA256   string `json:"sourceSha256"`
	SnapshotSHA256 string `json:"snapshotSha256"`
	DatabaseSHA256 string `json:"databaseSha256"`
	ZoneRows       int    `json:"zoneRows"`
	SiteSeriesRows int    `json:"siteSeriesRows"`
	DuplicateKeys  int    `json:"duplicateKeys"`
}

func validateBECCode(name string, value *string, maximum int) error {
	if value == nil {
		return nil
	}
	if *value == "" || len(utf16.Encode([]rune(*value))) > maximum {
		return fmt.Errorf("%s must be NULL or a nonempty string of at most %d characters", name, maximum)
	}
	return nil
}

func becExpectedSchema() ([]becSourceColumn, []becSourceColumn) {
	zones := []becSourceColumn{{"Zone", 10, 4}, {"SubZone", 10, 10}, {"ZoneDescription", 10, 50}, {"SubZoneVarDescription", 10, 80}}
	names := []string{"SiteSeries_ID", "SSCode", "BGCName", "BEC_Region", "BGC_Zone", "SubzVarPh", "BGC_Subzone", "BGC_Variant",
		"BGC_Phase", "SiteSeries", "SSPhase", "SSVariation", "Seral", "SiteSeriesDescription", "PlantAssociation", "Comments",
		"Ref_ID", "AddedDate", "ExpiredDate", "O_SS_ID", "MergedBGC_SS", "BECSuballiance", "BECAlliance", "MissingNpeNa", "TransferID", "Flag"}
	series := make([]becSourceColumn, len(names))
	for i, name := range names {
		series[i] = becSourceColumn{name, 10, 255}
		switch name {
		case "SiteSeries_ID", "Ref_ID", "O_SS_ID":
			series[i].DAOType, series[i].Size = 7, 8
		case "AddedDate":
			series[i].DAOType, series[i].Size = 8, 8
		case "Flag":
			series[i].DAOType, series[i].Size = 1, 1
		}
	}
	return zones, series
}

func validateBECSnapshot(data []byte, provenance becProvenance) (*becSnapshot, error) {
	if provenance.Version != 1 || provenance.Exporter != "native-access-dao-bec-v1" ||
		len(provenance.SourceSHA256) != 64 || becHash(data) != provenance.SnapshotSHA256 {
		return nil, errors.New("BEC snapshot provenance/checksum mismatch")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var snapshot becSnapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return nil, fmt.Errorf("BEC snapshot: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("BEC snapshot contains trailing data")
	}
	zones, series := becExpectedSchema()
	if snapshot.Version != provenance.Version || snapshot.Exporter != provenance.Exporter ||
		snapshot.SourceAccessSHA256 != provenance.SourceSHA256 || !reflect.DeepEqual(snapshot.ZoneSchema, zones) ||
		!reflect.DeepEqual(snapshot.SeriesSchema, series) {
		return nil, errors.New("BEC snapshot source schema/provenance mismatch")
	}
	if len(snapshot.Zones) != provenance.ZoneRows || len(snapshot.SiteSeries) != provenance.SiteSeriesRows {
		return nil, errors.New("BEC snapshot row count mismatch")
	}
	for i, row := range snapshot.Zones {
		if row.RowID != strconv.Itoa(i+1) {
			return nil, fmt.Errorf("BEC zone source-row identity mismatch at %d", i+1)
		}
	}
	for i, row := range snapshot.SiteSeries {
		if row.RowID != strconv.Itoa(i+1) || row.Selectable || row.Diagnostic != "" {
			return nil, fmt.Errorf("BEC catalogue source-row identity/derived data mismatch at %d", i+1)
		}
		for _, identity := range []*string{row.SourceID, row.OriginalSourceID} {
			if identity != nil {
				value, err := strconv.ParseFloat(*identity, 64)
				if err != nil || strings.TrimSpace(*identity) != *identity || math.IsNaN(value) || math.IsInf(value, 0) {
					return nil, fmt.Errorf("BEC source Double identity is invalid at %s", row.RowID)
				}
			}
		}
		if row.ReferenceID != nil && (math.IsNaN(*row.ReferenceID) || math.IsInf(*row.ReferenceID, 0)) {
			return nil, fmt.Errorf("BEC reference Double is invalid at %s", row.RowID)
		}
		if row.AddedDate != nil {
			if _, err := time.Parse("2006-01-02T15:04:05.999999999", *row.AddedDate); err != nil {
				return nil, fmt.Errorf("BEC source wall-clock date is invalid at %s: %w", row.RowID, err)
			}
		}
	}
	return &snapshot, nil
}

const becSchemaSQL = `
CREATE TABLE BECZoneList (RowID TEXT PRIMARY KEY NOT NULL, Zone TEXT, SubZone TEXT, ZoneDescription TEXT, Description TEXT);
CREATE TABLE BECSiteSeries (
 RowID TEXT PRIMARY KEY NOT NULL, SourceID TEXT, SSCode TEXT, Name TEXT, Region TEXT, Zone TEXT, SubZone TEXT,
 BaseSubZone TEXT, Variant TEXT, Phase TEXT, SiteSeries TEXT, SiteSeriesPhase TEXT, Variation TEXT, Seral TEXT,
 Description TEXT, PlantAssociation TEXT, Comments TEXT, ReferenceID REAL, AddedDate TEXT, ExpiredDate TEXT,
 OriginalSourceID TEXT, MergedBGC TEXT, Suballiance TEXT, Alliance TEXT, MissingNpeNa TEXT, TransferID TEXT,
 Flag BOOLEAN CHECK (Flag IS NULL OR Flag IN (0,1)));
CREATE INDEX BECZoneLookup ON BECZoneList(Zone COLLATE NOCASE, SubZone COLLATE NOCASE);
CREATE INDEX BECSiteSeriesLookup ON BECSiteSeries(Zone COLLATE NOCASE, SubZone COLLATE NOCASE);
CREATE TABLE BECProvenance (Version INTEGER NOT NULL, Exporter TEXT NOT NULL, SourceSHA256 TEXT NOT NULL, SnapshotSHA256 TEXT NOT NULL);
`

// This boundary consumes an evidenced native DAO snapshot, not an Access file.
func importBECSnapshot(db *sql.DB, data []byte, provenance becProvenance) error {
	snapshot, err := validateBECSnapshot(data, provenance)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(becSchemaSQL); err != nil {
		return err
	}
	zones, err := tx.Prepare(`INSERT INTO BECZoneList VALUES (?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer zones.Close()
	for _, row := range snapshot.Zones {
		if _, err := zones.Exec(row.RowID, row.Zone, row.SubZone, row.ZoneDescription, row.Description); err != nil {
			return err
		}
	}
	series, err := tx.Prepare(`INSERT INTO BECSiteSeries (` + becSeriesColumns + `) VALUES (` + strings.TrimSuffix(strings.Repeat("?,", 27), ",") + `)`)
	if err != nil {
		return err
	}
	defer series.Close()
	for _, row := range snapshot.SiteSeries {
		v := reflect.ValueOf(row)
		args := make([]any, 27)
		for i := range args {
			args[i] = v.Field(i).Interface()
		}
		if _, err := series.Exec(args...); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO BECProvenance VALUES (?,?,?,?)`, provenance.Version, provenance.Exporter, provenance.SourceSHA256, provenance.SnapshotSHA256); err != nil {
		return err
	}
	if _, err := validateBECDatabase(tx, provenance); err != nil {
		return err
	}
	return tx.Commit()
}

type becQueryer interface {
	QueryRow(string, ...any) *sql.Row
}

func validateBECDatabase(db becQueryer, provenance becProvenance) (BECCatalogueStatus, error) {
	var version int
	var exporter, source, snapshot string
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM BECProvenance`).Scan(&count); err != nil || count != 1 {
		return BECCatalogueStatus{}, fmt.Errorf("BEC provenance row is missing/ambiguous: %v", err)
	}
	if err := db.QueryRow(`SELECT Version, Exporter, SourceSHA256, SnapshotSHA256 FROM BECProvenance`).Scan(&version, &exporter, &source, &snapshot); err != nil {
		return BECCatalogueStatus{}, err
	}
	if version != provenance.Version || exporter != provenance.Exporter || source != provenance.SourceSHA256 || snapshot != provenance.SnapshotSHA256 {
		return BECCatalogueStatus{}, errors.New("BEC database provenance mismatch")
	}
	status := BECCatalogueStatus{Version: version, SourceSHA256: source, SnapshotSHA256: snapshot}
	queries := []struct {
		sql  string
		dest *int
	}{
		{`SELECT COUNT(*) FROM BECZoneList`, &status.ZoneRows},
		{`SELECT COUNT(*) FROM BECSiteSeries`, &status.SiteSeriesRows},
		{`SELECT COUNT(*) FROM (SELECT Zone,SubZone,SiteSeries FROM BECSiteSeries GROUP BY Zone,SubZone,SiteSeries HAVING COUNT(*)>1)`, &status.DuplicateKeys},
		{`SELECT COUNT(*) FROM BECSiteSeries WHERE SiteSeries IS NULL OR SiteSeries='' OR length(SiteSeries)>5`, &status.UnselectableRows},
	}
	for _, query := range queries {
		if err := db.QueryRow(query.sql).Scan(query.dest); err != nil {
			return BECCatalogueStatus{}, err
		}
	}
	if status.ZoneRows != provenance.ZoneRows || status.SiteSeriesRows != provenance.SiteSeriesRows || status.DuplicateKeys != provenance.DuplicateKeys {
		return BECCatalogueStatus{}, errors.New("BEC database catalogue counts mismatch")
	}
	return status, nil
}
