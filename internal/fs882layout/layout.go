// Package fs882layout extracts UI facts, not executable behavior or Access parity.
// Coordinates remain Access twips. All nodes are read-only until a consumer has
// independently verified the corresponding desktop workflow.
package fs882layout

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/boostao/vpro-wails/internal/accessinventory"
	"github.com/boostao/vpro-wails/internal/accesslayout"
)

type Value = accessinventory.Value
type Event = accessinventory.Event

type Layout struct {
	Root              string `json:"root"`
	GeometryUnit      string `json:"geometryUnit"`
	GeometryInference string `json:"geometryInference,omitempty"`
	CoordinateRule    string `json:"coordinateRule"`
	LayoutPolicy      string `json:"layoutPolicy"`
	Forms             []Form `json:"forms"`
}

type Form struct {
	Name             string     `json:"name"`
	Source           string     `json:"source"`
	SHA256           string     `json:"sha256"`
	RecordSource     string     `json:"recordSource,omitempty"`
	RecordSourceKind string     `json:"recordSourceKind,omitempty"`
	Pages            []Page     `json:"pages"`
	Controls         []Control  `json:"controls"`
	Fields           []Field    `json:"fields"`
	Embedded         []Embedded `json:"embedded"`
}

type Page struct {
	ControlID  string           `json:"controlId"`
	ParentID   string           `json:"parentId"`
	Name       string           `json:"name,omitempty"`
	Caption    string           `json:"caption,omitempty"`
	Line       int              `json:"line"`
	Properties map[string]Value `json:"properties"`
}

// Control is the complete instance tree, including static decorations, root,
// sections and anonymous groups. Structural attachment does not establish a
// coordinate origin: attached labels retain their raw exported coordinates.
type Control struct {
	ControlID   string           `json:"controlId"`
	ParentID    string           `json:"parentId,omitempty"`
	ControlName string           `json:"controlName,omitempty"`
	Type        string           `json:"type"`
	PageID      string           `json:"pageId,omitempty"`
	Line        int              `json:"line"`
	EndLine     int              `json:"endLine"`
	Properties  map[string]Value `json:"properties"`
	Events      []Event          `json:"events,omitempty"`
}

// Field includes labels and structural/grouping nodes as well as bound widgets.
// ControlID identifies an instance; Column is never used as a tree identity.
type Field struct {
	ControlID        string           `json:"controlId"`
	ParentID         string           `json:"parentId,omitempty"`
	ControlName      string           `json:"controlName,omitempty"`
	Column           string           `json:"column,omitempty"`
	Caption          string           `json:"caption,omitempty"`
	CaptionControlID string           `json:"captionControlId,omitempty"`
	Type             string           `json:"type"`
	PageID           string           `json:"pageId,omitempty"`
	Line             int              `json:"line"`
	EndLine          int              `json:"endLine"`
	Properties       map[string]Value `json:"properties"`
	Events           []Event          `json:"events,omitempty"`
	Binding          string           `json:"binding,omitempty"`
	BindingKind      string           `json:"bindingKind,omitempty"`
	Computed         string           `json:"computed,omitempty"`
	Widget           string           `json:"widget"`
	ReadOnly         bool             `json:"readOnly"`
	Implementation   string           `json:"implementation"`
}

// Embedded distinguishes the parent control instance from a referenced form
// definition. Unresolved/non-form sources stay explicit and are not navigable.
type Embedded struct {
	ControlID    string   `json:"controlId"`
	Form         string   `json:"form,omitempty"`
	SourceObject string   `json:"sourceObject"`
	MasterFields []string `json:"masterFields,omitempty"`
	ChildFields  []string `json:"childFields,omitempty"`
	Line         int      `json:"line"`
	Resolved     bool     `json:"resolved"`
	ReadOnly     bool     `json:"readOnly"`
}

var identifier = regexp.MustCompile(`^[\p{L}_][\p{L}\p{N}_]*$`)

func column(source string) string {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "[") && strings.HasSuffix(source, "]") {
		source = source[1 : len(source)-1]
	}
	if identifier.MatchString(source) {
		return source
	}
	return ""
}

