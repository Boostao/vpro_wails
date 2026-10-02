package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestVegetationSpeciesSourceClassesAndAliasAmbiguity(t *testing.T) {
	service, state := contextServiceFixture(t)
	before, err := os.ReadFile(state.ProjectPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		form  string
		count int
	}{
		{"SubVegAXL_BC", 547}, {"SubVegAhtXL", 547},
		{"SubVegCXL", 4742}, {"SubVegChtXL", 4742}, {"SubVegDXL", 3319},
	} {
		options, err := service.ListVegetationSpecies(context.Background(), state.ContextID, entry.form)
		if err != nil || len(options) != entry.count {
			t.Fatalf("%s source union/predicate: %d %v", entry.form, len(options), err)
		}
		for _, option := range options {
			if option.CodeType == nil || (*option.CodeType != "U" && *option.CodeType != "X") {
				t.Fatal("source code-type filter or literal metadata changed:", option)
			}
		}
	}
	aliases, err := service.ListVegetationSpeciesAliases(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "acarospo"})
	if err != nil || len(aliases) != 2 || aliases[0].Code == nil || aliases[1].Code == nil || *aliases[0].Code == *aliases[1].Code {
		t.Fatal("ambiguous old-code mappings were collapsed or guessed:", aliases, err)
	}
	missing, err := service.ListVegetationSpeciesAliases(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "  raw' "})
	if err != nil || len(missing) != 0 {
		t.Fatal("literal lookup was trimmed, completed or interpolated:", missing, err)
	}
	users, err := service.ListVegetationSpeciesUsers(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "abie_rk"})
	if err != nil || len(users) != 1 || users[0].Code == nil || *users[0].Code != "ABIE_RK" {
		t.Fatal("existing personal code lookup changed source literals:", users, err)
	}
	missingUsers, err := service.ListVegetationSpeciesUsers(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: "  raw' "})
	if err != nil || len(missingUsers) != 0 {
		t.Fatal("personal lookup trimmed or interpolated literal text:", missingUsers, err)
	}
	for _, code := range []string{"", "123456789", string([]byte{0xff})} {
		if _, err := service.ListVegetationSpeciesAliases(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: code}); err == nil {
			t.Fatal("invalid raw lookup accepted:", code)
		}
		if _, err := service.ListVegetationSpeciesUsers(context.Background(), state.ContextID, VegetationSpeciesLookup{Code: code}); err == nil {
			t.Fatal("invalid personal lookup accepted:", code)
		}
	}
	if _, err := service.ListVegetationSpecies(context.Background(), state.ContextID, "SubVegAXL"); err == nil {
		t.Fatal("inactive generic A form silently substituted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.ListVegetationSpecies(ctx, state.ContextID, "SubVegAXL_BC"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled reference lookup accepted:", err)
	}
	if _, err := service.ListVegetationSpeciesAliases(ctx, state.ContextID, VegetationSpeciesLookup{Code: "ACAROSPO"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled alias lookup accepted:", err)
	}
	if _, err := service.ListVegetationSpeciesUsers(ctx, state.ContextID, VegetationSpeciesLookup{Code: "ABIE_RK"}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled personal lookup accepted:", err)
	}
	if _, err := service.ListVegetationSpecies(context.Background(), "stale", "SubVegAXL_BC"); err == nil {
		t.Fatal("stale context reference lookup accepted")
	}
	if _, err := service.ListVegetationSpeciesAliases(context.Background(), "stale", VegetationSpeciesLookup{Code: "ACAROSPO"}); err == nil {
		t.Fatal("stale alias lookup accepted")
	}
	after, err := os.ReadFile(state.ProjectPath)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("reference calls changed project/history bytes")
	}
}

func TestVegetationSpeciesNullableTransportPreservesAllSourceFields(t *testing.T) {
	for _, payload := range []string{`{}`, `{"code":null}`, `{"code":"\ud800"}`, `{"code":"C","CODE":"\udfff"}`} {
		if err := json.Unmarshal([]byte(payload), &VegetationSpeciesLookup{}); err == nil {
			t.Fatal("invalid raw lookup transport accepted:", payload)
		}
	}
	empty, code, old := "", "RAW", "OLD"
	original := VegetationSpeciesAlias{
		VegetationSpeciesOption: VegetationSpeciesOption{Code: &code, ScientificName: nil, EnglishName: &empty, Lifeform: nil, CodeType: nil},
		OldCode:                 &old,
	}
	payload, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var decoded VegetationSpeciesAlias
	if err := json.Unmarshal(payload, &decoded); err != nil || !reflect.DeepEqual(original, decoded) {
		t.Fatal("NULL/empty metadata or embedded alias transport collapsed:", string(payload), err)
	}
	var properties map[string]json.RawMessage
	if err := json.Unmarshal(payload, &properties); err != nil || len(properties) != 6 {
		t.Fatal("source fields were omitted:", string(payload), err)
	}
}
