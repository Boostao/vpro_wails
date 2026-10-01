package main

import (
	"database/sql"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed resources/vlists.db
var vlistsFiles embed.FS

type SpeciesItem struct {
	Code           string `json:"code"`
	ScientificName string `json:"scientificName"`
	EnglishName    string `json:"englishName"`
	Lifeform       *int   `json:"lifeform,omitempty"`
	FamilyCode     string `json:"familyCode,omitempty"`
	Authority      string `json:"authority,omitempty"`
}

type ListItem struct {
	ListName        string   `json:"listName"`
	Item            string   `json:"item"`
	ItemDescription string   `json:"itemDescription"`
	ItemOrder       *float64 `json:"itemOrder,omitempty"`
	FieldUsedIn     string   `json:"fieldUsedIn,omitempty"`
}

type ReferenceService struct {
	mu      sync.RWMutex
	dataDir string
	db      *sql.DB
}

func NewReferenceService(dataDir string) (*ReferenceService, error) {
	if dataDir == "" {
		var err error
		dataDir, err = userDataDir()
		if err != nil {
			return nil, fmt.Errorf("determine data dir: %w", err)
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create reference data dir: %w", err)
	}

	targetPath := filepath.Join(dataDir, "vlists.db")
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		src, err := vlistsFiles.Open("resources/vlists.db")
		if err != nil {
			return nil, fmt.Errorf("read embedded vlists.db: %w", err)
		}
		defer src.Close()

		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return nil, fmt.Errorf("create vlists.db: %w", err)
		}
		defer dst.Close()

		if _, err := io.Copy(dst, src); err != nil {
			return nil, fmt.Errorf("copy vlists.db: %w", err)
		}
	}

	db, err := openReadOnly(targetPath)
	if err != nil {
		return nil, fmt.Errorf("open vlists.db: %w", err)
	}

	return &ReferenceService{
		dataDir: dataDir,
		db:      db,
	}, nil
}

func (s *ReferenceService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// SearchSpecies searches the BC master species list by code prefix or name fragment.
func (s *ReferenceService) SearchSpecies(query string, limit int) ([]SpeciesItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > 100 {
		limit = 30
	}
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	pattern := "%" + q + "%"
	prefix := strings.ToUpper(q) + "%"

	sqlQuery := `
		SELECT Code, ScientificName, EnglishName, Lifeform, FamilyCode, Authority
		FROM Species
		WHERE Code LIKE ? OR ScientificName LIKE ? OR EnglishName LIKE ?
		ORDER BY
			CASE WHEN Code LIKE ? THEN 0 ELSE 1 END,
			ScientificName
		LIMIT ?`

	rows, err := s.db.Query(sqlQuery, prefix, pattern, pattern, prefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []SpeciesItem
	for rows.Next() {
		var sp SpeciesItem
		var lf sql.NullInt64
		if err := rows.Scan(&sp.Code, &sp.ScientificName, &sp.EnglishName, &lf, &sp.FamilyCode, &sp.Authority); err != nil {
			return nil, err
		}
		if lf.Valid {
			v := int(lf.Int64)
			sp.Lifeform = &v
		}
		list = append(list, sp)
	}
	return list, nil
}

// GetSpecies returns full details for a single species by code.
func (s *ReferenceService) GetSpecies(code string) (*SpeciesItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return nil, nil
	}

	var sp SpeciesItem
	var lf sql.NullInt64
	err := s.db.QueryRow(`
		SELECT Code, ScientificName, EnglishName, Lifeform, FamilyCode, Authority
		FROM Species
		WHERE Code = ?`, c).Scan(&sp.Code, &sp.ScientificName, &sp.EnglishName, &lf, &sp.FamilyCode, &sp.Authority)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if lf.Valid {
		v := int(lf.Int64)
		sp.Lifeform = &v
	}
	return &sp, nil
}

// GetListItems returns sorted options for a specific VPRO dropdown list.
func (s *ReferenceService) GetListItems(listName string) ([]ListItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lname := strings.TrimSpace(listName)
	if lname == "" {
		return nil, nil
	}

	rows, err := s.db.Query(`
		SELECT ListName, Item, ItemDescription, ItemOrder, FieldUsedIn
		FROM Lists
		WHERE ListName = ?
		ORDER BY ItemOrder, Item`, lname)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ListItem
	for rows.Next() {
		var it ListItem
		var ord sql.NullFloat64
		if err := rows.Scan(&it.ListName, &it.Item, &it.ItemDescription, &ord, &it.FieldUsedIn); err != nil {
			return nil, err
		}
		if ord.Valid {
			it.ItemOrder = &ord.Float64
		}
		list = append(list, it)
	}
	return list, nil
}

// GetAllLists returns all dropdown items grouped by list name.
func (s *ReferenceService) GetAllLists() (map[string][]ListItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`
		SELECT ListName, Item, ItemDescription, ItemOrder, FieldUsedIn
		FROM Lists
		ORDER BY ListName, ItemOrder, Item`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]ListItem)
	for rows.Next() {
		var it ListItem
		var ord sql.NullFloat64
		if err := rows.Scan(&it.ListName, &it.Item, &it.ItemDescription, &ord, &it.FieldUsedIn); err != nil {
			return nil, err
		}
		if ord.Valid {
			it.ItemOrder = &ord.Float64
		}
		result[it.ListName] = append(result[it.ListName], it)
	}
	return result, nil
}
