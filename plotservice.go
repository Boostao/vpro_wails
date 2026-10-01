package main

import (
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// FS882Header captures the unified Env + Admin join record for a plot.
type FS882Header struct {
	PlotNumber               string   `json:"plotNumber"`
	ProjectID                *string  `json:"projectId"`
	Surveyor                 *string  `json:"surveyor"`
	Date                     *string  `json:"date"`
	FieldNo                  *string  `json:"fieldNo"`
	GeneralLocation          *string  `json:"generalLocation"`
	MapSheet                 *string  `json:"mapSheet"`
	UTMZone                  *string  `json:"utmZone"`
	Easting                  *float64 `json:"easting"`
	Northing                 *float64 `json:"northing"`
	Accuracy                 *int     `json:"accuracy"`
	PlotRepresenting         *string  `json:"plotRepresenting"`
	SiteSeries               *string  `json:"siteSeries"`
	Transition               *string  `json:"transition"`
	MapUnit                  *string  `json:"mapUnit"`
	MoistureRegime           *string  `json:"moistureRegime"`
	NutrientRegime           *string  `json:"nutrientRegime"`
	Successional             *string  `json:"successional"`
	StructuralStage          *string  `json:"structuralStage"`
	StandAge                 *int     `json:"standAge"`
	Elevation                *int     `json:"elevation"`
	Slope                    *float64 `json:"slope"`
	Aspect                   *int     `json:"aspect"`
	MesoSlopePos             *string  `json:"mesoSlopePos"`
	SurfaceShape             *string  `json:"surfaceShape"`
	MicrotopType             *string  `json:"microtopType"`
	MicrotopSize             *string  `json:"microtopSize"`
	FieldNotes               *string  `json:"fieldNotes"`
	OfficeNotes              *string  `json:"officeNotes"`
	StartDate                *int     `json:"startDate"`
	AirPhotoNum              *string  `json:"airPhotoNum"`
	BECSiteUnit              *string  `json:"becSiteUnit"`
	BedrockGeology1          *string  `json:"bedrockGeology1"`
	BedrockGeology2          *string  `json:"bedrockGeology2"`
	BedrockGeology3          *string  `json:"bedrockGeology3"`
	CoarseFragLith1          *string  `json:"coarseFragLith1"`
	CoarseFragLith2          *string  `json:"coarseFragLith2"`
	CoarseFragLith3          *string  `json:"coarseFragLith3"`
	Ecosection               *string  `json:"ecosection"`
	EnteredBy                *string  `json:"enteredBy"`
	Exposure1                *string  `json:"exposure1"`
	Exposure2                *string  `json:"exposure2"`
	FSRegionDistrict         *string  `json:"fsRegionDistrict"`
	FloodingRegimeDur        *string  `json:"floodingRegimeDur"`
	FloodingRegimeFreq       *string  `json:"floodingRegimeFreq"`
	GeoMorProSubSurf         *string  `json:"geoMorProSubSurf"`
	GeoMorProSurf            *string  `json:"geoMorProSurf"`
	HumusForm                *string  `json:"humusForm"`
	HumusFormPhase           *string  `json:"humusFormPhase"`
	HumusThickness           *float64 `json:"humusThickness"`
	HydroGeoSubSystem        *string  `json:"hydroGeoSubSystem"`
	HydroGeoSystem           *string  `json:"hydroGeoSystem"`
	Latitude                 *float64 `json:"latitude"`
	Longitude                *float64 `json:"longitude"`
	Photo                    *string  `json:"photo"`
	RealmClass               *string  `json:"realmClass"`
	RootRestrictingDepth     *int     `json:"rootRestrictingDepth"`
	RootRestrictingType      *string  `json:"rootRestrictingType"`
	RootZoneParticleSize     *string  `json:"rootZoneParticleSize"`
	RootingDepth             *int     `json:"rootingDepth"`
	SeepageDepth             *int     `json:"seepageDepth"`
	SiteDisturbance1         *string  `json:"siteDisturbance1"`
	SiteDisturbance2         *string  `json:"siteDisturbance2"`
	SiteDisturbance3         *string  `json:"siteDisturbance3"`
	SitePlotQuality          *string  `json:"sitePlotQuality"`
	SoilClassGroup           *string  `json:"soilClassGroup"`
	SoilClassSubGroup        *string  `json:"soilClassSubGroup"`
	SoilDrainage             *string  `json:"soilDrainage"`
	SoilNotes                *string  `json:"soilNotes"`
	SoilPlotQuality          *string  `json:"soilPlotQuality"`
	SoilSurveyor             *string  `json:"soilSurveyor"`
	SpeciesListComplete      *bool    `json:"speciesListComplete"`
	StrataCoverHerb          *float64 `json:"strataCoverHerb"`
	StrataCoverMoss          *float64 `json:"strataCoverMoss"`
	StrataCoverShrub         *float64 `json:"strataCoverShrub"`
	StrataCoverTree          *float64 `json:"strataCoverTree"`
	SubZone                  *string  `json:"subZone"`
	SubstrateBedRock         *float64 `json:"substrateBedRock"`
	SubstrateDecWood         *float64 `json:"substrateDecWood"`
	SubstrateMineralSoil     *float64 `json:"substrateMineralSoil"`
	SubstrateOrganicMatter   *float64 `json:"substrateOrganicMatter"`
	SubstrateRocks           *float64 `json:"substrateRocks"`
	SubstrateWater           *float64 `json:"substrateWater"`
	SurfaceExpSubSurf        *string  `json:"surfaceExpSubSurf"`
	SurfaceExpSurf           *string  `json:"surfaceExpSurf"`
	SurficialMaterialSubSurf *string  `json:"surficialMaterialSubSurf"`
	SurficialMaterialSurf    *string  `json:"surficialMaterialSurf"`
	TerrainTextureSubSurf    *string  `json:"terrainTextureSubSurf"`
	TerrainTextureSurf       *string  `json:"terrainTextureSurf"`
	UpdatedFromCards         *bool    `json:"updatedFromCards"`
	UserSiteUnit             *string  `json:"userSiteUnit"`
	VegNotes                 *string  `json:"vegNotes"`
	VegPlotQuality           *string  `json:"vegPlotQuality"`
	VegSurveyor              *string  `json:"vegSurveyor"`
	WaterSource              *string  `json:"waterSource"`
	XCoord                   *float64 `json:"xCoord"`
	YCoord                   *float64 `json:"yCoord"`
	Zone                     *string  `json:"zone"`
	Locked                   bool     `json:"locked"`
}

// VegRecord represents a vegetation entry (Layer A-D).
type VegRecord struct {
	ID           int64    `json:"id"`
	PlotNumber   string   `json:"plotNumber"`
	Species      string   `json:"species"`
	Layer        *string  `json:"layer,omitempty"`
	Cover1       *float64 `json:"cover1,omitempty"`
	Cover2       *float64 `json:"cover2,omitempty"`
	Cover3       *float64 `json:"cover3,omitempty"`
	TotalA       *float64 `json:"totalA,omitempty"`
	Cover4       *float64 `json:"cover4,omitempty"`
	Cover5       *float64 `json:"cover5,omitempty"`
	Cover5a      *float64 `json:"cover5a,omitempty"`
	Cover5b      *float64 `json:"cover5b,omitempty"`
	Cover5c      *float64 `json:"cover5c,omitempty"`
	TotalB       *float64 `json:"totalB,omitempty"`
	Cover6       *float64 `json:"cover6,omitempty"`
	Cover7       *float64 `json:"cover7,omitempty"`
	Cover8       *float64 `json:"cover8,omitempty"`
	Cover9       *float64 `json:"cover9,omitempty"`
	Cover10      *float64 `json:"cover10,omitempty"`
	Collected    *string  `json:"collected,omitempty"`
	Height1      *float64 `json:"height1"`
	Height2      *float64 `json:"height2"`
	Height3      *float64 `json:"height3"`
	Height4      *float64 `json:"height4"`
	Height5      *float64 `json:"height5"`
	Height6      *float64 `json:"height6"`
	LL           *int     `json:"ll"`
	AF           *int     `json:"af"`
	DC           *int     `json:"dc"`
	UT           *int     `json:"ut"`
	VI           *int     `json:"vi"`
	PV           *int     `json:"pv"`
	PG           *int     `json:"pg"`
	FFA          *int     `json:"ffa"`
	Cultural1    *int     `json:"cultural1"`
	Cultural2    *int     `json:"cultural2"`
	Other1       *int     `json:"other1"`
	Other2       *int     `json:"other2"`
	childPresent map[string]bool
}

// HumusRecord represents a soil humus layer entry.
type HumusRecord struct {
	ID                   int64    `json:"id"`
	PlotNumber           string   `json:"plotNumber"`
	Horizon              *string  `json:"horizon,omitempty"`
	UpperDepth           *float64 `json:"upperDepth,omitempty"`
	LowerDepth           *float64 `json:"lowerDepth,omitempty"`
	PH                   *float64 `json:"ph,omitempty"`
	Comment              *string  `json:"comment,omitempty"`
	HumusStructureDegree *string  `json:"humusStructureDegree"`
	HumusStructureKind   *string  `json:"humusStructureKind"`
	MycelAbundance       *string  `json:"mycelAbundance"`
	FecalAbundance       *string  `json:"fecalAbundance"`
	RootsAbundance       *string  `json:"rootsAbundance"`
	RootsSize            *string  `json:"rootsSize"`
	VonPost              *int     `json:"vonPost"`
	childPresent         map[string]bool
}

// MineralRecord represents a soil mineral layer entry.
type MineralRecord struct {
	ID                        int64    `json:"id"`
	PlotNumber                string   `json:"plotNumber"`
	Horizon                   *string  `json:"horizon,omitempty"`
	UpperDepth                *float64 `json:"upperDepth,omitempty"`
	LowerDepth                *float64 `json:"lowerDepth,omitempty"`
	Texture                   *string  `json:"texture,omitempty"`
	Colour                    *string  `json:"colour,omitempty"`
	Comments                  *string  `json:"comments,omitempty"`
	PitDepthLimit             *string  `json:"pitDepthLimit"`
	ASP                       *int     `json:"asp"`
	PercentCoarseFragsGravel  *int     `json:"percentCoarseFragsGravel"`
	PercentCoarseFragsCobbles *int     `json:"percentCoarseFragsCobbles"`
	PercentCoarseFragsStones  *int     `json:"percentCoarseFragsStones"`
	PercentCoarseFragsTotal   *int     `json:"percentCoarseFragsTotal"`
	PercentCoarseFragsShape   *string  `json:"percentCoarseFragsShape"`
	RootsAbundance            *string  `json:"rootsAbundance"`
	RootsSize                 *string  `json:"rootsSize"`
	MineralStructureClass     *string  `json:"mineralStructureClass"`
	MineralStructureKind      *string  `json:"mineralStructureKind"`
	MineralFormpH             *float64 `json:"mineralFormpH"`
	childPresent              map[string]bool
}

// OtherRecord represents an auxiliary data item.
type OtherRecord struct {
	ID           int64   `json:"id"`
	PlotNumber   string  `json:"plotNumber"`
	DataName     *string `json:"dataName,omitempty"`
	DataItem     *string `json:"dataItem,omitempty"`
	UserItem1    *string `json:"userItem1"`
	UserItem2    *string `json:"userItem2"`
	UserItem3    *string `json:"userItem3"`
	UserFlag1    *bool   `json:"userFlag1"`
	UserFlag2    *bool   `json:"userFlag2"`
	UserFlag3    *bool   `json:"userFlag3"`
	childPresent map[string]bool
}

// AuditEntry represents a record in <Project>_Audit.
type AuditEntry struct {
	RowID      string  `json:"rowId"`
	Project    string  `json:"project"`
	User       string  `json:"user"`
	PlotNumber string  `json:"plotNumber"`
	Table      string  `json:"table"`
	EditField  string  `json:"editField"`
	EditWhen   string  `json:"editWhen"`
	BeforeEdit *string `json:"beforeEdit"`
	AfterEdit  *string `json:"afterEdit"`
	Restore    bool    `json:"restore"`
	Flag       bool    `json:"flag"`
	ID         *int64  `json:"id"`
}

type PlotService struct {
	mu             sync.RWMutex
	projects       *ProjectService
	auditStrength  int
	currentUser    string
	siteCodes      *SiteCodeService
	siteCodesError error
	contextScoped  bool
}

func (s *PlotService) requireContextEdit() error {
	if s.contextScoped {
		return nil
	}
	s.projects.mu.RLock()
	defer s.projects.mu.RUnlock()
	if s.projects.sqlite != nil {
		return errors.New("plot mutations require the editor's current context identity")
	}
	return nil
}

func NewPlotService(projects *ProjectService) *PlotService {
	return &PlotService{
		projects:      projects,
		auditStrength: 1, // Default matching Access HKCU AuditStrength
		currentUser:   "User",
	}
}

func newPlotServiceWithPreferences(projects *ProjectService) (*PlotService, error) {
	if projects == nil || projects.preferences == nil {
		return nil, errors.New("shared project preferences are required")
	}
	service := NewPlotService(projects)
	values, err := projects.preferences.snapshot()
	if err != nil {
		return nil, err
	}
	service.auditStrength, err = configInt(values, "Audit", "AuditStrength", 0, 3)
	if err != nil {
		return nil, err
	}
	service.currentUser, err = configString(values, "Current", "User")
	if err != nil {
		return nil, err
	}
	return service, nil
}

func (s *PlotService) SetAuditStrength(strength int) error {
	if strength < 0 || strength > 3 {
		return errors.New("audit strength must be from 0 to 3")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projects.preferences != nil {
		if err := s.projects.preferences.update("Audit", map[string]any{"AuditStrength": strength}); err != nil {
			return err
		}
	}
	s.auditStrength = strength
	return nil
}

func (s *PlotService) SetCurrentUser(user string) error {
	if user == "" {
		return errors.New("audit user must not be empty")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.projects.preferences != nil {
		if err := s.projects.preferences.update("Current", map[string]any{"User": user}); err != nil {
			return err
		}
	}
	s.currentUser = user
	return nil
}

// getActiveDB returns a writable database connection for the active project.
func (s *PlotService) getActiveDB() (*sql.DB, string, error) {
	state, err := s.projects.GetState()
	if err != nil {
		return nil, "", err
	}
	if state.ActiveProject == "" {
		return nil, "", errors.New("no active project")
	}
	dbPath := state.ProjectPath
	if dbPath == "" {
		selected := compatibleProject(state.Projects, state.ActiveProject)
		if selected.Name == "" {
			return nil, "", errors.New("active project file is unavailable")
		}
		dbPath = s.projects.projectFile(selected)
	}
	dbPath, err = filepath.Abs(dbPath)
	if err != nil {
		return nil, "", err
	}
	db, err := sql.Open("sqlite3", sqliteFileURI(dbPath, "rw")+"&_foreign_keys=on")
	if err != nil {
		return nil, "", err
	}
	return db, state.ActiveProject, nil
}

// RestoreAuditRecords preserves the legacy API for rows marked Restore.
func (s *PlotService) RestoreAuditRecords(plotNumber string, removeAuditRows bool) error {
	action := AuditRestoreRetain
	if removeAuditRows {
		action = AuditRestorePrune
	}
	_, err := s.restoreAuditSelection(plotNumber, nil, action, true)
	return err
}

// ListAuditEntries returns the audit trail for a plot.
func (s *PlotService) ListAuditEntries(plotNumber string) ([]AuditEntry, error) {
	db, project, err := s.getActiveDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries, err := readAuditEntries(db, project, `"PlotNumber" = ?`, plotNumber)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Project != project {
			return nil, fmt.Errorf("audit rowId %s has foreign project ownership", entry.RowID)
		}
	}
	return entries, nil
}

// ListVegRecords returns all vegetation rows for a plot. NULL IDs are unsupported
// by the numeric DTO and produce an identity error, never a synthetic zero ID.
func (s *PlotService) ListVegRecords(plotNumber string) ([]VegRecord, error) {
	return listChildRecords[VegRecord](s, "Veg", plotNumber)
}

// SaveVegRecord creates when ID is zero, otherwise replaces an existing row.
func (s *PlotService) SaveVegRecord(r VegRecord) error {
	if strings.TrimSpace(r.PlotNumber) == "" || strings.TrimSpace(r.Species) == "" {
		return errors.New("PlotNumber and Species are required")
	}
	return s.saveChild("Veg", r, childSave)
}

// UpdateVegRecord replaces one existing plot/ID pair, including imported ID=0.
func (s *PlotService) UpdateVegRecord(r VegRecord) error {
	if strings.TrimSpace(r.PlotNumber) == "" || strings.TrimSpace(r.Species) == "" {
		return errors.New("PlotNumber and Species are required")
	}
	return s.saveChild("Veg", r, childUpdate)
}

// DeleteVegRecord deletes a vegetation entry by ID.
func (s *PlotService) DeleteVegRecord(plotNumber string, id int64) error {
	return s.saveChild("Veg", VegRecord{PlotNumber: plotNumber, ID: id}, childDelete)
}

// ListHumusRecords returns all humus rows for a plot.
func (s *PlotService) ListHumusRecords(plotNumber string) ([]HumusRecord, error) {
	return listChildRecords[HumusRecord](s, "Humus", plotNumber)
}

// SaveHumusRecord inserts or updates a humus row.
func (s *PlotService) SaveHumusRecord(r HumusRecord) error {
	return s.saveChild("Humus", r, childSave)
}

// UpdateHumusRecord replaces one existing plot/ID pair, including imported ID=0.
func (s *PlotService) UpdateHumusRecord(r HumusRecord) error {
	return s.saveChild("Humus", r, childUpdate)
}

// DeleteHumusRecord deletes a humus row by ID.
func (s *PlotService) DeleteHumusRecord(plotNumber string, id int64) error {
	return s.saveChild("Humus", HumusRecord{PlotNumber: plotNumber, ID: id}, childDelete)
}

// ListMineralRecords returns all mineral rows for a plot.
func (s *PlotService) ListMineralRecords(plotNumber string) ([]MineralRecord, error) {
	return listChildRecords[MineralRecord](s, "Mineral", plotNumber)
}

// SaveMineralRecord inserts or updates a mineral row.
func (s *PlotService) SaveMineralRecord(r MineralRecord) error {
	return s.saveChild("Mineral", r, childSave)
}

// UpdateMineralRecord replaces one existing plot/ID pair, including imported ID=0.
func (s *PlotService) UpdateMineralRecord(r MineralRecord) error {
	return s.saveChild("Mineral", r, childUpdate)
}

// DeleteMineralRecord deletes a mineral row by ID.
func (s *PlotService) DeleteMineralRecord(plotNumber string, id int64) error {
	return s.saveChild("Mineral", MineralRecord{PlotNumber: plotNumber, ID: id}, childDelete)
}

// ListOtherRecords returns all other data rows for a plot.
func (s *PlotService) ListOtherRecords(plotNumber string) ([]OtherRecord, error) {
	return listChildRecords[OtherRecord](s, "Other", plotNumber)
}

// SaveOtherRecord inserts or updates an other data row.
func (s *PlotService) SaveOtherRecord(r OtherRecord) error {
	return s.saveChild("Other", r, childSave)
}

// UpdateOtherRecord replaces one existing plot/ID pair, including imported ID=0.
func (s *PlotService) UpdateOtherRecord(r OtherRecord) error {
	return s.saveChild("Other", r, childUpdate)
}

// DeleteOtherRecord deletes an other data row by ID.
func (s *PlotService) DeleteOtherRecord(plotNumber string, id int64) error {
	return s.saveChild("Other", OtherRecord{PlotNumber: plotNumber, ID: id}, childDelete)
}
