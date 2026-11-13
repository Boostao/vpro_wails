package references

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
)

import (
	"bytes"
	"context"
	"database/sql"

	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/boostao/vpro-wails/resources"
	_ "github.com/mattn/go-sqlite3"
)

var vlistsFiles = resources.VListsFiles

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

const legacyVListsReferenceSHA256 = "abf77c39d1591441390e7de5302748b24a4a789e23fe3966daf3f198e5058f2e"

func NewReferenceService(dataDir string) (*ReferenceService, error) {
	if dataDir == "" {
		var err error
		dataDir, err = sqlutil.UserDataDir()
		if err != nil {
			return nil, fmt.Errorf("determine data dir: %w", err)
		}
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create reference data dir: %w", err)
	}

	targetPath := filepath.Join(dataDir, "vlists.db")
	sqliteMagic := []byte("SQLite format 3\x00")
	loadValidEmbedded := func() ([]byte, error) {
		embedded, err := vlistsFiles.ReadFile("resources/vlists.db")
		if err != nil {
			return nil, fmt.Errorf("read embedded vlists.db: %w", err)
		}
		if len(embedded) < 100 || !bytes.Equal(embedded[:16], sqliteMagic) {
			return nil, fmt.Errorf("embedded vlists.db has invalid SQLite header")
		}
		return embedded, nil
	}

	if info, err := os.Stat(targetPath); os.IsNotExist(err) {
		embedded, err := loadValidEmbedded()
		if err != nil {
			return nil, err
		}
		if err := sqlutil.ReplaceBundledCatalogueFile(targetPath, embedded, 0o644); err != nil {
			return nil, fmt.Errorf("install vlists.db: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("stat vlists.db: %w", err)
	} else {
		needsReplace := !info.Mode().IsRegular() || info.Size() < 100
		if !needsReplace {
			existing, readErr := os.ReadFile(targetPath)
			if readErr != nil || len(existing) < 100 || !bytes.Equal(existing[:16], sqliteMagic) {
				needsReplace = true
			} else if sqlutil.BecHash(existing) == legacyVListsReferenceSHA256 {
				needsReplace = true
			}
		}
		if needsReplace {
			embedded, err := loadValidEmbedded()
			if err != nil {
				return nil, err
			}
			if sqlutil.BecHash(embedded) != legacyVListsReferenceSHA256 || info.Size() < 100 {
				if err := sqlutil.ReplaceBundledCatalogueFile(targetPath, embedded, 0o644); err != nil {
					return nil, fmt.Errorf("upgrade vlists.db: %w", err)
				}
			}
		}
	}

	db, err := sqlutil.OpenReadOnly(targetPath)
	if err != nil {
		return nil, fmt.Errorf("open vlists.db: %w", err)
	}
	var speciesTables int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='Species'`).Scan(&speciesTables); err != nil || speciesTables != 1 {
		_ = db.Close()
		embedded, embErr := loadValidEmbedded()
		if embErr != nil {
			return nil, fmt.Errorf("verify vlists.db schema: %w", err)
		}
		if repErr := sqlutil.ReplaceBundledCatalogueFile(targetPath, embedded, 0o644); repErr != nil {
			return nil, fmt.Errorf("repair corrupt vlists.db: %w", repErr)
		}
		db, err = sqlutil.OpenReadOnly(targetPath)
		if err != nil {
			return nil, fmt.Errorf("reopen vlists.db: %w", err)
		}
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
func (s *ReferenceService) SearchSpecies(ctx context.Context, query string, limit int) ([]SpeciesItem, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("reference catalogue is closed")
	}

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

	rows, err := s.db.QueryContext(ctx, sqlQuery, prefix, pattern, pattern, prefix, limit)
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
	return list, rows.Err()
}

// GetSpecies returns full details for a single species by code.
func (s *ReferenceService) GetSpecies(ctx context.Context, code string) (*SpeciesItem, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("reference catalogue is closed")
	}

	c := strings.ToUpper(strings.TrimSpace(code))
	if c == "" {
		return nil, nil
	}

	var sp SpeciesItem
	var lf sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
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
func (s *ReferenceService) GetListItems(ctx context.Context, listName string) ([]ListItem, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("reference catalogue is closed")
	}

	lname := strings.TrimSpace(listName)
	if lname == "" {
		return nil, nil
	}

	rows, err := s.db.QueryContext(ctx, `
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
	return list, rows.Err()
}

// GetAllLists returns all dropdown items grouped by list name.
func (s *ReferenceService) GetAllLists(ctx context.Context) (map[string][]ListItem, error) {
	if err := sqlutil.AcquireReadLease(ctx, &s.mu); err != nil {
		return nil, err
	}
	defer s.mu.RUnlock()
	if s.db == nil {
		return nil, fmt.Errorf("reference catalogue is closed")
	}

	rows, err := s.db.QueryContext(ctx, `
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
	return result, rows.Err()
}
