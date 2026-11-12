package resources

import (
	"embed"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed Sample.db
var SampleDB []byte

//go:embed Sample.db
var rawSampleFS embed.FS

//go:embed vlists.db
var VListsDB []byte

//go:embed vlists.db
var rawVListsFS embed.FS

//go:embed database-family/*.db
var rawDatabaseFamilyFS embed.FS

//go:embed config.init.yml
var ConfigInitYML []byte

//go:embed bec.db
var BECDB []byte

//go:embed bec-provenance.json
var BECProvenanceJSON []byte

//go:embed working-unit.db
var WorkingUnitDB []byte

//go:embed working-unit-provenance.json
var WorkingUnitProvenanceJSON []byte

//go:embed site-codes.db
var SiteCodesDB []byte

//go:embed site-codes-provenance.json
var SiteCodesProvenanceJSON []byte

//go:embed soil-codes.db
var SoilCodesDB []byte

//go:embed soil-codes-provenance.json
var SoilCodesProvenanceJSON []byte

//go:embed parent-codes.db
var ParentCodesDB []byte

//go:embed parent-codes-provenance.json
var ParentCodesProvenanceJSON []byte

//go:embed geology-codes.db
var GeologyCodesDB []byte

//go:embed geology-codes-provenance.json
var GeologyCodesProvenanceJSON []byte

//go:embed region-codes.db
var RegionCodesDB []byte

//go:embed region-codes-provenance.json
var RegionCodesProvenanceJSON []byte

//go:embed plot-quality-site.db
var PlotQualitySiteDB []byte

//go:embed plot-quality-site-provenance.json
var PlotQualitySiteProvenanceJSON []byte

//go:embed standards-2026-provenance.json
var Standards2026ProvenanceJSON []byte

//go:embed help_bundle.json
var HelpBundleJSON []byte

//go:embed project-metadata-template.json
var ProjectMetadataTemplateJSON []byte

//go:embed fs1333-sivi-layout.json
var FS1333SIVILayoutJSON []byte

//go:embed fs1333-chars-sivi-layout.json
var FS1333CHARSSIVILayoutJSON []byte

//go:embed fs882-two-page-layout.json
var FS882TwoPageLayoutJSON []byte

//go:embed fs882-two-page-chars-layout.json
var FS882TwoPageCHARSLayoutJSON []byte

//go:embed maps/bc-overview.pmtiles
var BCOverviewPMTiles []byte

// PrefixedFS wraps an embed.FS embedded inside resources/ so callers passing
// either "resources/foo" or "foo" resolve identically.
type PrefixedFS struct {
	inner embed.FS
}

func trimResourcesPrefix(name string) string {
	return strings.TrimPrefix(name, "resources/")
}

func (p PrefixedFS) Open(name string) (fs.File, error) {
	return p.inner.Open(trimResourcesPrefix(name))
}

func (p PrefixedFS) ReadFile(name string) ([]byte, error) {
	return p.inner.ReadFile(trimResourcesPrefix(name))
}

func (p PrefixedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return p.inner.ReadDir(trimResourcesPrefix(name))
}

var (
	SampleFiles      = PrefixedFS{inner: rawSampleFS}
	VListsFiles      = PrefixedFS{inner: rawVListsFS}
	DatabaseFamilyFS = PrefixedFS{inner: rawDatabaseFamilyFS}
)

var repoRoot = func() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		abs, _ := filepath.Abs(".")
		return abs
	}
	return filepath.Dir(filepath.Dir(file))
}()

// RepoRoot returns the absolute path to the repository root directory.
func RepoRoot() string {
	return repoRoot
}

// Path joins relative path segments onto RepoRoot().
func Path(elem ...string) string {
	parts := make([]string, 0, len(elem)+1)
	parts = append(parts, repoRoot)
	parts = append(parts, elem...)
	return filepath.Join(parts...)
}
