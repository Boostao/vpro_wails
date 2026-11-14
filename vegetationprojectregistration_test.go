package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	vegreportpkg "github.com/boostao/vpro-wails/internal/vegreport"
)

func TestProjectRegistrationUnconditionalDefaultOn(t *testing.T) {
	scope := vegreportpkg.NewLongVegetationReportScopeService(nil)
	if value, err := scope.GetLongVegetationReportScope(context.Background(), "owned"); err == nil || value != nil {
		t.Fatal("unconfigured scope service authorized scope", value, err)
	}
	workbook := vegreportpkg.NewLongVegetationProjectWorkbookService(nil)
	if value, err := workbook.GetReview(context.Background(), "owned", `{}`); err == nil || value != nil {
		t.Fatal("unconfigured workbook service authorized review", value, err)
	}
	fset := token.NewFileSet()
	mainFile, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	domainFile, err := parser.ParseFile(fset, "domainservices_vegreport.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var builderCalls int
	ast.Inspect(mainFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "buildVegetationReportService" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "contextService" {
				builderCalls++
			}
		}
		return true
	})
	constructors := map[string][]string{}
	ast.Inspect(domainFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if sel.Sel.Name != "NewLongVegetationReportScopeService" && sel.Sel.Name != "NewLongVegetationProjectWorkbookService" {
			return true
		}
		for _, argument := range call.Args {
			ident, ok := argument.(*ast.Ident)
			if !ok {
				t.Fatal("project service registration uses unexpected argument expression")
			}
			constructors[sel.Sel.Name] = append(constructors[sel.Sel.Name], ident.Name)
		}
		return true
	})
	if builderCalls != 1 || !reflect.DeepEqual(constructors, map[string][]string{
		"NewLongVegetationReportScopeService":     {"core"},
		"NewLongVegetationProjectWorkbookService": {"core"},
	}) {
		t.Fatal("project service registration does not unconditionally wire contextService", builderCalls, constructors)
	}
}
