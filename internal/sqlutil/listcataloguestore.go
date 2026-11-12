package sqlutil

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

type catalogueStamp struct {
	info     os.FileInfo
	identity string
	change   uint64
}

func (s catalogueStamp) same(other catalogueStamp) bool {
	return s.info.Size() == other.info.Size() && s.info.ModTime().Equal(other.info.ModTime()) &&
		s.identity == other.identity && s.change == other.change && os.SameFile(s.info, other.info)
}

type ListCatalogueCache struct {
	Stamp       catalogueStamp `json:"-" yaml:"-"`
	provenance  listcatalog.Provenance
	Rows        []listcatalog.Choice `json:"-" yaml:"-"`
	Valid       bool                 `json:"-" yaml:"-"`
	Checksums   int                  `json:"-" yaml:"-"`
	Validations int                  `json:"-" yaml:"-"`
	BytesRead   int64                `json:"-" yaml:"-"`
}

func OpenListCatalogue(dataDir, filename, label string, database, provenanceJSON []byte, profile listcatalog.Profile) (string, listcatalog.Provenance, *ListCatalogueCache, error) {
	var p listcatalog.Provenance
	if dataDir == "" {
		var err error
		dataDir, err = UserDataDir()
		if err != nil {
			return "", p, nil, err
		}
	}
	p, err := listcatalog.DecodeProvenanceFor(provenanceJSON, profile)
	if err != nil {
		return "", p, nil, fmt.Errorf("%s catalogue provenance: %w", label, err)
	}
	if BecHash(database) != p.DatabaseSHA256 {
		return "", p, nil, fmt.Errorf("embedded %s catalogue checksum mismatch", label)
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return "", p, nil, err
	}
	target := filepath.Join(dataDir, filename)
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err == nil {
		_, writeErr := file.Write(database)
		if writeErr == nil {
			writeErr = file.Sync()
		}
		closeErr := file.Close()
		if writeErr != nil || closeErr != nil {
			return "", p, nil, errors.Join(writeErr, closeErr, os.Remove(target))
		}
	} else if errors.Is(err, os.ErrExist) {
		if profile.LegacyDatabaseSHA256 != "" {
			if existing, readErr := os.ReadFile(target); readErr == nil && BecHash(existing) == profile.LegacyDatabaseSHA256 {
				if err := ReplaceBundledCatalogueFile(target, database, 0644); err != nil {
					return "", p, nil, err
				}
			}
		}
	} else {
		return "", p, nil, err
	}
	cache := &ListCatalogueCache{}
	if _, err := cache.choices(target, p, profile, label, "", true); err != nil {
		return "", p, nil, err
	}
	return target, p, cache, nil
}

func ReplaceBundledCatalogueFile(target string, data []byte, perm os.FileMode) error {
	file, err := os.CreateTemp(filepath.Dir(target), ".catalogue-upgrade-*.db")
	if err != nil {
		return err
	}
	tempName := file.Name()
	defer os.Remove(tempName)
	if err := file.Chmod(perm); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, target)
}

func (c *ListCatalogueCache) choices(path string, p listcatalog.Provenance, profile listcatalog.Profile, label, list string, force bool) ([]listcatalog.Choice, error) {
	return c.ChoicesContext(context.Background(), path, p, profile, label, list, force)
}

func (c *ListCatalogueCache) ChoicesContext(ctx context.Context, path string, p listcatalog.Provenance, profile listcatalog.Profile, label, list string, force bool) ([]listcatalog.Choice, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	supported := list == ""
	for _, definition := range profile.Lists {
		supported = supported || definition.Name == list
	}
	if !supported {
		return nil, fmt.Errorf("unsupported site code list %q", list)
	}
	stamp, err := CatalogueFileStamp(path)
	if err != nil {
		c.Valid = false
		return nil, fmt.Errorf("%s catalogue: %w", label, err)
	}
	if force || !c.Valid || !stamp.same(c.Stamp) || !reflect.DeepEqual(p, c.provenance) {
		c.Valid = false
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s catalogue: %w", label, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.Checksums++
		c.BytesRead += int64(len(data))
		if BecHash(data) != p.DatabaseSHA256 {
			return nil, fmt.Errorf("existing %s catalogue checksum mismatch; file was not replaced", label)
		}
		candidate, err := OpenReadOnly(path)
		if err != nil {
			return nil, err
		}
		c.Validations++
		reader := ContextPlotDB{Db: candidate, Ctx: ctx}
		if err := listcatalog.ValidateDatabaseFor(reader, p, profile); err != nil {
			return nil, errors.Join(LabelledReadError(label+" catalogue", err), candidate.Close())
		}
		rows, err := listcatalog.ReadChoicesFor(reader, "", profile)
		if err != nil {
			return nil, errors.Join(LabelledReadError(label+" catalogue", err), candidate.Close())
		}
		hash, err := listcatalog.TypedHash(rows)
		if err != nil || hash != p.TypedCellsSHA256 {
			return nil, errors.Join(errors.New("catalogue snapshot typed-cell checksum mismatch"), err, candidate.Close())
		}
		current, err := CatalogueFileStamp(path)
		if err != nil || !stamp.same(current) {
			return nil, errors.Join(errors.New("catalogue changed during verification; retry"), err, candidate.Close())
		}
		if err := candidate.Close(); err != nil {
			return nil, fmt.Errorf("%s catalogue snapshot close: %w", label, err)
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		c.provenance = p
		c.provenance.SourceSchema = append([]listcatalog.Column(nil), p.SourceSchema...)
		c.Rows, c.Stamp, c.Valid = rows, current, true
	}
	result := []listcatalog.Choice{}
	for _, row := range c.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if list == "" || row.ListName != nil && *row.ListName == list {
			row.Code, row.ListName, row.ListFilter = CloneCatalogueValue(row.Code), CloneCatalogueValue(row.ListName), CloneCatalogueValue(row.ListFilter)
			row.Description, row.FieldUsedIn = CloneCatalogueValue(row.Description), CloneCatalogueValue(row.FieldUsedIn)
			row.ValidateLoops, row.Note = CloneCatalogueValue(row.ValidateLoops), CloneCatalogueValue(row.Note)
			row.ItemOrder, row.Validate, row.Flag = CloneCatalogueValue(row.ItemOrder), CloneCatalogueValue(row.Validate), CloneCatalogueValue(row.Flag)
			result = append(result, row)
		}
	}
	return result, nil
}

func CloneCatalogueValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func CheckListCatalogueChecksum(path string, p listcatalog.Provenance, label string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s catalogue: %w", label, err)
	}
	if BecHash(data) != p.DatabaseSHA256 {
		return fmt.Errorf("existing %s catalogue checksum mismatch; file was not replaced", label)
	}
	return nil
}
