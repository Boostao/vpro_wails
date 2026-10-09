package main

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSpecialCatalogueReadsCancelLocksAndSQLPoolWaits(t *testing.T) {
	bec := becFixture(t)
	quality, err := NewQualityService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { quality.Close() })
	unit, projects, _ := workingUnitServiceFixture(t)
	reads := []struct {
		name  string
		mutex *sync.RWMutex
		db    *sql.DB
		read  func(context.Context) error
	}{
		{"BEC zones", &bec.mu, bec.db, func(ctx context.Context) error {
			_, err := bec.ListBECZones(ctx)
			return err
		}},
		{"BEC subzones", &bec.mu, bec.db, func(ctx context.Context) error {
			_, err := bec.ListBECSubZones(ctx, becString("BG"))
			return err
		}},
		{"BEC series", &bec.mu, bec.db, func(ctx context.Context) error {
			_, err := bec.ListBECSiteSeries(ctx, becString("BG"), becString("xh1"))
			return err
		}},
		{"Quality", &quality.mu, quality.db, func(ctx context.Context) error {
			_, err := quality.ListPlotQualityChoices(ctx)
			return err
		}},
		{"Master metadata", &unit.mu, unit.db, func(ctx context.Context) error {
			_, err := unit.GetMasterWorkingUnitChoices(ctx)
			return err
		}},
		{"Master choices", &unit.mu, unit.db, func(ctx context.Context) error {
			_, err := unit.GetWorkingUnitChoices(ctx, "master")
			return err
		}},
		{"Environment choices", &projects.mu, nil, func(ctx context.Context) error {
			_, err := unit.GetWorkingUnitChoices(ctx, "env")
			return err
		}},
	}
	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			if err := read.read(cancelled); !errors.Is(err, context.Canceled) {
				t.Fatal("pre-cancelled read executed", err)
			}
			read.mutex.Lock()
			queued, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
			err := read.read(queued)
			stop()
			read.mutex.Unlock()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal("queued catalogue lock ignored deadline", err)
			}
			if read.db != nil {
				read.db.SetMaxOpenConns(1)
				conn, err := read.db.Conn(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				waiting, stop := context.WithTimeout(context.Background(), 50*time.Millisecond)
				err = read.read(waiting)
				stop()
				if closeErr := conn.Close(); closeErr != nil {
					t.Fatal(closeErr)
				}
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatal("catalogue SQL pool ignored deadline", err)
				}
			}
			if err := read.read(context.Background()); err != nil {
				t.Fatal("read did not recover after cancellation", err)
			}
		})
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bec.GetBECCatalogueStatus(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatal("catalogue status ignored cancellation", err)
	}
}

func TestWorkingUnitEnvironmentReadInterruptsSQLAndRecovers(t *testing.T) {
	service, _, db := workingUnitServiceFixture(t)
	original, err := service.GetWorkingUnitChoices(context.Background(), "env")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`ALTER TABLE Sample_Admin RENAME TO Sample_Admin_CancelProof;
 CREATE VIEW Sample_Admin AS SELECT * FROM Sample_Admin_CancelProof
 WHERE (WITH RECURSIVE work(x) AS
 (SELECT 1 UNION ALL SELECT x+1 FROM work WHERE x<1000000000)
 SELECT sum(x) FROM work)>0`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	rows, err := service.GetWorkingUnitChoices(ctx, "env")
	if !errors.Is(err, context.DeadlineExceeded) || rows != nil {
		t.Fatal("environment SQL did not interrupt without partial rows", rows, err)
	}
	if time.Since(started) > 3*time.Second {
		t.Fatal("cancelled environment SQL retained its read lease")
	}
	if _, err := db.Exec(`DROP VIEW Sample_Admin;
 ALTER TABLE Sample_Admin_CancelProof RENAME TO Sample_Admin`); err != nil {
		t.Fatal(err)
	}
	recovered, err := service.GetWorkingUnitChoices(context.Background(), "env")
	if err != nil || len(recovered) != len(original) {
		t.Fatal("environment lookup did not recover", recovered, err)
	}
	for index := range original {
		if *recovered[index].Code != *original[index].Code {
			t.Fatal("cancelled read changed environment choices")
		}
	}
}
