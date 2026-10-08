package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

const siviParentSharedReferenceEnvironment = "VPRO_SIVI_PARENT_REFERENCE_EDITING"

type siviParentSharedReferencePolicy struct {
	column, list, reader string
	maximum              int
	required             bool
}

var siviParentSharedReferenceFields = []siviParentSharedReferencePolicy{
	{"FSRegionDistrict", "Region", "region", 7, false},
	{"Zone", "Zone", "bec", 4, false}, {"SubZone", "SubZone", "bec", 8, false},
	{"RealmClass", "RealmClass", "parent", 5, false},
	{"MoistureRegime", "MoistureRegime", "family", 3, false},
	{"NutrientRegime", "NutrientRegime", "family", 2, false},
	{"MesoSlopePosition", "MesoSlopePosition", "family", 3, false},
	{"SurfaceShape", "SurfaceShape", "family", 3, false},
	{"Exposure1", "Exposure", "site", 2, true}, {"Exposure2", "Exposure", "site", 2, true},
	{"SiteDisturbance1", "SiteDisturbance", "site", 8, false},
	{"SiteDisturbance2", "SiteDisturbance", "site", 8, false},
	{"SiteDisturbance3", "SiteDisturbance", "site", 8, false},
	{"StructuralStage", "StructuralStage", "family", 6, false},
	{"SuccessionalStatus", "SuccessionalStatus", "family", 3, true},
	{"TerrainTextureSurf", "TerrainTexture", "parent", 3, false},
	{"SurficialMaterialSurf", "SurficialMaterial", "parent", 6, false},
	{"SurfaceExpSurf", "SurfaceExp", "parent", 3, false},
	{"GeoMorProSurf", "GeoMorPro", "parent", 3, false},
	{"BedrockGeology1", "BedrockType", "geology", 4, false},
	{"TerrainTextureSubSurf", "TerrainTexture", "parent", 3, false},
	{"SurficialMaterialSubSurf", "SurficialMaterial", "parent", 6, false},
	{"SurfaceExpSubSurf", "SurfaceExp", "parent", 3, false},
	{"GeoMorProSubSurf", "GeoMorPro", "parent", 3, false},
	{"BedrockGeology2", "BedrockType", "geology", 4, false},
	{"HumusForm", "HumusForm", "parent", 4, false},
	{"SoilDrainage", "SoilDrainage", "parent", 5, true},
	{"RootRestrictingType", "RootRestrictingType", "parent", 1, false},
}

func siviParentSharedReferenceField(column string) (siviParentSharedReferencePolicy, bool) {
	for _, field := range siviParentSharedReferenceFields {
		if field.column == column {
			return field, true
		}
	}
	return siviParentSharedReferencePolicy{}, false
}

// Borrowed readers must be injected before publication; their owner closes them.
type siviParentSharedReferenceReaders struct {
	region interface {
		ListRegionChoices(context.Context) ([]RegionCodeChoice, error)
	}
	site interface {
		ListExposureChoices(context.Context) ([]SiteCodeChoice, error)
		ListSiteDisturbanceChoices(context.Context) ([]SiteCodeChoice, error)
	}
	parent interface {
		ListChoices(context.Context, string) ([]ParentCodeChoice, error)
	}
	geology interface {
		ListBedrockChoices(context.Context) ([]GeologyCodeChoice, error)
	}
	bec interface {
		ListBECZones(context.Context) ([]BECZone, error)
		ListBECSubZones(context.Context, *string) ([]BECSubZone, error)
	}
}

type SIVIParentSharedReferenceChoice struct {
	RowID       string  `json:"rowId"`
	Code        *string `json:"code"`
	Description *string `json:"description"`
	Selectable  bool    `json:"selectable"`
	Diagnostic  string  `json:"diagnostic"`
}

