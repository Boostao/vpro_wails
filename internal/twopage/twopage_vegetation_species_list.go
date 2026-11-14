package twopage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

type twoPageVegetationSpeciesListMatch struct {
	Role       string
	Definition vprocore.ProjectMetadataRow
}

// Membership alone does not authorize mutation; the caller must independently own the row and snapshot.
func qualifyTwoPageVegetationSpeciesListSelection(ctx context.Context, references *twoPageVegetationSpeciesReferences,
	code string) ([]twoPageVegetationSpeciesListMatch, error) {
	if ctx == nil || references == nil {
		return nil, errors.New("two-page Species list selection requires owned physical references and a context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, value := range []string{references.ContextID, references.Project, references.Plot} {
		if value == "" || !utf8.ValidString(value) || strings.ContainsRune(value, '\x00') {
			return nil, errors.New("two-page Species list references require a valid literal owner")
		}
	}
	if references.Form != "SubVegAXL_BC" && references.Form != "SubVegAXL" {
		return nil, errors.New("two-page Species list selection is qualified only for existing A/BC and CHARS A rows")
	}
	if err := vprocore.ValidateVegetationSpeciesLookup(code); err != nil {
		return nil, err
	}
	matches := []twoPageVegetationSpeciesListMatch{}
	for _, source := range []struct {
		role     string
		table    vprocore.ProjectMetadataTable
		personal bool
	}{
		{"master", references.Master, false},
		{"personal", references.Personal, true},
	} {
		table, err := projectTwoPageVegetationSpeciesDefinitions(ctx, source.table, source.personal)
		if err != nil {
			return nil, fmt.Errorf("two-page Species list %s definitions: %w", source.role, err)
		}
		for _, row := range table.Rows {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			literal, lifeform, codetype := row.Cells[0], row.Cells[2], row.Cells[4]
			if literal.Text == nil || *literal.Text != code || lifeform.Integer == nil || codetype.Text == nil {
				continue
			}
			if *codetype.Text != "u" && *codetype.Text != "U" && *codetype.Text != "x" && *codetype.Text != "X" {
				continue
			}
			number, err := strconv.ParseInt(*lifeform.Integer, 10, 32)
			if err != nil {
				return nil, fmt.Errorf("two-page Species list lifeform: %w", err)
			}
			if number >= 1 && number <= 4 {
				matches = append(matches, twoPageVegetationSpeciesListMatch{source.role, row})
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, errors.New("two-page Species selection is not an exact qualified existing A/CHARS list code; manual and NotInList routes remain unavailable")
	}
	return matches, nil
}

func planTwoPageVegetationSpeciesListEdits(ctx context.Context, original *twoPageVegetationOriginal,
	edits []twoPageVegetationEdit, references *twoPageVegetationSpeciesReferences) ([]twoPageVegetationAssignment, error) {
	if references == nil {
		return nil, errors.New("two-page Species list planning requires current physical reference definitions")
	}
	return planTwoPageVegetationQualifiedEdits(ctx, original, edits, true, references)
}

func requireTwoPageVegetationSpeciesListCovers(original *twoPageVegetationOriginal, assignments []twoPageVegetationAssignment) error {
	covers := []string{"Cover1", "Cover2", "Cover3", "TotalA", "Cover4", "Cover5", "TotalB"}
	before, err := twoPageVegetationSourceRows(original.Records, original.RecordSource, covers)
	if err != nil {
		return fmt.Errorf("two-page Species list common-cover schema: %w", err)
	}
	after, err := twoPageVegetationSourceRows(twoPageVegetationPlannedTable(original.Records, assignments), original.RecordSource, covers)
	if err != nil {
		return fmt.Errorf("two-page Species list planned common-cover schema: %w", err)
	}
	for _, assignment := range assignments {
		if assignment.Column != "Species" {
			continue
		}
		if !slices.Contains(before, assignment.RowID) || !slices.Contains(after, assignment.RowID) {
			return errors.New("two-page Species list assignment with all seven common covers NULL requires the unavailable source warning/advance route")
		}
	}
	return nil
}

func (references *twoPageVegetationSpeciesReferences) UnmarshalJSON(data []byte) error {
	required := []string{"contextId", "project", "plot", "form", "source", "master", "personal"}
	properties, err := sourceform.SourceChildJSONObject(data, "two-page Species list references", required, nil)
	if err != nil {
		return err
	}
	if err := sourceform.SourceChildJSONNonNull(properties, "two-page Species list references", required...); err != nil {
		return err
	}
	for _, name := range []string{"master", "personal"} {
		if err := sourceform.SiviSpeciesTableJSON(properties[name]); err != nil {
			return err
		}
	}
	type plain twoPageVegetationSpeciesReferences
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Source != twoPageVegetationSpeciesReferenceSource {
		return errors.New("two-page Species references require their exact physical source provenance")
	}
	for _, source := range []struct {
		table    vprocore.ProjectMetadataTable
		personal bool
	}{{decoded.Master, false}, {decoded.Personal, true}} {
		projected, err := projectTwoPageVegetationSpeciesDefinitions(context.Background(), source.table, source.personal)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(projected, source.table) {
			return errors.New("two-page Species references require exactly their original projected physical columns")
		}
	}
	*references = twoPageVegetationSpeciesReferences(decoded)
	return nil
}

func planTwoPageVegetationRequest(ctx context.Context, request twoPageVegetationRequest, canonicalSingle bool) ([]twoPageVegetationAssignment, error) {
	plan, err := planTwoPageVegetationQualifiedEdits(ctx, request.Original, request.Edits, canonicalSingle, request.SpeciesList)
	if err != nil {
		return nil, err
	}
	if request.SpeciesList != nil {
		changedSpecies := false
		for _, assignment := range plan {
			changedSpecies = changedSpecies || assignment.Column == "Species"
		}
		if !changedSpecies {
			return nil, errors.New("two-page Species list request requires a changed qualified Species assignment")
		}
	}
	return plan, nil
}

type TwoPageVegetationSpeciesListService struct {
	vegetation *TwoPageVegetationService
}

func NewTwoPageVegetationSpeciesListService(contexts *vprocore.ContextService) (*TwoPageVegetationSpeciesListService, error) {
	vegetation, err := NewTwoPageVegetationService(contexts)
	if err != nil {
		return nil, err
	}
	return &TwoPageVegetationSpeciesListService{vegetation: vegetation}, nil
}

func (s *TwoPageVegetationSpeciesListService) Save(ctx context.Context, contextID, requestJSON string) (*TwoPageVegetationWriteResult, error) {
	return s.saveObserved(ctx, contextID, requestJSON, nil, nil)
}

func (s *TwoPageVegetationSpeciesListService) saveObserved(ctx context.Context, contextID, requestJSON string,
	beforeCommit func(*sql.Tx) error, afterCommit func(*twoPageVegetationWriteResult) error) (*twoPageVegetationWriteResult, error) {
	if s == nil || s.vegetation == nil {
		return nil, errors.New("two-page Species list selection requires owned vegetation services")
	}
	if err := s.vegetation.require(ctx, true); err != nil {
		return nil, err
	}
	return s.vegetation.saveTwoPageVegetationQualifiedObserved(ctx, contextID, requestJSON, true, beforeCommit, afterCommit)
}

func prepareTwoPageVegetationSpeciesListWriter(ctx context.Context, conn *sql.Conn, owner *vprocore.SqliteContext, request twoPageVegetationRequest) error {
	if request.SpeciesList == nil {
		return nil
	}
	if owner.AttachmentInfo["VUser"] == nil || owner.Attachments["VUser"] == "" {
		return errors.New("two-page Species list selection requires the owned personal reference file")
	}
	if err := owner.ValidateMetadataWriterFiles(); err != nil {
		return err
	}
	_, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS "two_page_species_user"`, sqlutil.SqliteFileURI(owner.Attachments["VUser"], "ro"))
	if err != nil {
		return fmt.Errorf("two-page Species list personal reference attachment: %w", err)
	}
	return nil
}

func observeTwoPageVegetationSpeciesListWriter(ctx context.Context, tx *sql.Tx, request twoPageVegetationRequest) error {
	if request.SpeciesList == nil {
		return nil
	}
	for _, source := range []struct{ alias, table string }{{"reference", "USysAllSpecs"}, {"two_page_species_user", "USysUserSpp"}} {
		if err := vprocore.RequireRollbackJournalReadLock(ctx, tx, source.alias, "two-page Species list reference definitions"); err != nil {
			return err
		}
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.alias)+
			`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return errors.New("two-page Species list mutation requires original physical reference tables")
		}
		if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, source.alias, source.table, "two-page Species list reference definitions"); err != nil {
			return err
		}
	}
	original := request.Original
	physical, err := sourceform.ReadPhysicalSpeciesReferences(ctx, tx, original.ContextID, original.Project, original.Plot,
		"reference", "two_page_species_user", projectTwoPageVegetationSpeciesDefinitions)
	if err != nil {
		return fmt.Errorf("two-page Species list current definitions: %w", err)
	}
	observed := twoPageVegetationSpeciesReferences{
		ContextID: original.ContextID, Project: original.Project, Plot: original.Plot, Form: original.Form,
		Source: twoPageVegetationSpeciesReferenceSource, Master: physical.Master, Personal: physical.Personal,
	}
	if !reflect.DeepEqual(*request.SpeciesList, observed) {
		return errors.New("two-page Species list complete physical definitions changed; cancel and reload")
	}
	return nil
}

