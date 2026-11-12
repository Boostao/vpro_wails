package twopage

import (
	"github.com/boostao/vpro-wails/internal/sourceform"
	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
)

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
