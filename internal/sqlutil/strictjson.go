package sqlutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf16"
	"unicode/utf8"
)

const maxStrictJSONDepth = 1000

var jsonRawMessageType = reflect.TypeOf(json.RawMessage(nil))

// StrictJSONOptions configures DecodeStrictJSON and raw wire duplicate-key policy.
type StrictJSONOptions struct {
	AllowUnknownFields     bool
	ExactDuplicateKeysOnly bool
	ExactMapKeyFields      []string
}

func mergeStrictJSONOptions(opts []StrictJSONOptions) StrictJSONOptions {
	var merged StrictJSONOptions
	for _, opt := range opts {
		if opt.AllowUnknownFields {
			merged.AllowUnknownFields = true
		}
		if opt.ExactDuplicateKeysOnly {
			merged.ExactDuplicateKeysOnly = true
		}
		if len(opt.ExactMapKeyFields) > 0 {
			merged.ExactMapKeyFields = append(merged.ExactMapKeyFields, opt.ExactMapKeyFields...)
		}
	}
	return merged
}

// ValidateRawJSONWire performs a single-pass recursive RFC 8259 check over data before
// encoding/json can repair malformed UTF-8, unpaired UTF-16 surrogates, or duplicate keys.
// Every object scope rejects both byte-identical and strings.EqualFold duplicate keys.
func ValidateRawJSONWire(data []byte) error {
	return ValidateRawJSONWireForType(data, nil)
}

// ValidateRawJSONWireExactMapKeys validates data like ValidateRawJSONWire, except object scopes
// reject only exact duplicate keys so user-keyed maps may carry case-distinct keys (e.g. "A1" and "a1").
func ValidateRawJSONWireExactMapKeys(data []byte) error {
	return ValidateRawJSONWireForType(data, nil, StrictJSONOptions{ExactDuplicateKeysOnly: true})
}

// ValidateRawJSONWireForType validates raw JSON wire against an optional Go targetType so struct
// objects reject strings.EqualFold duplicate keys while map[string]V objects reject exact duplicates.
func ValidateRawJSONWireForType(data []byte, targetType reflect.Type, opts ...StrictJSONOptions) error {
	merged := mergeStrictJSONOptions(opts)
	if !utf8.Valid(data) {
		return errors.New("malformed UTF-8 in JSON wire")
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return errors.New("raw NUL byte in JSON wire")
	}
	pos := skipJSONWhitespace(data, 0)
	if pos >= len(data) {
		return errors.New("empty JSON wire")
	}
	nextPos, err := walkRawJSONValue(data, pos, targetType, false, merged, 0)
	if err != nil {
		return err
	}
	endPos := skipJSONWhitespace(data, nextPos)
	if endPos != len(data) {
		return fmt.Errorf("unexpected trailing JSON data at byte %d", endPos)
	}
	return nil
}

// DecodeStrictJSON validates raw JSON wire before Go JSON repair, decodes into target with
// DisallowUnknownFields() by default, verifies zero trailing tokens, and leaves target untouched on error.
func DecodeStrictJSON(data []byte, target any, opts ...StrictJSONOptions) error {
	if target == nil {
		return errors.New("DecodeStrictJSON requires a non-nil pointer target")
	}
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return errors.New("DecodeStrictJSON requires a non-nil pointer target")
	}
	merged := mergeStrictJSONOptions(opts)
	if err := ValidateRawJSONWireForType(data, rv.Type(), merged); err != nil {
		return err
	}

	staged := reflect.New(rv.Type().Elem())
	if !rv.Elem().IsZero() {
		staged.Elem().Set(deepCloneValue(rv.Elem()))
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	if !merged.AllowUnknownFields {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(staged.Interface()); err != nil {
		return err
	}
	if tok, err := dec.Token(); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected trailing JSON token %v", tok)
	}
	rv.Elem().Set(staged.Elem())
	return nil
}

