package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/boostao/vpro-wails/internal/accessreader"
	"github.com/boostao/vpro-wails/internal/listcatalog"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	_ "github.com/mattn/go-sqlite3"
)

type SourceArchiveProvenance struct {
	ArchiveName   string `json:"archiveName"`
	ArchiveSHA256 string `json:"archiveSha256"`
	ExtractedName string `json:"extractedName"`
	ExtractedSHA  string `json:"extractedSha256"`
	Version       string `json:"version"`
}

type DatabaseArtifactProvenance struct {
	Path   string         `json:"path"`
	SHA256 string         `json:"sha256"`
	Counts map[string]int `json:"counts"`
}

type Standards2026Provenance struct {
	SchemaVersion int                          `json:"schemaVersion"`
	Generator     string                       `json:"generator"`
	Sources       []SourceArchiveProvenance    `json:"sources"`
	LMH25PDFSHA   string                       `json:"lmh25Pdf2023Sha256"`
	Artifacts     []DatabaseArtifactProvenance `json:"artifacts"`
}

func extractZipEntry(zipPath, entryName, destDir string) (string, string, string, error) {
	rawZip, err := os.ReadFile(zipPath)
	if err != nil {
		return "", "", "", err
	}
	zipSum := sha256.Sum256(rawZip)
	zipSHA := hex.EncodeToString(zipSum[:])

	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", "", "", err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if filepath.Base(f.Name) == entryName {
			rc, err := f.Open()
			if err != nil {
				return "", "", "", err
			}
			defer rc.Close()

			outPath := filepath.Join(destDir, entryName)
			outFile, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
			if err != nil {
				return "", "", "", err
			}
			h := sha256.New()
			if _, err := io.Copy(io.MultiWriter(outFile, h), rc); err != nil {
				outFile.Close()
				return "", "", "", err
			}
			if err := outFile.Close(); err != nil {
				return "", "", "", err
			}
			return outPath, zipSHA, hex.EncodeToString(h.Sum(nil)), nil
		}
	}
	return "", "", "", fmt.Errorf("entry %q not found in %q", entryName, zipPath)
}