type SIVIParentSharedReference struct {
	Column      string                            `json:"column"`
	ListName    string                            `json:"listName"`
	Required    bool                              `json:"required"`
	Source      string                            `json:"source"`
	Available   bool                              `json:"available"`
	Diagnostic  string                            `json:"diagnostic"`
	Definitions ProjectMetadataTable              `json:"definitions"`
	Choices     []SIVIParentSharedReferenceChoice `json:"choices"`
}

type SIVIParentSharedReferences struct {
	ContextID string                      `json:"contextId"`
	Project   string                      `json:"project"`
	Plot      string                      `json:"plot"`
	Zone      ProjectMetadataCell         `json:"zone"`
	Fields    []SIVIParentSharedReference `json:"fields"`
}

var siviParentReferenceColumns = []ProjectMetadataColumn{
	{"ListName", "TEXT"}, {"ListFilter", "TEXT"}, {"ItemOrder", "REAL"},
	{"Item", "TEXT"}, {"ItemDescription", "TEXT"}, {"FieldUsedIn", "TEXT"},
	{"ValidateLoops", "TEXT"}, {"Validate", "BOOLEAN"}, {"Note", "TEXT"}, {"Flag", "BOOLEAN"},
}

func siviReferenceText(value *string) ProjectMetadataCell {
	if value == nil {
		return ProjectMetadataCell{Storage: "null"}
	}
	copy := *value
	return ProjectMetadataCell{Storage: "text", Text: &copy}
}

func siviReferenceBool(value *bool) ProjectMetadataCell {
	if value == nil {
		return ProjectMetadataCell{Storage: "null"}
	}
	raw := "0"
	if *value {
		raw = "-1"
	}
	return ProjectMetadataCell{Storage: "integer", Integer: &raw}
}

func siviReferenceDefinitions(rows []listcatalog.Choice) ProjectMetadataTable {
	result := ProjectMetadataTable{Columns: append([]ProjectMetadataColumn(nil), siviParentReferenceColumns...), Rows: []ProjectMetadataRow{}}
	for _, row := range rows {
		order := ProjectMetadataCell{Storage: "null"}
		if row.ItemOrder != nil {
			value := *row.ItemOrder
			order = ProjectMetadataCell{Storage: "real", Real: &value}
		}
		result.Rows = append(result.Rows, ProjectMetadataRow{RowID: row.RowID, Cells: []ProjectMetadataCell{
			siviReferenceText(row.ListName), siviReferenceText(row.ListFilter), order, siviReferenceText(row.Code),
			siviReferenceText(row.Description), siviReferenceText(row.FieldUsedIn), siviReferenceText(row.ValidateLoops),
			siviReferenceBool(row.Validate), siviReferenceText(row.Note), siviReferenceBool(row.Flag),
		}})
	}
	return result
}

func readSIVISharedFamilyReference(ctx context.Context, tx *sql.Tx, alias, list string) (ProjectMetadataTable, error) {
	if err := validateSIVIPhysicalSchema(ctx, tx, alias, "USysTableOfLists", "shared reference definitions"); err != nil {
		return ProjectMetadataTable{}, err
	}
	result, err := readSQLiteStorageRows(ctx, tx, alias, "USysTableOfLists", "ListName", &list, "ItemOrder")
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(result.Columns, siviParentReferenceColumns) {
		return ProjectMetadataTable{}, errors.New("SIVI shared reference definitions require the exact ten-column imported schema")
	}
	if len(result.Rows) == 0 {
		return ProjectMetadataTable{}, fmt.Errorf("SIVI shared reference list %s is unavailable", list)
	}
	return result, nil
}

