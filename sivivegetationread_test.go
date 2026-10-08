package main

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestSIVIOwnedBlockedCancellationRetainsCoordinatorAndRetry(t *testing.T) {
	service, state := reportServiceFixture(t, false)
	before, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil {
		t.Fatal(err)
	}
	owner := service.projects.sqlite
	files := databaseBytes(t, owner.attachments)
	lock, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	conn, err := lock.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), "BEGIN EXCLUSIVE"); err != nil {
		t.Fatal(err)
	}
	locked := true
	defer func() {
		if locked {
			if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
				t.Error(err)
			}
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	got, err := service.readSIVIVegetation(ctx, state.ContextID, "108050", false)
	if !errors.Is(err, context.DeadlineExceeded) || got != nil {
		t.Fatal("blocked cancelled read returned partial/success-shaped result", got, err)
	}
	if _, err := conn.ExecContext(context.Background(), "ROLLBACK"); err != nil {
		t.Fatal(err)
	}
	locked = false
	again, err := service.readSIVIVegetation(context.Background(), state.ContextID, "108050", false)
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("cancelled snapshot discarded pinned coordinator/attachments", again, err)
	}
	assertProfileSUFiles(t, service, files)
}

func TestSIVIOwnedReadPreservesTypedHeightsAndSelectionWithoutWrites(t *testing.T) {
	for _, external := range []bool{false, true} {
		t.Run(map[bool]string{false: "project", true: "external"}[external], func(t *testing.T) {
			service, state := reportServiceFixture(t, external)
			owner := service.projects.sqlite
			db, err := sql.Open("sqlite3", sqliteFileURI(owner.attachments["project"], "rw"))
			if err != nil {
				t.Fatal(err)
			}

			defer db.Close()
			if _, err := db.Exec(`INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover5a,HeightA,HeightB)
				VALUES ('SIVI','RAW',0,0,-2,''),('SIVI','RAW',0,1,NULL,'  words  '),
				('SIVI','RAW',1,NULL,10,'height only');
				INSERT INTO Sample_Veg(PlotNumber,Species,ID,Cover6,Height6,HeightB)
				VALUES ('SIVI','RAW',-1,0,4,NULL)`); err != nil {
				t.Fatal(err)
			}
			before := databaseBytes(t, owner.attachments)
			got, err := service.readSIVIVegetation(context.Background(), state.ContextID, "SIVI", false)
			if err != nil || len(got) != 3 || len(got[0].Rows) != 2 || len(got[1].Rows) != 1 || len(got[2].Rows) != 0 {
				t.Fatal("owned source group/membership changed", got, err)
			}
			if got[0].Rows[0].RowID == got[0].Rows[1].RowID ||
				*got[0].Rows[0].Cells[0].Integer != "0" || *got[0].Rows[1].Cells[0].Integer != "0" ||
				*got[0].Rows[0].Cells[7].Real != -2 ||
				*got[0].Rows[0].Cells[14].Text != "" || *got[0].Rows[1].Cells[14].Text != "  words  " {
				t.Fatal("physical duplicate identity/raw aggregate height changed", got[0])
			}
			again, err := service.readSIVIVegetation(context.Background(), state.ContextID, "SIVI", false)
			if err != nil || !reflect.DeepEqual(got, again) {
				t.Fatal("repeated snapshot changed", again, err)
			}
			*got[0].Rows[0].Cells[14].Text = "changed"
			extended, err := service.readSIVIVegetation(context.Background(), state.ContextID, "SIVI", true)
			if err != nil || extended[0].Form != "SubVegA-SIVI" ||
				!reflect.DeepEqual(again[0].Rows, extended[0].Rows) {
				t.Fatal("presentation changed physical data", extended, err)
			}
			assertProfileSUFiles(t, service, before)
			if got, err := service.readSIVIVegetation(context.Background(), "stale", "SIVI", false); err == nil || got != nil {
				t.Fatal("stale context returned a successful projection", got, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if got, err := service.readSIVIVegetation(ctx, state.ContextID, "SIVI", false); !errors.Is(err, context.Canceled) || got != nil {
				t.Fatal("cancelled owned read returned partial output", got, err)
			}
			retry, err := service.readSIVIVegetation(context.Background(), state.ContextID, "SIVI", false)
			if err != nil || !reflect.DeepEqual(retry, again) {
				t.Fatal("rejected read discarded pinned connection", retry, err)
			}
			assertProfileSUFiles(t, service, before)
		})
	}
}
