package main

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func pictureServiceLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, present := values[name]
		return value, present
	}
}

func TestPictureServiceIndependentGateAndExplicitSources(t *testing.T) {
	for _, flag := range []string{"", "false", "true", "TRUE", " true", "1"} {
		lookup := func(name string) (string, bool) {
			if name != pictureReadingEnvironment {
				t.Fatal("disabled picture read inferred a library or borrowed authority", name)
			}
			return flag, flag != ""
		}
		if flag == "true" {
			lookup = pictureServiceLookup(map[string]string{pictureReadingEnvironment: flag})
		}
		service, err := NewPictureService(&ContextService{}, lookup)
		if flag != "" && flag != "false" {
			if err == nil || service != nil {
				t.Fatal("malformed gate or missing enabled library silently defaulted", service, err)
			}
			continue
		}
		if err != nil || service.enabled {
			t.Fatal("picture reader is not independently default-off", service, err)
		}
		if got, err := service.GetMetadata(context.Background(), "owned", "P"); err == nil || got != nil {
			t.Fatal("disabled metadata read succeeded", got, err)
		}
		if got, err := service.GetImage(context.Background(), "owned", "P", "{}"); err == nil || got != nil {
			t.Fatal("disabled preview borrowed authority", got, err)
		}
	}
	if service, err := NewPictureService(nil, pictureServiceLookup(nil)); service != nil || err == nil {
		t.Fatal("nil context owner constructed a picture service", service, err)
	}
	if service, err := NewPictureService(&ContextService{}, nil); service != nil || err == nil {
		t.Fatal("missing explicit lookup constructed a picture service", service, err)
	}
	var missing *PictureService
	if got, err := missing.GetMetadata(context.Background(), "owned", "P"); err == nil || got != nil {
		t.Fatal("nil service produced metadata", got, err)
	}
	if err := missing.require(nil); err == nil {
		t.Fatal("nil request context was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := missing.require(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled request produced success-shaped availability", err)
	}
}

func TestPictureServiceStrictReviewedPreviewAndReadOnlyMethods(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	source, path := pictureLibraryFixture(t, pictureFixtureSchema)
	directory, _ := pictureImageFixture(t, "jpeg", 448, 300)
	values := map[string]string{
		pictureReadingEnvironment:        "true",
		pictureLibraryEnvironment:        path,
		pictureChildDirectoryEnvironment: directory.path,
	}
	service, err := NewPictureService(contexts, pictureServiceLookup(values))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := service.GetMetadata(context.Background(), state.ContextID, "108050")
	expected, expectedErr := contexts.readPictureMetadata(context.Background(), state.ContextID, "108050", source)
	if err != nil || expectedErr != nil || metadata == nil || !reflect.DeepEqual(*metadata, expected) {
		t.Fatal("facade changed complete physical metadata", metadata, err, expectedErr)
	}
	request := PictureImageRequest{View: pictureChildView, Original: metadata.Records.Rows[3]}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	valid := string(data)
	for _, malformed := range []string{
		`{}`, `{"view":"child","original":null}`, `{"view":null,"original":{}}`,
		strings.Replace(valid, `"view":`, `"unknown":true,"view":`, 1),
		strings.Replace(valid, `"view":"child"`, `"view":"manager","view":"child"`, 1),
		strings.Replace(valid, `"view":"child"`, `"view":"Child"`, 1),
		strings.Replace(valid, `"original":`, `"Original":`, 1),
		strings.Replace(valid, `"rowId":`, `"rowId":"1","rowId":`, 1),
		strings.Replace(valid, `"rowId":`, `"rowId":"\ud800","ignored":`, 1),
		strings.Replace(valid, `"cells":`, `"cells":null,"cells":`, 1),
		strings.Replace(valid, `"storage":`, `"storage":"null","storage":`, 1),
		strings.Replace(valid, `"storage":`, `"Storage":`, 1),
		strings.Replace(valid, `"blobHex":`, `"unknown":null,"blobHex":`, 1),
		strings.Replace(valid, `"text":"Default"`, `"text":"\ud800"`, 1),
		strings.Replace(valid, `"text":"Default"`, "\"text\":\""+string([]byte{0xff})+"\"", 1),
		valid + `{}`,
	} {
		if got, err := service.GetImage(context.Background(), state.ContextID, "108050", malformed); err == nil || got != nil {
			t.Fatal("malformed/duplicate/raw Unicode authority returned a preview", malformed, got, err)
		}
	}
	for _, test := range []struct{ contextID, plot string }{
		{"stale", "108050"}, {state.ContextID, "missing"}, {state.ContextID, "108050 "},
	} {
		if got, err := service.GetImage(context.Background(), test.contextID, test.plot, valid); err == nil || got != nil {
			t.Fatal("foreign context/plot preview succeeded", got, err)
		}
	}
	got, err := service.GetImage(context.Background(), state.ContextID, "108050", valid)
	if err != nil || got == nil || got.Width != 448 || got.Height != 300 || got.RowID != request.Original.RowID {
		t.Fatal("facade did not preserve owned measured image", got, err)
	}
	request.View = pictureManagerView
	data, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := service.GetImage(context.Background(), state.ContextID, "108050", string(data)); err == nil || got != nil {
		t.Fatal("manager preview implicitly inherited child directory permission", got, err)
	}
	kind := reflect.TypeOf(service)
	if kind.NumMethod() != 2 || kind.Method(0).Name != "GetImage" || kind.Method(1).Name != "GetMetadata" {
		t.Fatal("picture read facade accidentally exposed write/import/installation authority", kind)
	}
}

func TestPictureServiceOptionalDirectoryConfigurationIsNotRepaired(t *testing.T) {
	contexts, state := reportServiceFixture(t, false)
	_, path := pictureLibraryFixture(t, pictureFixtureSchema)
	values := map[string]string{pictureReadingEnvironment: "true", pictureLibraryEnvironment: path}
	service, err := NewPictureService(contexts, pictureServiceLookup(values))
	if err != nil {
		t.Fatal("explicit metadata-only library required an inferred image directory", err)
	}
	if _, err := service.GetMetadata(context.Background(), state.ContextID, "108050"); err != nil {
		t.Fatal("metadata-only optional library was unavailable", err)
	}
	for _, name := range []string{pictureChildDirectoryEnvironment, pictureManagerDirectoryEnvironment} {
		for _, path := range []string{"", "relative", "invalid\x00", string([]byte{0xff})} {
			values[name] = path
			if got, err := NewPictureService(contexts, pictureServiceLookup(values)); err == nil || got != nil {
				t.Fatal("configured invalid/empty directory was silently defaulted or repaired", name, path, got, err)
			}
		}
		delete(values, name)
	}
}
