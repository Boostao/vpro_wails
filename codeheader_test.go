package main

import "testing"

func TestOrdinaryCodeHistoricalPolicies(t *testing.T) {
	groups := []struct {
		name                     string
		header                   func(*string) FS882Header
		validate                 func(FS882Header, *FS882Header) error
		malformedHistoricalError bool
	}{
		{"Region", func(value *string) FS882Header { return FS882Header{FSRegionDistrict: value} }, validateRegionHeaderValues, true},
		{"Soil", func(value *string) FS882Header { return FS882Header{SoilClassGroup: value} }, validateSoilHeaderValues, false},
		{"Geology", func(value *string) FS882Header { return FS882Header{BedrockGeology1: value} }, validateGeologyHeaderValues, false},
		{"Parent", func(value *string) FS882Header { return FS882Header{CoarseFragLith1: value} }, validateParentCodeHeaderValues, false},
	}
	for _, group := range groups {
		t.Run(group.name, func(t *testing.T) {
			for _, historical := range []string{"historical-overlength", ""} {
				old := group.header(&historical)
				if err := group.validate(old, &old); err != nil {
					t.Fatalf("unchanged historical %q: %v", historical, err)
				}
				if err := group.validate(old, nil); err == nil {
					t.Fatalf("new invalid value %q was accepted", historical)
				}
			}
			malformed := string([]byte{0xff})
			old := group.header(&malformed)
			if err := group.validate(old, &old); (err != nil) != group.malformedHistoricalError {
				t.Fatalf("historical malformed-text policy changed: %v", err)
			}
			if err := group.validate(old, nil); err == nil {
				t.Fatal("new malformed text was accepted")
			}
			if err := group.validate(group.header(nil), &old); err != nil {
				t.Fatalf("clearing historical value: %v", err)
			}
		})
	}
}

func TestOrdinaryCodeRestoreFieldOwnership(t *testing.T) {
	for _, fields := range [][]becHeaderField{regionHeaderFields(FS882Header{}), soilHeaderFields(FS882Header{})} {
		for _, field := range fields {
			if err := validateCodeRestoreValue(field.table, field.name, nil, fields); err != nil {
				t.Fatalf("%s NULL: %v", field.name, err)
			}
			if err := validateCodeRestoreValue(field.table, field.name, 1, fields); err == nil {
				t.Fatalf("%s numeric restore accepted", field.name)
			}
			if err := validateCodeRestoreValue("Admin", field.name, 1, fields); err != nil {
				t.Fatalf("%s leaked into another table: %v", field.name, err)
			}
		}
	}
}
