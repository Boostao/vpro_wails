//go:build linux || windows || darwin

package accessreader

/*
// Hide the vendored GLib shim so GTK cannot resolve its calls to the shim.
#cgo linux CFLAGS: -fvisibility=hidden -I${SRCDIR}/third_party/mdbtools/include -D_GNU_SOURCE -DHAVE_ICONV=1 -DHAVE_FMEMOPEN=1 -DHAVE_VASPRINTF=1 -DHAVE_GMTIME_R=1 -DHAVE_DECL_PROGRAM_INVOCATION_SHORT_NAME=1 -DHAVE_ATTRIBUTE_ALIAS=1 -DTLS=__thread -DICONV_CONST=
#cgo linux LDFLAGS: -lm
#cgo darwin CFLAGS: -fvisibility=hidden -I${SRCDIR}/third_party/mdbtools/include -DHAVE_ICONV=1 -DHAVE_REALLOCF=1 -DTLS=__thread -DICONV_CONST=
#cgo darwin LDFLAGS: -liconv -lm
#cgo windows CFLAGS: -I${SRCDIR}/third_party/mdbtools/include -DTLS=__thread
#cgo windows LDFLAGS: -static -lm
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"
)

// DB refers to an Access file. Each query opens an independent, read-only handle.
type DB struct{ path string }

// Open verifies that path is a readable Access database.
func Open(path string) (*DB, error) {
	if !utf8.ValidString(path) || strings.IndexByte(path, 0) >= 0 {
		return nil, errors.New("invalid Access path: expected UTF-8 without NUL")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	name := C.CString(path)
	defer C.free(unsafe.Pointer(name))
	var nativeErr C.int
	handle := C.go_mdb_open(name, &nativeErr)
	if handle == nil {
		if err := sourceError(nativeErr); err != nil {
			return nil, fmt.Errorf("open Access database %q: %w", path, err)
		}
		return nil, fmt.Errorf("open Access database %q: unsupported or unreadable file", path)
	}
	C.mdb_close(handle)
	return &DB{path: path}, nil
}

func sourceError(code C.int) error {
	switch code {
	case C.GO_MDB_ERROR_READ:
		return errors.New("Access source read error")
	case C.GO_MDB_ERROR_ENCODING:
		return errors.New("unsupported Access character encoding")
	case C.GO_MDB_ERROR_TEXT:
		return errors.New("invalid, incomplete, or oversized Access text encoding")
	case C.GO_MDB_ERROR_ROW:
		return errors.New("invalid Access source row metadata")
	case C.GO_MDB_ERROR_PROPERTIES:
		return errors.New("unsupported or malformed Access property sequence")
	case C.GO_MDB_ERROR_SCHEMA:
		return errors.New("unsupported, incomplete, or malformed Access native column schema")
	default:
		return nil
	}
}

// NativeColumns observes one local table definition without binding or reading
// data rows, following links, or interpreting properties as schema constraints.
// Results are detached from the native handle and preserve duplicate names.
func (db *DB) NativeColumns(table string) ([]NativeColumnObservation, error) {
	if !utf8.ValidString(table) || strings.IndexByte(table, 0) >= 0 {
		return nil, errors.New("invalid Access table name: expected UTF-8 without NUL")
	}
	path, name := C.CString(db.path), C.CString(table)
	defer C.free(unsafe.Pointer(path))
	defer C.free(unsafe.Pointer(name))
	var nativeErr C.int
	handle := C.go_mdb_open(path, &nativeErr)
	if handle == nil {
		if err := sourceError(nativeErr); err != nil {
			return nil, err
		}
		return nil, errors.New("cannot reopen Access database")
	}
	defer C.mdb_close(handle)
	definition := C.mdb_read_table_by_name(handle, name, C.MDB_TABLE)
	if definition != nil {
		defer C.mdb_free_tabledef(definition)
	}
	if err := sourceError(handle.go_error); err != nil {
		return nil, err
	}
	if definition == nil {
		return nil, fmt.Errorf("observe local Access table %q: table missing or unreadable", table)
	}
	if C.mdb_read_columns_observed(definition) == nil {
		if err := sourceError(handle.go_error); err != nil {
			return nil, err
		}
		return nil, errors.New("incomplete Access native column schema")
	}
	result := make([]NativeColumnObservation, int(definition.num_cols))
	for i := range result {
		col := C.go_mdb_schema_column(definition, C.uint(i))
		result[i] = NativeColumnObservation{
			Name: C.GoString(&col.name[0]), SourceColumnNumber: int(col.col_num),
			TypeCode: uint8(col.col_type), TypeLabel: C.GoString(C.mdb_get_colbacktype_string(col)),
			RawSize: uint16(col.go_raw_size), RawFlags: uint8(col.go_raw_flags),
			Source: "MDBTools.table-definition",
		}
	}
	return result, nil
}

// TableDescriptions observes catalog-local Description records before native
// compatibility hash lookup, retaining duplicate definitions and raw byte lengths.
// It does not infer DAO NULL/type semantics or the metadata of linked targets.
func (db *DB) TableDescriptions() ([]TableDescriptionObservation, error) {
	name := C.CString(db.path)
	defer C.free(unsafe.Pointer(name))
	var nativeErr C.int
	handle := C.go_mdb_open(name, &nativeErr)
	if handle == nil {
		if err := sourceError(nativeErr); err != nil {
			return nil, err
		}
		return nil, errors.New("cannot reopen Access database")
	}
	defer C.mdb_close(handle)
	catalog := C.mdb_read_catalog(handle, C.MDB_ANY)
	if err := sourceError(handle.go_error); err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, errors.New("cannot read Access catalog")
	}
	result := make([]TableDescriptionObservation, 0)
	for i := C.guint(0); i < catalog.len; i++ {
		entry := C.go_mdb_entry(catalog, C.uint(i))
		if entry.object_type != C.MDB_TABLE && entry.object_type != C.MDB_LINKED_TABLE {
			continue
		}
		if entry.object_type == C.MDB_TABLE && C.mdb_is_user_table(entry) == 0 {
			continue
		}
		observation := TableDescriptionObservation{
			Name: C.GoString(&entry.object_name[0]), Linked: entry.object_type == C.MDB_LINKED_TABLE,
			MetadataState: "unavailable", PropertyState: "unavailable", NullState: "unknown",
			Source: "MSysObjects.LvProp", DefinitionProvenance: "unknown",
			Entries: make([]DescriptionEntry, 0),
		}
		if entry.props != nil {
			observation.MetadataState, observation.PropertyState = "parsed", "missing"
			for blockIndex := C.guint(0); blockIndex < entry.props.len; blockIndex++ {
				block := C.go_mdb_property_block(entry, C.uint(blockIndex))
				if block.name != nil {
					continue // Named blocks describe columns, not the table object.
				}
				for entryIndex := C.guint(0); entryIndex < block.entries.len; entryIndex++ {
					property := C.go_mdb_property_entry(block, C.uint(entryIndex))
					if C.GoString(property.name) != "Description" {
						continue
					}
					observation.Entries = append(observation.Entries, DescriptionEntry{
						Block: int(blockIndex), Entry: int(entryIndex), RecordType: uint16(block.record_type),
						NameIndex: uint16(property.name_index),
						TypeCode:  uint8(property._type), Flags: uint8(property.flags),
						TypeState: "unknown",
						RawValue:  C.GoBytes(unsafe.Pointer(property.bytes), C.int(property.size)),
					})
					observation.PropertyState = "recorded"
				}
			}
		}
		result = append(result, observation)
	}
	return result, nil
}

// Tables lists local and linked table names without following linked tables.
func (db *DB) Tables() ([]Table, error) {
	name := C.CString(db.path)
	defer C.free(unsafe.Pointer(name))
	var nativeErr C.int
	handle := C.go_mdb_open(name, &nativeErr)
	if handle == nil {
		if err := sourceError(nativeErr); err != nil {
			return nil, err
		}
		return nil, errors.New("cannot reopen Access database")
	}
	defer C.mdb_close(handle)
	catalog := C.mdb_read_catalog(handle, C.MDB_ANY)
	if err := sourceError(handle.go_error); err != nil {
		return nil, err
	}
	if catalog == nil {
		return nil, errors.New("cannot read Access catalog")
	}
	tables := make([]Table, 0)
	for index := C.guint(0); index < catalog.len; index++ {
		entry := C.go_mdb_entry(catalog, C.uint(index))
		if entry.object_type != C.MDB_TABLE && entry.object_type != C.MDB_LINKED_TABLE {
			continue
		}
		if entry.object_type == C.MDB_TABLE && C.mdb_is_user_table(entry) == 0 {
			continue
		}
		table := Table{Name: C.GoString(&entry.object_name[0]), Linked: entry.object_type == C.MDB_LINKED_TABLE}
		if !table.Linked {
			definition := C.mdb_read_table(entry)
			if definition != nil {
				key := C.CString("Description")
				table.Description = C.GoString(C.mdb_table_get_prop(definition, key))
				C.free(unsafe.Pointer(key))
				C.mdb_free_tabledef(definition)
			}
		}
		tables = append(tables, table)
	}
	if err := sourceError(handle.go_error); err != nil {
		return nil, err
	}
	return tables, nil
}

// Reader streams one local table. Close it when finished.
type Reader struct {
	native         *C.GoMdbReader
	Columns        []Column
	row            Row
	err            error
	quarantineRows bool
	rowNumber      int
	Quarantined    []QuarantinedRow
}

// EnableRowQuarantine configures Next to quarantine individual historical rows
// with corrupted text/row metadata (resetting native iconv/mdb row error state)
// while keeping I/O and schema errors fatal. Default Reader behavior remains fail-closed.
func (reader *Reader) EnableRowQuarantine() {
	if reader != nil {
		reader.quarantineRows = true
	}
}

// EstimatedRowCount returns the table header's catalog row count without scanning pages.
func (reader *Reader) EstimatedRowCount() int {
	if reader == nil || reader.native == nil || reader.native.table == nil {
		return 0
	}
	return int(reader.native.table.num_rows)
}

// Scan opens a local table for sequential iteration. Linked tables cannot be scanned.
func (db *DB) Scan(table string) (*Reader, error) {
	path := C.CString(db.path)
	name := C.CString(table)
	defer C.free(unsafe.Pointer(path))
	defer C.free(unsafe.Pointer(name))
	var nativeErr C.int
	native := C.go_mdb_open_reader(path, name, &nativeErr)
	if native == nil {
		if err := sourceError(nativeErr); err != nil {
			return nil, fmt.Errorf("open local Access table %q: %w", table, err)
		}
		return nil, fmt.Errorf("open local Access table %q: table missing or unreadable", table)
	}
	reader := &Reader{native: native}
	for index := C.int(0); index < C.int(native.table.num_cols); index++ {
		column := C.go_mdb_column(native, index)
		reader.Columns = append(reader.Columns, Column{
			Name: C.GoString(&column.name[0]), Type: columnType(column.col_type),
		})
	}
	return reader, nil
}

func columnType(kind C.int) string {
	switch kind {
	case C.MDB_BOOL:
		return "boolean"
	case C.MDB_BYTE, C.MDB_INT, C.MDB_LONGINT:
		return "integer"
	case C.MDB_FLOAT, C.MDB_DOUBLE:
		return "real"
	case C.MDB_MONEY, C.MDB_NUMERIC:
		return "decimal"
	case C.MDB_DATETIME:
		return "datetime"
	case C.MDB_BINARY, C.MDB_OLE:
		return "binary"
	case C.MDB_TEXT, C.MDB_MEMO:
		return "text"
	case C.MDB_REPID:
		return "guid"
	default:
		return "unsupported"
	}
}

// Next advances to the next row. Check Err after it returns false.
func (reader *Reader) Next() bool {
	if reader.native == nil || reader.err != nil {
		return false
	}
nextRow:
	for {
		reader.rowNumber++
		status := C.go_mdb_next(reader.native)
		if status < 0 {
			code := reader.native.mdb.go_error
			rowErr := sourceError(code)
			if rowErr == nil {
				rowErr = errors.New("could not decode Access row")
			}
			if reader.quarantineRows && (code == C.GO_MDB_ERROR_TEXT || code == C.GO_MDB_ERROR_ROW) {
				reader.Quarantined = append(reader.Quarantined, QuarantinedRow{
					RowNumber: reader.rowNumber,
					Reason:    rowErr.Error(),
				})
				C.go_mdb_clear_row_error(reader.native)
				continue nextRow
			}
			reader.err = rowErr
			return false
		}
		if status == 0 {
			return false
		}
		reader.row = make(Row, len(reader.Columns))
		for index, column := range reader.Columns {
			if C.go_mdb_is_null(reader.native, C.int(index)) != 0 && column.Type != "boolean" {
				continue
			}
			nativeColumn := C.go_mdb_column(reader.native, C.int(index))
			if C.go_mdb_memo_too_large(reader.native, C.int(index)) != 0 {
				cellErr := fmt.Errorf("Access memo in column %q exceeds safe bind buffer", column.Name)
				if reader.quarantineRows {
					reader.Quarantined = append(reader.Quarantined, QuarantinedRow{RowNumber: reader.rowNumber, Reason: cellErr.Error()})
					C.go_mdb_clear_row_error(reader.native)
					continue nextRow
				}
				reader.err = cellErr
				return false
			}
			if column.Type == "unsupported" {
				reader.err = fmt.Errorf("unsupported Access type %d in column %q", nativeColumn.col_type, column.Name)
				return false
			}
			if column.Type == "binary" {
				var size C.size_t
				value := C.go_mdb_blob(reader.native, C.int(index), &size)
				if err := sourceError(reader.native.mdb.go_error); err != nil {
					C.free(value)
					cellErr := fmt.Errorf("binary column %q: %w", column.Name, err)
					if reader.quarantineRows {
						reader.Quarantined = append(reader.Quarantined, QuarantinedRow{RowNumber: reader.rowNumber, Reason: cellErr.Error()})
						C.go_mdb_clear_row_error(reader.native)
						continue nextRow
					}
					reader.err = cellErr
					return false
				}
				if value == nil || uint64(size) > 1<<31-1 {
					C.free(value)
					cellErr := fmt.Errorf("binary column %q is unreadable or too large", column.Name)
					if reader.quarantineRows {
						reader.Quarantined = append(reader.Quarantined, QuarantinedRow{RowNumber: reader.rowNumber, Reason: cellErr.Error()})
						C.go_mdb_clear_row_error(reader.native)
						continue nextRow
					}
					reader.err = cellErr
					return false
				}
				reader.row[index] = C.GoBytes(value, C.int(size))
				C.free(value)
				continue
			}
			if C.go_mdb_length(reader.native, C.int(index)) >= C.int(reader.native.mdb.bind_size)-1 {
				cellErr := fmt.Errorf("Access value in column %q exceeds bind buffer", column.Name)
				if reader.quarantineRows {
					reader.Quarantined = append(reader.Quarantined, QuarantinedRow{RowNumber: reader.rowNumber, Reason: cellErr.Error()})
					C.go_mdb_clear_row_error(reader.native)
					continue nextRow
				}
				reader.err = cellErr
				return false
			}
			value := C.GoString(C.go_mdb_text(reader.native, C.int(index)))
			var parseErr error
			switch column.Type {
			case "boolean":
				if value != "0" && value != "1" {
					parseErr = fmt.Errorf("column %q: invalid Access boolean %q", column.Name, value)
				} else {
					reader.row[index] = value == "1"
				}
			case "integer":
				reader.row[index], parseErr = strconv.ParseInt(value, 10, 64)
			case "real":
				reader.row[index], parseErr = strconv.ParseFloat(value, 64)
			case "datetime":
				reader.row[index], parseErr = time.ParseInLocation("2006-01-02 15:04:05", value, time.UTC)
			default:
				reader.row[index] = value // Decimals remain exact text, not float64.
			}
			if parseErr != nil {
				cellErr := fmt.Errorf("column %q: %w", column.Name, parseErr)
				if reader.quarantineRows {
					reader.Quarantined = append(reader.Quarantined, QuarantinedRow{RowNumber: reader.rowNumber, Reason: cellErr.Error()})
					C.go_mdb_clear_row_error(reader.native)
					continue nextRow
				}
				reader.err = cellErr
				return false
			}
		}
		return true
	}
}

// Row returns a snapshot of the current row; it remains valid after Next.
func (reader *Reader) Row() Row { return reader.row }

// Err reports a decoding error encountered during iteration.
func (reader *Reader) Err() error { return reader.err }

// Close releases all C memory associated with the reader.
func (reader *Reader) Close() error {
	if reader.native != nil {
		C.go_mdb_close_reader(reader.native)
		reader.native = nil
	}
	return nil
}

// ReadAll reads a small table completely into memory. Prefer Scan for imports.
func (db *DB) ReadAll(table string) ([]Column, []Row, error) {
	reader, err := db.Scan(table)
	if err != nil {
		return nil, nil, err
	}
	defer reader.Close()
	rows := make([]Row, 0)
	for reader.Next() {
		rows = append(rows, reader.Row())
	}
	return reader.Columns, rows, reader.Err()
}