// CloneForStaging returns a deep clone of exported pointers, maps, slices, arrays,
// interfaces, and structs in src so JSON unmarshaling into a staged value cannot
// overwrite caller-owned backing allocations on partial decode failure.
func CloneForStaging[T any](src T) T {
	v := reflect.ValueOf(src)
	if !v.IsValid() || v.IsZero() {
		return src
	}
	return deepCloneValue(v).Interface().(T)
}

func deepCloneValue(src reflect.Value) reflect.Value {
	if !src.IsValid() || src.IsZero() {
		if src.IsValid() {
			return reflect.Zero(src.Type())
		}
		return src
	}
	switch src.Kind() {
	case reflect.Pointer:
		cp := reflect.New(src.Type().Elem())
		cp.Elem().Set(deepCloneValue(src.Elem()))
		return cp
	case reflect.Map:
		if src.IsNil() {
			return reflect.Zero(src.Type())
		}
		cp := reflect.MakeMapWithSize(src.Type(), src.Len())
		iter := src.MapRange()
		for iter.Next() {
			cp.SetMapIndex(deepCloneValue(iter.Key()), deepCloneValue(iter.Value()))
		}
		return cp
	case reflect.Slice:
		if src.IsNil() {
			return reflect.Zero(src.Type())
		}
		cp := reflect.MakeSlice(src.Type(), src.Len(), src.Len())
		for i := 0; i < src.Len(); i++ {
			cp.Index(i).Set(deepCloneValue(src.Index(i)))
		}
		return cp
	case reflect.Array:
		cp := reflect.New(src.Type()).Elem()
		for i := 0; i < src.Len(); i++ {
			cp.Index(i).Set(deepCloneValue(src.Index(i)))
		}
		return cp
	case reflect.Struct:
		cp := reflect.New(src.Type()).Elem()
		cp.Set(src)
		cloneExportedStructFields(cp, src)
		return cp
	case reflect.Interface:
		if src.IsNil() {
			return reflect.Zero(src.Type())
		}
		cp := reflect.New(src.Type()).Elem()
		cp.Set(deepCloneValue(src.Elem()))
		return cp
	default:
		return src
	}
}

func cloneExportedStructFields(dst, src reflect.Value) {
	st := src.Type()
	for i := 0; i < src.NumField(); i++ {
		sf := src.Field(i)
		if sf.IsZero() {
			continue
		}
		f := st.Field(i)
		df := dst.Field(i)
		if f.IsExported() {
			df.Set(deepCloneValue(sf))
		} else if f.Anonymous && f.Type.Kind() == reflect.Struct {
			cloneExportedStructFields(df, sf)
		}
	}
}

func derefType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

