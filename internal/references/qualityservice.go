package references

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"context"
	"database/sql"

	"encoding/binary"
	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"math"
	"os"
	"path/filepath"
	"sync"
)

var QualityDatabase = resources.PlotQualitySiteDB

var QualityProvenanceJSON = resources.PlotQualitySiteProvenanceJSON

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
	Mu sync.RWMutex `json:"-" yaml:"-"`
	Db *sql.DB      `json:"-" yaml:"-"`
}

func NewQualityService(dataDir string) (*QualityService, error) {
	if dataDir == "" {
		var err error
		dataDir, err = sqlutil.UserDataDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	var provenance QualityProvenance
	if err := DecodeQualityJSON(QualityProvenanceJSON, &provenance); err != nil {
		return nil, fmt.Errorf("quality catalogue provenance: %w", err)
	}
	if err := ValidateQualityProvenance(provenance); err != nil {
		return nil, err
	}
	if sqlutil.BecHash(QualityDatabase) != provenance.DatabaseSHA256 {
		return nil, errors.New("embedded quality catalogue checksum mismatch")
	}
	target := filepath.Join(dataDir, "plot-quality-site.db")
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(QualityDatabase)
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
	if sqlutil.BecHash(data) != provenance.DatabaseSHA256 {
		return nil, errors.New("existing quality catalogue checksum mismatch; file was not replaced")
	}
	db, err := sqlutil.OpenReadOnly(target)
	if err != nil {
		return nil, err
	}
	if err := ValidateQualityDatabase(db, provenance); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return &QualityService{Db: db}, nil
}

func (s *QualityService) Close() error {
	s.Mu.Lock()
	defer s.Mu.Unlock()
	if s.Db == nil {
		return nil
	}
	err := s.Db.Close()
	s.Db = nil
	return err
}

func QualityChoiceStatus(row *PlotQualityChoice) {
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
	if err := sqlutil.ValidateQualityValue("Item", row.Code); err != nil {
		row.Diagnostic = err.Error()
		return
	}
	row.Selectable = true
}

func QualityChoices(db sqlutil.HeaderDB) ([]PlotQualityChoice, error) {
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
		QualityChoiceStatus(&row)
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *QualityService) ListPlotQualityChoices(ctx context.Context) ([]PlotQualityChoice, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.Mu); err != nil {
		return nil, err
	}
	defer s.Mu.RUnlock()
	if s.Db == nil {
		return nil, errors.New("quality catalogue is closed")
	}
	return QualityChoices(sqlutil.ContextPlotDB{Db: s.Db, Ctx: ctx})
}
