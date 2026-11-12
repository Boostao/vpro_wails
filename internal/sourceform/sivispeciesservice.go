package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
)

const SiviSpeciesFeatureEnvironment = "VPRO_SIVI_SPECIES_EDITING"

type SIVISpeciesEdit struct {
	RowID    string                       `json:"rowId"`
	Form     string                       `json:"form"`
	Expected vprocore.ProjectMetadataCell `json:"expected"`
	Value    string                       `json:"value"`
	Decision string                       `json:"decision,omitempty"`
	Entered  *string                      `json:"entered,omitempty"`
	Selected *string                      `json:"selected,omitempty"`
}

type SIVISpeciesReferences struct {
	ContextID string                        `json:"contextId"`
	Project   string                        `json:"project"`
	Plot      string                        `json:"plot"`
	Master    vprocore.ProjectMetadataTable `json:"master"`
	Personal  vprocore.ProjectMetadataTable `json:"personal"`
}

type SIVISpeciesWrite struct {
	Original   []SIVIVegetationProjection `json:"original"`
	References SIVISpeciesReferences      `json:"references"`
	Edits      []SIVISpeciesEdit          `json:"edits"`
}

// Validate each raw object before decoding: encoding/json otherwise accepts
// duplicate authority and recased properties, and repairs invalid Unicode.
func siviSpeciesObject(data []byte, required, optional []string) (map[string]json.RawMessage, error) {
	return SourceChildJSONObject(data, "Species", required, optional)
}

func siviSpeciesNonNull(properties map[string]json.RawMessage, names ...string) error {
	return SourceChildJSONNonNull(properties, "Species", names...)
}

func siviSpeciesCellJSON(data []byte) error {
	return SourceChildCellJSON(data, "Species")
}

func siviSpeciesRowsJSON(data []byte) error {
	return SourceChildRowsJSON(data, "Species")
}

