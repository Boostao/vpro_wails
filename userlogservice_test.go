package main

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestUserLogServiceIndependentLiteralGateAndRead(t *testing.T) {
	contexts, state := userLogFixture(t)
	before := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, item := range []struct {
		value   string
		present bool
		enabled bool
		invalid bool
	}{
		{"", false, false, false}, {"false", true, false, false}, {"true", true, true, false},
		{"", true, false, true}, {"TRUE", true, false, true}, {"1", true, false, true}, {" true", true, false, true},
	} {
		service, err := NewUserLogService(contexts, func(name string) (string, bool) {
			if name != "VPRO_USER_LOG_REVIEW" {
				t.Fatal("unexpected prerequisite flag", name)
			}
			return item.value, item.present
		})
		if item.invalid {
			if err == nil {
				t.Fatal("nonliteral flag accepted", item)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.GetReview(context.Background(), state.ContextID, 2, 0, 50)
		if item.enabled {
			if err != nil || result.Total != 3 || result.Mode != 2 {
				t.Fatal("enabled source filter unavailable", result, err)
			}
		} else if err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("disabled reader returned data", result, err)
		}
	}
	if !reflect.DeepEqual(before, databaseBytes(t, contexts.projects.sqlite.attachments)) {
		t.Fatal("gate checks/read changed the database family")
	}
}

func TestUserLogServiceInvalidOwnerRequestAndCancellation(t *testing.T) {
	contexts, state := userLogFixture(t)
	enabled := &UserLogService{contexts: contexts, enabled: true}
	if _, err := NewUserLogService(nil, func(string) (string, bool) { return "true", true }); err == nil {
		t.Fatal("missing context owner accepted")
	}
	if _, err := NewUserLogService(contexts, nil); err == nil {
		t.Fatal("implicit feature lookup accepted")
	}
	for _, service := range []*UserLogService{nil, {}, {enabled: true}} {
		if result, err := service.GetReview(context.Background(), state.ContextID, 2, 0, 50); err == nil || !reflect.DeepEqual(result, UserLogReview{}) {
			t.Fatal("absent/disabled service accepted", result, err)
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
