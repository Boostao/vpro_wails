package lifecycle

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"

	"encoding/json"
	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"strings"
)

var standards2026ProvenanceJSON = resources.Standards2026ProvenanceJSON

var appVersion = "0.1.0"

type StandardsSourceStamp struct {
	ArchiveName     string `json:"archiveName"`
	ArchiveSHA256   string `json:"archiveSha256"`
	ExtractedName   string `json:"extractedName"`
	ExtractedSHA256 string `json:"extractedSha256"`
	Version         string `json:"version"`
}

type StandardsArtifactStamp struct {
	Path   string         `json:"path"`
	SHA256 string         `json:"sha256"`
	Counts map[string]int `json:"counts"`
}

type LibraryStamps struct {
	SchemaVersion           int                      `json:"schemaVersion"`
	Generator               string                   `json:"generator"`
	BECVersion              string                   `json:"becVersion"`
	BECArchiveSHA256        string                   `json:"becArchiveSha256"`
	BECExtractedSHA256      string                   `json:"becExtractedSha256"`
	TaxonomyVersion         string                   `json:"taxonomyVersion"`
	TaxonomyArchiveSHA256   string                   `json:"taxonomyArchiveSha256"`
	TaxonomyExtractedSHA256 string                   `json:"taxonomyExtractedSha256"`
	LMH25Edition            string                   `json:"lmh25Edition"`
	LMH25PDFSHA256          string                   `json:"lmh25Pdf2023Sha256"`
	Sources                 []StandardsSourceStamp   `json:"sources"`
	Artifacts               []StandardsArtifactStamp `json:"artifacts"`
}

type AboutInfo struct {
	ProductName    string        `json:"productName"`
	FullName       string        `json:"fullName"`
	Organization   string        `json:"organization"`
	AppVersion     string        `json:"appVersion"`
	SchemaVersion  string        `json:"schemaVersion"`
	ReleaseChannel string        `json:"releaseChannel"`
	LibraryStamps  LibraryStamps `json:"libraryStamps"`
}

type HelpService struct {
	preferences *vprocore.DesktopConfig
	stamps      LibraryStamps
}

func parseStandards2026Provenance(data []byte) (LibraryStamps, error) {
	var raw struct {
		SchemaVersion      int                      `json:"schemaVersion"`
		Generator          string                   `json:"generator"`
		Sources            []StandardsSourceStamp   `json:"sources"`
		LMH25PDF2023SHA256 string                   `json:"lmh25Pdf2023Sha256"`
		Artifacts          []StandardsArtifactStamp `json:"artifacts"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return LibraryStamps{}, fmt.Errorf("decode standards provenance: %w", err)
	}
	if raw.SchemaVersion != 1 || raw.Generator == "" || raw.LMH25PDF2023SHA256 == "" || len(raw.Sources) == 0 || len(raw.Artifacts) == 0 {
		return LibraryStamps{}, errors.New("standards provenance manifest is incomplete")
	}
	stamps := LibraryStamps{
		SchemaVersion:  raw.SchemaVersion,
		Generator:      raw.Generator,
		LMH25Edition:   "LMH25 (2023)",
		LMH25PDFSHA256: raw.LMH25PDF2023SHA256,
		Sources:        make([]StandardsSourceStamp, len(raw.Sources)),
		Artifacts:      make([]StandardsArtifactStamp, len(raw.Artifacts)),
	}
	copy(stamps.Sources, raw.Sources)
	for i, artifact := range raw.Artifacts {
		counts := make(map[string]int, len(artifact.Counts))
		for k, v := range artifact.Counts {
			counts[k] = v
		}
		stamps.Artifacts[i] = StandardsArtifactStamp{
			Path:   artifact.Path,
			SHA256: artifact.SHA256,
			Counts: counts,
		}
	}
	for _, src := range raw.Sources {
		switch {
		case strings.HasPrefix(src.Version, "BECdb_v13"):
			stamps.BECVersion = src.Version
			stamps.BECArchiveSHA256 = src.ArchiveSHA256
			stamps.BECExtractedSHA256 = src.ExtractedSHA256
		case strings.HasPrefix(src.Version, "BCPlantTaxa_v19"):
			stamps.TaxonomyVersion = src.Version
			stamps.TaxonomyArchiveSHA256 = src.ArchiveSHA256
			stamps.TaxonomyExtractedSHA256 = src.ExtractedSHA256
		}
	}
	if stamps.BECVersion == "" || stamps.TaxonomyVersion == "" {
		return LibraryStamps{}, errors.New("standards provenance missing BECdb v13 or BCPlantTaxonomy v19 source stamp")
	}
	return stamps, nil
}

func cloneLibraryStamps(stamps LibraryStamps) LibraryStamps {
	cloned := stamps
	cloned.Sources = make([]StandardsSourceStamp, len(stamps.Sources))
	copy(cloned.Sources, stamps.Sources)
	cloned.Artifacts = make([]StandardsArtifactStamp, len(stamps.Artifacts))
	for i, artifact := range stamps.Artifacts {
		counts := make(map[string]int, len(artifact.Counts))
		for k, v := range artifact.Counts {
			counts[k] = v
		}
		cloned.Artifacts[i] = StandardsArtifactStamp{
			Path:   artifact.Path,
			SHA256: artifact.SHA256,
			Counts: counts,
		}
	}
	return cloned
}

func NewHelpService(preferences *vprocore.DesktopConfig) (*HelpService, error) {
	stamps, err := parseStandards2026Provenance(standards2026ProvenanceJSON)
	if err != nil {
		return nil, err
	}
	return &HelpService{
		preferences: preferences,
		stamps:      stamps,
	}, nil
}

func (s *HelpService) GetLibraryStamps(ctx context.Context) (LibraryStamps, error) {
	if ctx == nil {
		return LibraryStamps{}, errors.New("library stamp lookup requires a context")
	}
	if err := ctx.Err(); err != nil {
		return LibraryStamps{}, err
	}
	if s == nil {
		return LibraryStamps{}, errors.New("help service is unavailable")
	}
	return cloneLibraryStamps(s.stamps), nil
}

func (s *HelpService) GetReleaseChannel(ctx context.Context) (string, error) {
	if ctx == nil {
		return "", errors.New("release channel lookup requires a context")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s == nil || s.preferences == nil {
		return vprocore.ReleaseChannelStable, nil
	}
	return s.preferences.ReleaseChannel()
}

func (s *HelpService) GetAboutInfo(ctx context.Context) (AboutInfo, error) {
	stamps, err := s.GetLibraryStamps(ctx)
	if err != nil {
		return AboutInfo{}, err
	}
	channel, err := s.GetReleaseChannel(ctx)
	if err != nil {
		return AboutInfo{}, err
	}
	return AboutInfo{
		ProductName:    "VPro",
		FullName:       "VENUS — Vegetation and Environment NexUS — Professional",
		Organization:   "B.C. Ministry of Forests, Research Branch — Biogeoclimatic Ecosystem Classification (BEC) Program",
		AppVersion:     appVersion,
		SchemaVersion:  "VP08",
		ReleaseChannel: channel,
		LibraryStamps:  stamps,
	}, nil
}
