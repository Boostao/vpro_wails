package sourceform

import (
	referencespkg "github.com/boostao/vpro-wails/internal/references"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/boostao/vpro-wails/internal/listcatalog"
)

type SiviParentSharedReferencePolicy struct {
	Column, List, Reader string `json:"-" yaml:"-"`
	Maximum              int    `json:"-" yaml:"-"`
	Required             bool   `json:"-" yaml:"-"`
}

var SiviParentSharedReferenceFields = []SiviParentSharedReferencePolicy{
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

func SiviParentSharedReferenceField(column string) (SiviParentSharedReferencePolicy, bool) {
	for _, field := range SiviParentSharedReferenceFields {
		if field.Column == column {
			return field, true
		}
	}
	return SiviParentSharedReferencePolicy{}, false
}

// Borrowed readers must be injected before publication; their owner closes them.
type SiviParentSharedReferenceReaders struct {
	Region interface {
		ListRegionChoices(context.Context) ([]referencespkg.RegionCodeChoice, error)
	} `json:"-" yaml:"-"`
	Site interface {
		ListExposureChoices(context.Context) ([]referencespkg.SiteCodeChoice, error)
		ListSiteDisturbanceChoices(context.Context) ([]referencespkg.SiteCodeChoice, error)
	} `json:"-" yaml:"-"`
	Parent interface {
		ListChoices(context.Context, string) ([]referencespkg.ParentCodeChoice, error)
	} `json:"-" yaml:"-"`
	Geology interface {
		ListBedrockChoices(context.Context) ([]referencespkg.GeologyCodeChoice, error)
	} `json:"-" yaml:"-"`
	Bec interface {
		ListBECZones(context.Context) ([]referencespkg.BECZone, error)
		ListBECSubZones(context.Context, *string) ([]referencespkg.BECSubZone, error)
	} `json:"-" yaml:"-"`
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
	Definitions vprocore.ProjectMetadataTable     `json:"definitions"`
	Choices     []SIVIParentSharedReferenceChoice `json:"choices"`
}

type SIVIParentSharedReferences struct {
	ContextID string                       `json:"contextId"`
	Project   string                       `json:"project"`
	Plot      string                       `json:"plot"`
	Zone      vprocore.ProjectMetadataCell `json:"zone"`
	Fields    []SIVIParentSharedReference  `json:"fields"`
}

var SiviParentReferenceColumns = []vprocore.ProjectMetadataColumn{
	{Name: "ListName", DeclaredType: "TEXT"}, {Name: "ListFilter", DeclaredType: "TEXT"}, {Name: "ItemOrder", DeclaredType: "REAL"},
	{Name: "Item", DeclaredType: "TEXT"}, {Name: "ItemDescription", DeclaredType: "TEXT"}, {Name: "FieldUsedIn", DeclaredType: "TEXT"},
	{Name: "ValidateLoops", DeclaredType: "TEXT"}, {Name: "Validate", DeclaredType: "BOOLEAN"}, {Name: "Note", DeclaredType: "TEXT"}, {Name: "Flag", DeclaredType: "BOOLEAN"},
}

func SiviReferenceText(value *string) vprocore.ProjectMetadataCell {
	if value == nil {
		return vprocore.ProjectMetadataCell{Storage: "null"}
	}
	copy := *value
	return vprocore.ProjectMetadataCell{Storage: "text", Text: &copy}
}

func SiviReferenceBool(value *bool) vprocore.ProjectMetadataCell {
	if value == nil {
		return vprocore.ProjectMetadataCell{Storage: "null"}
	}
	raw := "0"
	if *value {
		raw = "-1"
	}
	return vprocore.ProjectMetadataCell{Storage: "integer", Integer: &raw}
}

func siviReferenceDefinitions(rows []listcatalog.Choice) vprocore.ProjectMetadataTable {
	result := vprocore.ProjectMetadataTable{Columns: append([]vprocore.ProjectMetadataColumn(nil), SiviParentReferenceColumns...), Rows: []vprocore.ProjectMetadataRow{}}
	for _, row := range rows {
		order := vprocore.ProjectMetadataCell{Storage: "null"}
		if row.ItemOrder != nil {
			value := *row.ItemOrder
			order = vprocore.ProjectMetadataCell{Storage: "real", Real: &value}
		}
		result.Rows = append(result.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID, Cells: []vprocore.ProjectMetadataCell{
			SiviReferenceText(row.ListName), SiviReferenceText(row.ListFilter), order, SiviReferenceText(row.Code),
			SiviReferenceText(row.Description), SiviReferenceText(row.FieldUsedIn), SiviReferenceText(row.ValidateLoops),
			SiviReferenceBool(row.Validate), SiviReferenceText(row.Note), SiviReferenceBool(row.Flag),
		}})
	}
	return result
}

