package main

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBreakReportConcurrentReviewAndContextSwitch(t *testing.T) {
	contexts, initial := breakReportFixture(t)
	service := &BreakReportService{contexts: contexts, enabled: true}
	paths := map[string]string{}
	for alias, path := range contexts.projects.sqlite.attachments {
		paths[alias] = path
	}
	before := databaseBytes(t, paths)
	var stateMu sync.RWMutex
	state := initial
	start, finished := make(chan struct{}), make(chan struct{})
	failures := make(chan error, 5)
	var success, stale atomic.Int64
	var workers sync.WaitGroup
	workers.Add(5)
	go func() {
		defer workers.Done()
		<-start
		current := initial
		for range 20 {
			next, err := contexts.SwitchContext(current.ContextID, contextSelection(current))
			if err != nil {
				failures <- fmt.Errorf("owned concurrent switch failed: %w", err)
				return
			}
			current = next
			stateMu.Lock()
			state = next
			stateMu.Unlock()
		}
	}()
	for range 4 {
		go func() {
			defer workers.Done()
			<-start
			for range 100 {
				stateMu.RLock()
				id := state.ContextID
				stateMu.RUnlock()
				result, err := service.GetReview(context.Background(), id, 0, 50)
				if err != nil {
					if err.Error() != "editor context is stale or closed; reload before editing" ||
						!reflect.DeepEqual(result, BreakReportReview{}) {
						failures <- fmt.Errorf("unexpected concurrent review failure: result=%+v err=%w", result, err)
						return
					}
					stale.Add(1)
					continue
				}
				if result.ContextID != id || result.SourcePath != paths["VPro64"] || result.Total != 7 || len(result.Rows) != 7 {
					failures <- fmt.Errorf("concurrent review crossed context/source ownership: %+v", result)
					return
				}
				success.Add(1)
			}
		}()
	}
	go func() {
		workers.Wait()
		close(finished)
	}()
	close(start)
	select {
	case <-finished:
	case <-time.After(30 * time.Second):
		t.Fatal("concurrent review/switch did not release context/snapshot leases")
	}
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	if success.Load()+stale.Load() != 400 {
		t.Fatal("planned concurrent reads did not all finish", success.Load(), stale.Load())
	}
	result, err := service.GetReview(context.Background(), state.ContextID, 0, 100)
	if err != nil || result.Total != 7 {
		t.Fatal("final owned context cannot read after concurrent switches", result, err)
	}
	if !reflect.DeepEqual(before, databaseBytes(t, paths)) {
		t.Fatal("readonly concurrent report requests changed physical database family")
	}
}
