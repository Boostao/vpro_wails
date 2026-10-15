package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestPlotLabelPDFRegistrationUsesIndependentDefaultOffCapability(t *testing.T) {
	for _, item := range []struct {
		value   string
		present bool
		enabled bool
		valid   bool
	}{
		{"", false, false, true}, {"false", true, false, true}, {"true", true, true, true},
		{"", true, false, false}, {"1", true, false, false}, {"TRUE", true, false, false}, {" true", true, false, false},
	} {
		enabled, err := siviFeature(plotLabelPDFExportEnvironment, func(name string) (string, bool) {
			if name != "VPRO_PLOT_LABEL_PDF_EXPORT" {
				t.Fatal("label PDF borrowed another capability", name)
			}
			return item.value, item.present
		})
		if enabled != item.enabled || (err == nil) != item.valid {
			t.Fatal("label PDF default/strict feature semantics changed", item, enabled, err)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var flagReads int
	var registrations [][]string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "siviFeature" && len(call.Args) == 2 {
			if argument, ok := call.Args[0].(*ast.Ident); ok && argument.Name == "plotLabelPDFExportEnvironment" {
				flagReads++
			}
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); !ok || selector.Sel.Name != "NewService" || len(call.Args) != 1 {
			return true
		}
		constructor, ok := call.Args[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := constructor.Fun.(*ast.Ident)
		if !ok || name.Name != "NewPlotLabelPDFExportService" {
			return true
		}
		var arguments []string
		for _, argument := range constructor.Args {
			name, ok := argument.(*ast.Ident)
			if !ok {
				t.Fatal("label PDF registration hardcodes or infers its capability")
			}
			arguments = append(arguments, name.Name)
		}
		registrations = append(registrations, arguments)
		return true
	})
	if flagReads != 1 || !reflect.DeepEqual(registrations, [][]string{{"contextService", "plotLabelPDFExportEnabled"}}) {
		t.Fatal("label PDF service is not independently registered exactly once", flagReads, registrations)
	}
}
