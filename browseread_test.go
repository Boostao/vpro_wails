package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestBrowseAndReferenceReadsObserveCancellationAndRecover(t *testing.T) {
	projects, _, _, _ := sqliteServiceFixture(t)
	references, err := NewReferenceService(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer references.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := []func(context.Context) error{
		func(ctx context.Context) error { _, err := projects.ListPlots(ctx, 0, 25); return err },
		func(ctx context.Context) error { _, err := projects.GetHierarchyNodes(ctx); return err },
		func(ctx context.Context) error { _, err := references.SearchSpecies(ctx, "ABIELAS", 10); return err },
		func(ctx context.Context) error { _, err := references.GetSpecies(ctx, "ABIELAS"); return err },
		func(ctx context.Context) error { _, err := references.GetListItems(ctx, "MoistureRegime"); return err },
		func(ctx context.Context) error { _, err := references.GetAllLists(ctx); return err },
	}
	for index, read := range reads {
		if err := read(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("read %d lost cancellation: %v", index, err)
		}
		if err := read(context.Background()); err != nil {
			t.Fatalf("read %d did not recover: %v", index, err)
		}
	}
}

func TestBrowseAndScopedReadsCancelQueuedCoordinatorAndSnapshotLocks(t *testing.T) {
	service, state, header := poolFixture(t)
	coordinator := service.projects.sqlite
	tests := []struct {
		name         string
		lock, unlock func()
		read         func(context.Context) error
	}{
		{"browse coordinator", coordinator.mu.Lock, coordinator.mu.Unlock, func(ctx context.Context) error {
			_, err := service.projects.ListPlots(ctx, 0, 25)
			return err
		}},
		{"hierarchy coordinator", coordinator.mu.Lock, coordinator.mu.Unlock, func(ctx context.Context) error {
			_, err := service.projects.GetHierarchyNodes(ctx)
			return err
		}},
		{"scoped pool coordinator", coordinator.mu.Lock, coordinator.mu.Unlock, func(ctx context.Context) error {
			_, err := service.GetPlot(ctx, state.ContextID, header.PlotNumber)
			return err
		}},
		{"selection snapshot", service.projects.mu.Lock, service.projects.mu.Unlock, func(ctx context.Context) error {
			_, err := service.GetPlot(ctx, state.ContextID, header.PlotNumber)
			return err
		}},
		{"plot preferences snapshot", service.plots.mu.Lock, service.plots.mu.Unlock, func(ctx context.Context) error {
			_, err := service.GetPlot(ctx, state.ContextID, header.PlotNumber)
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.lock()
			ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
			start := time.Now()
			err := test.read(ctx)
			cancel()
			test.unlock()
			if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > time.Second {
				t.Fatal("queued read retained its lock wait", err)
			}
			if err := test.read(context.Background()); err != nil {
				t.Fatal("subsequent read failed", err)
			}
		})
	}
}
