package gomegahandler

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

const (
	importPath = `"github.com/onsi/gomega"`
)

// Handler provide different handling, depend on the way gomega was imported, whether
// in imported with "." name, custom name or without any name.
type Handler interface {
	// GetActualFuncName returns the name of the gomega function, e.g. `Expect`
	GetGomegaBasicInfo(*ast.CallExpr) (*GomegaBasicInfo, bool)
	// ReplaceFunction replaces the function with another one, for fix suggestions
	ReplaceFunction(*ast.CallExpr, *ast.Ident)

	GetActualExpr(assertionFunc *ast.SelectorExpr) *ast.CallExpr

	GetActualExprClone(origFunc, funcClone *ast.SelectorExpr) *ast.CallExpr

	GetNewWrapperMatcher(name string, existing *ast.CallExpr) *ast.CallExpr
}

type GomegaBasicInfo struct {
	MethodName     string
	UseGomegaVar   bool
	HasErrorMethod bool
}

// GetGomegaHandler returns a gomegar handler according to the way gomega was imported in the specific file
func GetGomegaHandler(file *ast.File, pass *analysis.Pass) Handler {
	for _, imp := range file.Imports {
		if imp.Path.Value != importPath {
			continue
		}

		switch name := imp.Name.String(); {
		case name == ".":
			return &dotHandler{
				pass: pass,
			}
		case name == "<nil>": // import with no local name
			return &nameHandler{name: "gomega", pass: pass}
		default:
			return &nameHandler{name: name, pass: pass}
		}
	}

	return nil // no gomega import; this file does not use gomega
}
