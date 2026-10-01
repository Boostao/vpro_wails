// Package accesslayout reads Access SaveAsText layouts without flattening their
// anonymous containers, attached controls, or opaque binary properties.
package accesslayout

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

type Property struct {
	// Raw is the exact text after '=', including continuation lines and, for
	// opaque properties, the closing End. Value is decoded text, or Raw for blobs.
	Raw      string `json:"raw"`
	Value    string `json:"value"`
	Line     int    `json:"line"`
	Opaque   bool   `json:"opaque,omitempty"`
	DataType string `json:"dataType,omitempty"`
}

type Node struct {
	// Type is empty for an anonymous Begin container.
	Type       string              `json:"type"`
	Line       int                 `json:"line"`
	EndLine    int                 `json:"endLine"`
	Properties map[string]Property `json:"properties"`
	Children   []*Node             `json:"children,omitempty"`
	Default    bool                `json:"default,omitempty"`
	// Properties contains the last occurrence. DuplicateProperties retains all
	// occurrences in source order, including the first, under its original key.
	DuplicateProperties map[string][]Property `json:"duplicateProperties,omitempty"`
}

func (n *Node) Name() string {
	if n == nil {
		return ""
	}
	return n.Properties["Name"].Value
}

type Diagnostic struct {
	Line    int    `json:"line"`
	Message string `json:"message"`
}

