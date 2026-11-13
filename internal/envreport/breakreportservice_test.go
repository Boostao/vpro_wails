package envreport

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestBreakReportServiceLiteralIndependentFlag(t *testing.T) {
	contexts, state := breakReportFixture(t)
	if _, err := NewBreakReportService(nil, func(string) (string, bool) { return "", false }); err == nil {
		t.Fatal("absent contexts accepted")
	}
	if _, err := NewBreakReportService(contexts, nil); err == nil {
		t.Fatal("absent lookup accepted")
	}
	for _, item := range []struct {
		raw, mode string
		present   bool
	}{
		{"", "disabled", false}, {"false", "disabled", true}, {"true", "enabled", true},
		{"", "invalid", true}, {"TRUE", "invalid", true}, {"1", "invalid", true},
		{" true", "invalid", true}, {"true ", "invalid", true}, {"False", "invalid", true},
	} {
		t.Run(item.raw+item.mode, func(t *testing.T) {
			service, err := NewBreakReportService(contexts, func(name string) (string, bool) {
				if name != vprocore.BreakReportReviewEnvironment {
					t.Fatal("wrong prerequisite flag", name)
				}
				return item.raw, item.present
			})
			if item.mode == "invalid" {
				if err == nil || service != nil {
					t.Fatal("malformed literal flag enabled a service", service, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			before := vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)
			result, err := service.GetReview(context.Background(), state.ContextID, 0, 100)
			if item.mode == "enabled" {
				if err != nil || result.Total != 7 {
					t.Fatal("enabled owned service unavailable", result, err)
				}
			} else if err == nil || !reflect.DeepEqual(result, BreakReportReview{}) {
				t.Fatal("disabled service returned source data", result, err)
			}
			if !reflect.DeepEqual(before, vprotest.DatabaseBytes(t, contexts.Projects.Sqlite.Attachments)) {
				t.Fatal("service review/denial changed database family")
			}
		})
	}
}

func TestBreakReportServiceNilCancelledAndUnrelatedGate(t *testing.T) {
	contexts, state := breakReportFixture(t)
	var absent *BreakReportService
	if result, err := absent.GetReview(context.Background(), state.ContextID, 0, 1); err == nil ||
		!reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("nil service succeeded", result, err)
	}
	service, err := NewBreakReportService(contexts, func(name string) (string, bool) {
		if name == "VPRO_USER_LOG_REVIEW" || name == SubzoneMatrixFeatureEnvironment {
			return "true", true
		}
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.GetReview(context.Background(), state.ContextID, 0, 1); err == nil ||
		!reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("unrelated gate authorized break report", result, err)
	}
	if _, err := service.GetReview(nil, state.ContextID, 0, 1); err == nil {
		t.Fatal("nil request context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.GetReview(ctx, state.ContextID, 0, 1); !errors.Is(err, context.Canceled) ||
		!reflect.DeepEqual(result, BreakReportReview{}) {
		t.Fatal("cancellation became success-shaped denial", result, err)
	}
}
