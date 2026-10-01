package accessinventory

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/boostao/vpro-wails/internal/accesslayout"
)

type Inventory struct {
	Version     int          `json:"version"`
	Objects     []Object     `json:"objects"`
	Forms       []Form       `json:"forms"`
	Edges       []Edge       `json:"edges"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Object struct {
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Source     string      `json:"source"`
	SHA256     string      `json:"sha256"`
	Procedures []Procedure `json:"procedures,omitempty"`
}

type Procedure struct {
	Name string `json:"name"`
	Line int    `json:"line"`
}

type Form struct {
	Name         string    `json:"name"`
	Kind         string    `json:"kind"`
	Source       string    `json:"source"`
	RecordSource string    `json:"recordSource,omitempty"`
	Controls     []Control `json:"controls"`
}

type Control struct {
	ID             string           `json:"id"`
	ParentID       string           `json:"parentId"`
	Name           string           `json:"name"`
	Type           string           `json:"type"`
	Line           int              `json:"line"`
	EndLine        int              `json:"endLine"`
	Properties     map[string]Value `json:"properties"`
	Events         []Event          `json:"events,omitempty"`
	Implementation string           `json:"implementation"`
}

type Value struct {
	Value     string `json:"value,omitempty"`
	Line      int    `json:"line"`
	Inherited bool   `json:"inherited,omitempty"`
	Opaque    bool   `json:"opaque,omitempty"`
	RawBytes  int    `json:"rawBytes,omitempty"`
}

type Event struct {
	Property  string `json:"property"`
	Binding   string `json:"binding"`
	Procedure string `json:"procedure,omitempty"`
	Line      int    `json:"line"`
	Resolved  bool   `json:"resolved"`
}

type Edge struct {
	From         string   `json:"from"`
	To           string   `json:"to"`
	TargetKind   string   `json:"targetKind,omitempty"`
	Kind         string   `json:"kind"`
	Source       string   `json:"source"`
	Line         int      `json:"line"`
	MasterFields []string `json:"masterFields,omitempty"`
	ChildFields  []string `json:"childFields,omitempty"`
	Resolved     bool     `json:"resolved"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Source  string `json:"source"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

var procedurePattern = regexp.MustCompile(`(?im)^\s*(?:(?:Public|Private|Friend|Static)\s+)*(?:Sub|Function|Property\s+(?:Get|Let|Set))\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\(|$)`)

// Build inventories source objects without executing VBA or changing Access files.
func Build(root string) (*Inventory, error) {
	result := &Inventory{Version: 1, Objects: []Object{}, Forms: []Form{}, Edges: []Edge{}, Diagnostics: []Diagnostic{}}
	symbols := make(map[string]string)
	for _, folder := range []struct{ name, kind string }{
		{"Forms", "form"}, {"Reports", "report"}, {"Queries", "query"}, {"Modules", "module"}, {"Macros", "macro"}, {"Tables_Def", "table"},
	} {
		dir := filepath.Join(root, folder.name)
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) && folder.kind != "form" {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "missing-export-folder", Source: folder.name, Message: "No exports supplied; dependencies cannot be fully resolved"})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".txt") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", path, err)
			}
			name := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
			if folder.kind == "table" {
				name = strings.TrimSuffix(name, "_CreateSQL")
			}
			source := filepath.Join(folder.name, entry.Name())
			hash := sha256.Sum256(data)
			object := Object{Kind: folder.kind, Name: name, Source: source, SHA256: hex.EncodeToString(hash[:])}
			key := objectID(folder.kind, name)
			if _, exists := symbols[strings.ToLower(key)]; exists {
				return nil, fmt.Errorf("%s: duplicate object identity %s", source, key)
			}
			symbols[strings.ToLower(key)] = key
			if folder.kind == "form" || folder.kind == "report" || folder.kind == "query" {
				doc, err := accesslayout.Parse(data, source)
				if err != nil {
					return nil, err
				}
				for _, diagnostic := range doc.Diagnostics {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "layout-parser", Source: source, Line: diagnostic.Line, Message: diagnostic.Message})
				}
				object.Procedures = procedures(doc.Code, doc.CodeLine)
				switch folder.kind {
				case "form", "report":
					form, err := extractForm(doc, object, result)
					if err != nil {
						return nil, err
					}
					result.Forms = append(result.Forms, form)
				case "query":
					if sql, ok := doc.Root.Properties["SQL"]; ok {
						for _, target := range SQLReferences(sql.Value) {
							result.Edges = append(result.Edges, Edge{From: key, To: target, Kind: "query-source", Source: source, Line: sql.Line})
						}
					} else {
						result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "missing-query-sql", Source: source, Message: "Query export has no SQL property"})
					}
					if connection := doc.Root.Properties["Connect"]; connection.Value != "" {
						result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "external-query-connection", Source: source, Line: connection.Line, Message: "Query uses an external connection; extraction does not verify or copy that database"})
					}
				}
			}
			if folder.kind == "module" {
				var code string
				var err error
				hasUnicodeBOM := bytes.HasPrefix(data, []byte{0xff, 0xfe}) || bytes.HasPrefix(data, []byte{0xfe, 0xff})
				if hasUnicodeBOM || utf8.Valid(data) {
					code, err = accesslayout.DecodeText(data, source)
				} else {
					code, err = accesslayout.DecodeWindows1252Text(data, source)
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "legacy-module-encoding", Source: source, Line: 1, Message: "BOM-less non-UTF8 VBA export decoded explicitly as Windows-1252 (verified Windows Access export code page); other code pages require separate verification"})
				}
				if err != nil {
					return nil, err
				}
				object.Procedures = procedures(code, 1)
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "module-requires-semantic-validation", Source: source, Line: 1, Message: "Procedure declarations are inventoried; VBA calls, globals, and runtime SQL are not interpreted"})
			}
			if folder.kind == "macro" {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "macro-requires-semantic-validation", Source: source, Line: 1, Message: "Macro source is hashed and inventoried, not evaluated"})
			}
			result.Objects = append(result.Objects, object)
		}
	}
	if len(result.Forms) == 0 {
		return nil, fmt.Errorf("%s: no form exports found", root)
	}
	resolve(result, symbols)
	return result, nil
}

func objectID(kind, name string) string { return kind + ":" + name }

func extractForm(doc *accesslayout.Document, object Object, result *Inventory) (Form, error) {
	form := Form{Name: object.Name, Kind: object.Kind, Source: object.Source, Controls: []Control{}}
	var root *accesslayout.Node
	for _, node := range doc.Root.Children {
		if strings.EqualFold(node.Type, object.Kind) {
			if root != nil {
				return form, fmt.Errorf("%s: multiple root %s definitions", object.Source, object.Kind)
			}
			root = node
		}
	}
	if root == nil {
		return form, fmt.Errorf("%s: missing root %s", object.Source, object.Kind)
	}
	rootID := objectID(object.Kind, object.Name)
	seen := make(map[string]bool)
	var visit func(*accesslayout.Node, string) error
	visit = func(node *accesslayout.Node, parent string) error {
		name := node.Name()
		if node.Default {
			return nil
		}
		current := parent
		if node == root || node.Type != "" {
			id := rootID
			eventOwner := "Form"
			if object.Kind == "report" {
				eventOwner = "Report"
			}
			if node != root {
				if name == "" {
					id += fmt.Sprintf("/%s@%d", node.Type, node.Line)
				} else {
					id += "/" + name
					eventOwner = name
					if seen[strings.ToLower(name)] {
						return fmt.Errorf("%s:%d: duplicate control name %q", object.Source, node.Line, name)
					}
					seen[strings.ToLower(name)] = true
				}
			}
			props := doc.EffectiveProperties(node)
			control := Control{ID: id, ParentID: parent, Name: name, Type: node.Type, Line: node.Line, EndLine: node.EndLine,
				Properties: make(map[string]Value), Implementation: "unmapped"}
			if node == root {
				control.Name = object.Name
				if property, ok := props["RecordSource"]; ok {
					form.RecordSource = property.Value
					addSourceEdges(result, id, "record-source", object.Source, property)
				}
			}
			keys := make([]string, 0, len(props))
			for key := range props {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, key := range keys {
				property := props[key]
				value := Value{Value: property.Value, Line: property.Line, Opaque: property.Opaque}
				_, explicit := node.Properties[key]
				value.Inherited = !explicit
				if property.Opaque {
					value.Value = ""
					value.RawBytes = len(property.Raw)
				}
				control.Properties[key] = value
				if isEvent(key) {
					event := Event{Property: key, Binding: property.Value, Line: property.Line}
					if strings.EqualFold(property.Value, "[Event Procedure]") {
						event.Procedure = eventOwner + "_" + strings.TrimPrefix(key, "On")
						for _, procedure := range object.Procedures {
							if strings.EqualFold(procedure.Name, event.Procedure) {
								event.Resolved = true
								break
							}
						}
						if !event.Resolved {
							result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "unresolved-event", Source: object.Source, Line: property.Line, Message: id + ": " + event.Procedure + " not found in code-behind"})
						}
					} else {
						result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "event-expression-or-macro", Source: object.Source, Line: property.Line, Message: id + ": " + key + " requires expression/macro interpretation"})
					}
					control.Events = append(control.Events, event)
				}
			}
			if property, ok := props["RowSource"]; ok && !strings.EqualFold(props["RowSourceType"].Value, "Value List") {
				addSourceEdges(result, id, "row-source", object.Source, property)
			}
			if property, ok := props["SourceObject"]; ok && property.Value != "" {
				target := property.Value
				targetKind := ""
				for _, kind := range []string{"form", "report", "table", "query"} {
					prefix := kind + "."
					if strings.HasPrefix(strings.ToLower(target), prefix) {
						target, targetKind = target[len(prefix):], kind
						break
					}
				}
				edge := Edge{From: id, To: target, Kind: "embedded-object", Source: object.Source, Line: property.Line,
					TargetKind: targetKind, MasterFields: linkFields(props["LinkMasterFields"].Value), ChildFields: linkFields(props["LinkChildFields"].Value)}
				result.Edges = append(result.Edges, edge)
				if len(edge.MasterFields) != len(edge.ChildFields) {
					result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "mismatched-subform-links", Source: object.Source, Line: property.Line, Message: id + ": unequal master/child field counts"})
				}
			}
			switch node.Type {
			case "CustomControl", "BoundObjectFrame", "UnboundObjectFrame":
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "requires-native-replacement", Source: object.Source, Line: node.Line, Message: id + ": " + node.Type + " cannot be assumed to have a web equivalent"})
			}
			form.Controls = append(form.Controls, control)
			current = id
		}
		for _, child := range node.Children {
			if err := visit(child, current); err != nil {
				return err
			}
		}
		return nil
	}
	if err := visit(root, ""); err != nil {
		return form, err
	}
	return form, nil
}

func linkFields(value string) []string {
	var fields []string
	for _, field := range strings.Split(value, ";") {
		if name := strings.Trim(strings.TrimSpace(field), "[]"); name != "" {
			fields = append(fields, name)
		}
	}
	return fields
}

func addSourceEdges(result *Inventory, from, kind, source string, property accesslayout.Property) {
	value := strings.TrimSpace(property.Value)
	if value == "" {
		return
	}
	if strings.HasPrefix(value, "=") {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "dynamic-source", Source: source, Line: property.Line, Message: from + ": " + kind + " must be resolved at runtime"})
		return
	}
	if isSQL(value) {
		for _, target := range SQLReferences(value) {
			result.Edges = append(result.Edges, Edge{From: from, To: target, Kind: kind, Source: source, Line: property.Line})
		}
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "sql-requires-semantic-validation", Source: source, Line: property.Line, Message: from + ": lexical source dependencies do not validate SQL behavior"})
		return
	}
	result.Edges = append(result.Edges, Edge{From: from, To: strings.Trim(value, "[]"), Kind: kind, Source: source, Line: property.Line})
}

func isSQL(value string) bool {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToUpper(fields[0]) {
	case "SELECT", "PARAMETERS", "TRANSFORM", "INSERT", "UPDATE", "DELETE":
		return true
	}
	return false
}

func isEvent(key string) bool {
	if strings.HasPrefix(key, "On") && len(key) > 2 && key[2] >= 'A' && key[2] <= 'Z' {
		return true
	}
	switch key {
	case "BeforeUpdate", "AfterUpdate", "BeforeInsert", "AfterInsert", "BeforeDelConfirm", "AfterDelConfirm":
		return true
	}
	return false
}

func resolve(result *Inventory, symbols map[string]string) {
	for index := range result.Edges {
		edge := &result.Edges[index]
		kinds := []string{"query", "table"}
		if edge.Kind == "embedded-object" {
			kinds = []string{"form", "report", "query", "table"}
			if edge.TargetKind != "" {
				kinds = []string{edge.TargetKind}
			}
		}
		for _, kind := range kinds {
			if id, ok := symbols[strings.ToLower(objectID(kind, edge.To))]; ok {
				edge.To, edge.Resolved = id, true
				break
			}
		}
		if !edge.Resolved {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: "unresolved-source", Source: edge.Source, Line: edge.Line, Message: edge.From + ": " + edge.To + " may be missing, linked, or replaced by VBA"})
		}
	}
}

func procedures(code string, startLine int) []Procedure {
	result := []Procedure{}
	for _, match := range procedurePattern.FindAllStringSubmatchIndex(code, -1) {
		name := code[match[2]:match[3]]
		// The declaration regex can consume leading newlines; anchor to the name.
		line := startLine + strings.Count(code[:match[2]], "\n")
		result = append(result, Procedure{Name: name, Line: line})
	}
	return result
}

// ReachableForms follows subform instances, while retaining each form definition once.
func (inventory *Inventory) ReachableForms(name string) ([]Form, error) {
	byID := make(map[string]Form)
	for _, form := range inventory.Forms {
		byID[strings.ToLower(objectID(form.Kind, form.Name))] = form
	}
	id := objectID("form", name)
	if _, ok := byID[strings.ToLower(id)]; !ok {
		return nil, fmt.Errorf("form %q not found", name)
	}
	var result []Form
	seen := make(map[string]bool)
	var visit func(string)
	visit = func(id string) {
		key := strings.ToLower(id)
		if seen[key] {
			return
		}
		seen[key] = true
		form, ok := byID[key]
		if !ok {
			return
		}
		result = append(result, form)
		for _, edge := range inventory.Edges {
			if edge.Resolved && edge.Kind == "embedded-object" && strings.HasPrefix(strings.ToLower(edge.From), key+"/") {
				visit(edge.To)
			}
		}
	}
	visit(id)
	return result, nil
}

// TwipsToCSSPixels provides reference geometry; it is not a responsive layout policy.
func TwipsToCSSPixels(value string) (float64, error) {
	twips, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid twip dimension %q: %w", value, err)
	}
	return twips / 15, nil
}