type Document struct {
	Source      string       `json:"source"`
	Root        *Node        `json:"root"`
	Code        string       `json:"code,omitempty"`
	CodeLine    int          `json:"codeLine,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

var knownTypes = map[string]bool{
	"Form": true, "Report": true, "Section": true, "FormHeader": true,
	"FormFooter": true, "ReportHeader": true, "ReportFooter": true,
	"BreakLevel": true, "BreakHeader": true, "BreakFooter": true,
	"PageHeader": true, "PageFooter": true, "Label": true, "Rectangle": true,
	"Line": true, "CommandButton": true, "OptionButton": true, "CheckBox": true,
	"OptionGroup": true, "TextBox": true, "ListBox": true, "ComboBox": true,
	"Subform": true, "CustomControl": true, "ToggleButton": true, "Tab": true,
	"Page": true, "Image": true, "BoundObjectFrame": true,
	"UnboundObjectFrame": true, "EmptyCell": true,
	"InputTables": true, "OutputColumns": true, "Joins": true,
}

var defaultControlTypes = map[string]bool{
	"Label": true, "Rectangle": true, "Line": true, "CommandButton": true,
	"OptionButton": true, "CheckBox": true, "OptionGroup": true,
	"TextBox": true, "ListBox": true, "ComboBox": true, "Subform": true,
	"CustomControl": true, "ToggleButton": true, "Tab": true, "Image": true,
	"BoundObjectFrame": true, "UnboundObjectFrame": true,
}

var knownPropertyTypes = map[string]bool{
	"dbBoolean": true, "dbByte": true, "dbInteger": true, "dbLong": true,
	"dbCurrency": true, "dbSingle": true, "dbDouble": true, "dbDate": true,
	"dbBinary": true, "dbText": true, "dbLongBinary": true, "dbMemo": true,
	"dbGUID": true, "dbBigInt": true, "dbVarBinary": true, "dbChar": true,
	"dbNumeric": true, "dbDecimal": true, "dbFloat": true, "dbTime": true,
	"dbTimeStamp": true, "dbAttachment": true,
}

func ParseFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return Parse(data, path)
}

// DecodeText decodes UTF-8 or BOM-marked UTF-16 exports without interpreting
// their contents. Decode errors include the supplied source name.
func DecodeText(data []byte, source string) (string, error) {
	text, err := decode(data)
	if err != nil {
		return "", fmt.Errorf("%s: %w", source, err)
	}
	return text, nil
}

// DecodeWindows1252Text explicitly decodes legacy ANSI exports. It is never
// used as an automatic fallback by DecodeText or Parse. Undefined CP1252 bytes
// are rejected rather than replaced.
func DecodeWindows1252Text(data []byte, source string) (string, error) {
	c1 := [...]rune{
		'\u20ac', 0, '\u201a', '\u0192', '\u201e', '\u2026', '\u2020', '\u2021',
		'\u02c6', '\u2030', '\u0160', '\u2039', '\u0152', 0, '\u017d', 0,
		0, '\u2018', '\u2019', '\u201c', '\u201d', '\u2022', '\u2013', '\u2014',
		'\u02dc', '\u2122', '\u0161', '\u203a', '\u0153', 0, '\u017e', '\u0178',
	}
	var text strings.Builder
	text.Grow(len(data))
	for i, b := range data {
		r := rune(b)
		if b >= 0x80 && b <= 0x9f {
			r = c1[b-0x80]
			if r == 0 {
				return "", fmt.Errorf("%s: undefined Windows-1252 byte 0x%02X at byte offset %d", source, b, i)
			}
		}
		text.WriteRune(r)
	}
	return text.String(), nil
}

// Parse accepts UTF-8 (with or without BOM) and BOM-marked UTF-16. Syntax errors
// are fatal; unfamiliar but syntactically valid node types are retained and
// reported in Diagnostics. VBA after CodeBehindForm is preserved verbatim.
func Parse(data []byte, source string) (*Document, error) {
	text, err := DecodeText(data, source)
	if err != nil {
		return nil, err
	}
	lines := splitLines(text)
	d := &Document{
		Source: source,
		Root:   &Node{Type: "Document", Line: 1, EndLine: len(lines), Properties: map[string]Property{}},
	}
	stack := []*Node{d.Root}
	fail := func(line int, format string, args ...any) (*Document, error) {
		return nil, fmt.Errorf("%s:%d: %s", source, line, fmt.Sprintf(format, args...))
	}
	for i := 0; i < len(lines); i++ {
		s := strings.TrimSpace(lines[i].text)
		if s == "" {
			continue
		}
		n := stack[len(stack)-1]
		switch {
		case s == "CodeBehindForm":
			if len(stack) != 1 {
				return fail(i+1, "CodeBehindForm inside unclosed %q block", n.Type)
			}
			d.CodeLine = i + 2
			d.Code = text[lines[i].end:]
			if err := markDefaults(d.Root); err != nil {
				return fail(i+1, "%s", err)
			}
			return d, nil
		case s == "End":
			if len(stack) == 1 {
				return fail(i+1, "unexpected End")
			}
			n.EndLine = i + 1
			stack = stack[:len(stack)-1]
		case s == "Begin" || strings.HasPrefix(s, "Begin "):
			typ := strings.TrimSpace(strings.TrimPrefix(s, "Begin"))
			if typ != "" && !identifier(typ) {
				return fail(i+1, "invalid block type %q", typ)
			}
			child := &Node{Type: typ, Line: i + 1, Properties: map[string]Property{}}
			n.Children = append(n.Children, child)
			stack = append(stack, child)
			if typ != "" && !knownTypes[typ] {
				d.Diagnostics = append(d.Diagnostics, Diagnostic{i + 1, "unrecognized node type " + typ})
			}
		default:
			eq := assignmentIndex(lines[i].text)
			if eq < 0 {
				return fail(i+1, "unrecognized construct %q", s)
			}
			key, dataType, err := propertyName(strings.TrimSpace(lines[i].text[:eq]))
			if err != nil {
				return fail(i+1, "%s", err)
			}
			if dataType != "" && !knownPropertyTypes[dataType] {
				d.Diagnostics = append(d.Diagnostics, Diagnostic{i + 1, "unrecognized property type " + dataType})
			}
			duplicateKey := ""
			for existing := range n.Properties {
				if strings.EqualFold(existing, key) {
					duplicateKey = existing
					break
				}
			}
			start := i
			rawStart := lines[i].start + eq + 1
			value := strings.TrimSpace(lines[i].text[eq+1:])
			p := Property{Line: i + 1, DataType: dataType}
			if value == "Begin" {
				depth := 1
				for i++; i < len(lines); i++ {
					t := strings.TrimSpace(lines[i].text)
					if t == "Begin" || strings.HasPrefix(t, "Begin ") || propertyBegin(t) {
						depth++
					} else if t == "End" {
						depth--
						if depth == 0 {
							break
						}
					}
				}
				if depth != 0 {
					return fail(start+1, "unclosed opaque property %q", key)
				}
				p.Raw = text[rawStart:lines[i].contentEnd]
				p.Value, p.Opaque = p.Raw, true
			} else if strings.HasPrefix(value, "\"") {
				var decoded strings.Builder
				for {
					v, err := quoted(value)
					if err != nil {
						return fail(i+1, "property %q: %s", key, err)
					}
					decoded.WriteString(v)
					if i+1 >= len(lines) || !strings.HasPrefix(strings.TrimSpace(lines[i+1].text), "\"") {
						break
					}
					i++
					value = strings.TrimSpace(lines[i].text)
				}
				p.Raw = text[rawStart:lines[i].contentEnd]
				p.Value = decoded.String()
			} else {
				if value == "" {
					return fail(i+1, "empty unquoted property %q", key)
				}
				p.Raw = text[rawStart:lines[i].contentEnd]
				p.Value = value
			}
			if duplicateKey != "" {
				previous := n.Properties[duplicateKey]
				if n.DuplicateProperties == nil {
					n.DuplicateProperties = map[string][]Property{}
				}
				if len(n.DuplicateProperties[duplicateKey]) == 0 {
					n.DuplicateProperties[duplicateKey] = []Property{previous}
				}
				n.DuplicateProperties[duplicateKey] = append(n.DuplicateProperties[duplicateKey], p)
				d.Diagnostics = append(d.Diagnostics, Diagnostic{p.Line,
					fmt.Sprintf("duplicate property %q; previous occurrence at line %d; last value used", key, previous.Line)})
				key = duplicateKey
			}
			n.Properties[key] = p
		}
	}
	if len(stack) != 1 {
		n := stack[len(stack)-1]
		return fail(n.Line, "unclosed %q block", n.Type)
	}
	if err := markDefaults(d.Root); err != nil {
		return fail(len(lines), "%s", err)
	}
	return d, nil
}

func propertyBegin(s string) bool {
	eq := assignmentIndex(s)
	return eq >= 0 && strings.TrimSpace(s[eq+1:]) == "Begin"
}

func propertyName(s string) (string, string, error) {
	if identifier(s) {
		return s, "", nil
	}
	space := strings.IndexAny(s, " \t")
	if space > 0 {
		dataType := s[:space]
		name := strings.TrimSpace(s[space:])
		if strings.HasPrefix(dataType, "db") && identifier(dataType) && strings.HasPrefix(name, "\"") {
			value, err := quoted(name)
			if err == nil && value != "" {
				return value, dataType, nil
			}
		}
	}
	return "", "", fmt.Errorf("invalid property name %q", s)
}

// Typed query property names are quoted and may themselves contain '='.
func assignmentIndex(s string) int {
	inQuote := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if inQuote && i+1 < len(s) && (s[i+1] == '\\' || s[i+1] == '"') {
				i++
			}
		case '"':
			if inQuote && i+1 < len(s) && s[i+1] == '"' {
				i++
			} else {
				inQuote = !inQuote
			}
		case '=':
			if !inQuote {
				return i
			}
		}
	}
	return -1
}

func identifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return true
}

// SaveAsText uses both doubled quotes and backslash escapes, including octal
// character escapes. Unrecognized escapes retain their backslash (e.g. paths).
func quoted(s string) (string, error) {
	var b strings.Builder
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '"':
			if i+1 < len(s) && s[i+1] == '"' {
				b.WriteByte('"')
				i++
				continue
			}
			if strings.TrimSpace(s[i+1:]) != "" {
				return "", fmt.Errorf("unexpected text after quoted value")
			}
			return b.String(), nil
		case '\\':
			if i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\') {
				i++
				b.WriteByte(s[i])
			} else if i+3 < len(s) && octal(s[i+1]) && octal(s[i+2]) && octal(s[i+3]) {
				v := int(s[i+1]-'0')*64 + int(s[i+2]-'0')*8 + int(s[i+3]-'0')
				b.WriteRune(rune(v))
				i += 3
			} else {
				b.WriteByte('\\')
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return "", fmt.Errorf("unterminated quoted value")
}

func octal(b byte) bool { return b >= '0' && b <= '7' }

type sourceLine struct {
	text                   string
	start, contentEnd, end int
}

func splitLines(text string) []sourceLine {
	var lines []sourceLine
	for start := 0; start < len(text); {
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += start + 1
		}
		contentEnd := end
		if contentEnd > start && text[contentEnd-1] == '\n' {
			contentEnd--
		}
		if contentEnd > start && text[contentEnd-1] == '\r' {
			contentEnd--
		}
		lines = append(lines, sourceLine{text[start:contentEnd], start, contentEnd, end})
		start = end
	}
	return lines
}

func decode(data []byte) (string, error) {
	if len(data) >= 2 && ((data[0] == 0xff && data[1] == 0xfe) || (data[0] == 0xfe && data[1] == 0xff)) {
		var order binary.ByteOrder = binary.LittleEndian
		if data[0] == 0xfe {
			order = binary.BigEndian
		}
		data = data[2:]
		if len(data)%2 != 0 {
			return "", fmt.Errorf("odd-length UTF-16 input")
		}
		units := make([]uint16, len(data)/2)
		for i := range units {
			units[i] = order.Uint16(data[2*i:])
		}
		for i := 0; i < len(units); i++ {
			u := units[i]
			if u >= 0xd800 && u <= 0xdbff {
				if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
					return "", fmt.Errorf("invalid UTF-16 surrogate at code unit %d", i)
				}
				i++
			} else if u >= 0xdc00 && u <= 0xdfff {
				return "", fmt.Errorf("unpaired UTF-16 surrogate at code unit %d", i)
			}
		}
		return string(utf16.Decode(units)), nil
	}
	data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(data) {
		return "", fmt.Errorf("invalid UTF-8 input (UTF-16 requires a BOM)")
	}
	return string(data), nil
}

// Defaults are nameless control blocks directly at the form/report's top level,
// either directly underneath it or in its immediate anonymous collection.
// Structural nodes (including repeated report BreakLevel groups) and deeper
// nameless nodes are not defaults.
func formDefinitions(form *Node) []*Node {
	var definitions []*Node
	var collect func(*Node)
	collect = func(n *Node) {
		if n.Type == "" {
			for _, child := range n.Children {
				if child.Type != "" {
					collect(child)
				}
			}
		} else if defaultControlTypes[n.Type] {
			if _, named := n.Properties["Name"]; !named {
				definitions = append(definitions, n)
			}
		}
	}
	for _, child := range form.Children {
		collect(child)
	}
	return definitions
}

func markDefaults(root *Node) error {
	if root.Type == "Form" || root.Type == "Report" {
		seen := map[string]*Node{}
		for _, n := range formDefinitions(root) {
			if previous := seen[n.Type]; previous != nil {
				return fmt.Errorf("duplicate %s defaults at lines %d and %d", n.Type, previous.Line, n.Line)
			}
			seen[n.Type] = n
			n.Default = true
		}
	}
	for _, child := range root.Children {
		if err := markDefaults(child); err != nil {
			return err
		}
	}
	return nil
}

// EffectiveProperties returns an independent map of the enclosing form's type
// defaults followed by the node's overrides. It never inherits parent control
// properties. Nodes not belonging to the document receive only their own values.
func (d *Document) EffectiveProperties(n *Node) map[string]Property {
	result := map[string]Property{}
	if n == nil {
		return result
	}
	var form *Node
	var find func(*Node, *Node) bool
	find = func(current, enclosing *Node) bool {
		if current.Type == "Form" || current.Type == "Report" {
			enclosing = current
		}
		if current == n {
			form = enclosing
			return true
		}
		for _, child := range current.Children {
			if find(child, enclosing) {
				return true
			}
		}
		return false
	}
	if d != nil && d.Root != nil {
		find(d.Root, nil)
	}
	if form != nil && form != n {
		for _, def := range formDefinitions(form) {
			if def.Type == n.Type {
				for key, p := range def.Properties {
					result[key] = p
				}
			}
		}
	}
	for key, p := range n.Properties {
		result[key] = p
	}
	return result
}
