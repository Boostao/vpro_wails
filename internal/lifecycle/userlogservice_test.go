package lifecycle

import (
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestUserLogServiceReadOnlyReview(t *testing.T) {
	contexts, state := userLogFixture(t)
	before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
	service, err := NewUserLogService(contexts)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.GetReview(context.Background(), state.ContextID, 2, 0, 50)
	if err != nil || result.Total != 3 || result.Mode != 2 {
		t.Fatal("enabled source filter unavailable", result, err)
	}
	if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
		t.Fatal("read changed the database family")
	}
}

func TestUserLogServiceInvalidOwnerRequestAndCancellation(t *testing.T) {
	contexts, state := userLogFixture(t)
	enabled := &UserLogService{contexts: contexts}
	if _, err := NewUserLogService(nil); err == nil {
		t.Fatal("missing context owner accepted")
	}
	for _, service := range []*UserLogService{nil, {}} {
		if result, err := service.GetReview(context.Background(), state.ContextID, 2, 0, 50); err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("absent service accepted", result, err)
		}
	}
	if _, err := enabled.GetReview(nil, state.ContextID, 2, 0, 50); err == nil {
		t.Fatal("nil request context accepted")
	}
	if _, err := enabled.GetReview(context.Background(), "foreign", 2, 0, 50); err == nil {
		t.Fatal("foreign context accepted")
	}
	if _, err := enabled.GetReview(context.Background(), state.ContextID, 0, 0, 50); err == nil {
		t.Fatal("implicit mode accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := enabled.GetReview(ctx, state.ContextID, 2, 0, 50); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, UserLogReview{}) {
		t.Fatal("cancelled service returned a partial review", result, err)
	}
}
