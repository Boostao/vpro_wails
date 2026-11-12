package vprocore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func RequireRollbackJournalReadLock(ctx context.Context, tx *sql.Tx, alias, scope string) error {
	var journal string
	if err := tx.QueryRowContext(ctx, `PRAGMA `+QuoteHeaderIdentifier(alias)+`.journal_mode`).Scan(&journal); err != nil {
		return err
	}
	switch strings.ToLower(journal) {
	case "delete", "truncate", "persist":
		return nil
	default:
		return fmt.Errorf("%s require rollback-journal read locks through commit", scope)
	}
}
