package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

type VegetationDeletionRequest struct {
	ID       int64  `json:"id"`
	Form     string `json:"form"`
	Expected string `json:"expected"`
}

func (request *VegetationDeletionRequest) UnmarshalJSON(data []byte) error {
	type plain VegetationDeletionRequest
	if err := validateJSONTextProperties(data, map[string]string{"form": "Vegetation deletion form", "expected": "Vegetation deletion snapshot"}); err != nil {
		return err
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(data, &properties); err != nil {
		return err
	}
	for _, property := range []string{"id", "form", "expected"} {
		value, present := properties[property]
		if !present || strings.TrimSpace(string(value)) == "null" {
			return fmt.Errorf("vegetation deletion requires explicit non-NULL %s", property)
		}
	}
	return json.Unmarshal(data, (*plain)(request))
}

type VegetationDeletionReview struct {
	ID       int64    `json:"id"`
	Form     string   `json:"form"`
	Species  *string  `json:"species"`
	Expected string   `json:"expected"`
	Columns  []string `json:"columns"`
}

type vegetationDeletionCell struct {
	Name string
	Type string
	Tag  string
	Data []byte
}

type vegetationDeletionSnapshot struct {
	review VegetationDeletionReview
	extra  []childField
	before []any
}

func inspectVegetationDeletion(ctx context.Context, tx *sql.Tx, table, plot, form string, id int64) (vegetationDeletionSnapshot, error) {
	var snapshot vegetationDeletionSnapshot
	predicate, err := vegetationSpeciesRowPredicate(form)
	if err != nil {
		return snapshot, err
	}
	if id < math.MinInt32 || id > math.MaxInt32 {
		return snapshot, errors.New("vegetation deletion requires an exact signed32 identity")
	}
	if err := requireChildIdentity(tx, table, "Veg", plot, id); err != nil {
		return snapshot, err
	}
	var visible bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM `+table+
		` WHERE PlotNumber=? AND ID=? AND `+predicate+`)`, plot, id).Scan(&visible); err != nil {
		return snapshot, err
	}
	if !visible {
		return snapshot, fmt.Errorf("vegetation row %d no longer belongs to %s; cancel and reload", id, form)
	}
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return snapshot, err
	}
	var cells []vegetationDeletionCell
	for rows.Next() {
		var cid, required, primary int
		var name, typ string
		var defaultValue sql.NullString
		if err := rows.Scan(&cid, &name, &typ, &required, &defaultValue, &primary); err != nil {
			rows.Close()
			return snapshot, err
		}
		cells = append(cells, vegetationDeletionCell{Name: name, Type: typ})
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		return snapshot, err
	}
	columns := []string{`rowid`}
	for _, cell := range cells {
		column := quoteHeaderIdentifier(cell.Name)
		columns = append(columns, `CASE WHEN typeof(`+column+`)='integer' THEN CAST(`+column+` AS INTEGER) ELSE `+column+` END`)
	}
	values := make([]any, len(columns))
	destinations := make([]any, len(columns))
	for i := range values {
		destinations[i] = &values[i]
	}
	if err := tx.QueryRowContext(ctx, `SELECT `+strings.Join(columns, ",")+` FROM `+table+` WHERE PlotNumber=? AND ID=?`, plot, id).Scan(destinations...); err != nil {
		return snapshot, err
	}
	cells = append([]vegetationDeletionCell{{Name: "rowid"}}, cells...)
	mapped := map[string]bool{"id": true, "plotnumber": true, "rowid": true}
	for _, field := range childFields["Veg"] {
		mapped[strings.ToLower(field.column)] = true
	}
	snapshot.review = VegetationDeletionReview{ID: id, Form: form, Columns: []string{}}
	for i := range cells {
		cell, value := &cells[i], values[i]
		if !utf8.ValidString(cell.Name) || !utf8.ValidString(cell.Type) {
			return snapshot, errors.New("vegetation deletion column metadata contains malformed Unicode")
		}
		switch scalar := value.(type) {
		case nil:
			cell.Tag = "null"
		case int64:
			cell.Tag, cell.Data = "integer", []byte(strconv.FormatInt(scalar, 10))
		case float64:
			cell.Tag, cell.Data = "real", []byte(strconv.FormatUint(math.Float64bits(scalar), 16))
		case string:
			cell.Tag, cell.Data = "text", []byte(scalar)
		case []byte:
			return snapshot, fmt.Errorf("vegetation deletion cannot safely audit BLOB column %q", cell.Name)
		default:
			return snapshot, fmt.Errorf("vegetation deletion cannot safely audit column %q type %T", cell.Name, value)
		}
		if strings.EqualFold(cell.Name, "Species") && value != nil {
			species, text := value.(string)
			if !text || !utf8.ValidString(species) {
				return snapshot, errors.New("vegetation deletion species cannot be displayed without repairing its stored value")
			}
			snapshot.review.Species = &species
		}
		if !mapped[strings.ToLower(cell.Name)] {
			if strings.EqualFold(cell.Name, "Flag") || strings.EqualFold(cell.Type, "BOOLEAN") || strings.EqualFold(cell.Type, "BOOL") || strings.EqualFold(cell.Type, "BIT") {
				if number, integer := value.(int64); integer {
					value = number != 0
				} else if value != nil {
					return snapshot, fmt.Errorf("vegetation deletion boolean column %q is not an integer", cell.Name)
				}
			}
			snapshot.extra = append(snapshot.extra, childField{column: cell.Name})
			snapshot.before = append(snapshot.before, value)
		}
		if !strings.EqualFold(cell.Name, "rowid") {
			snapshot.review.Columns = append(snapshot.review.Columns, cell.Name)
		}
	}
	payload, err := json.Marshal(struct {
		Table string
		Plot  string
		Form  string
		Cells []vegetationDeletionCell
	}{table, plot, form, cells})
	if err != nil {
		return snapshot, err
	}
	digest := sha256.Sum256(payload)
	snapshot.review.Expected = hex.EncodeToString(digest[:])
	return snapshot, nil
}

func (s *ContextService) ReviewVegetationDeletion(ctx context.Context, contextID, plot, form string, id int64) (VegetationDeletionReview, error) {
	return withContextPlotRequest(ctx, s, contextID, func(plots *PlotService) (VegetationDeletionReview, error) {
		db, project, release, err := plots.getActiveDB()
		if err != nil {
			return VegetationDeletionReview{}, err
		}
		defer release()
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil {
			return VegetationDeletionReview{}, err
		}
		defer tx.Rollback()
		if err := childParent(tx, project, plot); err != nil {
			return VegetationDeletionReview{}, err
		}
		snapshot, err := inspectVegetationDeletion(ctx, tx, quoteHeaderIdentifier(project+"_Veg"), plot, form, id)
		return snapshot.review, err
	})
}

func (s *PlotService) DeleteReviewedVegetation(plot string, request VegetationDeletionRequest) error {
	digest, err := hex.DecodeString(request.Expected)
	if err != nil || len(digest) != sha256.Size || hex.EncodeToString(digest) != request.Expected {
		return errors.New("vegetation deletion requires the exact reviewed full-row snapshot")
	}
	return s.saveChildWithDeleteCheck("Veg", VegRecord{PlotNumber: plot, ID: request.ID}, childDelete,
		func(tx *sql.Tx, table, plot string, id int64) ([]childField, []any, error) {
			snapshot, err := inspectVegetationDeletion(s.operationContext(), tx, table, plot, request.Form, id)
			if err != nil {
				return nil, nil, err
			}
			if snapshot.review.Expected != request.Expected {
				return nil, nil, errors.New("vegetation deletion row changed since review; cancel and review the current row again")
			}
			return snapshot.extra, snapshot.before, nil
		})
}

func (s *ContextService) DeleteReviewedVegetation(contextID, plot string, request VegetationDeletionRequest) error {
	return s.edit(contextID, func(plots *PlotService) error { return plots.DeleteReviewedVegetation(plot, request) })
}
