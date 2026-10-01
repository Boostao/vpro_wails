package listcatalog

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
)

const (
	SourceSHA256      = "2a5ad098cf4665991acfbbca5f011c9488afcbb8272140ad262fe4f7b58f084d"
	SnapshotSHA256    = "099af9fd9347e2b8d908000928cb03f033e8510399a24da9ba94d240031b6887"
	Exporter          = "native-access-dao-site-codes-v1"
	SourceSQL         = `SELECT * FROM USysTableOfLists WHERE ListName IN ("SiteDisturbance","Exposure") ORDER BY ListName,ItemOrder`
	ItemOrderEncoding = "nullable IEEE754 binary64 big-endian 8-byte BLOB; native source ordinal order"
)

type Provenance struct {
	Version           int      `json:"version"`
	Exporter          string   `json:"exporter"`
	SourceSHA256      string   `json:"sourceSha256"`
	SnapshotSHA256    string   `json:"snapshotSha256"`
	DatabaseSHA256    string   `json:"databaseSha256"`
	TypedCellsSHA256  string   `json:"typedCellsSha256"`
	Rows              int      `json:"rows"`
	Cells             int      `json:"cells"`
	SourceTable       string   `json:"sourceTable"`
	SQL               string   `json:"sql"`
	SourceSchema      []Column `json:"sourceSchema"`
	ItemOrderEncoding string   `json:"itemOrderEncoding"`
}

func DecodeProvenance(data []byte) (Provenance, error) {
	return DecodeProvenanceFor(data, SiteProfile())
}

func DecodeProvenanceFor(data []byte, profile Profile) (Provenance, error) {
	var p Provenance
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return p, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return p, errors.New("unexpected trailing provenance JSON")
	}
	return p, ValidateProvenanceFor(p, profile)
}

func ValidateProvenance(p Provenance) error {
	return ValidateProvenanceFor(p, SiteProfile())
}

func ValidateProvenanceFor(p Provenance, profile Profile) error {
	for _, hash := range []string{p.SourceSHA256, p.SnapshotSHA256, p.DatabaseSHA256, p.TypedCellsSHA256} {
		if data, err := hex.DecodeString(hash); err != nil || len(data) != 32 || hash != strings.ToLower(hash) {
			return errors.New("site code provenance contains an invalid checksum")
		}
	}
	count := 0
	for _, list := range profile.Lists {
		count += list.Rows
	}
	if p.Version != 1 || p.Exporter != profile.Exporter || p.SourceSHA256 != profile.SourceSHA256 ||
		p.SnapshotSHA256 != profile.SnapshotSHA256 || p.Rows != count || p.Cells != count*10 ||
		p.SourceTable != "USysTableOfLists" || p.SQL != profile.SQL ||
		!reflect.DeepEqual(p.SourceSchema, ExpectedSchema()) || p.ItemOrderEncoding != ItemOrderEncoding {
		return errors.New("site code provenance source/schema mismatch")
	}
	return nil
}

type nativeSnapshot struct {
	Kind   string `json:"snapshotKind"`
	Source struct {
		Before string `json:"BeforeSHA256"`
		After  string `json:"AfterSHA256"`
	} `json:"source"`
	Schema struct {
		Name            string
		SourceTableName string
		Fields          []struct {
			Name string
			Type int
			Size int
		}
	} `json:"schema"`
	SQL             string `json:"sql"`
	TypedRowsSHA256 string `json:"typedRowsSHA256"`
	RowCount        int    `json:"rowCount"`
	CellCount       int    `json:"cellCount"`
	ReadOnly        bool   `json:"sourceReadOnly"`
	Unchanged       struct {
		Equal  bool
		Before string
		After  string
	} `json:"referenceUnchanged"`
	Lists map[string]struct {
		RowCount  int `json:"rowCount"`
		CellCount int `json:"cellCount"`
		Rows      []struct {
			Ordinal string `json:"sourceOrdinal"`
			Cells   []Cell `json:"cells"`
		} `json:"rowsInObservedNativeOrder"`
	} `json:"lists"`
}

func SnapshotRows(data []byte, p Provenance) ([]Choice, error) {
	if err := ValidateProvenance(p); err != nil {
		return nil, err
	}
	if checksum(data) != p.SnapshotSHA256 {
		return nil, errors.New("site code snapshot checksum mismatch")
	}
	return decodeSnapshot(data, p)
}

func decodeSnapshot(data []byte, p Provenance) ([]Choice, error) {
	var snapshot nativeSnapshot
	// The immutable native export includes observational evidence beyond the
	// conversion boundary; these fields are intentionally not imported.
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Kind != "Readonly native DAO snapshot fixture, not Access-reader implementation" ||
		!snapshot.ReadOnly || !snapshot.Unchanged.Equal || snapshot.SQL != p.SQL ||
		strings.ToLower(snapshot.Source.Before) != p.SourceSHA256 || snapshot.Source.Before != snapshot.Source.After ||
		snapshot.RowCount != p.Rows || snapshot.CellCount != p.Cells ||
		snapshot.Schema.Name != p.SourceTable || snapshot.Schema.SourceTableName != p.SourceTable ||
		snapshot.TypedRowsSHA256 == "" || snapshot.Unchanged.Before != snapshot.TypedRowsSHA256 ||
		snapshot.Unchanged.After != snapshot.TypedRowsSHA256 || len(snapshot.Schema.Fields) != 10 || len(snapshot.Lists) != 2 {
		return nil, errors.New("site code native snapshot provenance mismatch")
	}
	for i, field := range snapshot.Schema.Fields {
		if field.Name != p.SourceSchema[i].Name || field.Type != p.SourceSchema[i].DAOType || field.Size != p.SourceSchema[i].Size {
			return nil, errors.New("site code native source schema mismatch")
		}
	}
	result := make([]Choice, 0, p.Rows)
	for _, list := range []struct {
		name  string
		count int
	}{{"Exposure", 12}, {"SiteDisturbance", 127}} {
		source, exists := snapshot.Lists[list.name]
		if !exists || source.RowCount != list.count || source.CellCount != list.count*10 || len(source.Rows) != list.count {
			return nil, fmt.Errorf("site code list %s count mismatch", list.name)
		}
		for i, native := range source.Rows {
			if native.Ordinal != strconv.Itoa(i+1) {
				return nil, fmt.Errorf("site code list %s source ordinal mismatch", list.name)
			}
			row, err := DecodeCells(native.Ordinal, native.Cells)
			if err != nil {
				return nil, err
			}
			if row.ListName == nil || *row.ListName != list.name {
				return nil, fmt.Errorf("site code list %s membership mismatch", list.name)
			}
			result = append(result, row)
		}
	}
	return result, nil
}
