package accesslayout

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

func mustParse(t *testing.T, text string) *Document {
	t.Helper()
	d, err := Parse([]byte(text), "fixture.txt")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestContainmentDefaultsAndLocations(t *testing.T) {
	d := mustParse(t, `Version =21
Begin Form
    Caption ="Example"
    Begin
        Begin TextBox
            FontSize =10
            Width =100
        End
        Begin Label
            FontSize =8
        End
        Begin Section
            Name ="Detail"
            Begin
                Begin TextBox
                    Name ="Input"
                    Width =200
                    Begin
                        Begin Label
                            Name ="Attached"
                            Caption ="Input label"
                        End
                    End
                End
                Begin EmptyCell
                End
            End
        End
    End
End
`)
	if d.Root.Type != "Document" || d.Root.Properties["Version"].Value != "21" {
		t.Fatalf("root: %#v", d.Root)
	}
	form := d.Root.Children[0]
	container := form.Children[0]
	def, section := container.Children[0], container.Children[2]
	input := section.Children[0].Children[0]
	label := input.Children[0].Children[0]
	cell := section.Children[0].Children[1]
	if container.Type != "" || def.Name() != "" || !def.Default || !container.Children[1].Default {
		t.Fatalf("default/container classification: %#v", container)
	}
	if input.Name() != "Input" || label.Name() != "Attached" || cell.Default || section.Default || input.Default {
		t.Fatal("instance hierarchy or classification lost")
	}
	if form.Line != 2 || form.EndLine != 30 || input.Line != 15 || input.EndLine != 24 || label.Line != 19 || label.EndLine != 22 {
		t.Fatalf("locations form=%d..%d input=%d..%d label=%d..%d", form.Line, form.EndLine, input.Line, input.EndLine, label.Line, label.EndLine)
	}
	props := d.EffectiveProperties(input)
	if props["FontSize"].Value != "10" || props["FontSize"].Line != 6 || props["Width"].Value != "200" || props["Width"].Line != 17 {
		t.Fatalf("effective properties: %#v", props)
	}
	if _, ok := props["Caption"]; ok {
		t.Fatal("inherited form caption")
	}
	labelProps := d.EffectiveProperties(label)
	if labelProps["FontSize"].Value != "8" {
		t.Fatal("attached label did not inherit form-local label defaults")
	}
	delete(props, "Width")
	props["FontSize"] = Property{Value: "99"}
	if input.Properties["Width"].Value != "200" || def.Properties["FontSize"].Value != "10" {
		t.Fatal("effective map aliases original maps")
	}
}

func TestRawMultilineEscapesAndVBA(t *testing.T) {
	text := "Version =21\r\nBegin Form\r\n" +
		"    Caption = \"Say \"\"hi\"\" \\\"there\\\"\"\r\n" +
		"        \"\\015\\012C:\\Projects\\\\\"\r\n" +
		"    Data = Begin\r\n        0xdeadbeef ,\r\n        Begin StrangePayload\r\n" +
		"            Thing = Begin\r\n                unknown binary contents\r\n            End\r\n        End\r\n    End\r\n" +
		"End\r\nCodeBehindForm\r\nOption Explicit\r\n' Begin Form\r\nPrivate Sub X()\r\nEnd Sub\r\n"
	d := mustParse(t, text)
	form := d.Root.Children[0]
	p := form.Properties["Caption"]
	if p.Value != "Say \"hi\" \"there\"\r\nC:\\Projects\\" {
		t.Fatalf("decoded value: %q", p.Value)
	}
	wantRaw := " \"Say \"\"hi\"\" \\\"there\\\"\"\r\n        \"\\015\\012C:\\Projects\\\\\""
	if p.Raw != wantRaw || p.Line != 3 || p.Opaque {
		t.Fatalf("quoted property: %#v", p)
	}
	blob := form.Properties["Data"]
	if !blob.Opaque || blob.Line != 5 || blob.Value != blob.Raw || !strings.HasSuffix(blob.Raw, "    End") || !strings.Contains(blob.Raw, "unknown binary contents") {
		t.Fatalf("opaque property: %#v", blob)
	}
	if len(form.Children) != 0 {
		t.Fatal("opaque contents parsed as layout nodes")
	}
	if d.Code != "Option Explicit\r\n' Begin Form\r\nPrivate Sub X()\r\nEnd Sub\r\n" || d.CodeLine != 15 || form.EndLine != 13 {
		t.Fatalf("code line %d, end line %d, code %q", d.CodeLine, form.EndLine, d.Code)
	}
}

func TestEncodings(t *testing.T) {
	text := "Begin Form\nCaption =\"Été 🌲\"\nEnd\n"
	units := utf16.Encode([]rune(text))
	for _, tc := range []struct {
		name  string
		bom   []byte
		order binary.ByteOrder
	}{
		{"utf16le", []byte{0xff, 0xfe}, binary.LittleEndian},
		{"utf16be", []byte{0xfe, 0xff}, binary.BigEndian},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := append([]byte{}, tc.bom...)
			for _, u := range units {
				b := make([]byte, 2)
				tc.order.PutUint16(b, u)
				data = append(data, b...)
			}
			d, err := Parse(data, tc.name)
			if err != nil || d.Root.Children[0].Properties["Caption"].Value != "Été 🌲" {
				t.Fatalf("decode: %v, %#v", err, d)
			}
		})
	}
	for _, data := range [][]byte{[]byte(text), append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...)} {
		d, err := Parse(data, "utf8")
		if err != nil || d.Root.Children[0].Properties["Caption"].Value != "Été 🌲" {
			t.Fatalf("UTF-8 decode: %v", err)
		}
	}
	for _, data := range [][]byte{{0xff}, {0xff, 0xfe, 0}, {0xff, 0xfe, 0, 0xd8}, {0xff, 0xfe, 0, 0xdc}, {0xff, 0xfe, 0, 0xd8, 65, 0}} {
		if _, err := Parse(data, "bad"); err == nil {
			t.Fatalf("accepted invalid encoding %x", data)
		}
	}
}

