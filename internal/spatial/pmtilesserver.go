package spatial

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/boostao/vpro-wails/resources"
)

const (
	PMTilesOverviewArchiveName = "bc-overview.pmtiles"
	PMTilesBGCArchiveName      = "bc-bgc-v13.pmtiles"
	PMTilesAdminArchiveName    = "bc-admin-ecosections-districts.pmtiles"
	PMTilesContentType         = "application/vnd.pmtiles"
)

var defaultKnownPMTilesArchives = []string{
	PMTilesOverviewArchiveName,
	PMTilesBGCArchiveName,
	PMTilesAdminArchiveName,
}

// PMTilesArchiveStatus describes the resolution state of a single .pmtiles archive.
type PMTilesArchiveStatus struct {
	Name             string `json:"name"`
	URL              string `json:"url"`
	Source           string `json:"source"` // "dataDir" | "extraDir" | "embedded-overview" | "missing"
	Available        bool   `json:"available"`
	FullResolution   bool   `json:"fullResolution"`
	FallbackOverview bool   `json:"fallbackOverview"`
	MinZoom          int    `json:"minZoom"`
	MaxZoom          int    `json:"maxZoom"`
	SizeBytes        int64  `json:"sizeBytes"`
}

// PMTilesManifest reports available offline PMTiles archives to the Svelte 5 MapLibre workbench.
type PMTilesManifest struct {
	SchemaVersion int                             `json:"schemaVersion"`
	Archives      map[string]PMTilesArchiveStatus `json:"archives"`
	Ordered       []PMTilesArchiveStatus          `json:"ordered"`
}

type resolvedPMTilesSource struct {
	filePath         string
	embedded         []byte
	modTime          time.Time
	sizeBytes        int64
	source           string
	fullResolution   bool
	fallbackOverview bool
	minZoom          int
	maxZoom          int
}

func validatePMTilesBasename(raw string) (string, bool) {
	if raw == "" || strings.ContainsAny(raw, "/\\\x00") || strings.Contains(raw, "..") {
		return "", false
	}
	base := filepath.Base(raw)
	if base != raw || base == "." || base == ".." || base == ".pmtiles" {
		return "", false
	}
	if !strings.HasSuffix(strings.ToLower(base), ".pmtiles") {
		return "", false
	}
	for _, r := range base {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return "", false
		}
	}
	return base, true
}

func inspectPMTilesZoomBoundsFromBytes(header []byte) (minZ, maxZ int) {
	if len(header) >= 102 && string(header[:7]) == "PMTiles" && header[7] == 3 {
		_ = binary.LittleEndian.Uint64(header[8:16])
		return int(header[100]), int(header[101])
	}
	return 0, 0
}

func inspectPMTilesZoomBoundsFromFile(path string) (minZ, maxZ int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	var buf [127]byte
	n, err := f.Read(buf[:])
	if err != nil || n < 102 {
		return 0, 0
	}
	return inspectPMTilesZoomBoundsFromBytes(buf[:n])
}

func resolvePMTilesArchive(name, dataDir string, extraDirs []string) (resolvedPMTilesSource, bool) {
	clean, ok := validatePMTilesBasename(name)
	if !ok {
		return resolvedPMTilesSource{}, false
	}
	isOverviewName := strings.EqualFold(clean, PMTilesOverviewArchiveName)

	// 1. Check <dataDir>/maps/<name>.pmtiles
	if strings.TrimSpace(dataDir) != "" {
		candidate := filepath.Join(dataDir, "maps", clean)
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Size() >= 127 {
			minZ, maxZ := inspectPMTilesZoomBoundsFromFile(candidate)
			return resolvedPMTilesSource{
				filePath:       candidate,
				modTime:        info.ModTime().UTC(),
				sizeBytes:      info.Size(),
				source:         "dataDir",
				fullResolution: !isOverviewName && maxZ > 6,
				minZoom:        minZ,
				maxZoom:        maxZ,
			}, true
		}
	}

	// 2. Check extraDirs/<name>.pmtiles (and extraDirs/maps/<name>.pmtiles)
	for _, rawDir := range extraDirs {
		dir := strings.TrimSpace(rawDir)
		if dir == "" {
			continue
		}
		for _, candidate := range []string{filepath.Join(dir, clean), filepath.Join(dir, "maps", clean)} {
			if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Size() >= 127 {
				minZ, maxZ := inspectPMTilesZoomBoundsFromFile(candidate)
				return resolvedPMTilesSource{
					filePath:       candidate,
					modTime:        info.ModTime().UTC(),
					sizeBytes:      info.Size(),
					source:         "extraDir",
					fullResolution: !isOverviewName && maxZ > 6,
					minZoom:        minZ,
					maxZoom:        maxZ,
				}, true
			}
		}
	}

	// 3. Embedded overview fallback for bc-overview.pmtiles or absent provincial archives on fresh install
	canFallbackToEmbedded := isOverviewName ||
		strings.EqualFold(clean, PMTilesBGCArchiveName) ||
		strings.EqualFold(clean, PMTilesAdminArchiveName)
	if canFallbackToEmbedded && len(resources.BCOverviewPMTiles) >= 127 {
		minZ, maxZ := inspectPMTilesZoomBoundsFromBytes(resources.BCOverviewPMTiles)
		return resolvedPMTilesSource{
			embedded:         resources.BCOverviewPMTiles,
			modTime:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			sizeBytes:        int64(len(resources.BCOverviewPMTiles)),
			source:           "embedded-overview",
			fullResolution:   false,
			fallbackOverview: !isOverviewName,
			minZoom:          minZ,
			maxZoom:          maxZ,
		}, true
	}

	return resolvedPMTilesSource{}, false
}

