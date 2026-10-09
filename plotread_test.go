package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestScopedReadsRejectCancellationWithoutChangingData(t *testing.T) {
	service, state, original := poolFixture(t)
	before, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := []func() error{
		func() error { _, err := service.GetPlot(ctx, state.ContextID, original.PlotNumber); return err },
		func() error { _, err := service.GetHeaderCapabilities(ctx, state.ContextID); return err },
		func() error { _, err := service.GetChildCapabilities(ctx, state.ContextID, "Veg"); return err },
		func() error { _, err := service.ListVegRecords(ctx, state.ContextID, original.PlotNumber); return err },
		func() error {
			_, err := service.ListHumusRecords(ctx, state.ContextID, original.PlotNumber)
			return err
		},
		func() error {
			_, err := service.ListMineralRecords(ctx, state.ContextID, original.PlotNumber)
			return err
		},
		func() error {
			_, err := service.ListOtherRecords(ctx, state.ContextID, original.PlotNumber)
			return err
		},
		func() error {
			_, err := service.ListAuditEntries(ctx, state.ContextID, original.PlotNumber)
			return err
		},
	}
	for index, read := range reads {
		if err := read(); !errors.Is(err, context.Canceled) {
			t.Fatalf("read %d lost cancellation: %v", index, err)
		}
	}
	if _, err := service.GetPlot(context.Background(), state.ContextID, original.PlotNumber); err != nil {
		t.Fatal("valid retrieval failed after cancellation", err)
	}
	after, err := os.ReadFile(state.ProjectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("cancelled reads changed project/audit bytes", err)
	}
}

func TestScopedReadCancellationWhileSwitchLeaseIsUnavailable(t *testing.T) {
	service, state, original := poolFixture(t)
	service.projects.operationMu.Lock()
	defer service.projects.operationMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	if _, err := service.GetPlot(ctx, state.ContextID, original.PlotNumber); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("queued read did not observe cancellation", err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("cancelled read remained queued behind a switch")
	}
}

func TestScopedReadCancellationReleasesPoolWaitAndInterruptsSQLite(t *testing.T) {
	service, state, original := poolFixture(t)
	db := service.projects.sqlite.projectDB
	first, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := db.Conn(context.Background())
	if err != nil {
		first.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	_, err = service.GetPlot(ctx, state.ContextID, original.PlotNumber)
	cancel()
	closeErr := errors.Join(first.Close(), second.Close())
	if !errors.Is(err, context.DeadlineExceeded) || closeErr != nil {
		t.Fatal("pool wait was not cancelled", err, closeErr)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = withContextPlotRequest(ctx, service, state.ContextID, func(plots *PlotService) (int, error) {
		db, _, release, err := plots.getActiveDB()
		if err != nil {
			return 0, err
		}
		defer release()
		var total int
		err = plots.readDB(db).QueryRow(`WITH RECURSIVE work(x) AS
			(SELECT 1 UNION ALL SELECT x+1 FROM work WHERE x<1000000000)
			SELECT sum(x) FROM work`).Scan(&total)
		return total, err
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("SQLite did not observe request cancellation", err)
	}
	if db.Stats().InUse != 0 || !service.projects.operationMu.TryLock() {
		t.Fatal("cancelled query retained its connection or operation lease")
	}
	service.projects.operationMu.Unlock()
	if _, err := service.GetPlot(context.Background(), state.ContextID, original.PlotNumber); err != nil {
		t.Fatal("subsequent valid scoped read failed", err)
	}
}
