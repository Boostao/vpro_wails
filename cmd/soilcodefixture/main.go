package main

import (
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

func fixtureRows(data []byte) ([]listcatalog.Choice, listcatalog.Provenance, error) {
	return fixtureRowsFor(data, listcatalog.SoilProfile(), listcatalog.SoilSnapshotRows)
}

func fixtureRowsFor(data []byte, profile listcatalog.Profile, decode func([]byte, listcatalog.Provenance) ([]listcatalog.Choice, error)) ([]listcatalog.Choice, listcatalog.Provenance, error) {
	count := 0
	for _, list := range profile.Lists {
		count += list.Rows
	}
	p := listcatalog.Provenance{Version: 1, Exporter: profile.Exporter, SourceSHA256: profile.SourceSHA256,
		SnapshotSHA256: profile.SnapshotSHA256, DatabaseSHA256: strings.Repeat("0", 64), TypedCellsSHA256: strings.Repeat("0", 64),
		Rows: count, Cells: count * 10, SourceTable: "USysTableOfLists", SQL: profile.SQL,
		SourceSchema: listcatalog.ExpectedSchema(), ItemOrderEncoding: listcatalog.ItemOrderEncoding}
	rows, err := decode(data, p)
	if err != nil {
		return nil, p, err
	}
	p.TypedCellsSHA256, err = listcatalog.TypedHash(rows)
	return rows, p, err
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("soilcodefixture", flag.ContinueOnError)
	flags.SetOutput(output)
	profileName := flags.String("profile", "soil", "frozen current DAO profile: soil, geology or parent")
	snapshot := flags.String("snapshot", "", "exact checksum-pinned current readonly DAO gzip fixture")
	database := flags.String("database", "", "new catalogue database, never overwritten")
	provenance := flags.String("provenance", "", "new catalogue provenance, never overwritten")
	sourceProvenance := flags.String("source-provenance", "", "parent: sealed 15-list capture provenance")
	manifest := flags.String("manifest", "", "parent: sealed capture manifest")
	bedrock := flags.String("bedrock", "", "parent: existing frozen Bedrock gzip fixture")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *snapshot == "" || *database == "" || *provenance == "" {
		return errors.New("required flags: -snapshot -database -provenance")
	}
	profile, decode, nativeHash := listcatalog.SoilProfile(), listcatalog.SoilSnapshotRows, listcatalog.SoilNativeTypedCellsSHA256
	switch *profileName {
	case "soil":
	case "geology":
		profile, decode, nativeHash = listcatalog.GeologyProfile(), listcatalog.GeologySnapshotRows, listcatalog.GeologyNativeTypedCellsSHA256
	case "parent":
		if *sourceProvenance == "" || *manifest == "" || *bedrock == "" {
			return errors.New("parent requires -source-provenance -manifest -bedrock")
		}
		source, err := os.ReadFile(*sourceProvenance)
		if err != nil {
			return err
		}
		seal, err := os.ReadFile(*manifest)
		if err != nil {
			return err
		}
		geology, err := os.ReadFile(*bedrock)
		if err != nil {
			return err
		}
		profile, nativeHash = listcatalog.ParentProfile(), listcatalog.ParentNativeTypedCellsSHA256
		decode = func(data []byte, p listcatalog.Provenance) ([]listcatalog.Choice, error) {
			return listcatalog.ParentSnapshotRows(data, source, seal, geology, p)
		}
	default:
		return fmt.Errorf("unsupported fixture profile %q", *profileName)
	}
	if *profileName != "parent" && (*sourceProvenance != "" || *manifest != "" || *bedrock != "") {
		return errors.New("source provenance/manifest/bedrock inputs require parent profile")
	}
	data, err := os.ReadFile(*snapshot)
	if err != nil {
		return err
	}
	rows, p, err := fixtureRowsFor(data, profile, decode)
	if err != nil {
		return err
	}
	if err := listcatalog.GenerateDatabase(*database, *provenance, rows, p, profile); err != nil {
		return err
	}
	encoded, err := json.Marshal(struct {
		Rows          int    `json:"rows"`
		Cells         int    `json:"cells"`
		NativeDAOHash string `json:"nativeDAOCellsSha256"`
		StorageHash   string `json:"storageTypedCellsSha256"`
	}{p.Rows, p.Cells, nativeHash, p.TypedCellsSHA256})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(output, string(encoded))
	return err
}