func walkRawJSONValue(data []byte, pos int, targetType reflect.Type, forceExactMapKeys bool, opts StrictJSONOptions, depth int) (int, error) {
	if depth > maxStrictJSONDepth {
		return 0, errors.New("exceeded maximum JSON nesting depth")
	}
	if pos >= len(data) {
		return 0, io.ErrUnexpectedEOF
	}
	t := derefType(targetType)
	if t == jsonRawMessageType {
		t = nil
	}

	switch data[pos] {
	case '{':
		isEmptyInterface := t != nil && t.Kind() == reflect.Interface && t.NumMethod() == 0
		exactKeys := opts.ExactDuplicateKeysOnly || forceExactMapKeys || (t != nil && t.Kind() == reflect.Map) || isEmptyInterface
		pos = skipJSONWhitespace(data, pos+1)
		if pos >= len(data) {
			return 0, io.ErrUnexpectedEOF
		}
		if data[pos] == '}' {
			return pos + 1, nil
		}
		seen := make(map[string]string)
		for {
			if pos >= len(data) || data[pos] != '"' {
				return 0, fmt.Errorf("expected JSON object key string at byte %d", pos)
			}
			_, keyStr, afterKey, err := scanRawJSONString(data, pos, true)
			if err != nil {
				return 0, err
			}
			if exactKeys {
				if _, dup := seen[keyStr]; dup {
					return 0, fmt.Errorf("duplicate JSON map key %q", keyStr)
				}
				seen[keyStr] = keyStr
			} else {
				folded := simpleFoldKey(keyStr)
				if prev, dup := seen[folded]; dup && (prev == keyStr || strings.EqualFold(prev, keyStr)) {
					return 0, fmt.Errorf("duplicate JSON object key %q (conflicts with %q)", keyStr, prev)
				}
				seen[folded] = keyStr
			}

			pos = skipJSONWhitespace(data, afterKey)
			if pos >= len(data) || data[pos] != ':' {
				return 0, fmt.Errorf("expected ':' after JSON object key %q at byte %d", keyStr, pos)
			}
			pos = skipJSONWhitespace(data, pos+1)

			var childType reflect.Type
			childForceExact := false
			if t != nil {
				switch t.Kind() {
				case reflect.Map:
					childType = t.Elem()
				case reflect.Interface:
					if isEmptyInterface {
						childType = t
					}
				case reflect.Struct:
					childType = resolveJSONStructFieldType(t, keyStr)
				}
			}
			for _, f := range opts.ExactMapKeyFields {
				if f == keyStr || strings.EqualFold(f, keyStr) {
					childForceExact = true
					break
				}
			}

			afterVal, err := walkRawJSONValue(data, pos, childType, childForceExact, opts, depth+1)
			if err != nil {
				return 0, err
			}
			pos = skipJSONWhitespace(data, afterVal)
			if pos >= len(data) {
				return 0, io.ErrUnexpectedEOF
			}
			if data[pos] == '}' {
				return pos + 1, nil
			}
			if data[pos] != ',' {
				return 0, fmt.Errorf("expected ',' or '}' in JSON object at byte %d", pos)
			}
			pos = skipJSONWhitespace(data, pos+1)
		}

	case '[':
		var elemType reflect.Type
		if t != nil && (t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
			elemType = t.Elem()
		} else if t != nil && t.Kind() == reflect.Interface && t.NumMethod() == 0 {
			elemType = t
		}
		pos = skipJSONWhitespace(data, pos+1)
		if pos >= len(data) {
			return 0, io.ErrUnexpectedEOF
		}
		if data[pos] == ']' {
			return pos + 1, nil
		}
		for {
			afterElem, err := walkRawJSONValue(data, pos, elemType, false, opts, depth+1)
			if err != nil {
				return 0, err
			}
			pos = skipJSONWhitespace(data, afterElem)
			if pos >= len(data) {
				return 0, io.ErrUnexpectedEOF
			}
			if data[pos] == ']' {
				return pos + 1, nil
			}
			if data[pos] != ',' {
				return 0, fmt.Errorf("expected ',' or ']' in JSON array at byte %d", pos)
			}
			pos = skipJSONWhitespace(data, pos+1)
		}

	case '"':
		_, _, afterStr, err := scanRawJSONString(data, pos, false)
		return afterStr, err

	case 't':
		if bytes.HasPrefix(data[pos:], []byte("true")) {
			return pos + 4, nil
		}
		return 0, fmt.Errorf("invalid JSON literal at byte %d", pos)

	case 'f':
		if bytes.HasPrefix(data[pos:], []byte("false")) {
			return pos + 5, nil
		}
		return 0, fmt.Errorf("invalid JSON literal at byte %d", pos)

	case 'n':
		if bytes.HasPrefix(data[pos:], []byte("null")) {
			return pos + 4, nil
		}
		return 0, fmt.Errorf("invalid JSON literal at byte %d", pos)

	default:
		if data[pos] == '-' || (data[pos] >= '0' && data[pos] <= '9') {
			return scanRawJSONNumber(data, pos)
		}
		return 0, fmt.Errorf("unexpected character %q in JSON wire at byte %d", data[pos], pos)
	}
}

