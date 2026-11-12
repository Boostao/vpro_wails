package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"bytes"
	"context"
	"database/sql"

	"encoding/json"
	"errors"
	"fmt"
	"github.com/boostao/vpro-wails/resources"
	"reflect"
	"strconv"
)

var metadataTemplateMappingJSON = resources.ProjectMetadataTemplateJSON

type ProjectMetadataTemplateCreate struct {
	Blank  ProjectMetadataCreate            `json:"blank"`
	Master vprocore.ProjectMetadataTable    `json:"master"`
	RowID  string                           `json:"rowId"`
	Values []vprocore.ProjectMetadataChange `json:"values"`
}

func (request *ProjectMetadataTemplateCreate) UnmarshalJSON(data []byte) error {
	if err := vprocore.ValidateMetadataDraftJSON(data); err != nil {
		return err
	}
	type plain ProjectMetadataTemplateCreate
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var decoded plain
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, name := range []string{"blank", "master", "rowId", "values"} {
		if _, present := properties[name]; !present || bytes.Equal(bytes.TrimSpace(properties[name]), []byte("null")) {
			return fmt.Errorf("metadata template proposal requires explicit non-NULL %s", name)
		}
	}
	*request = ProjectMetadataTemplateCreate(decoded)
	return nil
}

func metadataTemplateMapping() ([]string, error) {
	var names []string
	if err := json.Unmarshal(metadataTemplateMappingJSON, &names); err != nil {
		return nil, fmt.Errorf("metadata template source mapping unavailable: %w", err)
	}
	if len(names) != 33 || names[0] != "ProjectID" {
		return nil, errors.New("metadata template requires the exact source33-field mapping")
	}
	seen := map[string]bool{"ProjectID": true}
	for _, name := range names[1:] {
		if _, available := vprocore.ProjectMetadataFields[name]; !available || seen[name] {
			return nil, fmt.Errorf("metadata template source field %q is unknown or repeated", name)
		}
		seen[name] = true
	}
	return names, nil
}

func CreateProjectMetadataFromTemplate(s *vprocore.ContextService, ctx context.Context, contextID string, request ProjectMetadataTemplateCreate) (vprocore.ProjectMetadataRow, error) {
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (vprocore.ProjectMetadataRow, error) {
		return CreateProjectMetadata(plots, request.Blank, &request)
	})
}

func metadataTemplatePlan(ctx context.Context, tx *sql.Tx, request ProjectMetadataTemplateCreate) ([]vprocore.ProjectMetadataChange, error) {
	rowID, err := strconv.ParseInt(request.RowID, 10, 64)
	if err != nil || strconv.FormatInt(rowID, 10) != request.RowID {
		return nil, errors.New("metadata template requires one explicit literal physical master row identity")
	}
	mapping, err := metadataTemplateMapping()
	if err != nil {
		return nil, err
	}
	current, err := vprocore.ReadProjectMetadataRows(ctx, tx, "metadataMaster", "ProjectMetaData", request.Blank.ProjectID, false)
	if err != nil {
		return nil, fmt.Errorf("metadata template source read unavailable: %w", err)
	}
	if request.Master.Columns == nil || request.Master.Rows == nil || !reflect.DeepEqual(current, request.Master) {
		return nil, errors.New("metadata master candidates/schema changed; Undo and reload before proposing a template")
	}
	selected := 0
	for _, row := range current.Rows {
		if row.RowID == request.RowID {
			selected++
		}
	}
	if selected != 1 {
		return nil, errors.New("select exactly one matching physical master template; duplicate bulk copy is unavailable")
	}
	columns := map[string]bool{}
	for _, column := range current.Columns {
		if columns[column.Name] {
			return nil, errors.New("metadata master schema has repeated source columns")
		}
		columns[column.Name] = true
	}
	for _, name := range mapping {
		if !columns[name] {
			return nil, fmt.Errorf("metadata master source field %q is missing; no defaults were inferred", name)
		}
	}
	if len(request.Values) != len(mapping)-1 {
		return nil, errors.New("metadata template requires all32 ordinary assignments, including explicit NULL/year/code decisions")
	}
	values := map[string]vprocore.ProjectMetadataCell{}
	for _, change := range request.Values {
		if _, duplicate := values[change.Column]; duplicate {
			return nil, errors.New("metadata template assignments cannot repeat a field")
		}
		values[change.Column] = change.Value
	}
	plan := make([]vprocore.ProjectMetadataChange, 0, len(mapping)-1)
	for _, name := range mapping[1:] {
		value, present := values[name]
		if !present {
			return nil, fmt.Errorf("metadata template requires explicit %s; identity/stamps and unmapped fields are not assigned", name)
		}
		if _, err := vprocore.ValidateProjectMetadataAssignment(name, value); err != nil {
			return nil, err
		}
		plan = append(plan, vprocore.ProjectMetadataChange{Column: name, Value: value})
	}
	return plan, nil
}
