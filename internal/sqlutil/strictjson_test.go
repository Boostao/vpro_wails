package sqlutil

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSimpleFoldKeyMatchesStringsEqualFoldAllRunes(t *testing.T) {
	// Collect every rune participating in a non-trivial SimpleFold equivalence class
	// and verify simpleFoldKey(a) == simpleFoldKey(b) <=> strings.EqualFold(a, b).
	var interesting []rune
	for r := rune(0); r <= utf8.MaxRune; r++ {
		if r >= 0xD800 && r <= 0xDFFF {
			continue
		}
		s := string(r)
		if simpleFoldKey(s) != s || r == 'S' || r == 's' || r == '\u017f' || r == 'K' || r == 'k' || r == '\u212a' || r == 'I' || r == 'i' || r == '\u0130' || r == '\u0131' {
			interesting = append(interesting, r)
		}
	}
	if len(interesting) < 1000 {
		t.Fatalf("expected >1000 case-foldable runes, got %d", len(interesting))
	}
	// Check critical tricky pairs across ASCII, Latin-Extended, Turkish I, Kelvin, Long S, Greek, Cyrillic.
	probe := []rune{'A', 'a', 'K', 'k', '\u212a', 'S', 's', '\u017f', 'I', 'i', '\u0130', '\u0131', 'ß', 'Μ', 'μ', 'µ', 'Ω', 'ω', '\u2126'}
	for _, r1 := range probe {
		s1 := string(r1)
		k1 := simpleFoldKey(s1)
		for _, r2 := range probe {
			s2 := string(r2)
			k2 := simpleFoldKey(s2)
			want := strings.EqualFold(s1, s2)
			got := k1 == k2
			if got != want {
				t.Fatalf("simpleFoldKey mismatch for U+%04X (%q -> %q) vs U+%04X (%q -> %q): got %v, EqualFold=%v",
					r1, s1, k1, r2, s2, k2, got, want)
			}
		}
	}
}

func TestValidateRawJSONWireRejectsMalformedAndDuplicates(t *testing.T) {
	invalidCases := []struct {
		name string
		wire []byte
	}{
		{"empty", []byte("")},
		{"whitespace_only", []byte("   \t\r\n ")},
		{"raw_0xff_in_value", []byte{'{', '"', 'a', '"', ':', '"', 0xff, '"', '}'}},
		{"raw_0xff_in_key", []byte{'{', '"', 0xff, '"', ':', '1', '}'}},
		{"raw_cesu8_surrogate", []byte{'{', '"', 'a', '"', ':', '"', 0xed, 0xa0, 0x80, '"', '}'}},
		{"raw_nul_in_string", []byte{'{', '"', 'a', '"', ':', '"', 0x00, '"', '}'}},
		{"raw_nul_outside_string", []byte{0x00, '{', '}'}},
		{"unescaped_newline_in_string", []byte("{\"a\":\"line1\nline2\"}")},
		{"lone_high_surrogate_value", []byte(`{"a":"\ud800"}`)},
		{"lone_high_surrogate_end_range", []byte(`{"a":"\udbff"}`)},
		{"lone_low_surrogate_value", []byte(`{"a":"\udc00"}`)},
		{"lone_low_surrogate_end_range", []byte(`{"a":"\udfff"}`)},
		{"lone_high_surrogate_key", []byte(`{"\ud800":1}`)},
		{"mismatched_surrogate_ascii", []byte(`{"a":"\ud800\u0041"}`)},
		{"mismatched_surrogate_double_high", []byte(`{"a":"\ud800\ud800"}`)},
		{"reversed_surrogate_pair", []byte(`{"a":"\udc00\ud800"}`)},
		{"surrogate_escaped_backslash_trap", []byte(`{"a":"\ud800\\udc00"}`)},
		{"truncated_unicode_escape", []byte(`{"a":"\u12"}`)},
		{"invalid_unicode_hex", []byte(`{"a":"\u12zz"}`)},
		{"nested_array_surrogate", []byte(`{"items":["ok",["\udfff"]]}`)},
		{"exact_duplicate_key", []byte(`{"a":1,"a":2}`)},
		{"escaped_exact_duplicate_key", []byte(`{"a":1,"\u0061":2}`)},
		{"ascii_case_duplicate_key", []byte(`{"plotNumber":"00337","PlotNumber":"108050"}`)},
		{"unicode_long_s_duplicate_key", []byte(`{"rootsSize":1,"rootſSize":2}`)},
		{"unicode_long_s_escaped_duplicate_key", []byte(`{"rootsSize":1,"root\u017fSize":2}`)},
		{"unicode_kelvin_duplicate_key", []byte(`{"unitK":1,"unitK":2}`)},
		{"nested_object_duplicate_key", []byte(`{"outer":{"inner":1,"INNER":2}}`)},
		{"array_element_object_duplicate_key", []byte(`[{"k":1},{"k":1,"K":2}]`)},
		{"trailing_brace_object", []byte(`{"a":1}{"b":2}`)},
		{"trailing_garbage", []byte(`{"a":1} trailing`)},
		{"trailing_comma_object", []byte(`{"a":1,}`)},
		{"trailing_comma_array", []byte(`[1,2,]`)},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRawJSONWire(tc.wire); err == nil {
				t.Fatalf("expected ValidateRawJSONWire to reject %s (%q), got nil", tc.name, tc.wire)
			}
		})
	}

	validCases := []struct {
		name string
		wire []byte
	}{
		{"valid_surrogate_pair_tree", []byte(`{"sp":"\uD83C\uDF32"}`)},
		{"valid_raw_utf8_tree", []byte(`{"sp":"🌲"}`)},
		{"literal_escaped_backslash_ud800", []byte(`{"note":"\\ud800"}`)},
		{"double_escaped_backslash_ud800", []byte(`{"note":"\\\\ud800"}`)},
		{"literal_ufffd", []byte(`{"note":"\ufffd"}`)},
		{"escaped_u0000_preserved_for_downstream", []byte(`{"expected":" historical\u0000field "}`)},
		{"distinct_sibling_objects_same_keys", []byte(`[{"a":1},{"A":2}]`)},
		{"turkish_dotless_i_not_equal_fold_ascii_i", []byte(`{"i":1,"ı":2,"İ":3}`)},
		{"numbers_and_primitives", []byte(`{"zero":0,"neg":-12.34e+5,"ok":true,"no":false,"empty":null}`)},
	}

	for _, tc := range validCases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateRawJSONWire(tc.wire); err != nil {
				t.Fatalf("expected ValidateRawJSONWire to accept %s (%q), got %v", tc.name, tc.wire, err)
			}
		})
	}
}