func (s *SIVIParentSharedService) readReference(ctx context.Context, tx *sql.Tx, alias string, field siviParentSharedReferencePolicy, zone ProjectMetadataCell) (SIVIParentSharedReference, error) {
	result := SIVIParentSharedReference{Column: field.column, ListName: field.list, Required: field.required,
		Source: "frozen-DAO-catalogue; source-ordinal identities", Definitions: ProjectMetadataTable{Columns: []ProjectMetadataColumn{}, Rows: []ProjectMetadataRow{}},
		Choices: []SIVIParentSharedReferenceChoice{}}
	var rows []listcatalog.Choice
	switch field.reader {
	case "family":
		result.Source = "configured-owned-imported-SQLite VLists.USysTableOfLists; physical rowid; not frozen DAO provenance"
		table, err := readSIVISharedFamilyReference(ctx, tx, alias, field.list)
		if err != nil {
			return result, err
		}
		result.Definitions = table
		for _, row := range table.Rows {
			code, description := row.Cells[3], row.Cells[4]
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: code.Text, Description: description.Text}
			if code.Storage != "text" || code.Text == nil {
				choice.Diagnostic = "Item is not nonempty TEXT"
			} else if err := validateSiteCodeText(field.column, code.Text, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	case "region":
		if s.references.region == nil {
			return result, errors.New("borrowed Region catalogue is unavailable")
		}
		values, err := s.references.region.ListRegionChoices(ctx)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "site":
		if s.references.site == nil {
			return result, errors.New("borrowed site catalogue is unavailable")
		}
		var values []SiteCodeChoice
		var err error
		if field.list == "Exposure" {
			values, err = s.references.site.ListExposureChoices(ctx)
		} else {
			values, err = s.references.site.ListSiteDisturbanceChoices(ctx)
		}
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "parent":
		if s.references.parent == nil {
			return result, errors.New("borrowed parent catalogue is unavailable")
		}
		values, err := s.references.parent.ListChoices(ctx, field.list)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "geology":
		if s.references.geology == nil {
			return result, errors.New("borrowed geology catalogue is unavailable")
		}
		values, err := s.references.geology.ListBedrockChoices(ctx)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "bec":
		result.Source = "verified BEC catalogue; Zone value identities and SubZone source-ordinal identities"
		if s.references.bec == nil {
			return result, errors.New("borrowed BEC catalogue is unavailable")
		}
		if field.column == "Zone" {
			values, err := s.references.bec.ListBECZones(ctx)
			if err != nil {
				return result, err
			}
			if len(values) == 0 {
				return result, errors.New("borrowed BEC Zone catalogue is empty")
			}
			for _, row := range values {
				result.Choices = append(result.Choices, SIVIParentSharedReferenceChoice{Code: row.Zone, Description: row.Description})
			}
		} else if zone.Storage == "text" && zone.Text != nil {
			if err := validateSiteCodeText("Zone", zone.Text, 4); err != nil {
				return result, err
			}
			values, err := s.references.bec.ListBECSubZones(ctx, zone.Text)
			if err != nil {
				return result, err
			}
			result.Definitions.Columns = []ProjectMetadataColumn{
				{"Zone", "TEXT"}, {"SubZone", "TEXT"}, {"ZoneDescription", "TEXT"}, {"Description", "TEXT"},
			}
			for _, row := range values {
				result.Choices = append(result.Choices, SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SubZone, Description: row.Description})
				result.Definitions.Rows = append(result.Definitions.Rows, ProjectMetadataRow{RowID: row.RowID,
					Cells: []ProjectMetadataCell{siviReferenceText(row.Zone), siviReferenceText(row.SubZone),
						siviReferenceText(row.ZoneDescription), siviReferenceText(row.Description)}})
			}
		} else if zone.Storage != "null" {
			return result, errors.New("historical nontext Zone cannot filter SubZone choices")
		}
		for i := range result.Choices {
			choice := &result.Choices[i]
			if choice.Code == nil {
				choice.Diagnostic = "Item is NULL"
			} else if err := validateSiteCodeText(field.column, choice.Code, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
		}
	default:
		return result, errors.New("unknown SIVI reference reader")
	}
	if field.reader != "family" && field.reader != "bec" {
		if len(rows) == 0 {
			return result, fmt.Errorf("borrowed %s catalogue is empty", field.list)
		}
		result.Definitions = siviReferenceDefinitions(rows)
		for _, row := range rows {
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.Code, Description: row.Description}
			if row.ListName == nil || *row.ListName != field.list || row.Code == nil {
				choice.Diagnostic = "Item is NULL or belongs to another list"
			} else if !row.Selectable {
				choice.Diagnostic = row.Diagnostic
				if choice.Diagnostic == "" {
					choice.Diagnostic = "Item is not selectable in the verified source catalogue"
				}
			} else if err := validateSiteCodeText(field.column, row.Code, field.maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	}
	result.Available = true
	return result, ctx.Err()
}

func (s *SIVIParentSharedService) GetReferences(ctx context.Context, contextID, plot string, zone ProjectMetadataCell) (*SIVIParentSharedReferences, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	if !s.referencesEnabled {
		return nil, errors.New("SIVI reference editing is independently disabled")
	}
	if _, err := metadataCellValue(zone); err != nil {
		return nil, err
	}
	return withContextPlotRequest(ctx, s.contexts, contextID, func(plots *PlotService) (result *SIVIParentSharedReferences, resultErr error) {
		owner := plots.projects.sqlite
		if err := acquireMutexLease(ctx, &owner.mu); err != nil {
			return nil, err
		}
		defer owner.mu.Unlock()
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		tx, err := owner.beginReadSnapshot(ctx)
		if err != nil {
			return nil, err
		}
		defer func() {
			if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
				resultErr = errors.Join(resultErr, err)
			}
			if resultErr != nil {
				result = nil
			}
		}()
		fresh, err := readSIVIParentSnapshot(ctx, owner, tx, contextID, plot)
		if err != nil {
			return nil, err
		}
		result = &SIVIParentSharedReferences{ContextID: contextID, Project: fresh.Project, Plot: plot, Zone: cloneSiteUnitCell(zone), Fields: []SIVIParentSharedReference{}}
		for _, field := range siviParentSharedReferenceFields {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := s.readReference(ctx, tx, "VLists", field, zone)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				row.Available, row.Diagnostic = false, err.Error()
			}
			result.Fields = append(result.Fields, row)
		}
		if err := profileOwnedFiles(owner); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (s *SIVIParentSharedService) saveWithReferences(ctx context.Context, contextID, plot string, request SIVIParentSharedWrite) (*SIVIParentWriteResult, error) {
	var owner *sqliteContext
	hooks := siviParentWriteHooks{
		prepare: func(conn *sql.Conn) (func(), error) {
			s.contexts.projects.mu.RLock()
			owner = s.contexts.projects.sqlite
			s.contexts.projects.mu.RUnlock()
			if err := profileOwnedFiles(owner); err != nil {
				return nil, err
			}
			_, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS sivi_refs`, sqliteFileURI(owner.attachments["VLists"], "ro"))
			return func() {}, err
		},
		plan: func(tx *sql.Tx, original *siviParentProjection) ([]siviParentScalarAssignment, error) {
			assignments, err := planSIVIParentSharedScope(ctx, original, request.Edits, true)
			if err != nil {
				return nil, err
			}
			for _, assignment := range assignments {
				field, reference := siviParentSharedReferenceField(assignment.Column)
				if !reference || !field.required || assignment.Value == nil {
					continue
				}
				value, ok := assignment.Value.(string)
				if !ok {
					return nil, fmt.Errorf("SIVI %s requires nullable TEXT storage", field.column)
				}
				row, err := s.readReference(ctx, tx, "sivi_refs", field, ProjectMetadataCell{Storage: "null"})
				if err != nil {
					return nil, fmt.Errorf("%s requires a verified catalogue: %w", field.column, err)
				}
				membership := map[string]bool{}
				for _, choice := range row.Choices {
					if choice.Selectable && choice.Code != nil {
						membership[*choice.Code] = true
					}
				}
				if err := canonicalItemError(field.column, field.list, value, membership); err != nil {
					return nil, err
				}
			}
			return assignments, nil
		},
		verify: func(*sql.Tx) error { return profileOwnedFiles(owner) },
	}
	return s.contexts.writeSIVIParentPlannedWithHooks(ctx, contextID, plot, request.Original, siviParentSharedHistory, nil, hooks)
}
