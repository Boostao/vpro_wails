package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestPlotLabelPDFRegistrationIsUnconditionallyRegisteredOnImportExportService(t *testing.T) {
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	domainFile, err := parser.ParseFile(fset, "domainservices_importexport.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var builderArgs [][]string
	var domainRegistered int
	ast.Inspect(mainFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "buildImportExportService" {
			var args []string
			for _, arg := range call.Args {
				ident, ok := arg.(*ast.Ident)
				if !ok {
					t.Fatal("buildImportExportService uses unexpected argument expression")
				}
				args = append(args, ident.Name)
			}
			builderArgs = append(builderArgs, args)
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewService" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "importExportService" {
				domainRegistered++
			}
		}
		return true
	})
	var constructorArgs [][]string
	ast.Inspect(domainFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "NewImportExportService" && len(call.Args) == 4 {
			constructor, ok := call.Args[3].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := constructor.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewPlotLabelPDFExportService" {
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
		return true
	})
	if domainRegistered != 1 || !reflect.DeepEqual(builderArgs, [][]string{{"contextService"}}) || !reflect.DeepEqual(constructorArgs, [][]string{{"core"}}) {
		t.Fatal("label PDF service is not unconditionally registered on ImportExportService once", domainRegistered, builderArgs, constructorArgs)
	}
}