func TestValidateRawJSONWireExactMapKeys(t *testing.T) {
	if err := ValidateRawJSONWireExactMapKeys([]byte(`{"A1":"keep_target","a1":"accept_source","rootsSize":"1","rootſSize":"2"}`)); err != nil {
		t.Fatalf("expected case-distinct map keys to pass ExactMapKeys check, got %v", err)
	}
	for _, bad := range []string{
		`{"A1":"keep_target","A1":"accept_source"}`,
		`{"A1":"keep_target","\u00411":"accept_source"}`,
		`{"A1":"\ud800"}`,
		`{"A1":"keep_target"}{}`,
	} {
		if err := ValidateRawJSONWireExactMapKeys([]byte(bad)); err == nil {
			t.Fatalf("expected ExactMapKeys check to reject %s", bad)
		}
	}
}

type sampleNestedRule struct {
	Code  string `json:"code"`
	Value int    `json:"value"`
}

type sampleUnexportedEmbedded struct {
	PromotedMap map[string]string `json:"promotedMap,omitempty"`
}

type sampleDeepExactEmbedded struct {
	Items map[string]string `json:"items,omitempty"`
}

type sampleOuterEmbedded struct {
	sampleUnexportedEmbedded
	sampleDeepExactEmbedded
	ITEMs   sampleNestedRule `json:"ITEMs"` // shallower folded name vs deeper exact "items"
	NotePtr *string          `json:"notePtr,omitempty"`
	Tags    []string         `json:"tags,omitempty"`
}

type sampleMergeDTO struct {
	SourcePath       string                      `json:"sourcePath"`
	PlotResolutions  map[string]string           `json:"plotResolutions,omitempty"`
	FieldResolutions map[string]string           `json:"fieldResolutions,omitempty"`
	NestedRuleMap    map[string]sampleNestedRule `json:"nestedRuleMap,omitempty"`
}

