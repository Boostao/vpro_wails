package fs882layout

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boostao/vpro-wails/internal/accessinventory"
)

func fixture(t *testing.T) string {
	t.Helper()
	root := filepath.Join(".", "test-work-"+t.Name())
	if err := os.MkdirAll(filepath.Join(root, "Forms"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	return root
}

func writeForm(t *testing.T, root, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "Forms", name+".txt"), []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
}

const parentFixture = `Begin Form
    RecordSource ="HEADER"
    Width =15000
    Left =-5400
    Top =1995
    Right =9360
    Bottom =11565
    Begin TextBox
        Visible = NotDefault
        Locked = NotDefault
        FontBold = NotDefault
        Width =1200
    End
    Begin
        Begin Section
            Name ="Detail"
            Height =9045
            Begin
                Begin Tab
                    Name ="Tabs"
                    Width =14535
                    Height =9045
                    Begin
                        Begin Page
                            Name ="VegOther"
                            Caption ="Veg Other"
                            Left =135
                            Top =450
                            Width =14265
                            Height =8460
                            LayoutCachedLeft =135
                            LayoutCachedTop =450
                            LayoutCachedWidth =14400
                            LayoutCachedHeight =8910
                            Begin
                                Begin TextBox
                                    Name ="PlotNumber"
                                    ControlSource ="[PlotNumber]"
                                    Left =12510
                                    Top =240
                                    TabIndex =7
                                    Format ="000"
                                    InputMask ="000"
                                    ValidationRule =">0"
                                    ValidationText ="Positive only"
                                    OnAfterUpdate ="[Event Procedure]"
                                    Begin
                                        Begin Label
                                            Name ="PlotLabel"
                                            Caption ="Plot number"
                                            Left =12510
                                            Top =450
                                            Width =900
                                            Height =240
                                        End
                                    End
                                End
                                Begin TextBox
                                    Name ="Alias"
                                    ControlSource ="PlotNumber"
                                    Enabled =0
                                End
                                Begin TextBox
                                    Name ="Total"
                                    ControlSource ="=[A]+[B]"
                                End
                                Begin TextBox
                                    Name ="UnrecognizedBinding"
                                    ControlSource ="HEADER.PlotNumber"
                                End
                                Begin Subform
                                    Name ="ChildInstance"
                                    SourceObject ="Form.Child"
                                    LinkMasterFields ="PlotNumber;[Site]"
                                    LinkChildFields ="Plot;Site"
                                End
                                Begin Subform
                                    Name ="ChildAgain"
                                    SourceObject ="Form.Child"
                                End
                                Begin Subform
                                    Name ="Missing"
                                    SourceObject ="Form.Missing"
                                End
                                Begin CommandButton
                                    Name ="Run"
                                    OnClick ="=DoSomething()"
                                End
                                Begin ComboBox
                                    Name ="SQLChoice"
                                    RowSourceType ="Table/Query"
                                    RowSource ="SELECT SecretColumn FROM SecretTable;"
                                    SourceSQL ="SELECT RawSource FROM SecretTable;"
                                    PictureData = Begin
                                        0x12345678
                                    End
                                End
                                Begin Rectangle
                                    Name ="PaperGroupOutline"
                                    Left =270
                                    Top =720
                                    Width =3600
                                    Height =1800
                                End
                            End
                        End
                    End
                End
            End
        End
    End
End
CodeBehindForm
Private Sub PlotNumber_AfterUpdate()
    DangerousVBASecret = 1
End Sub
`

func TestBuildHierarchyBindingsAndEvidence(t *testing.T) {
	root := fixture(t)
	writeForm(t, root, "Parent", parentFixture)
	writeForm(t, root, "Child", `Begin Form
    RecordSource ="SELECT HiddenSQL FROM HEADER;"
    Begin
        Begin TextBox
            Name ="ChildPlot"
            ControlSource ="Plot"
        End
    End
End
`)
	layout, err := Build(root, "parent")
	if err != nil {
		t.Fatal(err)
	}
	if layout.Root != "Parent" || len(layout.Forms) != 2 {
		t.Fatalf("closure: %+v", layout)
	}
	parent := layout.Forms[0]
	if layout.GeometryUnit != "twips" || len(parent.Controls) != len(parent.Fields) {
		t.Fatal("complete raw control tree missing")
	}
	controlByName := map[string]Control{}
	for _, control := range parent.Controls {
		controlByName[control.ControlName] = control
	}
	for name, expected := range map[string]map[string]string{
		"Parent":            {"Width": "15000", "Left": "-5400", "Top": "1995", "Right": "9360", "Bottom": "11565"},
		"Detail":            {"Height": "9045"},
		"Tabs":              {"Width": "14535", "Height": "9045"},
		"VegOther":          {"Left": "135", "Top": "450", "Width": "14265", "Height": "8460", "LayoutCachedHeight": "8910", "LayoutCachedWidth": "14400"},
		"PlotLabel":         {"Left": "12510", "Top": "450", "Width": "900", "Height": "240"},
		"PaperGroupOutline": {"Left": "270", "Top": "720", "Width": "3600", "Height": "1800"},
	} {
		for key, value := range expected {
			if got := controlByName[name].Properties[key]; got.Value != value || got.Inherited || got.Line == 0 {
				t.Fatalf("%s raw %s geometry lost: %+v", name, key, got)
			}
		}
	}
	if len(parent.SHA256) != 64 || parent.Source != filepath.Join("Forms", "Parent.txt") || parent.RecordSource != "HEADER" {
		t.Fatalf("provenance: %+v", parent)
	}
	if len(parent.Pages) != 1 || parent.Pages[0].Caption != "Veg Other" || len(parent.Embedded) != 3 {
		t.Fatalf("pages/embedded: %+v", parent)
	}
	fields := map[string]Field{}
	ids := map[string]Field{}
	for _, field := range parent.Fields {
		fields[field.ControlName], ids[field.ControlID] = field, field
		if !field.ReadOnly || field.Implementation != "unmapped" {
			t.Fatalf("unsafe metadata: %+v", field)
		}
	}
	plot := fields["PlotNumber"]
	if plot.Column != "PlotNumber" || plot.Caption != "Plot number" || plot.CaptionControlID != fields["PlotLabel"].ControlID {
		t.Fatalf("attached caption/binding: %+v", plot)
	}
	if ids[plot.ParentID].Type != "Group" || plot.PageID != parent.Pages[0].ControlID || plot.Line != lineOf(parentFixture, `Begin TextBox`, 2) {
		t.Fatalf("hierarchy/line: %+v", plot)
	}
	for key, value := range map[string]string{"Top": "240", "TabIndex": "7", "Format": "000", "InputMask": "000", "ValidationRule": ">0", "ValidationText": "Positive only"} {
		got := plot.Properties[key]
		fragment := key + " ="
		if key == "Top" {
			fragment += value
		}
		if got.Value != value || got.Inherited || got.Line != lineOf(parentFixture, fragment, 1) {
			t.Fatalf("%s evidence: %+v", key, got)
		}
	}
	if got := plot.Properties["Width"]; got.Value != "1200" || !got.Inherited || got.Line != lineOf(parentFixture, "Width =1200", 1) {
		t.Fatalf("default evidence: %+v", got)
	}
	for _, key := range []string{"Visible", "Locked", "FontBold"} {
		if got := plot.Properties[key]; got.Value != "NotDefault" || !got.Inherited || got.Line != lineOf(parentFixture, key+" =", 1) {
			t.Fatalf("%s raw default-dependent boolean changed: %+v", key, got)
		}
	}
	if got := plot.Properties["AttachedCaption"]; got.Line != lineOf(parentFixture, `Caption ="Plot number"`, 1) {
		t.Fatalf("caption evidence: %+v", got)
	}
	if len(plot.Events) != 1 || plot.Events[0].Line != lineOf(parentFixture, "OnAfterUpdate =", 1) {
		t.Fatalf("event evidence: %+v", plot.Events)
	}
	if fields["Alias"].Column != plot.Column || fields["Alias"].Caption != "" || len(parent.BindingColumns()) != 1 {
		t.Fatal("aliases lost or caption guessed")
	}
	if computed := fields["Total"]; computed.Column != "" || computed.Computed != "=[A]+[B]" || computed.BindingKind != "computed" || !computed.ReadOnly {
		t.Fatalf("computed: %+v", computed)
	}
	if fields["UnrecognizedBinding"].BindingKind != "unsupported" || fields["Run"].Widget != "unsupported" {
		t.Fatal("unknown behavior must remain unsupported")
	}
	if edge := parent.Embedded[0]; edge.Form != "Child" || !edge.Resolved || strings.Join(edge.MasterFields, ",") != "PlotNumber,Site" || strings.Join(edge.ChildFields, ",") != "Plot,Site" {
		t.Fatalf("instance link: %+v", edge)
	}
	if parent.Embedded[2].Resolved || parent.Embedded[2].Form != "" {
		t.Fatal("missing target incorrectly resolved")
	}
	if layout.Forms[1].RecordSource != "" || layout.Forms[1].RecordSourceKind != "unsupported-expression-or-sql" {
		t.Fatal("SQL record source published")
	}
	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"DangerousVBASecret", "HiddenSQL", "SecretColumn", "RawSource", "12345678", "DoSomething"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("raw implementation leaked: %s", forbidden)
		}
	}
	again, err := Build(root, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := json.Marshal(again)
	if string(other) != string(data) {
		t.Fatal("nondeterministic JSON")
	}
}

