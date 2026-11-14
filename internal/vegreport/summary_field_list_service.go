package vegreport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

const summaryVegetationFieldListTable = "USysShortVegSummaryFieldList"

var summaryVegetationFieldListColumns = []string{
	"Order", "Source", "Label", "Item", "Report", "FunctionName", "FieldNumber",
}

type summaryVegetationFieldListIndex struct {
	Visible  []string
	Selected []string
}

func readSummaryVegetationFieldList(ctx context.Context, tx *sql.Tx) (vprocore.ProjectMetadataTable, error) {
	var kind string
	if err := tx.QueryRowContext(ctx, `SELECT type FROM VPro64.sqlite_master WHERE name COLLATE BINARY=?`,
		summaryVegetationFieldListTable).Scan(&kind); err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("Summary environmental field-list physical table unavailable: %w", err)
	}
	if kind != "table" {
		return vprocore.ProjectMetadataTable{}, errors.New("Summary environmental field-list requires its physical VPro64 table")
	}
	if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, "VPro64", summaryVegetationFieldListTable, "Summary environmental field-list"); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	table, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "VPro64", summaryVegetationFieldListTable, "", nil, "")
	if err != nil {
		return vprocore.ProjectMetadataTable{}, fmt.Errorf("Summary environmental field-list physical read failed: %w", err)
	}
	if err := validateSummaryVegetationFieldListColumns(table); err != nil {
		return vprocore.ProjectMetadataTable{}, err
	}
	return table, nil
}

func validateSummaryVegetationFieldListColumns(table vprocore.ProjectMetadataTable) error {
	if len(table.Columns) != len(summaryVegetationFieldListColumns) {
		return errors.New("Summary environmental field-list requires the exact seven source columns")
	}
	for i, name := range summaryVegetationFieldListColumns {
		if table.Columns[i].Name != name {
			return errors.New("Summary environmental field-list requires the exact source column order and names")
		}
	}
	return nil
}

func indexSummaryVegetationFieldList(ctx context.Context, table vprocore.ProjectMetadataTable) (summaryVegetationFieldListIndex, error) {
	result := summaryVegetationFieldListIndex{Visible: []string{}, Selected: []string{}}
	if err := ctx.Err(); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	if err := validateSummaryVegetationFieldListColumns(table); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	type entry struct {
		rowID    string
		order    *int64
		selected bool
	}
	entries := make([]entry, 0, len(table.Rows))
	for _, row := range table.Rows {
		if err := ctx.Err(); err != nil {
			return summaryVegetationFieldListIndex{}, err
		}
		if len(row.Cells) != len(summaryVegetationFieldListColumns) {
			return summaryVegetationFieldListIndex{}, errors.New("Summary environmental field-list row does not contain all seven source cells")
		}
		item := entry{rowID: row.RowID}
		order, report := row.Cells[0], row.Cells[4]
		if order.Storage != "null" {
			if order.Storage != "integer" || order.Integer == nil {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Order is unavailable without coercion", row.RowID)
			}
			value, err := strconv.ParseInt(*order.Integer, 10, 16)
			if err != nil {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Order is outside source INTEGER storage: %w", row.RowID, err)
			}
			item.order = &value
		}
		if report.Storage != "null" {
			if report.Storage != "integer" || report.Integer == nil || (*report.Integer != "-1" && *report.Integer != "0") {
				return summaryVegetationFieldListIndex{}, fmt.Errorf("Summary environmental field-list row %s Report is unavailable without Boolean normalization", row.RowID)
			}
			item.selected = *report.Integer == "-1"
		}
		entries = append(entries, item)
	}
	// Physical reads are rowid-ordered; source equal-Order ties have no defined order.
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].order == nil {
			return entries[j].order != nil
		}
		return entries[j].order != nil && *entries[i].order < *entries[j].order
	})
	for _, item := range entries {
		if item.order != nil && *item.order > 0 {
			result.Visible = append(result.Visible, item.rowID)
		}
		if item.selected {
			result.Selected = append(result.Selected, item.rowID)
		}
	}
	if err := ctx.Err(); err != nil {
		return summaryVegetationFieldListIndex{}, err
	}
	return result, nil
}

