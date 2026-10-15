package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func presenceFeatureLookup(value string, present bool) func(string) (string, bool) {
	return func(name string) (string, bool) {
		if name != profileSpeciesPresenceEnvironment {
			panic("unexpected species-only feature lookup")
		}
		return value, present
	}
}

func TestProjectProfileSpeciesPresenceServiceFeatureGate(t *testing.T) {
	contexts := &ContextService{}
	for _, value := range []struct {
		value   string
		present bool
	}{
		{"", false}, {"false", true},
	} {
		service, err := NewProjectProfileSpeciesPresenceService(contexts, presenceFeatureLookup(value.value, value.present))
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Preview(context.Background(), "", "{}")
		if err == nil || result != nil || !strings.Contains(err.Error(), "independently enabled") {
			t.Fatal("disabled facade reached request parsing or context work", result, err)
		}
	}
	if service, err := NewProjectProfileSpeciesPresenceService(nil, presenceFeatureLookup("true", true)); err == nil || service != nil {
		t.Fatal("nil context owner accepted", service, err)
	}
	if service, err := NewProjectProfileSpeciesPresenceService(contexts, nil); err == nil || service != nil {
		t.Fatal("implicit feature lookup accepted", service, err)
	}
	if service, err := NewProjectProfileSpeciesPresenceService(contexts, presenceFeatureLookup("invalid", true)); err == nil || service != nil {
		t.Fatal("malformed feature accepted", service, err)
	}
	var absent *ProjectProfileSpeciesPresenceService
	if result, err := absent.Preview(context.Background(), "", "{}"); err == nil || result != nil {
		t.Fatal("nil facade accepted", result, err)
	}
}

func TestProjectProfileSpeciesPresenceServiceStrictRequest(t *testing.T) {
	contexts, _, request := profilePresenceContextFixture(t)
	service, err := NewProjectProfileSpeciesPresenceService(contexts, presenceFeatureLookup("true", true))
	if err != nil {
		t.Fatal(err)
	}
	valid, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeSpeciesPresenceRequest(valid)
	if err != nil || !reflect.DeepEqual(decoded, request) {
		t.Fatal("production request wire did not preserve complete typed witness", decoded, err)
	}
	cases := map[string][]byte{
		"empty":                  []byte("{}"),
		"null":                   []byte("null"),
		"array":                  []byte("[]"),
		"missing original":       []byte(`{"subvarieties":false}`),
		"null original":          []byte(`{"originalRules":null,"subvarieties":false}`),
		"missing boolean":        bytes.Replace(valid, []byte(`,"subvarieties":false`), nil, 1),
		"null boolean":           bytes.Replace(valid, []byte(`"subvarieties":false`), []byte(`"subvarieties":null`), 1),
		"unknown":                append([]byte(`{"unknown":1,`), valid[1:]...),
		"recased key":            bytes.Replace(valid, []byte(`"originalRules"`), []byte(`"OriginalRules"`), 1),
		"duplicate key":          append([]byte(`{"subvarieties":false,`), valid[1:]...),
		"escaped duplicate key":  append([]byte(`{"\u0073ubvarieties":false,`), valid[1:]...),
		"nested duplicate":       bytes.Replace(valid, []byte(`"storage":`), []byte(`"storage":"null","storage":`), 1),
		"nested unknown":         bytes.Replace(valid, []byte(`"declaredType":`), []byte(`"type":"TEXT","declaredType":`), 1),
		"nested recased":         bytes.Replace(valid, []byte(`"storage":`), []byte(`"Storage":`), 1),
		"nested case collision":  bytes.Replace(valid, []byte(`"storage":`), []byte(`"Storage":"null","storage":`), 1),
		"known field surrogate":  bytes.Replace(valid, []byte(`"text":"TARGET"`), []byte(`"text":"\ud800"`), 1),
		"known field UTF8":       bytes.Replace(valid, []byte(`"text":"TARGET"`), []byte{'"', 't', 'e', 'x', 't', '"', ':', '"', 0xff, '"'}, 1),
		"unpaired surrogate":     append([]byte(`{"unknown":"\ud800",`), valid[1:]...),
		"malformed UTF8":         append([]byte{'{', '"', 0xff, '"', ':', '0', ','}, valid[1:]...),
		"trailing JSON document": append(append([]byte{}, valid...), []byte(`{}`)...),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if bytes.Equal(data, valid) {
				t.Fatal("invalid request fixture did not change production wire")
			}
			result, err := service.Preview(context.Background(), "", string(data))
			if err == nil || result != nil || !strings.Contains(err.Error(), "request") {
				t.Fatal("invalid input reached owned context or returned a success-shaped DTO", result, err)
			}
		})
	}
}

