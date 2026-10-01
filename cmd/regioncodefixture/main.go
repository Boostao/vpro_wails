package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boostao/vpro-wails/internal/listcatalog"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("regioncodefixture", flag.ContinueOnError)
	flags.SetOutput(output)
	snapshot := flags.String("snapshot", "", "checksum-pinned native DAO fixture")
	manifest := flags.String("manifest", "", "frozen reference manifest")
	database := flags.String("database", "", "new database, never overwritten")
	provenance := flags.String("provenance", "", "new provenance, never overwritten")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *snapshot == "" || *manifest == "" || *database == "" || *provenance == "" {
		return errors.New("required flags: -snapshot -manifest -database -provenance")
	}
	data, err := os.ReadFile(*snapshot)
	if err != nil {
		return err
	}
	manifestData, err := os.ReadFile(*manifest)
	if err != nil {
		return err
	}
	rows, p, err := fixtureRows(data, manifestData)
	if err != nil {
		return err
	}
	if err := listcatalog.GenerateDatabase(*database, *provenance, rows, p, listcatalog.RegionProfile()); err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, "Generated region catalogue (164 rows, 1640 typed cells)")
	return err
}

func fixtureRows(data, manifestData []byte) ([]listcatalog.Choice, listcatalog.Provenance, error) {
	var m struct {
		Path      string         `json:"path"`
		SHA       string         `json:"sha256"`
		Rows      int            `json:"nativeRows"`
		Cells     int            `json:"nativeCells"`
		SourceSHA string         `json:"canonicalSourceSHA256"`
		ReadOnly  bool           `json:"sourceReadonly"`
		Counts    map[string]int `json:"listCounts"`
		Case      bool           `json:"actualListNameCasePreserved"`
		Order     string         `json:"ecosectionOrder"`
	}
	p := listcatalog.Provenance{}
	decoder := json.NewDecoder(bytes.NewReader(manifestData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return nil, p, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, p, errors.New("trailing manifest JSON")
	}
	digest := sha256.Sum256(data)
	profile := listcatalog.RegionProfile()
	if m.Path == "" || m.SHA != profile.SnapshotSHA256 || hex.EncodeToString(digest[:]) != m.SHA ||
		m.SourceSHA != profile.SourceSHA256 || m.Rows != 164 || m.Cells != 1640 || !m.ReadOnly || !m.Case ||
		len(m.Counts) != 2 || m.Counts["Region"] != 27 || m.Counts["Ecosection"] != 137 ||
		m.Order != "observed native unsorted query order" {
		return nil, p, errors.New("region frozen manifest provenance/checksum mismatch")
	}
	p = listcatalog.Provenance{Version: 1, Exporter: profile.Exporter, SourceSHA256: profile.SourceSHA256,
		SnapshotSHA256: profile.SnapshotSHA256, DatabaseSHA256: strings.Repeat("0", 64), TypedCellsSHA256: strings.Repeat("0", 64),
		Rows: 164, Cells: 1640, SourceTable: "USysTableOfLists", SQL: profile.SQL, SourceSchema: listcatalog.ExpectedSchema(),
		ItemOrderEncoding: listcatalog.ItemOrderEncoding}
	rows, err := listcatalog.RegionSnapshotRows(data, p)
	if err != nil {
		return nil, p, err
	}
	p.TypedCellsSHA256, err = listcatalog.TypedHash(rows)
	return rows, p, err
}
