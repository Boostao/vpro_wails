package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestProjectRegistrationUsesOneImmutableDefaultOffCapability(t *testing.T) {
	for _, item := range []struct {
		value   string
		present bool
		enabled bool
		valid   bool
	}{
		{"", false, false, true}, {"false", true, false, true}, {"true", true, true, true},
		{"", true, false, false}, {"1", true, false, false}, {"TRUE", true, false, false}, {" true", true, false, false},
	} {
		enabled, err := siviFeature(longVegetationProjectScopeFeatureEnvironment, func(name string) (string, bool) {
			if name != "VPRO_LONG_VEGETATION_PROJECT_SCOPE" {
				t.Fatal("project scope borrowed another capability", name)
			}
			return item.value, item.present
		})
		if enabled != item.enabled || (err == nil) != item.valid {
			t.Fatal("project scope default/strict feature semantics changed", item, enabled, err)
		}
		if !enabled {
			scope := NewLongVegetationReportScopeService(nil, enabled)
			if value, err := scope.GetLongVegetationReportScope(context.Background(), "owned"); err == nil || value != nil {
				t.Fatal("disabled registration authorized scope", value, err)
			}
			workbook := NewLongVegetationProjectWorkbookService(nil, enabled, true)
			if value, err := workbook.GetReview(context.Background(), "owned", `{}`); err == nil || value != nil {
				t.Fatal("workbook bypassed disabled project scope", value, err)
			}
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var flagReads int
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
		if name.Name == "siviFeature" && len(call.Args) == 2 {
			if argument, ok := call.Args[0].(*ast.Ident); ok && argument.Name == "longVegetationProjectScopeFeatureEnvironment" {
				flagReads++
			}
		}
		if name.Name != "NewLongVegetationReportScopeService" && name.Name != "NewLongVegetationProjectWorkbookService" {
			return true
		}
		for _, argument := range call.Args {
			switch value := argument.(type) {
			case *ast.Ident:
				constructors[name.Name] = append(constructors[name.Name], value.Name)
			case *ast.SelectorExpr:
				inner, ok := value.X.(*ast.SelectorExpr)
				if !ok {
					t.Fatal("workbook authorization no longer reuses the owned workbook capability")
				}
				root, ok := inner.X.(*ast.Ident)
				if !ok {
					t.Fatal("workbook capability has an unknown owner")
				}
				constructors[name.Name] = append(constructors[name.Name], root.Name+"."+inner.Sel.Name+"."+value.Sel.Name)
			default:
				t.Fatal("project service registration inferred or hardcoded capability")
			}
		}
		return true
	})
	if flagReads != 1 || !reflect.DeepEqual(constructors, map[string][]string{
		"NewLongVegetationReportScopeService":     {"contextService", "longVegetationProjectScopeEnabled"},
		"NewLongVegetationProjectWorkbookService": {"contextService", "longVegetationProjectScopeEnabled", "vegetationWorkbook.private.enabled"},
	}) {
		t.Fatal("project service registration does not share one immutable default-off capability", flagReads, constructors)
	}
}
