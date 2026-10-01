package main

import (
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

type listCatalogueCache struct {
	stamp       catalogueStamp
	provenance  listcatalog.Provenance
	rows        []listcatalog.Choice
	valid       bool
	checksums   int
	validations int
	bytesRead   int64
}

func openListCatalogue(dataDir, filename, label string, database, provenanceJSON []byte, profile listcatalog.Profile) (string, listcatalog.Provenance, *listCatalogueCache, error) {
	var p listcatalog.Provenance
	if dataDir == "" {
		var err error
		dataDir, err = userDataDir()
		if err != nil {
			return "", p, nil, err
		}
	}
	p, err := listcatalog.DecodeProvenanceFor(provenanceJSON, profile)
	if err != nil {
		return "", p, nil, fmt.Errorf("%s catalogue provenance: %w", label, err)
	}
	if becHash(database) != p.DatabaseSHA256 {
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
	} else if !errors.Is(err, os.ErrExist) {
		return "", p, nil, err
	}
	cache := &listCatalogueCache{}
	if _, err := cache.choices(target, p, profile, label, "", true); err != nil {
		return "", p, nil, err
	}
	return target, p, cache, nil
}

func (c *listCatalogueCache) choices(path string, p listcatalog.Provenance, profile listcatalog.Profile, label, list string, force bool) ([]listcatalog.Choice, error) {
	supported := list == ""
	for _, definition := range profile.Lists {
		supported = supported || definition.Name == list
	}
	if !supported {
		return nil, fmt.Errorf("unsupported site code list %q", list)
	}
	stamp, err := catalogueFileStamp(path)
	if err != nil {
		c.valid = false
		return nil, fmt.Errorf("%s catalogue: %w", label, err)
	}
	if force || !c.valid || !stamp.same(c.stamp) || !reflect.DeepEqual(p, c.provenance) {
		c.valid = false
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("%s catalogue: %w", label, err)
		}
		c.checksums++
		c.bytesRead += int64(len(data))
		if becHash(data) != p.DatabaseSHA256 {
			return nil, fmt.Errorf("existing %s catalogue checksum mismatch; file was not replaced", label)
		}
		candidate, err := openReadOnly(path)
		if err != nil {
			return nil, err
		}
		c.validations++
		if err := listcatalog.ValidateDatabaseFor(candidate, p, profile); err != nil {
			return nil, errors.Join(fmt.Errorf("%s catalogue: %w", label, err), candidate.Close())
		}
		rows, err := listcatalog.ReadChoicesFor(candidate, "", profile)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("%s catalogue: %w", label, err), candidate.Close())
		}
		hash, err := listcatalog.TypedHash(rows)
		if err != nil || hash != p.TypedCellsSHA256 {
			return nil, errors.Join(errors.New("catalogue snapshot typed-cell checksum mismatch"), err, candidate.Close())
		}
		current, err := catalogueFileStamp(path)
		if err != nil || !stamp.same(current) {
			return nil, errors.Join(errors.New("catalogue changed during verification; retry"), err, candidate.Close())
		}
		if err := candidate.Close(); err != nil {
			return nil, fmt.Errorf("%s catalogue snapshot close: %w", label, err)
		}
		c.provenance = p
		c.provenance.SourceSchema = append([]listcatalog.Column(nil), p.SourceSchema...)
		c.rows, c.stamp, c.valid = rows, current, true
	}
	result := []listcatalog.Choice{}
	for _, row := range c.rows {
		if list == "" || row.ListName != nil && *row.ListName == list {
			row.Code, row.ListName, row.ListFilter = cloneCatalogueValue(row.Code), cloneCatalogueValue(row.ListName), cloneCatalogueValue(row.ListFilter)
			row.Description, row.FieldUsedIn = cloneCatalogueValue(row.Description), cloneCatalogueValue(row.FieldUsedIn)
			row.ValidateLoops, row.Note = cloneCatalogueValue(row.ValidateLoops), cloneCatalogueValue(row.Note)
			row.ItemOrder, row.Validate, row.Flag = cloneCatalogueValue(row.ItemOrder), cloneCatalogueValue(row.Validate), cloneCatalogueValue(row.Flag)
			result = append(result, row)
		}
	}
	return result, nil
}

func cloneCatalogueValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func checkListCatalogueChecksum(path string, p listcatalog.Provenance, label string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("%s catalogue: %w", label, err)
	}
	if becHash(data) != p.DatabaseSHA256 {
		return fmt.Errorf("existing %s catalogue checksum mismatch; file was not replaced", label)
	}
	return nil
}
