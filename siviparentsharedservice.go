package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
)

const siviParentSharedFeatureEnvironment = "VPRO_SIVI_PARENT_SHARED_EDITING"
const siviParentSharedHistoryTable = "__VPRO_SIVIParentSharedHistory"
const siviParentSharedHistorySQL = `CREATE TABLE "__VPRO_SIVIParentSharedHistory"(ID INTEGER PRIMARY KEY,Created TEXT NOT NULL,Proposal TEXT NOT NULL,Restored TEXT)`

var siviParentSharedOwners = map[string]string{
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

var siviParentSharedNumberKinds = map[string]string{
	"SeepageDepth": "integer", "RootingDepth": "integer", "RootRestrictingDepth": "integer",
	"UTMEasting": "single", "UTMNorthing": "single", "SlopeGradient": "single",
	"LocationAccuracy": "integer", "Elevation": "integer", "Aspect": "integer", "StandAge": "integer",
	"StartDate": "integer", "Latitude": "latitude", "Longitude": "longitude",
}

func validateSIVIParentSharedNumber(column, kind string, cell ProjectMetadataCell) error {
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
		return validateOrdinaryInteger(column, &integer)
	}
	if cell.Storage != "real" || cell.Real == nil {
		return fmt.Errorf("SIVI %s requires nullable REAL storage", column)
	}
	switch kind {
	case "single":
		return validateSingleRangeChange(column, nil, cell.Real)
	case "latitude", "longitude":
		_, err := ConvertCoordinate(kind, CoordinateModeDD, CoordinateParts{Degrees: cell.Real})
		return err
	default:
		return fmt.Errorf("SIVI %s has no verified numeric policy", column)
	}
}

var siviParentSharedTextBounds = map[string]int{
	"SiteSurveyor": 30, "FieldNumber": 50, "Location": 255,
	"NtsMapSheet": 8, "UTMZone": 2, "PlotRepresenting": 255,
	"SiteSeries": 5, "MapUnit": 15, "SiteNotes": 0,
}

func siviParentSharedSourceBinding(column string) string {
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
	if err := decodeProfileLifecycleJSON(data, &decoded, "original", "edits"); err != nil {
		return fmt.Errorf("SIVI shared-field transport: %w", err)
	}
	*request = SIVIParentSharedWrite(decoded)
	return nil
}

// Registration is intentionally separate from the XL facade: shared storage
// does not authorize XL callbacks or its whole-header Save.
type SIVIParentSharedService struct {
	contexts          *ContextService
	enabled           bool
	referencesEnabled bool
	references        siviParentSharedReferenceReaders
}