// Build uses the inventory for closure, hashes and event references, and the
// layout tree for anonymous groups which the inventory deliberately flattens.
func Build(source, root string) (*Layout, error) {
	inventory, err := accessinventory.Build(source)
	if err != nil {
		return nil, err
	}
	reachable, err := inventory.ReachableForms(root)
	if err != nil {
		return nil, err
	}
	hashes := map[string]string{}
	for _, object := range inventory.Objects {
		hashes[object.Source] = object.SHA256
	}
	result := &Layout{Root: reachable[0].Name, GeometryUnit: "twips",
		CoordinateRule: "raw-export; coordinate spaces unverified; do not infer offsets from structural attachment",
		LayoutPolicy:   "preserve Access source positions; maximum fit scale 1, minimum fit scale 0.9, then horizontal scroll; no card regrouping or copied paper artwork",
		Forms:          []Form{}}
	if strings.EqualFold(result.Root, "FS882-6x4XL") {
		result.GeometryInference = "Observed FS882 parent export suggests form-absolute child positions: PlotNumber Left12510/Top720 minus Site Left135/Top450 gives x825/y18 CSS pixels; Width1887 gives right950.8 within Site Width14265 (951 pixels). This is an inference, not verified coordinate semantics; never add ancestor offsets, including attached labels"
	}
	for _, form := range reachable {
		data, err := os.ReadFile(filepath.Join(source, form.Source))
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != hashes[form.Source] {
			return nil, fmt.Errorf("%s: source changed during extraction", form.Source)
		}
		doc, err := accesslayout.Parse(data, form.Source)
		if err != nil {
			return nil, err
		}
		extracted, err := extract(doc, form, hashes[form.Source], inventory.Edges)
		if err != nil {
			return nil, err
		}
		result.Forms = append(result.Forms, extracted)
	}
	return result, nil
}

// Only UI facts are published: no SQL source, binary properties, VBA, or macro
// expressions. Missing defaults remain unknown, never invented boolean values.
var uiProperties = strings.Fields(`Name Caption ControlSource SourceObject
LinkMasterFields LinkChildFields Left Top Width Height TabIndex TabStop
Right Bottom InsideWidth InsideHeight GridX GridY
LayoutCachedLeft LayoutCachedTop LayoutCachedWidth LayoutCachedHeight
HorizontalSpacing VerticalSpacing LeftMargin TopMargin RightMargin BottomMargin
WebImagePaddingLeft WebImagePaddingTop WebImagePaddingRight WebImagePaddingBottom
HorizontalAnchor VerticalAnchor RowStart RowEnd ColumnStart ColumnEnd
ColumnWidth ColumnHidden ColumnOrder PageIndex Visible Enabled Locked
Format DecimalPlaces InputMask DefaultValue ValidationRule ValidationText
Required AllowZeroLength LimitToList BoundColumn ColumnCount ColumnWidths
ListRows ListWidth RowSourceType FontName FontSize FontWeight FontBold FontItalic
FontUnderline TextAlign BackColor ForeColor BorderColor BorderStyle
BorderWidth BackStyle SpecialEffect MultiSelect DisplayWhen
ScrollBars AutoHeight CanGrow CanShrink HideDuplicates OptionValue
TripleState LabelName ControlTipText StatusBarText`)