func TestDecodeStrictJSONStructAndMapKeySemanticsAndAtomicity(t *testing.T) {
	// 1. Valid case-distinct map keys "A1" and "a1" inside struct DTO succeed!
	validWire := []byte(`{
		"sourcePath": "/tmp/Stage.db",
		"PlotResolutions": {"A1": "keep_target", "a1": "accept_source"},
		"fieldResolutions": {"A1::Env::Zone": "keep_target", "a1::Env::Zone": "accept_source"},
		"nestedRuleMap": {"P1": {"code": "FD", "value": 10}, "p1": {"code": "HW", "value": 20}}
	}`)
	var got sampleMergeDTO
	if err := DecodeStrictJSON(validWire, &got); err != nil {
		t.Fatalf("expected valid merge DTO with A1/a1 map keys to decode, got %v", err)
	}
	if got.PlotResolutions["A1"] != "keep_target" || got.PlotResolutions["a1"] != "accept_source" {
		t.Fatalf("unexpected PlotResolutions: %#v", got.PlotResolutions)
	}
	if got.NestedRuleMap["P1"].Code != "FD" || got.NestedRuleMap["p1"].Code != "HW" {
		t.Fatalf("unexpected NestedRuleMap: %#v", got.NestedRuleMap)
	}

	// 2. Rejections: struct duplicate keys (exact + EqualFold), map exact duplicate keys,
	// nested struct inside map EqualFold duplicate keys, unknown fields, surrogates, raw 0xFF, trailing JSON.
	rejectPayloads := []struct {
		name string
		wire []byte
	}{
		{"struct_exact_dup", []byte(`{"sourcePath":"a","sourcePath":"b"}`)},
		{"struct_case_dup", []byte(`{"sourcePath":"a","SourcePath":"b"}`)},
		{"struct_unicode_fold_dup", []byte(`{"sourcePath":"a","ſourcePath":"b"}`)},
		{"map_exact_dup", []byte(`{"sourcePath":"a","plotResolutions":{"A1":"keep_target","A1":"accept_source"}}`)},
		{"map_escaped_exact_dup", []byte(`{"sourcePath":"a","plotResolutions":{"A1":"keep_target","\u00411":"accept_source"}}`)},
		{"struct_inside_map_fold_dup", []byte(`{"sourcePath":"a","nestedRuleMap":{"A1":{"code":"FD","CODE":"HW"}}}`)},
		{"unknown_struct_field", []byte(`{"sourcePath":"a","extraUnknown":true}`)},
		{"unknown_nested_struct_field", []byte(`{"sourcePath":"a","nestedRuleMap":{"A1":{"code":"FD","bogus":1}}}`)},
		{"surrogate_in_map_val", []byte(`{"sourcePath":"a","plotResolutions":{"A1":"\ud800"}}`)},
		{"surrogate_in_map_key", []byte(`{"sourcePath":"a","plotResolutions":{"\udfff":"keep_target"}}`)},
		{"raw_0xff", []byte{'{', '"', 's', 'o', 'u', 'r', 'c', 'e', 'P', 'a', 't', 'h', '"', ':', '"', 0xff, '"', '}'}},
		{"trailing_tokens", []byte(`{"sourcePath":"a"}{}`)},
		{"trailing_number", []byte(`{"sourcePath":"a"} 42`)},
		{"wrong_field_type", []byte(`{"sourcePath":123}`)},
	}

	for _, tc := range rejectPayloads {
		t.Run(tc.name, func(t *testing.T) {
			sentinel := sampleMergeDTO{SourcePath: "untouched"}
			before := sentinel
			if err := DecodeStrictJSON(tc.wire, &sentinel); err == nil {
				t.Fatalf("expected DecodeStrictJSON to reject %s, got %#v", tc.name, sentinel)
			}
			if !reflect.DeepEqual(sentinel, before) {
				t.Fatalf("DecodeStrictJSON mutated target on failure %s: got %#v, want %#v", tc.name, sentinel, before)
			}
		})
	}

	// 3. AllowUnknownFields option permits unknown fields while still enforcing wire/duplicate/trailing rules.
	var tolerant sampleMergeDTO
	if err := DecodeStrictJSON([]byte(`{"sourcePath":"ok","legacyExtra":{"x":1}}`), &tolerant, StrictJSONOptions{AllowUnknownFields: true}); err != nil {
		t.Fatalf("AllowUnknownFields rejected unknown field: %v", err)
	}
	if tolerant.SourcePath != "ok" {
		t.Fatalf("unexpected SourcePath: %q", tolerant.SourcePath)
	}
	if err := DecodeStrictJSON([]byte(`{"sourcePath":"ok","legacyExtra":{"x":1,"X":2}}`), &tolerant, StrictJSONOptions{AllowUnknownFields: true}); err == nil {
		t.Fatal("AllowUnknownFields failed to reject duplicate keys inside unknown object")
	}
	if err := DecodeStrictJSON([]byte(`{"sourcePath":"ok","legacyExtra":"\ud800"}`), &tolerant, StrictJSONOptions{AllowUnknownFields: true}); err == nil {
		t.Fatal("AllowUnknownFields failed to reject lone surrogate inside unknown field")
	}
}

