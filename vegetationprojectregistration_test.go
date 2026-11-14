package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestProjectRegistrationUnconditionalDefaultOn(t *testing.T) {
	scope := NewLongVegetationReportScopeService(nil)
	if value, err := scope.GetLongVegetationReportScope(context.Background(), "owned"); err == nil || value != nil {
		t.Fatal("unconfigured scope service authorized scope", value, err)
	}
	workbook := NewLongVegetationProjectWorkbookService(nil)
	if value, err := workbook.GetReview(context.Background(), "owned", `{}`); err == nil || value != nil {
		t.Fatal("unconfigured workbook service authorized review", value, err)
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	constructors := map[string][]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		if name.Name != "NewLongVegetationReportScopeService" && name.Name != "NewLongVegetationProjectWorkbookService" {
			return true
		}
		for _, argument := range call.Args {
			ident, ok := argument.(*ast.Ident)
			if !ok {
				t.Fatal("project service registration uses unexpected argument expression")
			}
			constructors[name.Name] = append(constructors[name.Name], ident.Name)
		}
		return true
	})
	if !reflect.DeepEqual(constructors, map[string][]string{
		"NewLongVegetationReportScopeService":     {"contextService"},
		"NewLongVegetationProjectWorkbookService": {"contextService"},
	}) {
		t.Fatal("project service registration does not unconditionally wire contextService", constructors)
	}
}
