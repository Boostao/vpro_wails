package pictures

import (
	"github.com/boostao/vpro-wails/internal/vprocore"
	"github.com/boostao/vpro-wails/internal/vprocore/vprotest"
)

import (
	"context"
	"errors"
	"testing"
)

func TestPictureMetadataServiceOwnerAndContextGuards(t *testing.T) {
	for _, pictures := range []*PictureService{nil, {}, {Contexts: &vprocore.ContextService{}}} {
		if service, err := NewPictureMetadataService(pictures); service != nil || err == nil {
			t.Fatal("missing picture owner accepted", service, err)
		}
	}
	var service *PictureMetadataService
	if err := service.require(nil); err == nil {
		t.Fatal("nil request context accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.require(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation became an availability success", err)
	}
}

func TestPictureMetadataServiceRejectsMalformedRequests(t *testing.T) {
	contexts, state := vprotest.ReportServiceFixture(t, false)
	_, path := pictureLibraryFixture(t, pictureFixtureSchema)
	pictures, err := NewPictureService(contexts, pictureServiceLookup(map[string]string{
		pictureLibraryEnvironment: path,
	}))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPictureMetadataService(pictures)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pictures.GetMetadata(context.Background(), state.ContextID, "108050"); err != nil {
		t.Fatal(err)
	}
	for _, request := range []string{"{}", "null", `{"requestId":"x","requestId":"y"}`, `{"requestId":"\ud800"}`} {
		if got, err := service.Save(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
			t.Fatal("malformed metadata request produced a Save", got, err)
		}
		if got, err := service.LookupReceipt(context.Background(), state.ContextID, "108050", request); err == nil || got != nil {
			t.Fatal("malformed metadata request produced a receipt", got, err)
		}
	}
}