func ReadSIVISharedFamilyReference(ctx context.Context, tx *sql.Tx, alias, list string) (vprocore.ProjectMetadataTable, error) {
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, alias, "USysTableOfLists", "shared reference definitions"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	result, err := vprocore.ReadSQLiteStorageRows(ctx, tx, alias, "USysTableOfLists", "ListName", &list, "ItemOrder")
	if err != nil {
		return result, err
	}
	if !reflect.DeepEqual(result.Columns, SiviParentReferenceColumns) {
		return vprocore.ProjectMetadataTable{}, errors.New("SIVI shared reference definitions require the exact ten-column imported schema")
	}
	if len(result.Rows) == 0 {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("SIVI shared reference list %s is unavailable", list)
	}
	return result, nil
}

func ProjectSIVIReferenceCatalogue(result *SIVIParentSharedReference, field SiviParentSharedReferencePolicy, rows []listcatalog.Choice) error {
	if len(rows) == 0 {
		return fmt.Errorf("borrowed %s catalogue is empty", field.List)
	}
	result.Definitions = siviReferenceDefinitions(rows)
	for _, row := range rows {
		choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.Code, Description: row.Description}
		if row.ListName == nil || *row.ListName != field.List || row.Code == nil {
			choice.Diagnostic = "Item is NULL or belongs to another list"
		} else if !row.Selectable {
			choice.Diagnostic = row.Diagnostic
			if choice.Diagnostic == "" {
				choice.Diagnostic = "Item is not selectable in the verified source catalogue"
			}
		} else if err := sqlutil.ValidateSiteCodeText(field.Column, row.Code, field.Maximum); err != nil {
			choice.Diagnostic = err.Error()
		} else {
			choice.Selectable = true
		}
		result.Choices = append(result.Choices, choice)
	}
	return nil
}

func (s *SIVIParentSharedService) ReadReference(ctx context.Context, tx *sql.Tx, alias string, field SiviParentSharedReferencePolicy, zone vprocore.ProjectMetadataCell) (SIVIParentSharedReference, error) {
	result := SIVIParentSharedReference{Column: field.Column, ListName: field.List, Required: field.Required,
		Source: "frozen-DAO-catalogue; source-ordinal identities", Definitions: vprocore.ProjectMetadataTable{Columns: []vprocore.ProjectMetadataColumn{}, Rows: []vprocore.ProjectMetadataRow{}},
		Choices: []SIVIParentSharedReferenceChoice{}}
	var rows []listcatalog.Choice
	switch field.Reader {
	case "family":
		result.Source = "configured-owned-imported-SQLite VLists.USysTableOfLists; physical rowid; not frozen DAO provenance"
		table, err := ReadSIVISharedFamilyReference(ctx, tx, alias, field.List)
		if err != nil {
			return result, err
		}
		result.Definitions = table
		for _, row := range table.Rows {
			code, description := row.Cells[3], row.Cells[4]
			choice := SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: code.Text, Description: description.Text}
			if code.Storage != "text" || code.Text == nil {
				choice.Diagnostic = "Item is not nonempty TEXT"
			} else if err := sqlutil.ValidateSiteCodeText(field.Column, code.Text, field.Maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
			result.Choices = append(result.Choices, choice)
		}
	case "region":
		if s.References.Region == nil {
			return result, errors.New("borrowed Region catalogue is unavailable")
		}
		values, err := s.References.Region.ListRegionChoices(ctx)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "site":
		if s.References.Site == nil {
			return result, errors.New("borrowed site catalogue is unavailable")
		}
		var values []referencespkg.SiteCodeChoice
		var err error
		if field.List == "Exposure" {
			values, err = s.References.Site.ListExposureChoices(ctx)
		} else {
			values, err = s.References.Site.ListSiteDisturbanceChoices(ctx)
		}
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "parent":
		if s.References.Parent == nil {
			return result, errors.New("borrowed parent catalogue is unavailable")
		}
		values, err := s.References.Parent.ListChoices(ctx, field.List)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "geology":
		if s.References.Geology == nil {
			return result, errors.New("borrowed geology catalogue is unavailable")
		}
		values, err := s.References.Geology.ListBedrockChoices(ctx)
		if err != nil {
			return result, err
		}
		for _, row := range values {
			rows = append(rows, listcatalog.Choice(row))
		}
	case "bec":
		result.Source = "verified BEC catalogue; Zone value identities and SubZone source-ordinal identities"
		if s.References.Bec == nil {
			return result, errors.New("borrowed BEC catalogue is unavailable")
		}
		if field.Column == "Zone" {
			values, err := s.References.Bec.ListBECZones(ctx)
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
			if err := sqlutil.ValidateSiteCodeText("Zone", zone.Text, 4); err != nil {
				return result, err
			}
			values, err := s.References.Bec.ListBECSubZones(ctx, zone.Text)
			if err != nil {
				return result, err
			}
			result.Definitions.Columns = []vprocore.ProjectMetadataColumn{
				{Name: "Zone", DeclaredType: "TEXT"}, {Name: "SubZone", DeclaredType: "TEXT"}, {Name: "ZoneDescription", DeclaredType: "TEXT"}, {Name: "Description", DeclaredType: "TEXT"},
			}
			for _, row := range values {
				result.Choices = append(result.Choices, SIVIParentSharedReferenceChoice{RowID: row.RowID, Code: row.SubZone, Description: row.Description})
				result.Definitions.Rows = append(result.Definitions.Rows, vprocore.ProjectMetadataRow{RowID: row.RowID,
					Cells: []vprocore.ProjectMetadataCell{SiviReferenceText(row.Zone), SiviReferenceText(row.SubZone),
						SiviReferenceText(row.ZoneDescription), SiviReferenceText(row.Description)}})
			}
		} else if zone.Storage != "null" {
			return result, errors.New("historical nontext Zone cannot filter SubZone choices")
		}
		for i := range result.Choices {
			choice := &result.Choices[i]
			if choice.Code == nil {
				choice.Diagnostic = "Item is NULL"
			} else if err := sqlutil.ValidateSiteCodeText(field.Column, choice.Code, field.Maximum); err != nil {
				choice.Diagnostic = err.Error()
			} else {
				choice.Selectable = true
			}
		}
	default:
		return result, errors.New("unknown SIVI reference reader")
	}
	if field.Reader != "family" && field.Reader != "bec" {
		if err := ProjectSIVIReferenceCatalogue(&result, field, rows); err != nil {
			return result, err
		}
	}
	result.Available = true
	return result, ctx.Err()
}