func TestDecodeStrictJSONDeepAtomicityEmbeddedResolutionAndDynamicMaps(t *testing.T) {
	// 1. Pre-populated non-nil pointer, map, and slice backing array must NOT be mutated when a later field fails.
	origNote := "keep-note"
	backingTags := []string{"tag0", "tag1"}
	prePopulated := sampleOuterEmbedded{
		sampleUnexportedEmbedded: sampleUnexportedEmbedded{
			PromotedMap: map[string]string{"A1": "keep_target"},
		},
		NotePtr: &origNote,
		Tags:    backingTags[:2],
	}

	// Valid wire up to unknownField: encoding/json mutates PromotedMap, *NotePtr, and Tags[0] BEFORE hitting unknownField!
	failingWire := []byte(`{"promotedMap":{"A1":"accept_source"},"notePtr":"mutated","tags":["overwritten"],"unknownField":1}`)
	if err := DecodeStrictJSON(failingWire, &prePopulated); err == nil {
		t.Fatal("expected unknownField error")
	}
	if prePopulated.PromotedMap["A1"] != "keep_target" {
		t.Fatalf("DecodeStrictJSON mutated pre-populated map in place: %#v", prePopulated.PromotedMap)
	}
	if prePopulated.NotePtr == nil || *prePopulated.NotePtr != "keep-note" || origNote != "keep-note" {
		t.Fatalf("DecodeStrictJSON mutated pre-populated pointer in place: %q / %q", *prePopulated.NotePtr, origNote)
	}
	if backingTags[0] != "tag0" || prePopulated.Tags[0] != "tag0" {
		t.Fatalf("DecodeStrictJSON mutated pre-populated slice backing array in place: %#v", prePopulated.Tags)
	}

	// 2. Unexported embedded struct promoted map[string]string AND deeper exact match ("items") beating shallower folded match ("ITEMs").
	var embedded sampleOuterEmbedded
	embeddedWire := []byte(`{
		"promotedMap": {"A1": "keep_target", "a1": "accept_source"},
		"items": {"P1": "keep_target", "p1": "accept_source"}
	}`)
	if err := DecodeStrictJSON(embeddedWire, &embedded); err != nil {
		t.Fatalf("embedded unexported map / deeper exact match rejected valid wire: %v", err)
	}
	if embedded.PromotedMap["A1"] != "keep_target" || embedded.PromotedMap["a1"] != "accept_source" {
		t.Fatalf("unexpected PromotedMap: %#v", embedded.PromotedMap)
	}
	if embedded.Items["P1"] != "keep_target" || embedded.Items["p1"] != "accept_source" {
		t.Fatalf("unexpected deeper exact Items: %#v", embedded.Items)
	}
	var shallowExact sampleOuterEmbedded
	if err := DecodeStrictJSON([]byte(`{"ITEMs":{"code":"FD","value":5}}`), &shallowExact); err != nil || shallowExact.ITEMs.Code != "FD" {
		t.Fatalf("shallow exact ITEMs failed: %v / %#v", err, shallowExact.ITEMs)
	}
	if err := DecodeStrictJSON([]byte(`{"items":{},"ITEMs":{"code":"FD","value":5}}`), &sampleOuterEmbedded{}); err == nil {
		t.Fatal("expected EqualFold duplicate key rejection when both items and ITEMs appear in same object")
	}

	// 3. Dynamic map[string]any allows nested case-distinct keys while rejecting exact duplicates.
	var dyn map[string]any
	if err := DecodeStrictJSON([]byte(`{"nested":{"A":1,"a":2}}`), &dyn); err != nil {
		t.Fatalf("dynamic map[string]any rejected nested case-distinct keys: %v", err)
	}
	if err := DecodeStrictJSON([]byte(`{"nested":{"A":1,"A":2}}`), &dyn); err == nil {
		t.Fatal("dynamic map[string]any accepted exact duplicate nested keys")
	}
}

