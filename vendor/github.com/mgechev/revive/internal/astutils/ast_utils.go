// Package astutils provides utility functions for working with AST nodes
package astutils

import (
	"go/ast"
)

// FuncSignatureIs returns true if the given func decl satisfies a signature characterized
// by the given name, parameters types and return types; false otherwise.
//
// Example: to check if a function declaration has the signature Foo(int, string) (bool,error)
// call to FuncSignatureIs(funcDecl,"Foo",[]string{"int","string"},[]string{"bool","error"})
func FuncSignatureIs(funcDecl *ast.FuncDecl, wantName string, wantParametersTypes, wantResultsTypes []string) bool {
	if wantName != funcDecl.Name.String() {
		return false // func name doesn't match expected one
	}

	funcParametersTypes := getTypeNames(funcDecl.Type.Params)
	if len(wantParametersTypes) != len(funcParametersTypes) {
		return false // func has not the expected number of parameters
	}

	funcResultsTypes := getTypeNames(funcDecl.Type.Results)
	if len(wantResultsTypes) != len(funcResultsTypes) {
		return false // func has not the expected number of return values
	}

	for i, wantType := range wantParametersTypes {
		if wantType != funcParametersTypes[i] {
			return false // type of a func's parameter does not match the type of the corresponding expected parameter
		}
	}

	for i, wantType := range wantResultsTypes {
		if wantType != funcResultsTypes[i] {
			return false // type of a func's return value does not match the type of the corresponding expected return value
		}
	}

	return true
}

func getTypeNames(fields *ast.FieldList) []string {
	result := []string{}

	if fields == nil {
		return result
	}

	for _, field := range fields.List {
		typeName := getFieldTypeName(field.Type)
		if field.Names == nil { // unnamed field
			result = append(result, typeName)
			continue
		}

		for range field.Names { // add one type name for each field name
			result = append(result, typeName)
		}
	}

	return result
}

func getFieldTypeName(typ ast.Expr) string {
	switch f := typ.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name + "." + getFieldTypeName(f.X)
	case *ast.StarExpr:
		return "*" + getFieldTypeName(f.X)
	case *ast.IndexExpr:
		return getFieldTypeName(f.X) + "[" + getFieldTypeName(f.Index) + "]"
	case *ast.ArrayType:
		return "[]" + getFieldTypeName(f.Elt)
	case *ast.InterfaceType:
		return "interface{}"
	default:
		return "UNHANDLED_TYPE"
	}
}