func NewSIVIParentSharedService(contexts *ContextService, lookup func(string) (string, bool)) (*SIVIParentSharedService, error) {
	if contexts == nil || lookup == nil {
		return nil, errors.New("SIVI shared-field service requires context ownership and an explicit feature lookup")
	}
	enabled, err := siviFeature(siviParentSharedFeatureEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	referencesEnabled, err := siviFeature(siviParentSharedReferenceEnvironment, lookup)
	if err != nil {
		return nil, err
	}
	return &SIVIParentSharedService{contexts: contexts, enabled: enabled, referencesEnabled: referencesEnabled}, nil
}

func (s *SIVIParentSharedService) require(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || !s.enabled || !s.contexts.siviParentReviewEnabled {
		return errors.New("SIVI shared-field editing requires independently enabled parent review and shared-field editing")
	}
	return nil
}

func (s *SIVIParentSharedService) GetOriginal(ctx context.Context, contextID, plot string) (*SIVIParentProjection, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.readSIVIParent(ctx, contextID, plot)
}

func planSIVIParentShared(ctx context.Context, original *siviParentProjection, edits []SIVIParentCellEdit) ([]siviParentScalarAssignment, error) {
	return planSIVIParentSharedScope(ctx, original, edits, false)
}

func planSIVIParentSharedScope(ctx context.Context, original *siviParentProjection, edits []SIVIParentCellEdit, references bool) ([]siviParentScalarAssignment, error) {
	if original == nil || len(original.Rows) != 1 {
		return nil, errors.New("SIVI shared-field editing requires one owned physical parent pair")
	}
	env := ProjectMetadataTable{Columns: original.EnvColumns, Rows: []ProjectMetadataRow{original.Rows[0].Env}}
	admin := ProjectMetadataTable{Columns: original.AdminColumns, Rows: []ProjectMetadataRow{original.Rows[0].Admin}}
	sourceOwners := make(map[string]string, len(siviParentSharedOwners))
	for column, owner := range siviParentSharedOwners {
		sourceOwners[siviParentSharedSourceBinding(column)] = owner
	}
	if references {
		for _, field := range siviParentSharedReferenceFields {
			sourceOwners[field.column] = "Env"
		}
	}
	sourceEdits := append([]SIVIParentCellEdit(nil), edits...)
	// The source planner uses literal bindings; writes/history use physical column names.
	for i, edit := range sourceEdits {
		_, allowed := siviParentSharedOwners[edit.Column]
		_, reference := siviParentSharedReferenceField(edit.Column)
		if !allowed && !(references && reference) {
			return nil, fmt.Errorf("SIVI shared field %s is outside the canonical editor scope", edit.Column)
		}
		if edit.Column == "SiteNotes" {
			index, err := siteUnitTransferColumns(env, "SiteNotes")
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
		sourceEdits[i].Column = siviParentSharedSourceBinding(edit.Column)
	}
	assignments, err := planSIVIParentCells(ctx, original.ContextID, original.Project, original.Plot, env, admin, sourceEdits,
		sourceOwners, func(column string, cell ProjectMetadataCell) error {
			if field, reference := siviParentSharedReferenceField(column); reference {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return fmt.Errorf("SIVI %s requires nullable TEXT storage", column)
				}
				return validateSiteCodeText(column, cell.Text, field.maximum)
			}
			if column == "siteNotes" {
				column = "SiteNotes"
			}
			if column == "Date" {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return errors.New("SIVI Date requires nullable TEXT wall-clock storage")
				}
				return validateSIVIParentDateTimestamp(cell.Text)
			}
			if maximum, text := siviParentSharedTextBounds[column]; text {
				if cell.Storage != "null" && (cell.Storage != "text" || cell.Text == nil) {
					return fmt.Errorf("SIVI %s requires nullable TEXT storage", column)
				}
				return validateOrdinaryText(becHeaderField{name: column, table: "Env", maximum: maximum, value: cell.Text})
			}
			value, err := metadataCellValue(cell)
			if err != nil {
				return err
			}
			if kind, number := siviParentSharedNumberKinds[column]; number {
				return validateSIVIParentSharedNumber(column, kind, cell)
			}
			return validateOrdinaryRestoreValue(siviParentSharedOwners[column], column, value)
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

var siviParentSharedHistory = siviParentHistoryDomain{
	siviParentSharedHistoryTable, siviParentSharedHistorySQL, "SIVI shared fields",
	func(ctx context.Context, event siviParentHistory, project, plot string) ([]siviParentScalarAssignment, error) {
		if event.ProjectAssignment != nil {
			return nil, errors.New("ProjectID history is not shared-field history")
		}
		original, err := validateSIVIParentHistoryOriginal(ctx, event, project, plot)
		if err != nil {
			return nil, err
		}
		edits := make([]SIVIParentCellEdit, 0, len(event.Changes))
		for _, change := range event.Changes {
			edits = append(edits, SIVIParentCellEdit{original.ContextID, change.Table, change.RowID, change.Column, change.Before, change.After})
		}
		assignments, err := planSIVIParentSharedScope(ctx, original, edits, true)
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
	if s.referencesEnabled {
		return s.saveWithReferences(ctx, contextID, plot, request)
	}
	return s.contexts.writeSIVIParentPlanned(ctx, contextID, plot, request.Original, siviParentSharedHistory,
		func(observed *siviParentProjection) ([]siviParentScalarAssignment, error) {
			return planSIVIParentShared(ctx, observed, request.Edits)
		})
}

func (s *SIVIParentSharedService) Restore(ctx context.Context, contextID, plot, historyID string, action AuditRestoreAction) (*AuditRestoreResult, error) {
	if err := s.require(ctx); err != nil {
		return nil, err
	}
	return s.contexts.restoreSIVIParentHistory(ctx, contextID, plot, historyID, action, siviParentSharedHistory)
}
