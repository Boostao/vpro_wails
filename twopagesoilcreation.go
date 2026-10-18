package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func validateTwoPageSoilRequestPlan(ctx context.Context, request twoPageSoilRequest) error {
	if request.Horizon != nil {
		if len(request.Edits) != 0 {
			return errors.New("soil creation and existing-row edits require separate explicit requests")
		}
		_, err := validateTwoPageSoilCreation(ctx, request.Original, *request.Horizon)
		return err
	}
	_, err := planTwoPageSoilEdits(ctx, request.Original, request.Edits)
	return err
}

func validateTwoPageSoilCreation(ctx context.Context, original *twoPageSoilOriginal, horizon string) (map[string]int, error) {
	if ctx == nil || original == nil {
		return nil, errors.New("soil creation requires an owned physical original and context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original.ContextID == "" || original.Project == "" || original.Plot == "" ||
		!utf8.ValidString(original.ContextID) || !utf8.ValidString(original.Project) {
		return nil, errors.New("soil creation requires literal context, project and parent identities")
	}
	if err := validateChildPhysicalText("soil creation PlotNumber", original.Plot, 7); err != nil {
		return nil, err
	}
	if horizon == "" {
		return nil, errors.New("explicit horizon creation requires nonempty Horizon text; no blank record was inferred")
	}
	var kind string
	var unbound []string
	switch original.Form {
	case "SoilHumusXL":
		kind = "Humus"
		unbound = []string{"Consistence", "Character", "Fauna"}
	case "SoilMineralXL":
		kind = "Mineral"
		unbound = []string{"MottlesAbundance", "MottlesSize", "MottlesContrast",
			"ClayFilmsFreq", "ClayFilmThickness", "Effervescence", "Porosity"}
	default:
		return nil, fmt.Errorf("soil creation does not support source form %q", original.Form)
	}
	if err := validateSoilChildField(kind, "Horizon", horizon); err != nil {
		return nil, err
	}
	if original.RecordSource != "Usys"+kind || original.Table != original.Project+"_"+kind {
		return nil, errors.New("soil creation original belongs to another physical source")
	}
	allowed := map[string]bool{"PlotNumber": true, "ID": true, "Flag": true}
	for _, field := range childFields[kind] {
		allowed[field.column] = true
	}
	for _, column := range unbound {
		allowed[column] = true
	}
	columns, err := siteUnitTransferColumns(original.Records, "ID", "PlotNumber", "Horizon", "Flag")
	if err != nil {
		return nil, fmt.Errorf("soil creation complete schema: %w", err)
	}
	if len(columns) != len(allowed) {
		return nil, errors.New("soil creation requires the complete original source schema; no missing or unknown defaults were inferred")
	}
	for _, column := range original.Records.Columns {
		if !allowed[column.Name] || !utf8.ValidString(column.DeclaredType) {
			return nil, fmt.Errorf("soil creation source column %q is unknown or malformed", column.Name)
		}
	}
	for _, row := range original.Records.Rows {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, cell := range row.Cells {
			if _, err := metadataCellValue(cell); err != nil {
				return nil, fmt.Errorf("soil creation original row %s: %w", row.RowID, err)
			}
		}
		parent := row.Cells[columns["PlotNumber"]]
		if parent.Storage != "text" || parent.Text == nil || *parent.Text != original.Plot {
			return nil, errors.New("soil creation original contains a row outside its literal TEXT parent")
		}
	}
	return columns, ctx.Err()
}

func planTwoPageSoilCreation(ctx context.Context, original *twoPageSoilOriginal, id int64, horizon string) ([]ProjectMetadataCell, error) {
	columns, err := validateTwoPageSoilCreation(ctx, original, horizon)
	if err != nil {
		return nil, err
	}
	if id < -2147483648 || id > 2147483647 {
		return nil, errors.New("soil creation requires a server-allocated signed32 identity")
	}
	identity := strconv.FormatInt(id, 10)
	for _, row := range original.Records.Rows {
		if siviIdentityCellMatches(row.Cells[columns["ID"]], identity) {
			return nil, errors.New("soil creation identity already belongs to an original physical row")
		}
	}
	cells := make([]ProjectMetadataCell, len(original.Records.Columns))
	for i := range cells {
		cells[i] = ProjectMetadataCell{Storage: "null"}
	}
	plot, flag := original.Plot, "0"
	cells[columns["PlotNumber"]] = ProjectMetadataCell{Storage: "text", Text: &plot}
	cells[columns["Horizon"]] = ProjectMetadataCell{Storage: "text", Text: &horizon}
	cells[columns["ID"]] = ProjectMetadataCell{Storage: "integer", Integer: &identity}
	cells[columns["Flag"]] = ProjectMetadataCell{Storage: "integer", Integer: &flag}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return cells, nil
}

func appendTwoPageSoilCreationRow(table ProjectMetadataTable, row ProjectMetadataRow) (ProjectMetadataTable, error) {
	table.Rows = append(append([]ProjectMetadataRow{}, table.Rows...), row)
	if _, err := siteUnitTransferColumns(table); err != nil {
		return ProjectMetadataTable{}, err
	}
	sort.Slice(table.Rows, func(i, j int) bool {
		a, _ := strconv.ParseInt(table.Rows[i].RowID, 10, 64)
		b, _ := strconv.ParseInt(table.Rows[j].RowID, 10, 64)
		return a < b
	})
	return table, nil
}

func twoPageSoilCreationEditHistory(ctx context.Context, history twoPageSoilHistory) (twoPageSoilHistory, error) {
	request := history.Request
	if request.Horizon == nil || history.CreatedRowID == nil || len(request.Edits) != 0 {
		return twoPageSoilHistory{}, errors.New("soil creation history requires a distinct Horizon request and created physical identity")
	}
	columns, err := validateTwoPageSoilCreation(ctx, request.Original, *request.Horizon)
	if err != nil {
		return twoPageSoilHistory{}, err
	}
	var created *ProjectMetadataRow
	for i := range history.Committed.Rows {
		if history.Committed.Rows[i].RowID == *history.CreatedRowID {
			created = &history.Committed.Rows[i]
		}
	}
	if created == nil || len(created.Cells) != len(request.Original.Records.Columns) {
		return twoPageSoilHistory{}, errors.New("soil creation history lacks the complete created row")
	}
	identity := created.Cells[columns["ID"]]
	if identity.Storage != "integer" || identity.Integer == nil {
		return twoPageSoilHistory{}, errors.New("soil creation history requires its exact allocated integer ID")
	}
	id, err := strconv.ParseInt(*identity.Integer, 10, 32)
	if err != nil || strconv.FormatInt(id, 10) != *identity.Integer {
		return twoPageSoilHistory{}, errors.New("soil creation history allocated ID is not canonical signed32")
	}
	cells, err := planTwoPageSoilCreation(ctx, request.Original, id, *request.Horizon)
	if err != nil || !reflect.DeepEqual(created.Cells, cells) {
		return twoPageSoilHistory{}, errors.Join(err, errors.New("soil creation history differs from its complete source defaults"))
	}
	after := cloneSiteUnitCell(cells[columns["Horizon"]])
	cells[columns["Horizon"]] = ProjectMetadataCell{Storage: "null"}
	original := *request.Original
	original.Records, err = appendTwoPageSoilCreationRow(original.Records, ProjectMetadataRow{RowID: created.RowID, Cells: cells})
	if err != nil {
		return twoPageSoilHistory{}, err
	}
	history.Request = twoPageSoilRequest{RequestID: request.RequestID, Original: &original,
		Edits: []twoPageSoilEdit{{ContextID: original.ContextID, Form: original.Form, RowID: created.RowID,
			Column: "Horizon", Expected: ProjectMetadataCell{Storage: "null"}, Value: after}}}
	history.CreatedRowID = nil
	return history, nil
}

type twoPageSoilCreationPreparation struct {
	Baseline map[string]ProjectMetadataTable
	Ledger   ProjectMetadataTable
	Original *twoPageSoilOriginal
	Soil     ProjectMetadataTable
	RowID    string
	Edits    []twoPageSoilEdit
}

func prepareTwoPageSoilCreation(ctx context.Context, tx *sql.Tx, request twoPageSoilRequest, before map[string]ProjectMetadataTable) (*twoPageSoilCreationPreparation, error) {
	original := request.Original
	if request.Horizon == nil {
		return nil, errors.New("soil creation preparation requires explicit Horizon")
	}
	columns, err := validateTwoPageSoilCreation(ctx, original, *request.Horizon)
	if err != nil {
		return nil, err
	}
	kind := "Humus"
	if original.Form == "SoilMineralXL" {
		kind = "Mineral"
	}
	if _, exists := before[siviIdentityLedger]; exists {
		if err := validateSIVIPhysicalSchema(ctx, tx, "main", siviIdentityLedger, "soil creation reservations"); err != nil {
			return nil, err
		}
	}
	reserved, err := sourceChildIdentityReservedIDs(original.Project, kind, "soil creation", before)
	if err != nil {
		return nil, err
	}
	cells := []ProjectMetadataCell{}
	for _, row := range before[original.Table].Rows {
		cell := row.Cells[columns["ID"]]
		if err := validateSIVIIdentityCell("soil creation original ID", cell); err != nil {
			return nil, err
		}
		cells = append(cells, cell)
	}
	for id := range reserved {
		number := id
		cells = append(cells, ProjectMetadataCell{Storage: "integer", Integer: &number})
	}
	baseline, ledger, err := reserveSourceChildIdentityPlan(ctx, tx, original.Project, kind, "soil creation", before, cells)
	if err != nil {
		return nil, err
	}
	id, err := allocateChildID(tx, quoteHeaderIdentifier(original.Table))
	if err != nil {
		return nil, err
	}
	created, err := planTwoPageSoilCreation(ctx, original, id, *request.Horizon)
	if err != nil {
		return nil, err
	}
	reservationSource := make(map[string]ProjectMetadataTable, len(baseline))
	for name, table := range baseline {
		reservationSource[name] = table
	}
	reservationSource[siviIdentityLedger] = ledger
	_, ledger, err = reserveSourceChildIdentityPlan(ctx, tx, original.Project, kind, "soil creation", reservationSource,
		[]ProjectMetadataCell{created[columns["ID"]]})
	if err != nil {
		return nil, err
	}
	horizon := cloneSiteUnitCell(created[columns["Horizon"]])
	created[columns["Horizon"]] = ProjectMetadataCell{Storage: "null"}
	names, marks, values := []string{}, []string{}, []any{}
	for i, column := range original.Records.Columns {
		value, err := metadataCellValue(created[i])
		if err != nil {
			return nil, err
		}
		names = append(names, quoteHeaderIdentifier(column.Name))
		marks = append(marks, "?")
		values = append(values, value)
	}
	inserted, err := tx.ExecContext(ctx, "INSERT INTO "+quoteHeaderIdentifier(original.Table)+
		" ("+strings.Join(names, ",")+") VALUES ("+strings.Join(marks, ",")+")", values...)
	if err != nil {
		return nil, err
	}
	if count, err := inserted.RowsAffected(); err != nil || count != 1 {
		return nil, errors.Join(err, errors.New("soil creation did not insert exactly one physical row"))
	}
	rowID, err := inserted.LastInsertId()
	if err != nil {
		return nil, err
	}
	row := ProjectMetadataRow{RowID: strconv.FormatInt(rowID, 10), Cells: created}
	scoped := *original
	scoped.Records, err = appendTwoPageSoilCreationRow(original.Records, row)
	if err != nil {
		return nil, err
	}
	soil, err := appendTwoPageSoilCreationRow(before[original.Table], row)
	if err != nil {
		return nil, err
	}
	return &twoPageSoilCreationPreparation{Baseline: baseline, Ledger: ledger, Original: &scoped,
		Soil: soil, RowID: row.RowID, Edits: []twoPageSoilEdit{{ContextID: original.ContextID, Form: original.Form,
			RowID: row.RowID, Column: "Horizon", Expected: ProjectMetadataCell{Storage: "null"}, Value: horizon}}}, nil
}