func TestDecodeText(t *testing.T) {
	text := "Option Explicit\r\nPublic Sub Été🌲()\r\nEnd Sub\r\n"
	utf16Data := []byte{0xff, 0xfe}
	for _, u := range utf16.Encode([]rune(text)) {
		var b [2]byte
		binary.LittleEndian.PutUint16(b[:], u)
		utf16Data = append(utf16Data, b[:]...)
	}
	for name, data := range map[string][]byte{
		"utf8":     []byte(text),
		"utf8bom":  append([]byte{0xef, 0xbb, 0xbf}, []byte(text)...),
		"utf16le":  utf16Data,
		"empty":    nil,
		"only-bom": {0xff, 0xfe},
	} {
		t.Run(name, func(t *testing.T) {
			want := text
			if name == "empty" || name == "only-bom" {
				want = ""
			}
			got, err := DecodeText(data, "Module.bas")
			if err != nil || got != want {
				t.Fatalf("DecodeText = %q, %v; want %q", got, err, want)
			}
		})
	}
}

func TestDecodeTextMalformed(t *testing.T) {
	for name, data := range map[string][]byte{
		"invalid-utf8":       {0xff},
		"truncated-utf8":     {0xe2, 0x82},
		"odd-utf16":          {0xff, 0xfe, 0x41},
		"high-surrogate":     {0xff, 0xfe, 0x00, 0xd8},
		"low-surrogate":      {0xff, 0xfe, 0x00, 0xdc},
		"invalid-pair":       {0xff, 0xfe, 0x00, 0xd8, 0x41, 0},
		"big-endian-invalid": {0xfe, 0xff, 0xdc, 0x00},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := DecodeText(data, "Module.bas")
			if got != "" || err == nil || !strings.HasPrefix(err.Error(), "Module.bas: ") {
				t.Fatalf("expected source-qualified decode error, got %q, %v", got, err)
			}
			_, parseErr := Parse(data, "Module.bas")
			if parseErr == nil || parseErr.Error() != err.Error() {
				t.Fatalf("Parse does not reuse decode errors: %v versus %v", parseErr, err)
			}
		})
	}
}

