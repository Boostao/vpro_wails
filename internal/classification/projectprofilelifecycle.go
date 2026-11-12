package classification

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"

	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type ProjectPlotProfileCreation struct {
	OriginalRules vprocore.ProjectMetadataTable    `json:"originalRules"`
	Values        []vprocore.ProjectMetadataChange `json:"values"`
}

type ProjectPlotProfileDeletion struct {
	OriginalRules vprocore.ProjectMetadataTable `json:"originalRules"`
	RowID         string                        `json:"rowId"`
	Confirmed     bool                          `json:"confirmed"`
}

func (request *ProjectPlotProfileCreation) UnmarshalJSON(data []byte) error {
	type plain ProjectPlotProfileCreation
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "originalRules", "values"); err != nil {
		return err
	}
	*request = ProjectPlotProfileCreation(decoded)
	return nil
}

func (request *ProjectPlotProfileDeletion) UnmarshalJSON(data []byte) error {
	type plain ProjectPlotProfileDeletion
	var decoded plain
	if err := vprocore.DecodeProfileLifecycleJSON(data, &decoded, "originalRules", "rowId", "confirmed"); err != nil {
		return err
	}
	*request = ProjectPlotProfileDeletion(decoded)
	return nil
}

func prepareProfileRuleCreation(original vprocore.ProjectMetadataTable, values []vprocore.ProjectMetadataChange, rowID string) (vprocore.ProjectMetadataTable, vprocore.ProjectMetadataRow, error) {
	if len(original.Columns) != len(profileRuleColumns) || len(values) != 8 {
		return vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataRow{}, errors.New("profile creation requires the original nine-column schema and eight explicit nullable assignments; extra/default fields are unavailable")
	}
	seen := map[string]bool{}
	for _, change := range values {
		if !slices.Contains(profileRuleColumns[:8], change.Column) || seen[change.Column] {
			return vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataRow{}, errors.New("profile creation repeats or invents an input assignment")
		}
		seen[change.Column] = true
	}
	row := vprocore.ProjectMetadataRow{RowID: rowID, Cells: make([]vprocore.ProjectMetadataCell, len(original.Columns))}
	for index := range row.Cells {
		row.Cells[index] = vprocore.ProjectMetadataCell{Storage: "null"}
	}
	augmented := vprocore.ProjectMetadataTable{Columns: original.Columns,
		Rows: append(append([]vprocore.ProjectMetadataRow{}, original.Rows...), row)}
	planned, _, err := prepareProfileRuleDrafts(augmented, []profileRuleDraft{{RowID: rowID, Changes: values}})
	if err != nil {
		return vprocore.ProjectMetadataTable{}, vprocore.ProjectMetadataRow{}, err
	}
	return planned, planned.Rows[len(planned.Rows)-1], nil
}

func CreateProfileRule(c *vprocore.SqliteContext, ctx context.Context, request ProjectPlotProfileCreation, user string) (vprocore.ProjectMetadataRow, error) {
	var created vprocore.ProjectMetadataRow
	var reservations vprocore.ProjectMetadataTable
	err := MutateProfileRules(c, ctx, request.OriginalRules, user, func(tx *sql.Tx, table string, fresh vprocore.ProjectMetadataTable) (*vprocore.ProjectMetadataTable, error) {
		if err := requireProfileCreationSchema(ctx, tx, table); err != nil {
			return nil, err
		}
		ids, err := reserveProfileIdentities(ctx, tx, table, fresh)
		if err != nil {
			return nil, err
		}
		var maximum int64
		for id := range ids {
			if id > maximum {
				maximum = id
			}
		}
		if maximum == math.MaxInt64 {
			return nil, errors.New("profile physical identity space is exhausted; no historical ID is reused")
		}
		id := maximum + 1
		planned, row, err := prepareProfileRuleCreation(fresh, request.Values, strconv.FormatInt(id, 10))
		if err != nil {
			return nil, err
		}
		reservations, err = vprocore.ReadSQLiteStorageRows(ctx, tx, "main", "__VPRO_ProfileIdentity", "", nil, "")
		if err != nil {
			return nil, err
		}
		columns, placeholders, args := []string{"rowid"}, []string{"?"}, []any{id}
		for index, column := range fresh.Columns {
			value, err := vprocore.MetadataCellValue(row.Cells[index])
			if err != nil {
				return nil, err
			}
			columns = append(columns, vprocore.QuoteHeaderIdentifier(column.Name))
			placeholders = append(placeholders, "?")
			args = append(args, value)
		}
		result, err := tx.ExecContext(ctx, "INSERT INTO "+vprocore.QuoteHeaderIdentifier(table)+"("+strings.Join(columns, ",")+
			") VALUES("+strings.Join(placeholders, ",")+")", args...)
		if err != nil {
			return nil, err
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return nil, errors.Join(fmt.Errorf("profile creation expected one physical row, found %d", count), err)
		}
		if err := addProfileReservation(ctx, tx, table, id, &reservations); err != nil {
			return nil, err
		}
		created = row
		return &planned, nil
	}, func(tx *sql.Tx) error { return verifyProfileReservations(ctx, tx, reservations) })
	if err != nil {
		return vprocore.ProjectMetadataRow{}, err
	}
	return created, nil
}

