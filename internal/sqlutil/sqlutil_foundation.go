package sqlutil

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

func BecHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var parentCodeListMaxByName = map[string]int{
	"SoilDrainage":         5,
	"BedrockType":          12,
	"FloodingRegimeDur":    2,
	"FloodingRegimeFreq":   7,
	"GeoMorPro":            3,
	"HumusForm":            4,
	"HumusFormPhase":       50,
	"HydrogeoSubsystem":    2,
	"HydrogeoSystem":       3,
	"RealmClass":           5,
	"RootRestrictingType":  1,
	"RootZoneParticleSize": 6,
	"SurfaceExp":           3,
	"SurficialMaterial":    6,
	"TerrainTexture":       3,
	"WaterSource":          5,
}

func ParentCodeListMaximum(list string) (int, bool) {
	max, ok := parentCodeListMaxByName[list]
	return max, ok
}

type HeaderDB interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func LabelledReadError(label string, err error) error {
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return context.DeadlineExceeded
	}
	return fmt.Errorf("%s: %w", label, err)
}

func AcquireReadLease(ctx context.Context, mutex *sync.RWMutex) error {
	return AcquireContextLock(ctx, mutex.TryRLock, mutex.RLock, mutex.RUnlock)
}

func AcquireMutexLease(ctx context.Context, mutex *sync.Mutex) error {
	return AcquireContextLock(ctx, mutex.TryLock, mutex.Lock, mutex.Unlock)
}

func AcquireContextLock(ctx context.Context, tryLock func() bool, lock, unlock func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if ctx.Done() == nil {
		lock()
		return nil
	}
	if tryLock() {
		if err := ctx.Err(); err != nil {
			unlock()
			return err
		}
		return nil
	}

	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if tryLock() {
				if err := ctx.Err(); err != nil {
					unlock()
					return err
				}
				return nil
			}
		}
	}
}

type contextSQLReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type ContextPlotDB struct {
	Db  contextSQLReader `json:"-" yaml:"-"`
	Ctx context.Context  `json:"-" yaml:"-"`
}

func UserDataDir() (string, error) {
	if directory := os.Getenv("VPRO_DATA_DIR"); directory != "" {
		return filepath.Abs(directory)
	}
	if runtime.GOOS == "windows" {
		if directory := os.Getenv("LOCALAPPDATA"); directory != "" {
			return filepath.Join(directory, "vpro"), nil
		}
	}
	if runtime.GOOS == "darwin" {
		directory, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(directory, "vpro"), nil
	}
	if runtime.GOOS == "linux" {
		if directory := os.Getenv("XDG_DATA_HOME"); directory != "" {
			return filepath.Join(directory, "vpro"), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(home, "AppData", "Local", "vpro"), nil
	}
	return filepath.Join(home, ".local", "share", "vpro"), nil
}

func UserConfigDir() (string, error) {
	if directory := os.Getenv("VPRO_CONFIG_DIR"); directory != "" {
		return filepath.Abs(directory)
	}
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "vpro"), nil
}

func OpenReadOnly(path string) (*sql.DB, error) {
	return OpenReadOnlyContext(context.Background(), path)
}

func OpenReadOnlyContext(ctx context.Context, path string) (*sql.DB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite3", SqliteFileURI(absolute, "ro"))
	if err == nil {
		database.SetMaxOpenConns(1)
		err = database.PingContext(ctx)
	}
	if err != nil {
		if database != nil {
			database.Close()
		}
		return nil, err
	}
	return database, nil
}

func ValidateQualityValue(name string, value *string) error {
	if value == nil {
		return nil
	}
	if !utf8.ValidString(*value) {
		return fmt.Errorf("%s must contain valid Unicode (invalid UTF-8)", name)
	}
	if *value == "" || len(utf16.Encode([]rune(*value))) > 15 {
		return fmt.Errorf("%s must be NULL or a nonempty string of at most 15 UTF-16 units", name)
	}
	return nil
}

