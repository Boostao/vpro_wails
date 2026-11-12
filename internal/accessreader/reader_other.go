//go:build !linux && !windows && !darwin

package accessreader

import "errors"

var errUnsupportedPlatform = errors.New("Access database reading is only supported on Windows, macOS, and Linux")

type DB struct{ path string }

type Reader struct {
	Columns []Column
}

func Open(path string) (*DB, error) {
	return nil, errUnsupportedPlatform
}

func (db *DB) NativeColumns(table string) ([]NativeColumnObservation, error) {
	return nil, errUnsupportedPlatform
}

func (db *DB) TableDescriptions() ([]TableDescriptionObservation, error) {
	return nil, errUnsupportedPlatform
}

func (db *DB) Tables() ([]Table, error) {
	return nil, errUnsupportedPlatform
}

func (db *DB) Scan(table string) (*Reader, error) {
	return nil, errUnsupportedPlatform
}

func (reader *Reader) Next() bool { return false }
func (reader *Reader) Row() Row   { return nil }
func (reader *Reader) Err() error { return errUnsupportedPlatform }
func (reader *Reader) Close() error { return nil }

func (db *DB) ReadAll(table string) ([]Column, []Row, error) {
	return nil, nil, errUnsupportedPlatform
}