func TestDecodeWindows1252Text(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"empty", nil, ""},
		{"ascii", []byte("Option Compare Database\r\n"), "Option Compare Database\r\n"},
		{"accents", []byte{0xfc, 0xf6, 0xfc, 0xe4}, "üöüä"},
		{"quotes", []byte{0x91, 'a', 0x92, ' ', 0x93, 'b', 0x94}, "‘a’ “b”"},
		{"c1", []byte{
			0x80, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8e,
			0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9e, 0x9f,
		}, "€‚ƒ„…†‡ˆ‰Š‹ŒŽ‘’“”•–—˜™š›œžŸ"},
		{"latin1", []byte{0x00, 0x7f, 0xa0, 0xa3, 0xc9, 0xdf, 0xff}, "\x00\x7f\u00a0£Éßÿ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeWindows1252Text(tc.data, "LegacyModule.bas")
			if err != nil || got != tc.want {
				t.Fatalf("CP1252 = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if _, err := DecodeText([]byte{0xfc, 0xf6}, "LegacyModule.bas"); err == nil {
		t.Fatal("strict decoder unexpectedly falls back to CP1252")
	}
	if _, err := Parse([]byte{0xfc, 0xf6}, "LegacyModule.bas"); err == nil {
		t.Fatal("parser unexpectedly falls back to CP1252")
	}
}

func TestDecodeWindows1252UndefinedBytes(t *testing.T) {
	for _, b := range []byte{0x81, 0x8d, 0x8f, 0x90, 0x9d} {
		got, err := DecodeWindows1252Text([]byte{'a', b}, "LegacyModule.bas")
		if got != "" || err == nil || !strings.HasPrefix(err.Error(), "LegacyModule.bas: ") ||
			!strings.Contains(err.Error(), "undefined Windows-1252 byte") ||
			!strings.Contains(err.Error(), "byte offset 1") {
			t.Fatalf("undefined byte %x: got %q, %v", b, got, err)
		}
	}
}

func TestSyntaxErrors(t *testing.T) {
	cases := map[string]string{
		"extra end":           "End\n",
		"missing end":         "Begin Form\n",
		"opaque missing end":  "Begin Form\nData = Begin\n0x1234\n",
		"duplicate defaults":  "Begin Form\nBegin\nBegin TextBox\nEnd\nBegin TextBox\nEnd\nEnd\nEnd",
		"unterminated string": "Begin Form\nName =\"a\nEnd",
		"bad continuation":    "Begin Form\nName =\"a\"\n\"b\nEnd",
		"trailing text":       "Begin Form\nName =\"a\" junk\nEnd",
		"orphan string":       "\"orphan\"\n",
		"unknown statement":   "Something unknown\n",
		"bad block":           "Begin Form extra\nEnd",
		"bad property":        "Begin Form\nbad name =1\nEnd",
		"empty value":         "Begin Form\nName =\nEnd",
		"code inside form":    "Begin Form\nCodeBehindForm\nEnd",
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			d, err := Parse([]byte(text), "bad.txt")
			if err == nil || d != nil || !strings.Contains(err.Error(), "bad.txt:") {
				t.Fatalf("expected located fatal error, got %#v, %v", d, err)
			}
		})
	}
}

func TestUnknownTypesRetainedWithDiagnostic(t *testing.T) {
	d := mustParse(t, "Begin FutureControl\nName =\"future\"\nNewProperty =SomeNewValue\nEnd")
	n := d.Root.Children[0]
	if n.Type != "FutureControl" || n.Name() != "future" || n.Properties["NewProperty"].Value != "SomeNewValue" {
		t.Fatal("unknown node or property discarded")
	}
	if len(d.Diagnostics) != 1 || d.Diagnostics[0].Line != 1 || !strings.Contains(d.Diagnostics[0].Message, "FutureControl") {
		t.Fatalf("diagnostics: %#v", d.Diagnostics)
	}
}

