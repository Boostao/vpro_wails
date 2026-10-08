package main

import (
	"context"
	"errors"
	"testing"
)

func TestPictureMetadataServiceIndependentGate(t *testing.T) {
	pictures := &PictureService{contexts: &ContextService{}}
	for _, flag := range []string{"", "false", "true", "TRUE", " true", "1"} {
		values := map[string]string{}
		if flag != "" {
			values[pictureMetadataWritingEnvironment] = flag
		}

		service, err := NewPictureMetadataService(pictures, pictureServiceLookup(values))
		if flag != "" && flag != "false" {
			if err == nil || service != nil {
				t.Fatal("malformed or read-authority-free metadata gate silently enabled", service, err)
			}
			if service, err := NewPictureMetadataService(pictures, pictureServiceLookup(map[string]string{
				pictureMetadataWritingEnvironment: "",
			})); service != nil || err == nil {
				t.Fatal("explicit empty metadata gate was silently repaired", service, err)
			}
			continue
		}
		if err != nil || service.enabled {
			t.Fatal("metadata writing is not independently default-off", service, err)
		}
		if got, err := service.Save(context.Background(), "owned", "P", "{}"); err == nil || got != nil {
			t.Fatal("disabled metadata Save produced success", got, err)
		}
		if got, err := service.LookupReceipt(context.Background(), "owned", "P", "{}"); err == nil || got != nil {
			t.Fatal("disabled metadata receipt borrowed read authority", got, err)
		}
	}
	for _, pictures := range []*PictureService{nil, {}} {
		if service, err := NewPictureMetadataService(pictures, pictureServiceLookup(nil)); service != nil || err == nil {
			t.Fatal("missing picture owner accepted", service, err)
		}
	}
	if service, err := NewPictureMetadataService(pictures, nil); service != nil || err == nil {
		t.Fatal("missing explicit metadata lookup accepted", service, err)
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

func TestPictureMetadataServiceReadingDoesNotGrantWrites(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	_, path := pictureLibraryFixture(t, pictureFixtureSchema)
	pictures, err := NewPictureService(contexts, pictureServiceLookup(map[string]string{
		pictureReadingEnvironment: "true",
		pictureLibraryEnvironment: path,
	}))
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewPictureMetadataService(pictures, pictureServiceLookup(nil))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pictures.GetMetadata(context.Background(), state.ContextID, "108050"); err != nil {
		t.Fatal(err)
	}
	if got, err := service.Save(context.Background(), state.ContextID, "108050", "{}"); err == nil || got != nil {
		t.Fatal("owned reading implicitly granted metadata writing", got, err)
	}
	service, err = NewPictureMetadataService(pictures, pictureServiceLookup(map[string]string{
		pictureMetadataWritingEnvironment: "true",
	}))
	if err != nil {
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
