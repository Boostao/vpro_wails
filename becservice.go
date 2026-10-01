package main

import (
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

//go:embed resources/bec.db
var becDatabase []byte

//go:embed resources/bec-provenance.json
var becProvenanceJSON []byte

type BECZone struct {
	Zone        *string `json:"zone"`
	Description *string `json:"description"`
}

type BECSubZone struct {
	RowID           string  `json:"rowId"`
	Zone            *string `json:"zone"`
	SubZone         *string `json:"subZone"`
	ZoneDescription *string `json:"zoneDescription"`
	Description     *string `json:"description"`
}

type BECSiteSeries struct {
	RowID            string   `json:"rowId"`
	SourceID         *string  `json:"sourceId"`
	SSCode           *string  `json:"ssCode"`
	Name             *string  `json:"name"`
	Region           *string  `json:"region"`
	Zone             *string  `json:"zone"`
	SubZone          *string  `json:"subZone"`
	BaseSubZone      *string  `json:"baseSubZone"`
	Variant          *string  `json:"variant"`
	Phase            *string  `json:"phase"`
	SiteSeries       *string  `json:"siteSeries"`
	SiteSeriesPhase  *string  `json:"siteSeriesPhase"`
	Variation        *string  `json:"variation"`
	Seral            *string  `json:"seral"`
	Description      *string  `json:"description"`
	PlantAssociation *string  `json:"plantAssociation"`
	Comments         *string  `json:"comments"`
	ReferenceID      *float64 `json:"referenceId"`
	AddedDate        *string  `json:"addedDate"`
	ExpiredDate      *string  `json:"expiredDate"`
	OriginalSourceID *string  `json:"originalSourceId"`
	MergedBGC        *string  `json:"mergedBGC"`
	Suballiance      *string  `json:"suballiance"`
	Alliance         *string  `json:"alliance"`
	MissingNpeNa     *string  `json:"missingNpeNa"`
	TransferID       *string  `json:"transferId"`
	Flag             *bool    `json:"flag"`
	Selectable       bool     `json:"selectable"`
	Diagnostic       string   `json:"diagnostic"`
}

type BECCatalogueStatus struct {
	Version          int    `json:"version"`
	ZoneRows         int    `json:"zoneRows"`
	SiteSeriesRows   int    `json:"siteSeriesRows"`
	DuplicateKeys    int    `json:"duplicateKeys"`
	UnselectableRows int    `json:"unselectableRows"`
	SourceSHA256     string `json:"sourceSha256"`
	SnapshotSHA256   string `json:"snapshotSha256"`
}

type BECService struct {
	mu     sync.RWMutex
	db     *sql.DB
	status BECCatalogueStatus
}

func NewBECService(dataDir string) (*BECService, error) {
	if dataDir == "" {
		var err error
		dataDir, err = userDataDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	var provenance becProvenance
	if err := json.Unmarshal(becProvenanceJSON, &provenance); err != nil {
		return nil, fmt.Errorf("BEC provenance: %w", err)
	}
	if becHash(becDatabase) != provenance.DatabaseSHA256 {
		return nil, errors.New("embedded BEC catalogue checksum mismatch")
	}
	target := filepath.Join(dataDir, "bec.db")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(becDatabase)
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			os.Remove(target)
			return nil, errors.Join(writeErr, closeErr)
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if becHash(data) != provenance.DatabaseSHA256 {
		return nil, errors.New("existing BEC catalogue checksum mismatch; file was not replaced")
	}
	db, err := openReadOnly(target)
	if err != nil {
		return nil, err
	}
	status, err := validateBECDatabase(db, provenance)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &BECService{db: db, status: status}, nil
}

func becHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (s *BECService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func (s *BECService) GetBECCatalogueStatus() (BECCatalogueStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return BECCatalogueStatus{}, errors.New("BEC catalogue is closed")
	}
	return s.status, nil
}

func (s *BECService) ListBECZones() ([]BECZone, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := []BECZone{}
	if s.db == nil {
		return nil, errors.New("BEC catalogue is closed")
	}
	rows, err := s.db.Query(`SELECT DISTINCT Zone, ZoneDescription FROM BECZoneList ORDER BY Zone COLLATE NOCASE, Zone, ZoneDescription`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row BECZone
		if err := rows.Scan(&row.Zone, &row.Description); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *BECService) ListBECSubZones(zone *string) ([]BECSubZone, error) {
	if err := validateBECCode("Zone", zone, 4); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("BEC catalogue is closed")
	}
	query := `SELECT RowID, Zone, SubZone, ZoneDescription, Description FROM BECZoneList`
	var args []any
	if zone != nil {
		query += ` WHERE Zone = ? COLLATE NOCASE`
		args = append(args, *zone)
	}
	query += ` ORDER BY Zone COLLATE NOCASE, Zone, SubZone COLLATE NOCASE, SubZone, CAST(RowID AS INTEGER)`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []BECSubZone{}
	for rows.Next() {
		var row BECSubZone
		if err := rows.Scan(&row.RowID, &row.Zone, &row.SubZone, &row.ZoneDescription, &row.Description); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

const becSeriesColumns = `RowID, SourceID, SSCode, Name, Region, Zone, SubZone, BaseSubZone, Variant, Phase, SiteSeries,
 SiteSeriesPhase, Variation, Seral, Description, PlantAssociation, Comments, ReferenceID, AddedDate, ExpiredDate,
 OriginalSourceID, MergedBGC, Suballiance, Alliance, MissingNpeNa, TransferID, Flag`

func becSeriesScan(row *BECSiteSeries) []any {
	return []any{&row.RowID, &row.SourceID, &row.SSCode, &row.Name, &row.Region, &row.Zone, &row.SubZone,
		&row.BaseSubZone, &row.Variant, &row.Phase, &row.SiteSeries, &row.SiteSeriesPhase, &row.Variation,
		&row.Seral, &row.Description, &row.PlantAssociation, &row.Comments, &row.ReferenceID, &row.AddedDate,
		&row.ExpiredDate, &row.OriginalSourceID, &row.MergedBGC, &row.Suballiance, &row.Alliance,
		&row.MissingNpeNa, &row.TransferID, &row.Flag}
}

func (s *BECService) ListBECSiteSeries(zone, subZone *string) ([]BECSiteSeries, error) {
	if err := validateBECCode("Zone", zone, 4); err != nil {
		return nil, err
	}
	if err := validateBECCode("SubZone", subZone, 8); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("BEC catalogue is closed")
	}
	result := []BECSiteSeries{}
	if zone == nil || subZone == nil {
		return result, nil
	}
	rows, err := s.db.Query(`SELECT `+becSeriesColumns+` FROM BECSiteSeries
 WHERE Zone = ? COLLATE NOCASE AND SubZone = ? COLLATE NOCASE
 ORDER BY SiteSeries COLLATE NOCASE, SiteSeries, CAST(RowID AS INTEGER)`, *zone, *subZone)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var row BECSiteSeries
		if err := rows.Scan(becSeriesScan(&row)...); err != nil {
			return nil, err
		}
		row.Selectable = row.SiteSeries != nil && validateBECCode("SiteSeries", row.SiteSeries, 5) == nil
		if !row.Selectable {
			row.Diagnostic = "Catalogue SiteSeries code is NULL, empty, or exceeds the stored five-character limit"
		}
		result = append(result, row)
	}
	return result, rows.Err()
}