func extract(doc *accesslayout.Document, source accessinventory.Form, hash string, edges []accessinventory.Edge) (Form, error) {
	form := Form{Name: source.Name, Source: source.Source, SHA256: hash,
		Pages: []Page{}, Controls: []Control{}, Fields: []Field{}, Embedded: []Embedded{}}
	if source.RecordSource != "" {
		form.RecordSource = column(source.RecordSource)
		if form.RecordSource == "" {
			form.RecordSourceKind = "unsupported-expression-or-sql"
		} else {
			form.RecordSourceKind = "identifier"
		}
	}
	controls := map[int]accessinventory.Control{}
	for _, control := range source.Controls {
		controls[control.Line] = control
	}
	var root *accesslayout.Node
	for _, node := range doc.Root.Children {
		if strings.EqualFold(node.Type, source.Kind) {
			root = node
			break
		}
	}
	if root == nil {
		return form, fmt.Errorf("%s: missing definition", source.Source)
	}
	rootID := source.Kind + ":" + source.Name
	var walk func(*accesslayout.Node, string, string)
	walk = func(node *accesslayout.Node, parent, page string) {
		if node.Default {
			return
		}
		control, typed := controls[node.Line]
		id := control.ID
		typ := node.Type
		if !typed {
			id = fmt.Sprintf("%s/group@%d", rootID, node.Line)
			typ = "Group"
		}
		props := doc.EffectiveProperties(node)
		field := Field{ControlID: id, ParentID: parent, ControlName: node.Name(),
			Type: typ, PageID: page, Line: node.Line, EndLine: node.EndLine,
			Properties: map[string]Value{}, Widget: widget(typ), ReadOnly: true, Implementation: "unmapped"}
		if node == root {
			field.ControlName = source.Name
		}
		for _, key := range uiProperties {
			if property, ok := props[key]; ok && !property.Opaque {
				_, explicit := node.Properties[key]
				field.Properties[key] = Value{Value: property.Value, Line: property.Line, Inherited: !explicit}
			}
		}
		if property, ok := props["RecordSource"]; ok && !property.Opaque {
			_, explicit := node.Properties["RecordSource"]
			field.Properties["RecordSource"] = Value{Value: column(property.Value), Line: property.Line, Inherited: !explicit}
		}
		// Value-list rows are facts; query/table RowSource and SourceSQL are not.
		if strings.EqualFold(props["RowSourceType"].Value, "Value List") {
			if property, ok := props["RowSource"]; ok && !property.Opaque {
				_, explicit := node.Properties["RowSource"]
				field.Properties["RowSource"] = Value{Value: property.Value, Line: property.Line, Inherited: !explicit}
			}
		}
		field.Caption = props["Caption"].Value
		if binding := props["ControlSource"].Value; binding != "" {
			field.Binding = binding
			field.Column = column(binding)
			switch {
			case field.Column != "":
				field.BindingKind = "column"
			case strings.HasPrefix(strings.TrimSpace(binding), "="):
				field.BindingKind = "computed"
				field.Computed = binding
			default:
				field.BindingKind = "unsupported"
			}
		}
		for _, event := range control.Events {
			// Arbitrary macro/expression bodies are not executable UI metadata.
			if !strings.EqualFold(event.Binding, "[Event Procedure]") {
				event.Binding = "unsupported-expression-or-macro"
			}
			field.Events = append(field.Events, event)
		}
		if typ == "Page" {
			page = id
			field.PageID = id
			form.Pages = append(form.Pages, Page{ControlID: id, ParentID: parent, Name: node.Name(), Caption: field.Caption, Line: node.Line, Properties: field.Properties})
		}
		if property := props["SourceObject"]; property.Value != "" && !property.Opaque {
			embedded := Embedded{ControlID: id, SourceObject: property.Value, Line: property.Line, ReadOnly: true}
			for _, edge := range edges {
				if edge.From == id && edge.Kind == "embedded-object" {
					embedded.MasterFields, embedded.ChildFields = edge.MasterFields, edge.ChildFields
					if edge.Resolved && strings.HasPrefix(edge.To, "form:") {
						embedded.Form = strings.TrimPrefix(edge.To, "form:")
						embedded.Resolved = true
					}
					break
				}
			}
			form.Embedded = append(form.Embedded, embedded)
		}
		form.Fields = append(form.Fields, field)
		form.Controls = append(form.Controls, Control{ControlID: id, ParentID: parent, ControlName: field.ControlName,
			Type: typ, PageID: field.PageID, Line: node.Line, EndLine: node.EndLine, Properties: field.Properties, Events: field.Events})
		for _, child := range node.Children {
			walk(child, id, page)
		}
	}
	walk(root, "", "")
	byID := map[string]int{}
	for i, field := range form.Fields {
		byID[field.ControlID] = i
	}
	for _, label := range form.Fields {
		if label.Type != "Label" {
			continue
		}
		parent := label.ParentID
		for parent != "" {
			index, ok := byID[parent]
			if !ok {
				break
			}
			target := &form.Fields[index]
			if target.Type == "Group" {
				parent = target.ParentID
				continue
			}
			if target.Widget != "structure" && target.Type != "Label" && target.CaptionControlID == "" {
				target.Caption, target.CaptionControlID = label.Caption, label.ControlID
				if value, ok := label.Properties["Caption"]; ok {
					target.Properties["AttachedCaption"] = value
				}
			}
			break
		}
	}
	return form, nil
}

func widget(typ string) string {
	switch typ {
	case "TextBox":
		return "text"
	case "ComboBox", "ListBox":
		return "choice"
	case "CheckBox", "OptionButton", "ToggleButton":
		return "boolean-or-option"
	case "Label":
		return "label"
	case "Form", "Report", "Section", "Group", "Page", "Tab", "FormHeader", "FormFooter", "OptionGroup":
		return "structure"
	case "Subform":
		return "embedded"
	default:
		return "unsupported"
	}
}

// BindingColumns is an optional consumption-time alias projection, scoped to a
// single form. It preserves the first source spelling/order; Fields retains all
// instances, even when they repeat a column on different pages.
func (form Form) BindingColumns() []string {
	columns := []string{}
	seen := map[string]bool{}
	for _, field := range form.Fields {
		key := strings.ToLower(field.Column)
		if key != "" && !seen[key] {
			seen[key] = true
			columns = append(columns, field.Column)
		}
	}
	return columns
}
