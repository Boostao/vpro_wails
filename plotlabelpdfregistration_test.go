package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestPlotLabelPDFRegistrationIsUnconditionallyRegisteredOnImportExportService(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var constructorArgs [][]string
	var domainRegistered int
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "NewImportExportService" && len(call.Args) == 4 {
			constructor, ok := call.Args[3].(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := constructor.Fun.(*ast.Ident)
			if !ok || fn.Name != "NewPlotLabelPDFExportService" {
				return true
			}
			var arguments []string
			for _, argument := range constructor.Args {
				ident, ok := argument.(*ast.Ident)
				if !ok {
					t.Fatal("label PDF registration uses unexpected argument expression")
				}
				arguments = append(arguments, ident.Name)
			}
			constructorArgs = append(constructorArgs, arguments)
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewService" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "importExportService" {
				domainRegistered++
			}
		}
		return true
	})
	if domainRegistered != 1 || !reflect.DeepEqual(constructorArgs, [][]string{{"contextService"}}) {
		t.Fatal("label PDF service is not unconditionally registered on ImportExportService once", domainRegistered, constructorArgs)
	}
}