func SiviSpeciesTableJSON(data []byte) error {
	properties, err := siviSpeciesObject(data, []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := siviSpeciesNonNull(properties, "columns", "rows"); err != nil {
		return err
	}
	var columns []json.RawMessage
	if err := json.Unmarshal(properties["columns"], &columns); err != nil {
		return err
	}
	for _, raw := range columns {
		column, err := siviSpeciesObject(raw, []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		if err := siviSpeciesNonNull(column, "name", "declaredType"); err != nil {
			return err
		}
	}
	return siviSpeciesRowsJSON(properties["rows"])
}

func (edit *SIVISpeciesEdit) UnmarshalJSON(data []byte) error {
	properties, err := siviSpeciesObject(data, []string{"rowId", "form", "expected", "value"}, []string{"decision", "entered", "selected"})
	if err != nil {
		return err
	}
	if err := siviSpeciesNonNull(properties, "rowId", "form", "expected", "value", "decision", "entered", "selected"); err != nil {
		return err
	}
	if err := siviSpeciesCellJSON(properties["expected"]); err != nil {
		return err
	}
	type plain SIVISpeciesEdit
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	switch decoded.Decision {
	case "":
		if decoded.Entered != nil || decoded.Selected != nil {
			return errors.New("listed Species draft must not carry a decision")
		}
	case "keep":
		if decoded.Entered == nil || decoded.Selected != nil {
			return errors.New("keep Species draft requires entered code and no replacement")
		}
	case "replace", "user":
		if decoded.Entered == nil || decoded.Selected == nil {
			return errors.New("Species decision requires entered and selected codes")
		}
	default:
		return errors.New("unavailable Species decision")
	}
	*edit = SIVISpeciesEdit(decoded)
	return nil
}

func (references *SIVISpeciesReferences) UnmarshalJSON(data []byte) error {
	properties, err := siviSpeciesObject(data, []string{"contextId", "project", "plot", "master", "personal"}, nil)
	if err != nil {
		return err
	}
	if err := siviSpeciesNonNull(properties, "contextId", "project", "plot", "master", "personal"); err != nil {
		return err
	}
	for _, name := range []string{"master", "personal"} {
		if err := SiviSpeciesTableJSON(properties[name]); err != nil {
			return err
		}
	}
	type plain SIVISpeciesReferences
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	for _, source := range []struct {
		table    vprocore.ProjectMetadataTable
		personal bool
	}{{decoded.Master, false}, {decoded.Personal, true}} {
		projected, err := ProjectSIVISpeciesDefinitions(context.Background(), source.table, source.personal)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(projected, source.table) {
			return errors.New("Species review requires exactly the original projected reference columns")
		}
	}
	*references = SIVISpeciesReferences(decoded)
	return nil
}

func (request *SIVISpeciesWrite) UnmarshalJSON(data []byte) error {
	properties, err := siviSpeciesObject(data, []string{"original", "references", "edits"}, nil)
	if err != nil {
		return err
	}
	if err := siviSpeciesNonNull(properties, "original", "references", "edits"); err != nil {
		return err
	}
	var groups []json.RawMessage
	if err := json.Unmarshal(properties["original"], &groups); err != nil {
		return err
	}
	for _, raw := range groups {
		group, err := siviSpeciesObject(raw, []string{"Form", "Query", "Columns", "Rows"}, nil)
		if err != nil {
			return err
		}
		if err := siviSpeciesNonNull(group, "Form", "Query", "Columns", "Rows"); err != nil {
			return err
		}
		if err := siviSpeciesRowsJSON(group["Rows"]); err != nil {
			return err
		}
	}
	type plain SIVISpeciesWrite
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*request = SIVISpeciesWrite(decoded)
	return nil
}

type SIVISpeciesService struct {
	contexts *vprocore.ContextService
	Enabled  bool `json:"-" yaml:"-"`
}

func NewSIVISpeciesService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*SIVISpeciesService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI Species requires context ownership and an explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(SiviSpeciesFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVISpeciesService{contexts, enabled}, nil
}

func (s *SIVISpeciesService) require(ctx context.Context) error {
	if ctx == nil {
		return errors.New("SIVI Species requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.Enabled {
		return errors.New("SIVI Species editing is disabled in this session")
	}
	return nil
}

func (s *SIVISpeciesService) GetOriginal(ctx context.Context, contextID, plot string, extended bool) ([]SIVIVegetationProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReadSIVIVegetation(s.contexts, ctx, contextID, plot, extended)
}

func readSIVISpeciesReferences(ctx context.Context, db vprocore.ProjectMetadataQueryer, contextID, project, plot, masterAlias, personalAlias string) (SIVISpeciesReferences, error) {
	return ReadPhysicalSpeciesReferences(ctx, db, contextID, project, plot, masterAlias, personalAlias, ProjectSIVISpeciesDefinitions)
}

func ReadPhysicalSpeciesReferences(ctx context.Context, db vprocore.ProjectMetadataQueryer, contextID, project, plot, masterAlias, personalAlias string,
	projectDefinitions func(context.Context, vprocore.ProjectMetadataTable, bool) (vprocore.ProjectMetadataTable, error)) (SIVISpeciesReferences, error) {
	if ctx == nil || projectDefinitions == nil {
		return SIVISpeciesReferences{}, errors.New("Species references require a context and explicit physical projection")
	}
	result := SIVISpeciesReferences{ContextID: contextID, Project: project, Plot: plot}
	for _, source := range []struct {
		alias, table string
		personal     bool
		target       *vprocore.ProjectMetadataTable
	}{{masterAlias, "USysAllSpecs", false, &result.Master}, {personalAlias, "USysUserSpp", true, &result.Personal}} {
		raw, err := vprocore.ReadSQLiteStorageRows(ctx, db, source.alias, source.table, "", nil, "")
		if err != nil {
			return SIVISpeciesReferences{}, err
		}
		*source.target, err = projectDefinitions(ctx, raw, source.personal)
		if err != nil {
			return SIVISpeciesReferences{}, err
		}
	}
	return result, ctx.Err()
}

func (s *SIVISpeciesService) GetReferences(ctx context.Context, contextID, plot string) (SIVISpeciesReferences, error) {
	if err := s.require(ctx); err != nil {
		return SIVISpeciesReferences{}, err
	}
	return GetSIVISpeciesReferences(s.contexts, ctx, contextID, plot)
}

func GetSIVISpeciesReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot string) (SIVISpeciesReferences, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (SIVISpeciesReferences, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (SIVISpeciesReferences, error) {
			if err := SiviHeightContextParent(ctx, owner, plot); err != nil {
				return SIVISpeciesReferences{}, err
			}
			if err := SiviSpeciesReadParents(ctx, tx, owner.Selection.Project, plot); err != nil {
				return SIVISpeciesReferences{}, err
			}
			return readSIVISpeciesReferences(ctx, tx, contextID, owner.Selection.Project, plot, "VLists", "VUser")
		})
	})
}