func generateListCatalogDB(resDir, dbName, provName string, rows []listcatalog.Choice, p listcatalog.Provenance, profile listcatalog.Profile) (string, error) {
	tmpDir, err := os.MkdirTemp(resDir, ".listcat2026-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tmpDB := filepath.Join(tmpDir, dbName)
	tmpProv := filepath.Join(tmpDir, provName)
	if err := listcatalog.GenerateDatabase(tmpDB, tmpProv, rows, p, profile); err != nil {
		return "", err
	}
	rawDB, err := os.ReadFile(tmpDB)
	if err != nil {
		return "", err
	}
	rawProv, err := os.ReadFile(tmpProv)
	if err != nil {
		return "", err
	}
	if err := atomicWriteFile(filepath.Join(resDir, dbName), rawDB, 0644); err != nil {
		return "", err
	}
	if err := atomicWriteFile(filepath.Join(resDir, provName), rawProv, 0644); err != nil {
		return "", err
	}
	return sha256Hex(rawDB), nil
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("standards2026", flag.ContinueOnError)
	fs.SetOutput(stdout)
	becZip := fs.String("bec-zip", "", "Required read-only path to BECdb_v13_2026.zip")
	taxaZip := fs.String("taxa-zip", "", "Required read-only path to BCPlantTaxa2025_access.zip")
	lmh25PDF := fs.String("lmh25-pdf", "", "Optional read-only path to LMH25_2023_web.pdf")
	resDir := fs.String("resources", "resources", "Path to resources directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %v", fs.Args())
	}
	if strings.TrimSpace(*becZip) == "" {
		return fmt.Errorf("-bec-zip is required (path to BECdb_v13_2026.zip)")
	}
	if strings.TrimSpace(*taxaZip) == "" {
		return fmt.Errorf("-taxa-zip is required (path to BCPlantTaxa2025_access.zip)")
	}

	lmhSHA := listcatalog.LMH252023PDFSHA256
	if strings.TrimSpace(*lmh25PDF) != "" {
		pdfBytes, err := os.ReadFile(*lmh25PDF)
		if err != nil {
			return fmt.Errorf("read -lmh25-pdf %q: %w", *lmh25PDF, err)
		}
		actual := sha256Hex(pdfBytes)
		if actual != lmhSHA {
			return fmt.Errorf("LMH25_2023_web.pdf SHA256 %s != expected %s", actual, lmhSHA)
		}
	}

	tmpDir, err := os.MkdirTemp("", "vpro-standards2026-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	becPath, becZipSHA, becAccdbSHA, err := extractZipEntry(*becZip, "BECdb_v13_2026.accdb", tmpDir)
	if err != nil {
		return fmt.Errorf("extract BECdb_v13_2026.accdb: %w", err)
	}
	if becAccdbSHA != listcatalog.BECdbV132026AccdbSHA256 {
		return fmt.Errorf("BECdb_v13_2026.accdb SHA256 mismatch: got %s, want %s", becAccdbSHA, listcatalog.BECdbV132026AccdbSHA256)
	}

	taxaPath, taxaZipSHA, taxaAccdbSHA, err := extractZipEntry(*taxaZip, "BCPlantTaxonomy.accdb", tmpDir)
	if err != nil {
		return fmt.Errorf("extract BCPlantTaxonomy.accdb: %w", err)
	}
	if taxaAccdbSHA != listcatalog.BCPlantTaxa2025AccdbSHA256 {
		return fmt.Errorf("BCPlantTaxonomy.accdb SHA256 mismatch: got %s, want %s", taxaAccdbSHA, listcatalog.BCPlantTaxa2025AccdbSHA256)
	}

	becAccdb, err := accessreader.Open(becPath)
	if err != nil {
		return err
	}
	taxaAccdb, err := accessreader.Open(taxaPath)
	if err != nil {
		return err
	}

	// 1. Upgrade bec.db + bec-provenance.json
	becDBPath := filepath.Join(*resDir, "bec.db")
	becProvPath := filepath.Join(*resDir, "bec-provenance.json")
	zones, series, becProv, err := buildBECv13Data(becAccdb, becAccdbSHA, becDBPath)
	if err != nil {
		return fmt.Errorf("build BEC v13 data: %w", err)
	}
	becProv, err = writeBECDatabase(becDBPath, becProvPath, zones, series, becProv)
	if err != nil {
		return fmt.Errorf("write bec.db: %w", err)
	}

	// 2. Upgrade working-unit.db + working-unit-provenance.json
	wuDBPath := filepath.Join(*resDir, "working-unit.db")
	wuProvPath := filepath.Join(*resDir, "working-unit-provenance.json")
	masterUnits, wuProv, err := buildAndWriteWorkingUnitv13(becAccdb, becAccdbSHA, wuDBPath, wuProvPath, wuDBPath, wuProvPath)
	if err != nil {
		return fmt.Errorf("write working-unit.db: %w", err)
	}

	// 3. Upgrade VLists.db + vlists.db
	vlistsFamilyPath := filepath.Join(*resDir, "database-family", "VLists.db")
	vlistsReadPath := filepath.Join(*resDir, "vlists.db")
	vlistsRO, err := sqlutil.OpenReadOnly(vlistsFamilyPath)
	if err != nil {
		return err
	}
	specs, err := mergeTaxonomy2025(taxaAccdb, vlistsRO)
	vlistsRO.Close()
	if err != nil {
		return fmt.Errorf("merge taxonomy 2025: %w", err)
	}
	vlistsFamilySHA, vlistsReadSHA, vlistsListsRows, vlistsSpeciesRows, err := upgradeVListsFamilyAndReadModel(
		vlistsFamilyPath, vlistsReadPath, specs, zones, series, masterUnits,
	)
	if err != nil {
		return fmt.Errorf("upgrade VLists.db / vlists.db: %w", err)
	}

	// 4. Upgrade site-codes.db, soil-codes.db, parent-codes.db from verified DAO fixtures + LMH25 (2023)
	siteSnap, err := os.ReadFile(filepath.Join(*resDir, "site-codes-fixture.json"))
	if err != nil {
		return err
	}
	siteRows, siteProv, err := listcatalog.SiteBundled2026Rows(siteSnap)
	if err != nil {
		return err
	}
	siteDBSHA, err := generateListCatalogDB(*resDir, "site-codes.db", "site-codes-provenance.json",
		siteRows, siteProv, listcatalog.SiteBundledProfile2026())
	if err != nil {
		return fmt.Errorf("generate site-codes.db: %w", err)
	}

	soilSnap, err := os.ReadFile(filepath.Join(*resDir, "soil-codes-fixture.json.gz"))
	if err != nil {
		return err
	}
	soilRows, soilProv, err := listcatalog.SoilBundled2026Rows(soilSnap)
	if err != nil {
		return err
	}
	soilDBSHA, err := generateListCatalogDB(*resDir, "soil-codes.db", "soil-codes-provenance.json",
		soilRows, soilProv, listcatalog.SoilBundledProfile2026())
	if err != nil {
		return fmt.Errorf("generate soil-codes.db: %w", err)
	}

	parentSnap, err := os.ReadFile(filepath.Join(*resDir, "parent-codes-fixture.json"))
	if err != nil {
		return err
	}
	parentCapProv, err := os.ReadFile(filepath.Join(*resDir, "parent-codes-source-provenance.json"))
	if err != nil {
		return err
	}
	parentManifest, err := os.ReadFile(filepath.Join(*resDir, "parent-codes-fixture-manifest.json"))
	if err != nil {
		return err
	}
	geoSnap, err := os.ReadFile(filepath.Join(*resDir, "geology-codes-fixture.json.gz"))
	if err != nil {
		return err
	}
	parentRows, parentProv, err := listcatalog.ParentBundled2026Rows(parentSnap, parentCapProv, parentManifest, geoSnap)
	if err != nil {
		return err
	}
	parentDBSHA, err := generateListCatalogDB(*resDir, "parent-codes.db", "parent-codes-provenance.json",
		parentRows, parentProv, listcatalog.ParentBundledProfile2026())
	if err != nil {
		return fmt.Errorf("generate parent-codes.db: %w", err)
	}

	// 5. Write master resources/standards-2026-provenance.json
	masterProv := Standards2026Provenance{
		SchemaVersion: 1,
		Generator:     "cmd/standards2026",
		Sources: []SourceArchiveProvenance{
			{
				ArchiveName:   filepath.Base(strings.ReplaceAll(*becZip, `\`, `/`)),
				ArchiveSHA256: becZipSHA,
				ExtractedName: "BECdb_v13_2026.accdb",
				ExtractedSHA:  becAccdbSHA,
				Version:       "BECdb_v13_2026 (2026-05-28)",
			},
			{
				ArchiveName:   filepath.Base(strings.ReplaceAll(*taxaZip, `\`, `/`)),
				ArchiveSHA256: taxaZipSHA,
				ExtractedName: "BCPlantTaxonomy.accdb",
				ExtractedSHA:  taxaAccdbSHA,
				Version:       "BCPlantTaxa_v19_2025 (2026-02-09)",
			},
		},
		LMH25PDFSHA: lmhSHA,
		Artifacts: []DatabaseArtifactProvenance{
			{
				Path:   "resources/database-family/VLists.db",
				SHA256: vlistsFamilySHA,
				Counts: map[string]int{
					"USysAllSpecs":        len(specs),
					"USysZoneList":        len(zones),
					"USysSiteSeriesNames": len(series),
					"MasterSiteUnitList":  len(masterUnits),
					"USysTableOfLists":    vlistsListsRows,
				},
			},
			{
				Path:   "resources/vlists.db",
				SHA256: vlistsReadSHA,
				Counts: map[string]int{
					"Species": vlistsSpeciesRows,
				},
			},
			{
				Path:   "resources/bec.db",
				SHA256: becProv.DatabaseSHA256,
				Counts: map[string]int{
					"BECZoneList":   becProv.ZoneRows,
					"BECSiteSeries": becProv.SiteSeriesRows,
					"duplicateKeys": becProv.DuplicateKeys,
				},
			},
			{
				Path:   "resources/working-unit.db",
				SHA256: wuProv.DatabaseSHA256,
				Counts: map[string]int{
					"MasterSiteUnitList": wuProv.MasterRows,
					"UserSiteUnitList":   wuProv.UserRows,
				},
			},
			{
				Path:   "resources/site-codes.db",
				SHA256: siteDBSHA,
				Counts: map[string]int{"rows": len(siteRows)},
			},
			{
				Path:   "resources/soil-codes.db",
				SHA256: soilDBSHA,
				Counts: map[string]int{"rows": len(soilRows)},
			},
			{
				Path:   "resources/parent-codes.db",
				SHA256: parentDBSHA,
				Counts: map[string]int{"rows": len(parentRows)},
			},
		},
	}

	masterBytes, err := json.MarshalIndent(masterProv, "", "  ")
	if err != nil {
		return err
	}
	masterBytes = append(masterBytes, '\n')
	if err := atomicWriteFile(filepath.Join(*resDir, "standards-2026-provenance.json"), masterBytes, 0644); err != nil {
		return err
	}

	_, err = fmt.Fprintf(stdout, "%s", masterBytes)
	return err
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "standards2026:", err)
		os.Exit(1)
	}
}