type SummaryVegetationFieldListAction struct {
	Kind     string `json:"kind"`
	RowID    string `json:"rowId"`
	Selected bool   `json:"selected"`
}

type SummaryVegetationFieldListRequest struct {
	RequestID string                             `json:"requestId"`
	Review    SummaryVegetationFieldListReview   `json:"review"`
	Actions   []SummaryVegetationFieldListAction `json:"actions"`
}

func (request SummaryVegetationFieldListRequest) plannedActions() []summaryVegetationFieldListAction {
	if request.Actions == nil {
		return nil
	}
	actions := make([]summaryVegetationFieldListAction, len(request.Actions))
	for i, action := range request.Actions {
		actions[i] = summaryVegetationFieldListAction{Kind: action.Kind, RowID: action.RowID, Selected: action.Selected}
	}
	return actions
}

func (request *SummaryVegetationFieldListRequest) UnmarshalJSON(data []byte) error {
	const operation = "Summary field-list selection request"
	properties, err := sourceform.SourceChildJSONObject(data, operation, []string{"requestId", "review", "actions"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, operation, "requestId", "review", "actions"); err != nil {
		return err
	}
	reviewKeys := []string{"contextId", "project", "projectPath", "su", "suPath", "sourcePath", "source", "physical"}
	review, err := sourceform.SourceChildJSONObject(properties["review"], operation+" review", reviewKeys, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(review, operation+" review", reviewKeys...); err != nil {
		return err
	}
	physical, err := sourceform.SourceChildJSONObject(review["physical"], operation+" physical", []string{"columns", "rows"}, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(physical, operation+" physical", "columns", "rows"); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(physical["columns"], func(raw json.RawMessage) error {
		column, err := sourceform.SourceChildJSONObject(raw, operation+" column", []string{"name", "declaredType"}, nil)
		if err != nil {
			return err
		}
		return sourceform.SourceChildJSONNonNull(column, operation+" column", "name", "declaredType")
	}); err != nil {
		return err
	}
	if err := sourceform.SourceChildRowsJSON(physical["rows"], operation); err != nil {
		return err
	}
	if err := sourceform.SourceChildArrayJSON(properties["actions"], func(raw json.RawMessage) error {
		action, err := sourceform.SourceChildJSONObject(raw, operation+" action", []string{"kind", "rowId", "selected"}, nil)
		if err != nil {
			return err
		}
		return sourceform.SourceChildJSONNonNull(action, operation+" action", "kind", "rowId", "selected")
	}); err != nil {
		return err
	}
	type plain SummaryVegetationFieldListRequest
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if !vprocore.SiviDeletionUUID.MatchString(decoded.RequestID) {
		return errors.New("Summary field-list selection request requires a stable lowercase UUIDv4")
	}
	authority := decoded.Review
	if err := vprocore.ValidateGoogleEarthReviewStrings(authority.ContextID, authority.Project, authority.ProjectPath,
		authority.SU, authority.SUPath, authority.SourcePath, authority.Source); err != nil {
		return err
	}
	for _, value := range []string{authority.ContextID, authority.Project, authority.ProjectPath,
		authority.SU, authority.SUPath, authority.SourcePath, authority.Source} {
		if strings.ContainsRune(value, 0) {
			return errors.New("Summary field-list owned provenance must not contain NUL")
		}
	}
	if authority.ContextID == "" || authority.Project == "" || authority.ProjectPath == "" || authority.SU == "" ||
		authority.SU != "None" && authority.SUPath == "" || authority.SourcePath == "" || authority.Source != summaryVegetationFieldListSource {
		return errors.New("Summary field-list selection request requires complete literal owned provenance")
	}
	result := SummaryVegetationFieldListRequest(decoded)
	if _, err := planSummaryVegetationFieldList(context.Background(), authority.Physical, authority.Physical, result.plannedActions()); err != nil {
		return err
	}
	*request = result
	return nil
}

type SummaryVegetationFieldListAssignment struct {
	Before vprocore.ProjectMetadataRow `json:"before"`
	After  vprocore.ProjectMetadataRow `json:"after"`
}

type SummaryVegetationFieldListCommittedPlan struct {
	Original    vprocore.ProjectMetadataTable          `json:"original"`
	Proposed    vprocore.ProjectMetadataTable          `json:"proposed"`
	Assignments []SummaryVegetationFieldListAssignment `json:"assignments"`
}

type SummaryVegetationFieldListOutcome struct {
	ContextID    string                                   `json:"contextId"`
	RequestID    string                                   `json:"requestId"`
	Changed      bool                                     `json:"changed"`
	Committed    bool                                     `json:"committed"`
	Replayed     bool                                     `json:"replayed"`
	Verified     bool                                     `json:"verified"`
	Plan         *SummaryVegetationFieldListCommittedPlan `json:"plan"`
	ErrorMessage string                                   `json:"errorMessage"`
}

func (s *SummaryVegetationFieldListService) SaveSummaryVegetationFieldList(ctx context.Context, contextID, requestJSON string) *SummaryVegetationFieldListOutcome {
	return s.saveSummaryVegetationFieldList(ctx, contextID, requestJSON, summaryFieldListWriterHooks{})
}

func (s *SummaryVegetationFieldListService) saveSummaryVegetationFieldList(ctx context.Context, contextID, requestJSON string,
	hooks summaryFieldListWriterHooks) *SummaryVegetationFieldListOutcome {
	outcome := &SummaryVegetationFieldListOutcome{ContextID: contextID}
	err := s.require(ctx, contextID)
	var request SummaryVegetationFieldListRequest
	if err == nil {
		err = json.Unmarshal([]byte(requestJSON), &request)
	}
	if err == nil {
		outcome.RequestID = request.RequestID
		if request.Review.ContextID != contextID {
			err = errors.New("Summary field-list Save request does not match the requested context")
		}
	}
	if err == nil {
		result, commitErr := commitSummaryVegetationFieldList(ctx, s, requestJSON, hooks)
		outcome.Changed, outcome.Committed, outcome.Replayed, outcome.Verified =
			result.Changed, result.Committed, result.Replayed, result.Verified
		if result.Plan != nil {
			plan := &SummaryVegetationFieldListCommittedPlan{
				Original:    cloneSummaryVegetationFieldListTable(result.Plan.Original),
				Proposed:    cloneSummaryVegetationFieldListTable(result.Plan.Proposed),
				Assignments: []SummaryVegetationFieldListAssignment{},
			}
			for _, assignment := range result.Plan.Assignments {
				plan.Assignments = append(plan.Assignments, SummaryVegetationFieldListAssignment{
					Before: cloneSummaryVegetationFieldListRow(assignment.Before),
					After:  cloneSummaryVegetationFieldListRow(assignment.After),
				})
			}
			outcome.Plan = plan
		}
		err = commitErr
	}
	if err != nil {
		outcome.ErrorMessage = err.Error()
	}
	return outcome
}

const summaryVegetationFieldListSource = "owned imported SQLite VPro64.USysShortVegSummaryFieldList; complete physical rows; Label read-only"

type SummaryVegetationFieldListReview struct {
	ContextID   string                        `json:"contextId"`
	Project     string                        `json:"project"`
	ProjectPath string                        `json:"projectPath"`
	SU          string                        `json:"su"`
	SUPath      string                        `json:"suPath"`
	SourcePath  string                        `json:"sourcePath"`
	Source      string                        `json:"source"`
	Physical    vprocore.ProjectMetadataTable `json:"physical"`
}

type SummaryVegetationFieldListSelection struct {
	Review   SummaryVegetationFieldListReview `json:"review"`
	Visible  []string                         `json:"visible"`
	Selected []string                         `json:"selected"`
}

type SummaryVegetationFieldListService struct {
	contexts *vprocore.ContextService
	snapshot vprocore.PublicationReadSnapshotHooks
}

func NewSummaryVegetationFieldListService(contexts *vprocore.ContextService) *SummaryVegetationFieldListService {
	return &SummaryVegetationFieldListService{contexts: contexts}
}

func (s *SummaryVegetationFieldListService) require(ctx context.Context, contextID string) error {
	if ctx == nil {
		return errors.New("Summary environmental field-list requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if s == nil || s.contexts == nil || s.contexts.Projects == nil || s.contexts.Projects.Sqlite == nil || s.contexts.Plots == nil {
		return errors.New("Summary environmental field-list context service is unavailable")
	}
	return vprocore.ValidateGoogleEarthReviewStrings(contextID)
}

func (s *SummaryVegetationFieldListService) GetSummaryVegetationFieldList(ctx context.Context, contextID string) (*SummaryVegetationFieldListReview, error) {
	if err := s.require(ctx, contextID); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s.contexts, contextID, func(plots *vprocore.PlotService) (*SummaryVegetationFieldListReview, error) {
		return vprocore.WithOwnedPreferenceSnapshot(ctx, plots, s.snapshot, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*SummaryVegetationFieldListReview, error) {
			physical, err := readSummaryVegetationFieldList(ctx, tx)
			if err != nil {
				return nil, err
			}
			return &SummaryVegetationFieldListReview{
				ContextID: contextID, Project: owner.Selection.Project, ProjectPath: owner.Selection.ProjectPath,
				SU: owner.Selection.SU, SUPath: owner.Selection.SUPath, SourcePath: owner.Attachments["VPro64"],
				Source: summaryVegetationFieldListSource, Physical: physical,
			}, nil
		})
	})
}

func (s *SummaryVegetationFieldListService) GetSummaryVegetationFieldListIndex(ctx context.Context, contextID string) (*SummaryVegetationFieldListSelection, error) {
	review, err := s.GetSummaryVegetationFieldList(ctx, contextID)
	if err != nil {
		return nil, err
	}
	index, err := indexSummaryVegetationFieldList(ctx, review.Physical)
	if err != nil {
		return nil, err
	}
	return &SummaryVegetationFieldListSelection{Review: *review, Visible: index.Visible, Selected: index.Selected}, nil
}

// The caller owns the transaction and must roll back any error, including history.
func applySummaryVegetationFieldList(ctx context.Context, tx *sql.Tx, expected vprocore.ProjectMetadataTable,
	actions []summaryVegetationFieldListAction) (*summaryVegetationFieldListPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if tx == nil {
		return nil, errors.New("Summary field-list assignments require a caller-owned transaction")
	}
	current, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return nil, err
	}
	plan, err := planSummaryVegetationFieldList(ctx, current, expected, actions)
	if err != nil {
		return nil, err
	}
	for _, assignment := range plan.Assignments {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		value, err := vprocore.MetadataCellValue(assignment.After.Cells[4])
		if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, `UPDATE VPro64.USysShortVegSummaryFieldList SET Report=? WHERE rowid=?`,
			value, assignment.Before.RowID)
		if err != nil {
			return nil, fmt.Errorf("Summary field-list row %s Report assignment failed: %w", assignment.Before.RowID, err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			return nil, errors.Join(fmt.Errorf("Summary field-list row %s assignment expected one physical row, found %d",
				assignment.Before.RowID, affected), err)
		}
	}
	observed, err := readSummaryVegetationFieldList(ctx, tx)
	if err != nil {
		return nil, err
	}
	if !reflect.DeepEqual(observed, plan.Proposed) {
		return nil, errors.New("Summary field-list committed plan observation differs; roll back the transaction before retrying")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return plan, nil
}
