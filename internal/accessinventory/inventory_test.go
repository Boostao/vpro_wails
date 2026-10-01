package accessinventory

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeExport(t *testing.T, root, folder, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, folder), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, folder, name+".txt"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestInventoryContainmentDefaultsAndSubforms(t *testing.T) {
	root := t.TempDir()
	writeExport(t, root, "Forms", "Parent", `Version =21
Begin Form
    RecordSource ="ParentQuery"
    OnCurrent ="[Event Procedure]"
    Begin TextBox
        FontSize =10
        Locked =NotDefault
    End
    Begin
        Begin Section
            Name ="Detail"
            Begin
                Begin Tab
                    Name ="Tabs"
                    Begin
                        Begin Page
                            Name ="Site"
                            Begin
                                Begin TextBox
                                    Name ="PlotNumber"
                                    ControlSource ="PlotNumber"
                                    OnLostFocus ="[Event Procedure]"
                                    Begin
                                        Begin Label
                                            Name ="PlotLabel"
                                            Caption ="Plot Number"
                                        End
                                    End
                                End
                                Begin Subform
                                    Name ="Veg"
                                    SourceObject ="Form.Child"
                                    LinkMasterFields ="PlotNumber;Project"
                                    LinkChildFields ="[PlotNumber];[Project]"
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
Attribute VB_GlobalNameSpace = False
Option Explicit
Private Sub Form_Current()
End Sub
Private Sub PlotNumber_LostFocus()
End Sub
`)
	writeExport(t, root, "Forms", "Child", `Begin Form
RecordSource ="ChildTable"
Begin
Begin Section
Name ="Detail"
Begin
Begin TextBox
Name ="Species"
ControlSource ="Species"
End
Begin Subform
Name ="BackLink"
SourceObject ="Form.Parent"
End
End
End
End
End
`)
	writeExport(t, root, "Queries", "ParentQuery", `dbMemo "SQL" ="SELECT * FROM [ParentTable] INNER JOIN ChildTable ON ParentTable.ID=ChildTable.ID;"
Begin
End
`)
	writeExport(t, root, "Tables_Def", "ParentTable_CreateSQL", "CREATE TABLE ParentTable (ID INTEGER);")
	writeExport(t, root, "Tables_Def", "ChildTable_CreateSQL", "CREATE TABLE ChildTable (ID INTEGER);")
	inventory, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := inventory.ReachableForms("parent")
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) != 2 {
		t.Fatalf("cycle traversal: got %d forms, want 2", len(forms))
	}
	var plot, label Control
	for _, control := range forms[0].Controls {
		switch control.Name {
		case "PlotNumber":
			plot = control
		case "PlotLabel":
			label = control
		}
	}
	if plot.ParentID != "form:Parent/Site" || label.ParentID != plot.ID {
		t.Fatalf("containment: plot parent=%q label parent=%q", plot.ParentID, label.ParentID)
	}
	if value := plot.Properties["FontSize"]; value.Value != "10" || !value.Inherited {
		t.Fatalf("default provenance lost: %+v", value)
	}
	if len(plot.Events) != 1 || !plot.Events[0].Resolved || plot.Events[0].Procedure != "PlotNumber_LostFocus" {
		t.Fatalf("event resolution: %+v", plot.Events)
	}
	for _, edge := range inventory.Edges {
		if !edge.Resolved {
			t.Errorf("unexpected unresolved dependency: %+v", edge)
		}
		if edge.From == "form:Parent/Veg" {
			want := []string{"PlotNumber", "Project"}
			if !reflect.DeepEqual(edge.MasterFields, want) || !reflect.DeepEqual(edge.ChildFields, want) {
				t.Errorf("links: %+v", edge)
			}
		}
	}
}

func TestInventoryReportsMissingBindingsAndLinkMismatch(t *testing.T) {
	root := t.TempDir()
	writeExport(t, root, "Forms", "Parent", `Begin Form
RecordSource ="RuntimeQuery"
Begin
Begin Section
Name ="Detail"
Begin
Begin Subform
Name ="Child"
SourceObject ="Form.Missing"
LinkMasterFields ="Plot;Project"
LinkChildFields ="Plot"
OnEnter ="[Event Procedure]"
End
End
End
End
End
`)
	inventory, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	codes := make(map[string]bool)
	for _, diagnostic := range inventory.Diagnostics {
		codes[diagnostic.Code] = true
	}
	for _, code := range []string{"unresolved-source", "unresolved-event", "mismatched-subform-links", "missing-export-folder"} {
		if !codes[code] {
			t.Errorf("missing explicit diagnostic %s", code)
		}
	}
}

func TestInventoryRejectsMalformedExport(t *testing.T) {
	root := t.TempDir()
	writeExport(t, root, "Forms", "Bad", "Begin Form\nBegin Section\n")
	if _, err := Build(root); err == nil {
		t.Fatal("malformed nesting accepted")
	}
}

func TestTwipConversion(t *testing.T) {
	if pixels, err := TwipsToCSSPixels("1440"); err != nil || pixels != 96 {
		t.Fatalf("one inch conversion: %v, %v", pixels, err)
	}
	if _, err := TwipsToCSSPixels("NotDefault"); err == nil {
		t.Fatal("invalid dimension silently accepted")
	}
}