func ValidateJSONTextProperties(data []byte, names map[string]string) error {
	var syntax json.RawMessage
	if err := json.Unmarshal(data, &syntax); err != nil {
		return err
	}
	if len(syntax) == 0 || syntax[0] != '{' {
		return nil
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return nil
	}
	seen := make(map[string]string)
	var duplicateErr error
	for decoder.More() {
		start := decoder.InputOffset()
		key, err := decoder.Token()
		if err != nil {
			return err
		}
		rawKey := strings.TrimSpace(string(data[start:decoder.InputOffset()]))
		for len(rawKey) > 0 && (rawKey[0] == ',' || rawKey[0] == '{') {
			rawKey = strings.TrimSpace(rawKey[1:])
		}
		if err := ValidateQualityJSONToken([]byte(rawKey)); err != nil {
			return err
		}
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return err
		}
		name, ok := key.(string)
		if !ok {
			continue
		}
		for property, label := range names {
			if strings.EqualFold(name, property) {
				if err := ValidateQualityJSONToken(raw); err != nil {
					return fmt.Errorf("%s: %w", label, err)
				}
				break
			}
		}
		folded := simpleFoldKey(name)
		if prev, dup := seen[folded]; dup && duplicateErr == nil && (prev == name || strings.EqualFold(prev, name)) {
			duplicateErr = fmt.Errorf("duplicate JSON object key %q (conflicts with %q)", name, prev)
		} else if !dup {
			seen[folded] = name
		}
	}
	return duplicateErr
}

func ValidateQualityJSONToken(raw []byte) error {
	if len(raw) == 0 || raw[0] != '"' {
		return nil
	}
	if len(raw) < 2 || raw[len(raw)-1] != '"' {
		return fmt.Errorf("unterminated quality JSON string")
	}
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8 in quality JSON string")
	}
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw)-1 {
			return fmt.Errorf("incomplete escape in quality JSON string")
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw)-1 {
			return fmt.Errorf("truncated Unicode escape in quality JSON string")
		}
		unit := qualityHexUnit(raw[i+1 : i+5])
		i += 4
		if unit >= 0xDC00 && unit <= 0xDFFF {
			return fmt.Errorf("unpaired low UTF-16 surrogate in quality JSON string")
		}
		if unit < 0xD800 || unit > 0xDBFF {
			continue
		}
		if i+6 >= len(raw)-1 || raw[i+1] != '\\' || raw[i+2] != 'u' {
			return fmt.Errorf("unpaired high UTF-16 surrogate in quality JSON string")
		}
		low := qualityHexUnit(raw[i+3 : i+7])
		if low < 0xDC00 || low > 0xDFFF {
			return fmt.Errorf("mismatched UTF-16 surrogate pair in quality JSON string")
		}
		i += 6
	}
	return nil
}

func qualityHexUnit(data []byte) uint16 {
	var value uint16
	for _, digit := range data {
		value <<= 4
		switch {
		case digit >= '0' && digit <= '9':
			value += uint16(digit - '0')
		case digit >= 'a' && digit <= 'f':
			value += uint16(digit-'a') + 10
		case digit >= 'A' && digit <= 'F':
			value += uint16(digit-'A') + 10
		}
	}
	return value
}

func ValidateSiteCodeText(name string, value *string, limit int) error {
	if value == nil {
		return nil
	}
	if !utf8.ValidString(*value) {
		return fmt.Errorf("%s must contain valid Unicode (invalid UTF-8)", name)
	}
	if *value == "" || len(utf16.Encode([]rune(*value))) > limit {
		return fmt.Errorf("%s must be NULL or a nonempty string of at most %d UTF-16 units", name, limit)
	}
	return nil
}

func SqliteFileURI(path, mode string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	if !strings.HasPrefix(clean, "/") {
		clean = "/" + clean
	}
	address := url.URL{Scheme: "file", Path: clean}
	query := url.Values{"mode": {mode}}
	address.RawQuery = query.Encode()
	return address.String()
}

func (db ContextPlotDB) Query(query string, args ...any) (*sql.Rows, error) {
	return db.Db.QueryContext(db.Ctx, query, args...)
}

func (db ContextPlotDB) QueryRow(query string, args ...any) *sql.Row {
	return db.Db.QueryRowContext(db.Ctx, query, args...)
}
