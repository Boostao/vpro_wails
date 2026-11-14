package sivi

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
)

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestSIVICreationServiceGuards(t *testing.T) {
	if _, err := NewSIVICreationService(nil); err == nil {
		t.Fatal("missing context ownership accepted")
	}
	service, err := NewSIVICreationService(&vprocore.ContextService{})
	if err != nil || service == nil {
		t.Fatalf("creation service unavailable: %v", err)
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
	service, err := NewSIVICreationService(&vprocore.ContextService{})
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
