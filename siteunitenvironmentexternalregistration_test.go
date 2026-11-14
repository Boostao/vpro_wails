package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestExternalSUEnvironmentRegistrationUnconditionalDefaultOn(t *testing.T) {
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	domainFile, err := parser.ParseFile(fset, "domainservices_classification.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var builderCalls int
	var domainRegistered int
	ast.Inspect(mainFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "buildClassificationService" && len(call.Args) == 3 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "contextService" {
				builderCalls++
			}
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewService" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "classificationService" {
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
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "NewClassificationService" && len(call.Args) == 6 {
			constructor, ok := call.Args[3].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := constructor.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "NewExternalSUEnvironmentService" {
				return true
			}
			var arguments []string
			for _, argument := range constructor.Args {
				ident, ok := argument.(*ast.Ident)
				if !ok {
					t.Fatal("external transfer registration uses unexpected argument expression")
				}
				arguments = append(arguments, ident.Name)
			}
			constructorArgs = append(constructorArgs, arguments)
		}
		return true
	})
	if domainRegistered != 1 || builderCalls != 1 || !reflect.DeepEqual(constructorArgs, [][]string{{"core"}}) {
		t.Fatal("external transfer not unconditionally registered on ClassificationService once", domainRegistered, builderCalls, constructorArgs)
	}
}