func TestSQLReferences(t *testing.T) {
	for _, test := range []struct {
		sql  string
		want []string
	}{
		{"SELECT * FROM [Sample_Env] INNER JOIN Sample_Admin ON Sample_Env.PlotNumber=Sample_Admin.Plot", []string{"Sample_Env", "Sample_Admin"}},
		{"SELECT 'FROM Fake', \"JOIN FakeToo\" FROM Actual -- JOIN NotReal\n", []string{"Actual"}},
		{"SELECT * FROM FirstTable, SecondTable WHERE x IN (SELECT x FROM ThirdTable)", []string{"FirstTable", "SecondTable", "ThirdTable"}},
		{"SELECT * FROM (SELECT * FROM Nested) AS n JOIN [Spaced Name] ON 1=1", []string{"Nested", "Spaced Name"}},
		{"UPDATE [Target] SET Note='JOIN Fake'", []string{"Target"}},
		{"SELECT * FROM Real /* JOIN Fake */ JOIN Real ON 1=1", []string{"Real"}},
		{"SELECT [FROM], [JOIN] FROM [ORDER] INNER JOIN [WHERE] ON 1=1", []string{"ORDER", "WHERE"}},
		{"SELECT * FROM (SELECT * FROM InnerTable) n, OuterTable", []string{"InnerTable", "OuterTable"}},
		{"SELECT * FROM (([Env] INNER JOIN [Admin] ON Env.ID=Admin.ID) LEFT JOIN Veg ON Env.ID=Veg.ID)", []string{"Env", "Admin", "Veg"}},
	} {
		if got := SQLReferences(test.sql); !reflect.DeepEqual(got, test.want) {
			t.Errorf("%s: got %v, want %v", test.sql, got, test.want)
		}
	}
}

func TestProcedureSourceLines(t *testing.T) {
	code := "\nOption Explicit\n\nPrivate Sub Form_Current()\nEnd Sub\nPublic Function Test(x As Integer) As String\nEnd Function\n"
	got := procedures(code, 10)
	if len(got) != 2 || got[0].Line != 13 || got[1].Line != 15 {
		t.Fatalf("source lines: %+v", got)
	}
}

func TestModuleAndMacroInventory(t *testing.T) {
	root := t.TempDir()
	writeExport(t, root, "Forms", "Parent", "Begin Form\nEnd\n")
	writeExport(t, root, "Modules", "Helpers", "Option Explicit\nPublic Sub SelectProject()\nEnd Sub\n")
	writeExport(t, root, "Modules", "Legacy", "'caf"+string([]byte{0xe9})+"\nPublic Sub SaveProject()\nEnd Sub\n")
	writeExport(t, root, "Macros", "AutoKeys", "Version =196611\n")
	inventory, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	var moduleFound, macroFound, legacyFound bool
	for _, object := range inventory.Objects {
		if object.Kind == "module" && object.Name == "Helpers" {
			moduleFound = len(object.Procedures) == 1 && object.Procedures[0].Name == "SelectProject" && object.Procedures[0].Line == 2
		}
		if object.Kind == "macro" && object.Name == "AutoKeys" {
			macroFound = len(object.SHA256) == 64
		}
		if object.Kind == "module" && object.Name == "Legacy" {
			legacyFound = len(object.Procedures) == 1 && object.Procedures[0].Name == "SaveProject" && object.Procedures[0].Line == 2
		}
	}
	if !moduleFound || !macroFound || !legacyFound {
		t.Fatalf("module declarations or macro hash missing: module=%v macro=%v legacy=%v", moduleFound, macroFound, legacyFound)
	}
}

func TestRealFS882Inventory(t *testing.T) {
	root := os.Getenv("ACCESS_EXPORT_DIR")
	if root == "" {
		t.Skip("set ACCESS_EXPORT_DIR to run against local Access source exports")
	}
	inventory, err := Build(root)
	if err != nil {
		t.Fatal(err)
	}
	forms, err := inventory.ReachableForms("FS882-6x4XL")
	if err != nil {
		t.Fatal(err)
	}
	if len(forms) < 12 {
		t.Fatalf("expected FS882 parent plus vegetation/soils/other/audit/picture children, got %d", len(forms))
	}
	rootForm := forms[0]
	var plotFound, siteFound, vegOtherFound bool
	named := 0
	for _, control := range rootForm.Controls {
		if control.Name != "" && control.Type != "Form" {
			named++
		}
		if control.Name == "PlotNumber" {
			plotFound = control.Properties["ControlSource"].Value == "PlotNumber"
		}
		if strings.EqualFold(control.Type, "Page") {
			caption := strings.ReplaceAll(control.Properties["Caption"].Value, "&", "")
			if caption == "Site" {
				siteFound = control.ParentID == "form:FS882-6x4XL/tabPages"
			}
			if caption == "Veg Other" {
				vegOtherFound = control.ParentID == "form:FS882-6x4XL/tabPages"
			}
		}
		if control.ParentID == control.ID {
			t.Errorf("self-parent control: %s", control.ID)
		}
	}
	if named < 304 || !plotFound || !siteFound || !vegOtherFound {
		t.Fatalf("incomplete XL extraction: named=%d plot=%v site=%v vegOther=%v", named, plotFound, siteFound, vegOtherFound)
	}
}