func TestProjectProfileSpeciesPresenceServiceWireAndRefusals(t *testing.T) {
	contexts, state, request := profilePresenceContextFixture(t)
	service, err := NewProjectProfileSpeciesPresenceService(contexts, presenceFeatureLookup("true", true))
	if err != nil {
		t.Fatal(err)
	}
	files := databaseBytes(t, contexts.projects.sqlite.attachments)
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Preview(context.Background(), state.ContextID, string(wire))
	expected := &ProjectProfileSpeciesPresencePreview{
		ContextID: state.ContextID, Mode: "saved-species-only-readonly",
		Project: "Sample", Table: "Sample_Profile", SU: "None",
		TotalPlots: 52, PlotNumbers: []string{"108050"},
	}
	if err != nil || !reflect.DeepEqual(result, expected) {
		t.Fatal("species-only facade changed native-calibrated presence semantics", result, err)
	}
	output, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(output, &properties); err != nil || len(properties) != 7 {
		t.Fatal("unexpected readonly result shape", string(output), err)
	}
	for _, key := range []string{"contextId", "mode", "project", "table", "su", "totalPlots", "plotNumbers"} {
		if _, ok := properties[key]; !ok {
			t.Fatal("missing production output property", key)
		}
	}
	var reopened ProjectProfileSpeciesPresencePreview
	decoder := json.NewDecoder(bytes.NewReader(output))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reopened); err != nil || !reflect.DeepEqual(&reopened, expected) {
		t.Fatal("strict Go output roundtrip differs", reopened, err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if result, err := service.Preview(cancelled, state.ContextID, string(wire)); !errors.Is(err, context.Canceled) || result != nil {
		t.Fatal("cancelled request was not zeroed", result, err)
	}
	if result, err := service.Preview(nil, state.ContextID, string(wire)); err == nil || result != nil {
		t.Fatal("nil request context accepted", result, err)
	}
	if result, err := service.Preview(context.Background(), "foreign", string(wire)); err == nil || result != nil {
		t.Fatal("foreign context accepted", result, err)
	}
	request.Subvarieties = true
	wire, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.Preview(context.Background(), state.ContextID, string(wire)); err == nil || result != nil || !strings.Contains(err.Error(), "subvariety") {
		t.Fatal("unqualified subvarieties accepted", result, err)
	}
	request.Subvarieties = false
	assertPresenceContextUnchanged(t, contexts, state, request, files)
}

func TestProjectProfileSpeciesPresenceServiceOptionalAndActiveLumpWire(t *testing.T) {
	contexts, state, request := profilePresenceContextFixture(t)
	service, err := NewProjectProfileSpeciesPresenceService(contexts, presenceFeatureLookup("true", true))
	if err != nil {
		t.Fatal(err)
	}
	request.ProjectLump = nil
	files := databaseBytes(t, contexts.projects.sqlite.attachments)
	for _, includeNull := range []bool{false, true} {
		properties := map[string]any{"originalRules": request.OriginalRules, "subvarieties": false}
		if includeNull {
			properties["projectLump"] = nil
		}
		wire, err := json.Marshal(properties)
		if err != nil {
			t.Fatal(err)
		}
		result, err := service.Preview(context.Background(), state.ContextID, string(wire))
		if err != nil || result == nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
			t.Fatal("omitted or explicit NULL optional Lump changed Veg-only matching", includeNull, result, err)
		}
		assertPresenceContextUnchanged(t, contexts, state, request, files)
	}
	mutatePresenceContextFixture(t, contexts, `UPDATE Sample_Profile SET "Table"='Lump',Species='LT'`)
	review, err := contexts.ReviewProjectPlotProfile(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	lump, err := contexts.ReviewProjectPlotProfileLump(context.Background(), state.ContextID)
	if err != nil {
		t.Fatal(err)
	}
	request.OriginalRules = review.Rules
	request.ProjectLump = &lump
	files = databaseBytes(t, contexts.projects.sqlite.attachments)
	wire, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Preview(context.Background(), state.ContextID, string(wire))
	if err != nil || result == nil || !reflect.DeepEqual(result.PlotNumbers, []string{"108050"}) {
		t.Fatal("complete active Lump witness failed production wire", result, err)
	}
	assertPresenceContextUnchanged(t, contexts, state, request, files)
	request.ProjectLump = nil
	wire, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := service.Preview(context.Background(), state.ContextID, string(wire)); err == nil || result != nil {
		t.Fatal("active Lump silently accepted missing witness", result, err)
	}
	request.ProjectLump = &lump
	assertPresenceContextUnchanged(t, contexts, state, request, files)
}
