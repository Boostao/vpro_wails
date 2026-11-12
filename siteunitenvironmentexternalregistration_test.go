package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"
)

func TestExternalSUEnvironmentRegistrationIndependentDefaultOff(t *testing.T) {
	for _, item := range []struct {
		value   string
		present bool
		enabled bool
		valid   bool
	}{
		{"", false, true, true}, {"false", true, false, true}, {"true", true, true, true},
		{"", true, false, false}, {"1", true, false, false}, {"TRUE", true, false, false}, {" true", true, false, false},
	} {
		enabled, err := siviFeature(externalSUEnvironmentEditingEnvironment, func(name string) (string, bool) {
			if name != "VPRO_EXTERNAL_SU_ENVIRONMENT_EDITING" {
				t.Fatal("external transfer borrowed another capability", name)
			}
			return item.value, item.present
		})
		if enabled != item.enabled || (err == nil) != item.valid {
			t.Fatal("external transfer feature default/strict parsing changed", item, enabled, err)
		}
	}
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var flagReads int
	var constructorArgs [][]string
	var domainRegistered int
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "siviFeature" && len(call.Args) == 2 {
			if argument, ok := call.Args[0].(*ast.Ident); ok && argument.Name == "externalSUEnvironmentEditingEnvironment" {
				flagReads++
			}
		}
		if name, ok := call.Fun.(*ast.Ident); ok && name.Name == "NewClassificationService" && len(call.Args) == 6 {
			constructor, ok := call.Args[3].(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := constructor.Fun.(*ast.Ident)
			if !ok || fn.Name != "NewExternalSUEnvironmentService" {
				return true
			}
			var arguments []string
			for _, argument := range constructor.Args {
				ident, ok := argument.(*ast.Ident)
				if !ok {
					t.Fatal("external transfer registration infers its capability")
				}
				arguments = append(arguments, ident.Name)
			}
			constructorArgs = append(constructorArgs, arguments)
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "NewService" && len(call.Args) == 1 {
			if ident, ok := call.Args[0].(*ast.Ident); ok && ident.Name == "classificationService" {
				domainRegistered++
			}
		}
		return true
	})
	if flagReads != 1 || domainRegistered != 1 || !reflect.DeepEqual(constructorArgs, [][]string{{"contextService", "externalSUEnvironmentEditingEnabled"}}) {
		t.Fatal("external transfer not independently registered on ClassificationService once", flagReads, domainRegistered, constructorArgs)
	}
}