func lineOf(text, fragment string, occurrence int) int {
	for i, line := range strings.Split(text, "\n") {
		if strings.Contains(line, fragment) {
			occurrence--
			if occurrence == 0 {
				return i + 1
			}
		}
	}
	return 0
}

func TestColumnIdentifiers(t *testing.T) {
	for input, expected := range map[string]string{
		"[PlotNumber]": "PlotNumber", " PlotNumber ": "PlotNumber", "Étage": "Étage",
		"=[PlotNumber]": "", "HEADER.PlotNumber": "", "[A]+[B]": "", "A B": "", "123": "", "[A": "",
	} {
		if got := column(input); got != expected {
			t.Errorf("column(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestRealFS882(t *testing.T) {
	root := os.Getenv("ACCESS_EXPORT_DIR")
	if root == "" {
		t.Skip("opt in with ACCESS_EXPORT_DIR")
	}
	layout, err := Build(root, "FS882-6x4XL")
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Forms) != 12 || len(layout.Forms[0].Pages) != 6 {
		t.Fatalf("closure/pages: %d/%d", len(layout.Forms), len(layout.Forms[0].Pages))
	}
	controlNamed := 0
	for _, control := range layout.Forms[0].Controls {
		if control.ControlName != "" {
			controlNamed++
		}
	}
	if controlNamed != 305 || len(layout.Forms[0].Controls) != len(layout.Forms[0].Fields) {
		t.Fatalf("complete tree: %d root/named controls, %d total", controlNamed, len(layout.Forms[0].Controls))
	}
	named, bound, vegOther := 0, 0, false
	for _, field := range layout.Forms[0].Fields {
		if field.ControlName != "" && field.Type != "Form" {
			named++
		}
		if field.Binding != "" {
			bound++
		}
	}
	for _, page := range layout.Forms[0].Pages {
		if page.Caption == "Veg Other" {
			vegOther = true
		}
	}
	if named != 304 || !vegOther {
		t.Fatalf("named nodes/Veg Other: %d/%t", named, vegOther)
	}
	inventory, err := accessinventory.Build(root)
	if err != nil {
		t.Fatal(err)
	}
	reachable, err := inventory.ReachableForms(layout.Root)
	if err != nil {
		t.Fatal(err)
	}
	for i, form := range reachable {
		controlByID := map[string]Control{}
		for _, control := range layout.Forms[i].Controls {
			controlByID[control.ControlID] = control
		}
		for _, original := range form.Controls {
			got, ok := controlByID[original.ID]
			if !ok || got.Line != original.Line || got.EndLine != original.EndLine {
				t.Fatalf("%s source node lost: %s", form.Name, original.ID)
			}
			for _, key := range []string{"Left", "Top", "Width", "Height", "Right", "Bottom", "LayoutCachedLeft", "LayoutCachedTop", "LayoutCachedWidth", "LayoutCachedHeight"} {
				if value, exists := original.Properties[key]; exists && got.Properties[key] != value {
					t.Fatalf("%s %s raw %s geometry/evidence changed", form.Name, original.ID, key)
				}
			}
		}
		instances := map[string]accessinventory.Value{}
		for _, control := range form.Controls {
			if value := control.Properties["ControlSource"]; value.Value != "" {
				instances[control.ID] = value
			}
		}
		got := 0
		for _, field := range layout.Forms[i].Fields {
			if field.Binding != "" {
				got++
				expected, ok := instances[field.ControlID]
				if !ok || field.Binding != expected.Value || field.Properties["ControlSource"].Line != expected.Line {
					t.Fatalf("%s binding/line lost: %+v", form.Name, field)
				}
			}
		}
		if got != len(instances) {
			t.Fatalf("%s captured %d/%d binding instances", form.Name, got, len(instances))
		}
	}
	t.Logf("parent: %d named nodes, %d bindings, %d unique columns; closure: %d forms", named, bound, len(layout.Forms[0].BindingColumns()), len(layout.Forms))
}