func skipJSONWhitespace(data []byte, pos int) int {
	for pos < len(data) {
		switch data[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func isASCIIHex(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func scanRawJSONString(data []byte, start int, unescape bool) ([]byte, string, int, error) {
	hasEscape := false
	i := start + 1
	for i < len(data) {
		c := data[i]
		if c == '"' {
			raw := data[start : i+1]
			if err := ValidateQualityJSONToken(raw); err != nil {
				return nil, "", 0, err
			}
			if !unescape {
				return raw, "", i + 1, nil
			}
			if !hasEscape {
				return raw, string(data[start+1 : i]), i + 1, nil
			}
			s, err := unescapeValidatedJSONString(data[start+1 : i])
			if err != nil {
				return nil, "", 0, err
			}
			return raw, s, i + 1, nil
		}
		if c < 0x20 {
			return nil, "", 0, fmt.Errorf("unescaped control character 0x%02x in JSON string at byte %d", c, i)
		}
		if c == '\\' {
			hasEscape = true
			i++
			if i >= len(data) {
				return nil, "", 0, io.ErrUnexpectedEOF
			}
			switch data[i] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i++
			case 'u':
				if i+4 >= len(data) || !isASCIIHex(data[i+1]) || !isASCIIHex(data[i+2]) || !isASCIIHex(data[i+3]) || !isASCIIHex(data[i+4]) {
					return nil, "", 0, fmt.Errorf("invalid \\uXXXX escape in JSON string at byte %d", i-1)
				}
				i += 5
			default:
				return nil, "", 0, fmt.Errorf("invalid escape \\%c in JSON string at byte %d", data[i], i-1)
			}
			continue
		}
		i++
	}
	return nil, "", 0, errors.New("unterminated JSON string")
}

func unescapeValidatedJSONString(body []byte) (string, error) {
	var b strings.Builder
	b.Grow(len(body))
	for i := 0; i < len(body); i++ {
		if body[i] != '\\' {
			b.WriteByte(body[i])
			continue
		}
		i++
		switch body[i] {
		case '"', '\\', '/':
			b.WriteByte(body[i])
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'u':
			u1 := qualityHexUnit(body[i+1 : i+5])
			i += 4
			if u1 >= 0xD800 && u1 <= 0xDBFF {
				// ValidateQualityJSONToken already verified the following \uDC00..\uDFFF low surrogate.
				u2 := qualityHexUnit(body[i+3 : i+7])
				i += 6
				b.WriteRune(utf16.DecodeRune(rune(u1), rune(u2)))
			} else {
				b.WriteRune(rune(u1))
			}
		}
	}
	return b.String(), nil
}

func scanRawJSONNumber(data []byte, start int) (int, error) {
	i := start
	if data[i] == '-' {
		i++
		if i >= len(data) {
			return 0, errors.New("incomplete negative JSON number")
		}
	}
	if data[i] == '0' {
		i++
	} else if data[i] >= '1' && data[i] <= '9' {
		i++
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	} else {
		return 0, fmt.Errorf("invalid JSON number at byte %d", start)
	}
	if i < len(data) && data[i] == '.' {
		i++
		if i >= len(data) || data[i] < '0' || data[i] > '9' {
			return 0, fmt.Errorf("invalid fraction in JSON number at byte %d", start)
		}
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	if i < len(data) && (data[i] == 'e' || data[i] == 'E') {
		i++
		if i < len(data) && (data[i] == '+' || data[i] == '-') {
			i++
		}
		if i >= len(data) || data[i] < '0' || data[i] > '9' {
			return 0, fmt.Errorf("invalid exponent in JSON number at byte %d", start)
		}
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			i++
		}
	}
	return i, nil
}

// simpleFoldKey maps each rune to the canonical minimum code point in its unicode.SimpleFold
// equivalence class, so simpleFoldKey(a) == simpleFoldKey(b) iff strings.EqualFold(a, b).
func simpleFoldKey(s string) string {
	needsFold := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf || (c >= 'a' && c <= 'z') {
			needsFold = true
			break
		}
	}
	if !needsFold {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			b.WriteByte(byte(r - ('a' - 'A')))
			continue
		}
		if r < utf8.RuneSelf {
			b.WriteByte(byte(r))
			continue
		}
		minR := r
		for next := unicode.SimpleFold(r); next != r && next > 0; next = unicode.SimpleFold(next) {
			if next < minR {
				minR = next
			}
		}
		b.WriteRune(minR)
	}
	return b.String()
}

type jsonStructFieldCandidate struct {
	name   string
	typ    reflect.Type
	tagged bool
}

// resolveJSONStructFieldType mirrors encoding/json's struct field resolution:
// 1. Traverse anonymous embedded structs breadth-first (including unexported embedded struct types).
// 2. Shallowest field of a given exact name shadows deeper fields; same-depth ties are resolved by single explicit tag or canceled.
// 3. Exact name match across all active fields takes precedence over first strings.EqualFold match.
func resolveJSONStructFieldType(structType reflect.Type, jsonKey string) reflect.Type {
	structType = derefType(structType)
	if structType == nil || structType.Kind() != reflect.Struct {
		return nil
	}
	visited := map[reflect.Type]bool{}
	queue := []reflect.Type{structType}
	resolvedNames := map[string]bool{}
	var active []jsonStructFieldCandidate

	for len(queue) > 0 {
		var nextQueue []reflect.Type
		nextCount := map[reflect.Type]int{}
		var levelFields []jsonStructFieldCandidate

		for _, current := range queue {
			visited[current] = true
			for i := 0; i < current.NumField(); i++ {
				f := current.Field(i)
				if f.Anonymous {
					ft := derefType(f.Type)
					if !f.IsExported() && (ft == nil || ft.Kind() != reflect.Struct) {
						continue
					}
				} else if !f.IsExported() {
					continue
				}
				tag := f.Tag.Get("json")
				if tag == "-" {
					continue
				}
				name, _, _ := strings.Cut(tag, ",")
				tagged := name != ""
				if f.Anonymous && !tagged {
					ft := derefType(f.Type)
					if ft != nil && ft.Kind() == reflect.Struct {
						if !visited[ft] {
							nextCount[ft]++
							if nextCount[ft] == 1 {
								nextQueue = append(nextQueue, ft)
							}
						}
						continue
					}
				}
				if name == "" {
					name = f.Name
				}
				levelFields = append(levelFields, jsonStructFieldCandidate{name: name, typ: f.Type, tagged: tagged})
			}
		}

		// Group same-depth candidates by exact name to apply encoding/json tie-breaking.
		byName := map[string][]jsonStructFieldCandidate{}
		var order []string
		for _, cand := range levelFields {
			if resolvedNames[cand.name] {
				continue
			}
			if _, seen := byName[cand.name]; !seen {
				order = append(order, cand.name)
			}
			byName[cand.name] = append(byName[cand.name], cand)
		}
		for _, name := range order {
			resolvedNames[name] = true
			group := byName[name]
			if len(group) == 1 {
				active = append(active, group[0])
				continue
			}
			var taggedMatches []jsonStructFieldCandidate
			for _, g := range group {
				if g.tagged {
					taggedMatches = append(taggedMatches, g)
				}
			}
			if len(taggedMatches) == 1 {
				active = append(active, taggedMatches[0])
			}
		}

		// Only advance to embedded types that were not ambiguously duplicated at this depth.
		filteredNext := nextQueue[:0]
		for _, nt := range nextQueue {
			if nextCount[nt] == 1 && !visited[nt] {
				filteredNext = append(filteredNext, nt)
			}
		}
		queue = filteredNext
	}

	for _, f := range active {
		if f.name == jsonKey {
			return f.typ
		}
	}
	for _, f := range active {
		if strings.EqualFold(f.name, jsonKey) {
			return f.typ
		}
	}
	return nil
}
