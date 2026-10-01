// sitecodefixture converts only the checksum-pinned, read-only DAO JSON fixture.
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

const nativeTypedRowsSHA256 = "824366db6d74eefc7b9355e4799b6b2054d308175f1d9411ccacd582e9187cff"

type manifest struct {
	Path                  string `json:"path"`
	SHA256                string `json:"sha256"`
	NativeRows            int    `json:"nativeRows"`
	NativeCells           int    `json:"nativeCells"`
	CanonicalSourceSHA256 string `json:"canonicalSourceSHA256"`
	TypedRowsSHA256       string `json:"typedRowsSHA256"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("sitecodefixture", flag.ContinueOnError)
	flags.SetOutput(output)
	snapshot := flags.String("snapshot", "", "frozen native DAO JSON snapshot")
	manifestPath := flags.String("manifest", "", "frozen reference manifest")
	database := flags.String("database", "", "new SQLite catalogue output (never overwritten)")
	provenancePath := flags.String("provenance", "", "new provenance JSON output (never overwritten)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *snapshot == "" || *manifestPath == "" || *database == "" || *provenancePath == "" {
		return errors.New("required flags: -snapshot -manifest -database -provenance")
	}
	data, err := os.ReadFile(*snapshot)
	if err != nil {
		return err
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	p, err := fixtureProvenance(data, manifestData)
	if err != nil {
		return err
	}
	if err := generate(*database, *provenancePath, data, p); err != nil {
		return err
	}
	_, err = fmt.Fprintf(output, "Generated %s and %s (139 rows, 1390 cells)\n", *database, *provenancePath)
	return err
}

func fixtureProvenance(data, manifestData []byte) (listcatalog.Provenance, error) {
	var m manifest
	decoder := json.NewDecoder(bytes.NewReader(manifestData))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&m); err != nil {
		return listcatalog.Provenance{}, err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return listcatalog.Provenance{}, errors.New("unexpected trailing manifest JSON")
	}
	if m.Path == "" || m.SHA256 != listcatalog.SnapshotSHA256 || hash(data) != m.SHA256 ||
		strings.ToLower(m.CanonicalSourceSHA256) != listcatalog.SourceSHA256 ||
		m.NativeRows != 139 || m.NativeCells != 1390 || strings.ToLower(m.TypedRowsSHA256) != nativeTypedRowsSHA256 {
		return listcatalog.Provenance{}, errors.New("frozen manifest provenance/checksum mismatch")
	}
	p := listcatalog.Provenance{
		Version: 1, Exporter: listcatalog.Exporter, SourceSHA256: listcatalog.SourceSHA256,
		SnapshotSHA256: listcatalog.SnapshotSHA256, DatabaseSHA256: strings.Repeat("0", 64),
		TypedCellsSHA256: strings.Repeat("0", 64), Rows: 139, Cells: 1390,
		SourceTable: "USysTableOfLists", SQL: listcatalog.SourceSQL,
		SourceSchema: listcatalog.ExpectedSchema(), ItemOrderEncoding: listcatalog.ItemOrderEncoding,
	}
	rows, err := listcatalog.SnapshotRows(data, p)
	if err != nil {
		return p, err
	}
	p.TypedCellsSHA256, err = listcatalog.TypedHash(rows)
	return p, err
}

func generate(database, provenancePath string, data []byte, p listcatalog.Provenance) (err error) {
	rows, err := listcatalog.SnapshotRows(data, p)
	if err != nil {
		return err
	}
	return listcatalog.GenerateDatabase(database, provenancePath, rows, p, listcatalog.SiteProfile())
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