// BuildPMTilesManifest inspects dataDir, extraDirs, and the embedded overview archive.
func BuildPMTilesManifest(dataDir string, extraDirs ...string) PMTilesManifest {
	names := make(map[string]struct{}, len(defaultKnownPMTilesArchives))
	for _, n := range defaultKnownPMTilesArchives {
		names[n] = struct{}{}
	}
	scanDir := func(dir string) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if clean, ok := validatePMTilesBasename(e.Name()); ok {
				names[clean] = struct{}{}
			}
		}
	}
	if strings.TrimSpace(dataDir) != "" {
		scanDir(filepath.Join(dataDir, "maps"))
	}
	for _, d := range extraDirs {
		if strings.TrimSpace(d) != "" {
			scanDir(d)
			scanDir(filepath.Join(d, "maps"))
		}
	}

	sortedNames := make([]string, 0, len(names))
	for n := range names {
		sortedNames = append(sortedNames, n)
	}
	sort.Strings(sortedNames)

	archives := make(map[string]PMTilesArchiveStatus, len(sortedNames))
	ordered := make([]PMTilesArchiveStatus, 0, len(sortedNames))
	for _, name := range sortedNames {
		resolved, ok := resolvePMTilesArchive(name, dataDir, extraDirs)
		status := PMTilesArchiveStatus{
			Name: name,
			URL:  "/maps/" + name,
		}
		if ok {
			status.Source = resolved.source
			status.Available = true
			status.FullResolution = resolved.fullResolution
			status.FallbackOverview = resolved.fallbackOverview
			status.MinZoom = resolved.minZoom
			status.MaxZoom = resolved.maxZoom
			status.SizeBytes = resolved.sizeBytes
		} else {
			status.Source = "missing"
		}
		archives[name] = status
		ordered = append(ordered, status)
	}
	return PMTilesManifest{
		SchemaVersion: 1,
		Archives:      archives,
		Ordered:       ordered,
	}
}

// NewPMTilesAssetHandler wraps Wails' asset file server to serve /maps/<name>.pmtiles
// with RFC 7233 HTTP Range (206 Partial Content) support and /maps/manifest.json.
func NewPMTilesAssetHandler(fallback http.Handler, dataDir string, extraDirs ...string) http.Handler {
	copiedExtra := append([]string(nil), extraDirs...)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawURI := r.URL.EscapedPath()
		if rawURI == "" {
			rawURI = r.URL.Path
		}
		lowerURI := strings.ToLower(rawURI)
		if strings.HasPrefix(rawURI, "/maps/") || rawURI == "/maps" {
			// Reject encoded traversal sequences before URL path normalization.
			if strings.Contains(lowerURI, "%2e") || strings.Contains(lowerURI, "%2f") || strings.Contains(lowerURI, "%5c") || strings.Contains(rawURI, "..") {
				http.Error(w, "invalid map archive path", http.StatusBadRequest)
				return
			}
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.Header().Set("Allow", "GET, HEAD")
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			rel := strings.TrimPrefix(r.URL.Path, "/maps/")
			if rel == "manifest.json" {
				manifest := BuildPMTilesManifest(dataDir, copiedExtra...)
				payload, err := json.Marshal(manifest)
				if err != nil {
					http.Error(w, "failed to encode manifest", http.StatusInternalServerError)
					return
				}
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Header().Set("Cache-Control", "no-cache")
				http.ServeContent(w, r, "manifest.json", time.Time{}, bytes.NewReader(payload))
				return
			}
			clean, valid := validatePMTilesBasename(rel)
			if !valid {
				http.Error(w, "invalid PMTiles archive basename", http.StatusBadRequest)
				return
			}
			resolved, found := resolvePMTilesArchive(clean, dataDir, copiedExtra)
			if !found {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", PMTilesContentType)
			w.Header().Set("X-VPro-PMTiles-Source", resolved.source)
			if resolved.filePath != "" {
				f, err := os.Open(resolved.filePath)
				if err != nil {
					http.NotFound(w, r)
					return
				}
				defer f.Close()
				http.ServeContent(w, r, clean, resolved.modTime, f)
				return
			}
			http.ServeContent(w, r, clean, resolved.modTime, bytes.NewReader(resolved.embedded))
			return
		}
		if fallback != nil {
			fallback.ServeHTTP(w, r)
			return
		}
		http.NotFound(w, r)
	})
}