func TestDuplicatesRetainedWithDiagnostic(t *testing.T) {
	d := mustParse(t, "Begin Form\nName =\"a\"\nname =\"b\"\nName =\"c\"\nEnd")
	n := d.Root.Children[0]
	history := n.DuplicateProperties["Name"]
	if n.Name() != "c" || len(n.Properties) != 1 || len(history) != 3 {
		t.Fatalf("duplicate handling: %#v", n)
	}
	for i, want := range []string{"a", "b", "c"} {
		if history[i].Value != want || history[i].Line != i+2 {
			t.Fatalf("lost duplicate history: %#v", history)
		}
	}
	if len(d.Diagnostics) != 2 || d.Diagnostics[0].Line != 3 || d.Diagnostics[1].Line != 4 {
		t.Fatalf("duplicate diagnostics: %#v", d.Diagnostics)
	}
}

func TestTypedQueryProperties(t *testing.T) {
	d := mustParse(t, "dbMemo \"SQL\" =\"SELECT \"\"Label\"\", * \"\r\n"+
		"    \"FROM Example WHERE ID = 1;\"\r\n"+
		"dbBoolean \"ReturnsRecords\" =\"-1\"\r\n"+
		"dbBinary \"GUID\" = Begin\r\n"+
		"    0xdeadbeef\r\n"+
		"End\r\n"+
		"dbText \"A=B\" =\"quoted name\"\r\n"+
		"Begin\r\nEnd\r\n")
	props := d.Root.Properties
	sql := props["SQL"]
	if sql.Value != "SELECT \"Label\", * FROM Example WHERE ID = 1;" ||
		sql.Raw != "\"SELECT \"\"Label\"\", * \"\r\n    \"FROM Example WHERE ID = 1;\"" ||
		sql.Line != 1 || sql.DataType != "dbMemo" {
		t.Fatalf("SQL property: %#v", sql)
	}
	if p := props["ReturnsRecords"]; p.Value != "-1" || p.Raw != "\"-1\"" || p.Line != 3 || p.DataType != "dbBoolean" {
		t.Fatalf("boolean property: %#v", p)
	}
	if p := props["GUID"]; !p.Opaque || p.Raw != " Begin\r\n    0xdeadbeef\r\nEnd" || p.Line != 4 || p.DataType != "dbBinary" {
		t.Fatalf("binary property: %#v", p)
	}
	if props["A=B"].Value != "quoted name" || len(props) != 4 || len(d.Diagnostics) != 0 {
		t.Fatalf("typed property normalization: %#v, %#v", props, d.Diagnostics)
	}
	if len(d.Root.Children) != 1 || d.Root.Children[0].Type != "" || d.Root.Children[0].Line != 8 || d.Root.Children[0].EndLine != 9 {
		t.Fatal("query trailing anonymous block lost")
	}
}

func TestTypedPropertyDiagnosticsAndErrors(t *testing.T) {
	d := mustParse(t, "dbFuture \"Extension\" =\"value\"\ndbMemo \"SQL\" =\"first\"\ndbMemo \"sql\" =\"second\"")
	if d.Root.Properties["Extension"].DataType != "dbFuture" || len(d.Diagnostics) != 2 ||
		!strings.Contains(d.Diagnostics[0].Message, "unrecognized property type") ||
		d.Root.Properties["SQL"].Value != "second" || len(d.Root.DuplicateProperties["SQL"]) != 2 {
		t.Fatalf("typed diagnostics: %#v", d)
	}
	for _, text := range []string{
		"dbMemo SQL =\"SELECT 1\"", "dbMemo \"\" =\"value\"",
		"dbMemo \"SQL\" junk =\"value\"", "dbMemo \"SQL =\"value\"",
		"SomeType \"SQL\" =\"value\"",
	} {
		if _, err := Parse([]byte(text), "bad-query"); err == nil {
			t.Fatalf("accepted malformed typed declaration: %q", text)
		}
	}
}