func (s *SIVIParentSharedService) GetReferences(ctx context.Context, contextID, plot string, zone vprocore.ProjectMetadataCell) (*SIVIParentSharedReferences, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	if _, err := vprocore.MetadataCellValue(zone); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.Contexts, contextID, func(plots *vprocore.PlotService) (result *SIVIParentSharedReferences, resultErr error) {
		owner := plots.Projects.Sqlite
		if err := sqlutil.AcquireMutexLease(ctx, &owner.Mu); err != nil {
			return nil, err
		}
		defer owner.Mu.Unlock()
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
			return nil, err
		}
		tx, err := owner.BeginReadSnapshot(ctx)
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
		result = &SIVIParentSharedReferences{ContextID: contextID, Project: fresh.Project, Plot: plot, Zone: vprocore.CloneSiteUnitCell(zone), Fields: []SIVIParentSharedReference{}}
		for _, field := range SiviParentSharedReferenceFields {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			row, err := s.ReadReference(ctx, tx, "VLists", field, zone)
			if err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				row.Available, row.Diagnostic = false, err.Error()
			}
			result.Fields = append(result.Fields, row)
		}
		if err := vprocore.ProfileOwnedFiles(owner); err != nil {
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
	var owner *vprocore.SqliteContext
	hooks := SiviParentWriteHooks{
		Prepare: func(conn *sql.Conn) (func(), error) {
			s.Contexts.Projects.Mu.RLock()
			owner = s.Contexts.Projects.Sqlite
			s.Contexts.Projects.Mu.RUnlock()
			if err := vprocore.ProfileOwnedFiles(owner); err != nil {
				return nil, err
			}
			_, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS sivi_refs`, sqlutil.SqliteFileURI(owner.Attachments["VLists"], "ro"))
			return func() {}, err
		},
		Plan: func(tx *sql.Tx, original *SiviParentProjection) ([]SiviParentScalarAssignment, error) {
			assignments, err := PlanSIVIParentSharedScope(ctx, original, request.Edits, true)
			if err != nil {
				return nil, err
			}
			for _, assignment := range assignments {
				field, reference := SiviParentSharedReferenceField(assignment.Column)
				if !reference || !field.Required || assignment.Value == nil {
					continue
				}
				value, ok := assignment.Value.(string)
				if !ok {
					return nil, fmt.Errorf("SIVI %s requires nullable TEXT storage", field.Column)
				}
				row, err := s.ReadReference(ctx, tx, "sivi_refs", field, vprocore.ProjectMetadataCell{Storage: "null"})
				if err != nil {
					return nil, fmt.Errorf("%s requires a verified catalogue: %w", field.Column, err)
				}
				membership := map[string]bool{}
				for _, choice := range row.Choices {
					if choice.Selectable && choice.Code != nil {
						membership[*choice.Code] = true
					}
				}
				if err := vprocore.CanonicalItemError(field.Column, field.List, value, membership); err != nil {
					return nil, err
				}
			}
			return assignments, nil
		},
		Verify: func(*sql.Tx) error { return vprocore.ProfileOwnedFiles(owner) },
	}
	return WriteSIVIParentPlannedWithHooks(s.Contexts, ctx, contextID, plot, request.Original, siviParentSharedHistory, nil, hooks)
}
