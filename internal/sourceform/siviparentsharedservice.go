package sourceform

import (
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

const SiviParentSharedFeatureEnvironment = "VPRO_SIVI_PARENT_SHARED_EDITING"
const siviParentSharedHistoryTable = "__VPRO_SIVIParentSharedHistory"
const siviParentSharedHistorySQL = `CREATE TABLE "__VPRO_SIVIParentSharedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var SiviParentSharedOwners = map[string]string{
	"AirPhotoNum": "Env", "XCoord": "Env", "YCoord": "Env",
	"VegNotes":        "Env",
	"StrataCoverTree": "Env", "StrataCoverShrub": "Env",
	"StrataCoverHerb": "Env", "StrataCoverMoss": "Env",
	"HumusThickness": "Admin",
	"SeepageDepth":   "Env", "RootingDepth": "Env", "RootRestrictingDepth": "Env",
	"SiteSurveyor": "Env", "FieldNumber": "Env", "Location": "Env",
	"NtsMapSheet": "Env", "UTMZone": "Env", "PlotRepresenting": "Env",
	"SiteSeries": "Env", "MapUnit": "Env", "SiteNotes": "Env",
	"UTMEasting": "Env", "UTMNorthing": "Env", "SlopeGradient": "Env",
	"LocationAccuracy": "Env", "Elevation": "Env", "Aspect": "Env", "StandAge": "Env",
	"StartDate": "Admin", "Latitude": "Env", "Longitude": "Env",
	"Date": "Env",
}

var SiviParentSharedNumberKinds = map[string]string{
	"SeepageDepth": "integer", "RootingDepth": "integer", "RootRestrictingDepth": "integer",
	"UTMEasting": "single", "UTMNorthing": "single", "SlopeGradient": "single",
	"LocationAccuracy": "integer", "Elevation": "integer", "Aspect": "integer", "StandAge": "integer",
	"StartDate": "integer", "Latitude": "latitude", "Longitude": "longitude",
}

func ValidateSIVIParentSharedNumber(column, kind string, cell vprocore.ProjectMetadataCell) error {
	if cell.Storage == "null" {
		return nil
	}
	if kind == "integer" {
		if cell.Storage != "integer" || cell.Integer == nil {
			return fmt.Errorf("SIVI %s requires nullable INTEGER storage", column)
		}
		integer, err := strconv.Atoi(*cell.Integer)
		if err != nil {
			return fmt.Errorf("SIVI %s integer: %w", column, err)
		}
		return vprocore.ValidateOrdinaryInteger(column, &integer)
	}
	if cell.Storage != "real" || cell.Real == nil {
		return fmt.Errorf("SIVI %s requires nullable REAL storage", column)
	}
	switch kind {
	case "single":
		return vprocore.ValidateSingleRangeChange(column, nil, cell.Real)
	case "latitude", "longitude":
		_, err := vprocore.ConvertCoordinate(kind, vprocore.CoordinateModeDD, vprocore.CoordinateParts{Degrees: cell.Real})
		return err
	default:
		return fmt.Errorf("SIVI %s has no verified numeric policy", column)
	}
}

var SiviParentSharedTextBounds = map[string]int{
	"SiteSurveyor": 30, "FieldNumber": 50, "Location": 255,
	"NtsMapSheet": 8, "UTMZone": 2, "PlotRepresenting": 255,
	"SiteSeries": 5, "MapUnit": 15, "SiteNotes": 0,
}

func SiviParentSharedSourceBinding(column string) string {
	if column == "SiteNotes" {
		return "siteNotes"
	}
	return column
}

type SIVIParentSharedWrite struct {
	Original *SIVIParentProjection `json:"original"`
	Edits    []SIVIParentCellEdit  `json:"edits"`
}

func (request *SIVIParentSharedWrite) UnmarshalJSON(data []byte) error {
	type plain SIVIParentSharedWrite
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("SIVI shared-field transport: %w", err)
	}
	*request = SIVIParentSharedWrite(decoded)
	return nil
}

// Registration is intentionally separate from the XL facade: shared storage
// does not authorize XL callbacks or its whole-header Save.
type SIVIParentSharedService struct {
	Contexts          *vprocore.ContextService         `json:"-" yaml:"-"`
	Enabled           bool                             `json:"-" yaml:"-"`
	ReferencesEnabled bool                             `json:"-" yaml:"-"`
	References        SiviParentSharedReferenceReaders `json:"-" yaml:"-"`
}

func NewSIVIParentSharedService(contexts *vprocore.ContextService, lookup func(string) (string, bool)) (*SIVIParentSharedService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI shared-field service requires context ownership and an explicit feature lookup")
	}
	enabled, err := vprocore.SiviFeature(SiviParentSharedFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	referencesEnabled, err := vprocore.SiviFeature(SiviParentSharedReferenceEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVIParentSharedService{Contexts: contexts, Enabled: enabled, ReferencesEnabled: referencesEnabled}, nil
}

func (s *SIVIParentSharedService) require(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.Contexts == nil || !s.Enabled || !s.Contexts.SiviParentReviewEnabled {
		return errors.New("SIVI shared-field editing requires independently enabled parent review and shared-field editing")
	}
	return nil
}

func (s *SIVIParentSharedService) GetOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return ReadSIVIParent(s.Contexts, ctx, contextID, plot)
}

func PlanSIVIParentShared(ctx context.Context, original *SiviParentProjection, edits []SIVIParentCellEdit) ([]SiviParentScalarAssignment, error) {
	return PlanSIVIParentSharedScope(ctx, original, edits, false)
}

func PlanSIVIParentSharedScope(ctx context.Context, original *SiviParentProjection, edits []SIVIParentCellEdit, references bool) ([]SiviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("SIVI shared-field editing requires one owned physical parent pair")
	}
	env := vprocore.ProjectMetadataTable{Columns: original.EnvColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Env}}
	admin := vprocore.ProjectMetadataTable{Columns: original.AdminColumns, Rows: []vprocore.ProjectMetadataRow{original.Rows[0].Admin}}
	sourceOwners := make(map[string]string, len(SiviParentSharedOwners))
	for column, owner := range SiviParentSharedOwners {
		sourceOwners[SiviParentSharedSourceBinding(column)] = owner
	}
	if references {
		for _, field := range SiviParentSharedReferenceFields {
			sourceOwners[field.Column] = "Env"
		}
	}
	sourceEdits := append([]SIVIParentCellEdit(nil), edits...)
	// The source planner uses literal bindings; writes/history use physical column names.
	for i, edit := range sourceEdits {
		_, allowed := SiviParentSharedOwners[edit.Column]
		_, reference := SiviParentSharedReferenceField(edit.Column)
		if !allowed && !(references && reference) {
			return nil, fmt.Errorf("SIVI shared field %s is outside the canonical editor scope", edit.Column)
		}
		if edit.Column == "SiteNotes" {
			index, err := vprocore.SiteUnitTransferColumns(env, "SiteNotes")
			if err != nil {
				return nil, err
			}
			found := false
			for _, binding := range original.Bindings {
				if binding.Binding == "siteNotes" && !binding.Implicit &&
					binding.Table == original.EnvTable && binding.Column == index["SiteNotes"] {
					found = true
				}
			}
			if !found {
				return nil, errors.New("SIVI Notes requires literal siteNotes bound to physical Env.SiteNotes")
			}
		}
		sourceEdits[i].Column = SiviParentSharedSourceBinding(edit.Column)
	}
	assignments, err := planSIVIParentCells(ctx, original.ContextID, original.Project, original.Plot, env, admin, sourceEdits,
		sourceOwners, func(column string, cell vprocore.ProjectMetadataCell) error {
			if field, reference := SiviParentSharedReferenceField(column); reference {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return fmt.Errorf("SIVI %s requires nullable TEXT storage", column)
				}
				return sqlutil.ValidateSiteCodeText(column, cell.Text, field.Maximum)
			}
			if column == "siteNotes" {
				column = "SiteNotes"
			}
			if column == "Date" {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return errors.New("SIVI Date requires nullable TEXT wall-clock storage")
				}
				return ValidateSIVIParentDateTimestamp(cell.Text)
			}
			if maximum, text := SiviParentSharedTextBounds[column]; text {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return fmt.Errorf("SIVI %s requires nullable TEXT storage", column)
				}
				return vprocore.ValidateOrdinaryText(vprocore.BecHeaderField{Name: column, Table: "Env", Maximum: maximum, Value: cell.Text})
			}
			value, err := vprocore.MetadataCellValue(cell)
			if err != nil {
				return err
			}
			if kind, number := SiviParentSharedNumberKinds[column]; number {
				return ValidateSIVIParentSharedNumber(column, kind, cell)
			}
			return vprocore.ValidateOrdinaryRestoreValue(SiviParentSharedOwners[column], column, value)
		})
	if err != nil {
		return nil, err
	}
	for i := range assignments {
		if assignments[i].Column == "siteNotes" {
			assignments[i].Column = "SiteNotes"
		}
	}
	return assignments, nil
}

var siviParentSharedHistory = SiviParentHistoryDomain{
	siviParentSharedHistoryTable, siviParentSharedHistorySQL, "SIVI shared fields",
	func(ctx context.Context, event SiviParentHistory, project, plot string) ([]SiviParentScalarAssignment, error) {
		if event.ProjectAssignment != nil {
			return nil, errors.New("ProjectID history is not shared-field history")
		}
		original, err := ValidateSIVIParentHistoryOriginal(ctx, event, project, plot)
		if err != nil {
			return nil, err
		}
		edits := make([]SIVIParentCellEdit, 0, len(event.Changes))
		for _, change := range event.Changes {
			edits = append(edits, SIVIParentCellEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
		}
		assignments, err := PlanSIVIParentSharedScope(ctx, original, edits, true)
		if err != nil || len(assignments) != len(event.Changes) {
			return nil, errors.Join(err, errors.New("typed shared-field history contains repeated or unchanged assignments"))
		}
		return assignments, nil
	},
}

func (s *SIVIParentSharedService) Save(ctx context.Context, contextID, plot string, request SIVIParentSharedWrite) (*SIVIParentWriteResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	if s.ReferencesEnabled {
		return s.saveWithReferences(ctx, contextID, plot, request)
	}
	return WriteSIVIParentPlanned(s.Contexts, ctx, contextID, plot, request.Original, siviParentSharedHistory,
		func(observed *SiviParentProjection) ([]SiviParentScalarAssignment, error) {
			return PlanSIVIParentShared(ctx, observed, request.Edits)
		})
}

func (s *SIVIParentSharedService) Restore(ctx context.Context, contextID, plot, historyID string, action vprocore.AuditRestoreAction) (*vprocore.AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return RestoreSIVIParentHistory(s.Contexts, ctx, contextID, plot, historyID, action, siviParentSharedHistory)
}