func TestQueryOctalWhitespaceAndWindowsPaths(t *testing.T) {
	text := `dbMemo "SQL" ="SELECT *\015\012FROM FirstTable;\015\012UNION SELECT *\015\012FROM Sec"
    "ondTable;\015\012"
dbMemo "Connect" ="C:\Program Files\Reports\data.accdb"
dbText "NumericFolder" ="C:\\015\\archive.accdb"
dbText "LiteralEscapes" ="C:\new\table.accdb"
Begin
End`
	d := mustParse(t, text)
	p := d.Root.Properties["SQL"]
	if p.Value != "SELECT *\r\nFROM FirstTable;\r\nUNION SELECT *\r\nFROM SecondTable;\r\n" {
		t.Fatalf("SQL octal escapes did not become whitespace: %q", p.Value)
	}
	if p.Raw != "\"SELECT *\\015\\012FROM FirstTable;\\015\\012UNION SELECT *\\015\\012FROM Sec\"\n    \"ondTable;\\015\\012\"" {
		t.Fatalf("SQL raw escapes were changed: %q", p.Raw)
	}
	for key, want := range map[string]string{
		"Connect":        `C:\Program Files\Reports\data.accdb`,
		"NumericFolder":  `C:\015\archive.accdb`,
		"LiteralEscapes": `C:\new\table.accdb`,
	} {
		if got := d.Root.Properties[key].Value; got != want {
			t.Fatalf("%s path changed: got %q, want %q", key, got, want)
		}
	}
}

func TestFormLocalDefaults(t *testing.T) {
	d := mustParse(t, `Begin Form
Begin TextBox
FontSize =10
End
Begin Section
Name ="first"
Begin TextBox
Name ="input1"
End
End
End
Begin Form
Begin TextBox
FontSize =20
End
Begin Section
Name ="second"
Begin TextBox
Name ="input2"
End
End
End`)
	for i, want := range []string{"10", "20"} {
		form := d.Root.Children[i]
		n := form.Children[1].Children[0]
		if d.EffectiveProperties(n)["FontSize"].Value != want {
			t.Fatal("defaults leaked between forms")
		}
		if _, ok := d.EffectiveProperties(form)["FontSize"]; ok {
			t.Fatal("control defaults applied to form")
		}
	}
	foreign := &Node{Properties: map[string]Property{"FontSize": {Value: "30"}}}
	if d.EffectiveProperties(foreign)["FontSize"].Value != "30" || len(d.EffectiveProperties(nil)) != 0 {
		t.Fatal("foreign or nil node handling")
	}
	var nilDocument *Document
	if nilDocument.EffectiveProperties(foreign)["FontSize"].Value != "30" {
		t.Fatal("nil document handling")
	}
}

func TestReportGroupingIsNotDefaults(t *testing.T) {
	d := mustParse(t, `Begin Report
    Begin
        Begin TextBox
            FontSize =12
        End
        Begin BreakLevel
            ControlSource ="Zone"
        End
        Begin BreakLevel
            ControlSource ="PlotNumber"
        End
        Begin BreakHeader
            Height =50
        End
        Begin BreakHeader
            Height =100
        End
        Begin Section
            Name ="Detail"
            Begin
                Begin TextBox
                    Name ="Plot"
                End
            End
        End
    End
End`)
	children := d.Root.Children[0].Children[0].Children
	if len(d.Diagnostics) != 0 || !children[0].Default || len(children) != 6 {
		t.Fatalf("report layout: %#v, diagnostics=%#v", children, d.Diagnostics)
	}
	for _, n := range children[1:] {
		if n.Default {
			t.Fatalf("structural report node %s treated as default", n.Type)
		}
	}
	if children[1].Properties["ControlSource"].Value != "Zone" ||
		children[2].Properties["ControlSource"].Value != "PlotNumber" ||
		children[3].Properties["Height"].Value != "50" ||
		children[4].Properties["Height"].Value != "100" {
		t.Fatal("report group nodes lost or merged")
	}
	if p := d.EffectiveProperties(children[2]); len(p) != 1 || p["ControlSource"].Value != "PlotNumber" {
		t.Fatalf("report grouping inherited another group: %#v", p)
	}
	input := children[5].Children[0].Children[0]
	if d.EffectiveProperties(input)["FontSize"].Value != "12" {
		t.Fatal("report control defaults not inherited")
	}
}

