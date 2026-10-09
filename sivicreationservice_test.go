package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSIVICreationServiceIndependentGateAndGuards(t *testing.T) {
	lookup := func(value string, present bool) func(string) (string, bool) {
		return func(name string) (string, bool) {
			if name == siviCreationFeatureEnvironment {
				return value, present
			}
			return "false", true
		}
	}
	if _, err := NewSIVICreationService(nil, lookup("true", true)); err == nil {
		t.Fatal("missing context ownership accepted")
	}
	if _, err := NewSIVICreationService(&ContextService{}, nil); err == nil {
		t.Fatal("missing feature lookup accepted")
	}
	for _, value := range []string{"", "unknown", " true "} {
		if _, err := NewSIVICreationService(&ContextService{}, lookup(value, true)); err == nil {
			t.Fatalf("malformed present creation gate %q accepted", value)
		}
	}
	for _, absent := range []bool{false, true} {
		service, err := NewSIVICreationService(&ContextService{}, lookup("false", !absent))
		if err != nil || service.enabled {
			t.Fatalf("default/off gate: %+v %v", service, err)
		}
		for _, operation := range []func() error{
			func() error { _, err := service.GetReferences(context.Background(), "C", "P"); return err },
			func() error { _, err := service.Create(context.Background(), "C", "{}"); return err },
			func() error { _, err := service.LookupReceipt(context.Background(), "C", "{}"); return err },
		} {
			if err := operation(); err == nil || !strings.Contains(err.Error(), "disabled") {
				t.Fatalf("independent gate failed: %v", err)
			}
		}
	}
	service, err := NewSIVICreationService(&ContextService{}, lookup("true", true))
	if err != nil || !service.enabled {
		t.Fatalf("creation incorrectly depends on another editing gate: %v", err)
	}
	if err := service.require(nil); err == nil {
		t.Fatal("nil request context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.require(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled request accepted: %v", err)
	}
	var missing *SIVICreationService
	if err := missing.require(context.Background()); err == nil {
		t.Fatal("nil service accepted")
	}
}

func TestSIVICreationServiceRawRequestsFailBeforeOwnership(t *testing.T) {
	service, err := NewSIVICreationService(&ContextService{}, func(name string) (string, bool) {
		return "true", name == siviCreationFeatureEnvironment
	})
	if err != nil {
		t.Fatal(err)
	}
	base := `{"requestId":"00000000-0000-4000-8000-000000000001","contextId":"C","project":"Sample","plot":"P","form":"SubVegC-SIVI","species":"HERB","covers":[{"column":"Cover6","value":{"storage":"real","text":null,"integer":null,"real":0,"blobHex":null}}]}`
	for _, raw := range []string{
		``, `{}`, `null`,
		strings.Replace(base, `"HERB"`, `"\ud800"`, 1),
		strings.Replace(base, `"requestId":`, `"requestId":"duplicate","requestId":`, 1),
		strings.Replace(base, `"requestId":`, `"RequestId":`, 1),
		strings.Replace(base, `"covers":`, `"ID":1,"covers":`, 1),
	} {
		for _, operation := range []func() error{
			func() error { _, err := service.Create(context.Background(), "C", raw); return err },
			func() error { _, err := service.LookupReceipt(context.Background(), "C", raw); return err },
		} {
			if err := operation(); err == nil {
				t.Fatalf("malformed raw request accepted: %q", raw)
			}
		}
	}
}
