package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestRollbackJournalReadLockModesAndOperationErrors(t *testing.T) {
	for _, mode := range []string{"delete", "truncate", "persist", "wal", "memory", "off"} {
		t.Run(mode, func(t *testing.T) {
			db, err := sql.Open("sqlite3", sqliteFileURI(filepath.Join(t.TempDir(), "journal.db"), "rwc"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			db.SetMaxOpenConns(1)
			var actual string
			if err := db.QueryRow("PRAGMA journal_mode=" + mode).Scan(&actual); err != nil || actual != mode {
				t.Fatal("explicit fixture journal mode differs", actual, err)
			}
			tx, err := db.Begin()
			if err != nil {
				t.Fatal(err)
			}
			err = requireRollbackJournalReadLock(context.Background(), tx, "main", "soil scope observations")
			wantAllowed := mode == "delete" || mode == "truncate" || mode == "persist"
			if wantAllowed && err != nil || !wantAllowed && (err == nil || !strings.Contains(err.Error(), "soil scope observations require rollback-journal")) {
				t.Fatal("source reservation capability or operation-specific error differs", mode, err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