func TestQueryDesignerCollections(t *testing.T) {
	d := mustParse(t, `Operation =1
Begin InputTables
    Name ="FirstTable"
    Name ="SecondTable"
End
Begin OutputColumns
    Expression ="FirstTable.ID"
    Expression ="SecondTable.ID"
End
Begin Joins
    LeftTable ="FirstTable"
    RightTable ="SecondTable"
End
Begin
    Begin
        dbText "Name" ="FirstTable.ID"
    End
End`)
	if len(d.Root.Children) != 4 || len(d.Diagnostics) != 2 {
		t.Fatalf("query designer nodes/diagnostics: %#v", d)
	}
	tables := d.Root.Children[0]
	columns := d.Root.Children[1]
	if tables.Type != "InputTables" || tables.Default || len(tables.DuplicateProperties["Name"]) != 2 ||
		tables.DuplicateProperties["Name"][0].Value != "FirstTable" ||
		tables.DuplicateProperties["Name"][1].Value != "SecondTable" ||
		columns.Type != "OutputColumns" || len(columns.DuplicateProperties["Expression"]) != 2 {
		t.Fatal("query designer collections lost")
	}
	if d.Root.Children[3].Children[0].Name() != "FirstTable.ID" {
		t.Fatal("query anonymous collection nesting lost")
	}
}

func TestParseFileMissing(t *testing.T) {
	_, err := ParseFile(filepath.Join("testdata", "does-not-exist.txt"))
	if err == nil {
		t.Fatal("missing file succeeded")
	}
}

// Canonical exports are read-only and optional. The independent source counts
// below include type defaults, but exclude opaque-property Begin blocks.
func TestAccessExports(t *testing.T) {
	dir := os.Getenv("ACCESS_EXPORT_DIR")
	if dir == "" {
		t.Skip("set ACCESS_EXPORT_DIR to opt into read-only real Access export tests")
	}
	dirs := []string{dir}
	if info, err := os.Stat(filepath.Join(dir, "Forms")); err == nil && info.IsDir() {
		dirs = []string{filepath.Join(dir, "Forms"), filepath.Join(dir, "Reports"), filepath.Join(dir, "Queries")}
	}
	var files []string
	for _, folder := range dirs {
		matches, err := filepath.Glob(filepath.Join(folder, "*.txt"))
		if err != nil || len(matches) == 0 {
			t.Fatalf("no export files in %q: %v", folder, err)
		}
		files = append(files, matches...)
	}
	foundFS882 := false
	for _, path := range files {
		name, err := filepath.Rel(dir, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Run(name, func(t *testing.T) {
			d, err := ParseFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, diagnostic := range d.Diagnostics {
				if !strings.HasPrefix(diagnostic.Message, "duplicate property ") {
					t.Fatalf("unrecognized export construct: %#v", diagnostic)
				}
			}
			if filepath.Base(path) != "FS882-6x4XL.txt" {
				return
			}
			foundFS882 = true
			counts := map[string]int{}
			named, defaults := 0, 0
			var walk func(*Node)
			walk = func(n *Node) {
				if n != d.Root {
					counts[n.Type]++
				}
				if n.Name() != "" {
					named++
				}
				if n.Default {
					defaults++
				}
				for _, child := range n.Children {
					walk(child)
				}
			}
			walk(d.Root)
			want := map[string]int{
				"Form": 1, "": 94, "Label": 137, "Rectangle": 2,
				"CommandButton": 16, "OptionButton": 6, "CheckBox": 6,
				"OptionGroup": 5, "TextBox": 59, "ListBox": 1, "ComboBox": 54,
				"Subform": 12, "CustomControl": 1, "ToggleButton": 7, "Tab": 2,
				"FormHeader": 1, "Section": 1, "Page": 6, "FormFooter": 1,
			}
			if !reflect.DeepEqual(counts, want) || named != 304 || defaults != 13 {
				t.Fatalf("counts=%v named=%d defaults=%d", counts, named, defaults)
			}
			if d.CodeLine != 8891 || !strings.Contains(d.Code, "Private Sub tabPages_Change()") {
				t.Fatal("real VBA missing or incorrectly located")
			}
			t.Logf("FS882: %d named instances, %d defaults, %d anonymous containers", named, defaults, counts[""])
		})
	}
	if !foundFS882 {
		t.Fatal("ACCESS_EXPORT_DIR must be the export root or Forms folder, including FS882-6x4XL.txt")
	}
	t.Logf("Accepted %d exports across %d folders", len(files), len(dirs))
}