const twoPageVegetationSpeciesReferenceSource = "configured-owned-imported-SQLite VLists.USysAllSpecs/VUser.USysUserSpp; physical definitions, not source union membership"

type twoPageVegetationSpeciesReferences struct {
	ContextID string                        `json:"contextId"`
	Project   string                        `json:"project"`
	Plot      string                        `json:"plot"`
	Form      string                        `json:"form"`
	Source    string                        `json:"source"`
	Master    vprocore.ProjectMetadataTable `json:"master"`
	Personal  vprocore.ProjectMetadataTable `json:"personal"`
}

func ReadTwoPageVegetationSpeciesReferences(s *vprocore.ContextService, ctx context.Context, contextID, plot, form string) (*twoPageVegetationSpeciesReferences, error) {
	if ctx == nil || s == nil {
		return nil, errors.New("two-page vegetation Species references require an owned context")
	}
	if plot == "" || !utf8.ValidString(plot) {
		return nil, errors.New("two-page vegetation Species references require an exact valid literal parent")
	}
	if _, _, err := twoPageVegetationSource(form); err != nil {
		return nil, err
	}
	return vprocore.WithContextPlotRequest(ctx, s, contextID, func(plots *vprocore.PlotService) (*twoPageVegetationSpeciesReferences, error) {
		return vprocore.WithOwnedSIVISnapshot(ctx, plots, func(owner *vprocore.SqliteContext, tx *sql.Tx) (*twoPageVegetationSpeciesReferences, error) {
			project := owner.Selection.Project
			if err := sourceform.RequireTwoPageChildReadScope(ctx, tx, project, project+"_Veg", plot, "vegetation"); err != nil {
				return nil, fmt.Errorf("two-page vegetation Species reference scope: %w", err)
			}
			for _, source := range []struct{ alias, table string }{{"VLists", "USysAllSpecs"}, {"VUser", "USysUserSpp"}} {
				var count int
				if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+vprocore.QuoteHeaderIdentifier(source.alias)+
					`.sqlite_master WHERE type='table' AND name COLLATE BINARY=?`, source.table).Scan(&count); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
				if count != 1 {
					return nil, fmt.Errorf("two-page vegetation Species requires original physical table %s.%s", source.alias, source.table)
				}
				if err := vprocore.ValidateSIVIPhysicalSchema(ctx, tx, source.alias, source.table, "vegetation Species definitions"); err != nil {
					return nil, fmt.Errorf("two-page vegetation Species physical schema: %w", err)
				}
			}
			references, err := sourceform.ReadPhysicalSpeciesReferences(ctx, tx, contextID, project, plot, "VLists", "VUser",
				projectTwoPageVegetationSpeciesDefinitions)
			if err != nil {
				return nil, fmt.Errorf("two-page vegetation Species physical definitions: %w", err)
			}
			return &twoPageVegetationSpeciesReferences{
				ContextID: contextID, Project: project, Plot: plot, Form: form,
				Source: twoPageVegetationSpeciesReferenceSource,
				Master: references.Master, Personal: references.Personal,
			}, nil
		})
	})
}

func projectTwoPageVegetationSpeciesDefinitions(ctx context.Context, source vprocore.ProjectMetadataTable, personal bool) (vprocore.ProjectMetadataTable, error) {
	if personal {
		return sourceform.ProjectSIVISpeciesDefinitions(ctx, source, true)
	}
	return sourceform.ProjectPhysicalSpeciesDefinitions(ctx, source, false, func(cell vprocore.ProjectMetadataCell) error {
		if _, err := vprocore.MetadataCellValue(cell); err != nil {
			return err
		}
		if cell.Storage == "null" {
			return nil
		}
		if cell.Storage != "integer" {
			return errors.New("master Species Lifeform requires exact nullable source LONG integer storage; no coercion")
		}
		if _, err := strconv.ParseInt(*cell.Integer, 10, 32); err != nil {
			return errors.New("master Species Lifeform exceeds source signed32 LONG range")
		}
		return nil
	})
}
