package main

import (
	"database/sql"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
)

//go:embed resources/plot-quality-site.db
var qualityDatabase []byte

//go:embed resources/plot-quality-site-provenance.json
var qualityProvenanceJSON []byte

type PlotQualityChoice struct {
	RowID         string   `json:"rowId"`
	Code          *string  `json:"code"`
	ListName      *string  `json:"listName"`
	ListFilter    *string  `json:"listFilter"`
	ItemOrder     *float64 `json:"itemOrder"`
	Description   *string  `json:"description"`
	FieldUsedIn   *string  `json:"fieldUsedIn"`
	ValidateLoops *string  `json:"validateLoops"`
	Validate      *bool    `json:"validate"`
	Note          *string  `json:"note"`
	Flag          *bool    `json:"flag"`
	Selectable    bool     `json:"selectable"`
	Diagnostic    string   `json:"diagnostic"`
}

type QualityService struct {
	mu sync.RWMutex
	db *sql.DB
}

func NewQualityService(dataDir string) (*QualityService, error) {
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
	var provenance qualityProvenance
	if err := decodeQualityJSON(qualityProvenanceJSON, &provenance); err != nil {
		return nil, fmt.Errorf("quality catalogue provenance: %w", err)
	}
	if err := validateQualityProvenance(provenance); err != nil {
		return nil, err
	}
	if becHash(qualityDatabase) != provenance.DatabaseSHA256 {
		return nil, errors.New("embedded quality catalogue checksum mismatch")
	}
	target := filepath.Join(dataDir, "plot-quality-site.db")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(qualityDatabase)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return nil, errors.Join(writeErr, closeErr, os.Remove(target))
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return nil, err
	}
	if becHash(data) != provenance.DatabaseSHA256 {
		return nil, errors.New("existing quality catalogue checksum mismatch; file was not replaced")
	}
	db, err := openReadOnly(target)
	if err != nil {
		return nil, err
	}
	if err := validateQualityDatabase(db, provenance); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &QualityService{db: db}, nil
}

func (s *QualityService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func qualityChoiceStatus(row *PlotQualityChoice) {
	row.Selectable = false
	row.Diagnostic = ""
	if row.ListName == nil || *row.ListName != "PlotQualitySite" {
		row.Diagnostic = "Quality row does not belong to PlotQualitySite"
		return
	}
	if row.Code == nil {
		row.Diagnostic = "Quality code is NULL"
		return
	}
	if err := validateQualityValue("Item", row.Code); err != nil {
		row.Diagnostic = err.Error()
		return
	}
	row.Selectable = true
}

func qualityChoices(db headerDB) ([]PlotQualityChoice, error) {
	rows, err := db.Query(`SELECT RowID,Item,ListName,ListFilter,ItemOrder,ItemDescription,
 FieldUsedIn,ValidateLoops,CAST(Validate AS INTEGER),Note,CAST(Flag AS INTEGER)
 FROM PlotQualityChoices ORDER BY CAST(RowID AS INTEGER)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []PlotQualityChoice{}
	for rows.Next() {
		var row PlotQualityChoice
		var order []byte
		var validate, flag sql.NullInt64
		if err := rows.Scan(&row.RowID, &row.Code, &row.ListName, &row.ListFilter, &order,
			&row.Description, &row.FieldUsedIn, &row.ValidateLoops, &validate, &row.Note, &flag); err != nil {
			return nil, err
		}
		if order != nil {
			if len(order) != 8 {
				return nil, fmt.Errorf("quality ItemOrder at row %s is not exact IEEE64", row.RowID)
			}
			value := math.Float64frombits(binary.BigEndian.Uint64(order))
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return nil, fmt.Errorf("quality ItemOrder at row %s is not finite", row.RowID)
			}
			row.ItemOrder = &value
		}
		for _, field := range []struct {
			name string
			src  sql.NullInt64
			dst  **bool
		}{{"Validate", validate, &row.Validate}, {"Flag", flag, &row.Flag}} {
			if !field.src.Valid {
				continue
			}
			if field.src.Int64 != 0 && field.src.Int64 != 1 {
				return nil, fmt.Errorf("quality %s at row %s is not normalized Boolean", field.name, row.RowID)
			}
			value := field.src.Int64 == 1
			*field.dst = &value
		}
		qualityChoiceStatus(&row)
		result = append(result, row)
	}
	return result, rows.Err()
}

func (s *QualityService) ListPlotQualityChoices() ([]PlotQualityChoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, errors.New("quality catalogue is closed")
	}
	return qualityChoices(s.db)
}