func SiviSpeciesReadParents(ctx context.Context, tx *sql.Tx, project, plot string) error {
	for _, parent := range []struct{ table, column string }{{"Env", "PlotNumber"}, {"Admin", "Plot"}} {
		name := vprocore.QuoteHeaderIdentifier(parent.column)
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM project.`+vprocore.QuoteHeaderIdentifier(project+"_"+parent.table)+
			` WHERE typeof(`+name+`)='text' AND `+name+` COLLATE BINARY IS ?`, plot).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("Species references require unique literal source parents")
		}
	}
	return nil
}

func (s *SIVISpeciesService) SaveReviewed(ctx context.Context, contextID, plot string, extended bool, requestJSON string) (*SIVIHeightWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	var request SIVISpeciesWrite
	if err := json.Unmarshal([]byte(requestJSON), &request); err != nil {
		return nil, err
	}
	if len(request.Original) != 3 || request.References.ContextID != contextID || request.References.Plot != plot {
		return nil, errors.New("Species Save requires three reviewed source groups and owned references")
	}
	edits, err := speciesSIVIDrafts(ctx, request.Edits)
	if err != nil {
		return nil, err
	}
	hooks := SiviVegetationWriteHooks{
		Prepare: func(ctx context.Context, conn *sql.Conn, owner *vprocore.SqliteContext) error {
			if request.References.Project != owner.Selection.Project {
				return errors.New("Species references belong to another project")
			}
			if owner.AttachmentInfo["VUser"] == nil || owner.Attachments["VUser"] == "" {
				return errors.New("Species requires the owned personal reference file")
			}
			if err := owner.ValidateMetadataWriterFiles(); err != nil {
				return err
			}
			_, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "sivi_species_user"`, sqlutil.SqliteFileURI(owner.Attachments["VUser"], "ro"))
			return err
		},
		Validate: func(ctx context.Context, tx *sql.Tx, owner *vprocore.SqliteContext, assignments []SiviHeightAssignment) error {
			observed, err := readSIVISpeciesReferences(ctx, tx, contextID, owner.Selection.Project, plot, "reference", "sivi_species_user")
			if err != nil {
				return err
			}
			if !reflect.DeepEqual(request.References, observed) {
				return errors.New("Species reviewed reference definitions changed; cancel and reload")
			}
			return validateSIVISpeciesAssignments(ctx, assignments, request.Edits, observed)
		},
	}
	return WriteSIVIVegetationCellsWithHooks(s.contexts, ctx, contextID, plot, extended, request.Original, edits, siviSpeciesWritePolicy(), hooks)
}

func (s *SIVISpeciesService) RestoreReviewed(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return RestoreSIVIVegetationCells(s.contexts, ctx, contextID, plot, historyID, action, siviSpeciesWritePolicy())
}