func DeleteProfileRule(c *vprocore.SqliteContext, ctx context.Context, request ProjectPlotProfileDeletion, user string) error {
	if !request.Confirmed {
		return errors.New("profile deletion requires explicit confirmation of the reviewed physical rule")
	}
	id, err := strconv.ParseInt(request.RowID, 10, 64)
	if err != nil || strconv.FormatInt(id, 10) != request.RowID {
		return errors.New("profile deletion requires one literal signed64 physical identity")
	}
	var reservations vprocore.ProjectMetadataTable
	return MutateProfileRules(c, ctx, request.OriginalRules, user, func(tx *sql.Tx, table string, fresh vprocore.ProjectMetadataTable) (*vprocore.ProjectMetadataTable, error) {
		planned := vprocore.ProjectMetadataTable{Columns: fresh.Columns, Rows: []vprocore.ProjectMetadataRow{}}
		found := false
		for _, row := range fresh.Rows {
			if row.RowID == request.RowID {
				found = true
			} else {
				planned.Rows = append(planned.Rows, row)
			}
		}
		if !found {
			return nil, errors.New("profile deletion does not identify a reviewed physical rule")
		}
		if _, err := reserveProfileIdentities(ctx, tx, table, fresh); err != nil {
			return nil, err
		}
		reservations, err = vprocore.ReadSQLiteStorageRows(ctx, tx, "main", "__VPRO_ProfileIdentity", "", nil, "")
		if err != nil {
			return nil, err
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM "+vprocore.QuoteHeaderIdentifier(table)+" WHERE rowid=?", id)
		if err != nil {
			return nil, err
		}
		if count, err := result.RowsAffected(); err != nil || count != 1 {
			return nil, errors.Join(fmt.Errorf("profile deletion expected one physical row, found %d", count), err)
		}
		return &planned, nil
	}, func(tx *sql.Tx) error { return verifyProfileReservations(ctx, tx, reservations) })
}

func verifyProfileReservations(ctx context.Context, tx *sql.Tx, expected vprocore.ProjectMetadataTable) error {
	actual, err := vprocore.ReadSQLiteStorageRows(ctx, tx, "main", "__VPRO_ProfileIdentity", "", nil, "")
	if err != nil {
		return err
	}
	if !sameProfileStorageRows(expected, actual) {
		return errors.New("profile physical identity reservations changed outside the plan; mutation/history rolled back")
	}
	return nil
}

func requireProfileCreationSchema(ctx context.Context, tx *sql.Tx, table string) (resultErr error) {
	rows, err := tx.QueryContext(ctx, "PRAGMA table_xinfo("+vprocore.QuoteHeaderIdentifier(table)+")")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	names := []string{}
	for rows.Next() {
		var cid, required, primary, hidden int
		var name, declared string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &declared, &required, &defaultValue, &primary, &hidden); err != nil {
			return err
		}
		if hidden != 0 || !slices.Contains(profileRuleColumns[:], name) {
			return errors.New("profile creation has unmapped/generated fields; no defaults or hidden assignments are inferred")
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(names) != 9 {
		return errors.New("profile creation requires the complete nine-column physical schema")
	}
	return nil
}
