package paralleltest

import (
	"flag"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/ast/inspector"
)

const Doc = `check that tests use t.Parallel() method
It also checks that the t.Parallel is used if multiple tests cases are run as part of single test.
As part of ensuring parallel tests works as expected it checks for reinitialising of the range value
over the test cases.(https://tinyurl.com/y6555cy6)`

func NewAnalyzer() *analysis.Analyzer {
	return newParallelAnalyzer().analyzer
}

// parallelAnalyzer is an internal analyzer that makes options available to a
// run pass. It wraps an `analysis.Analyzer` that should be returned for
// linters.
type parallelAnalyzer struct {
	analyzer              *analysis.Analyzer
	ignoreMissing         bool
	ignoreMissingSubtests bool
	ignoreLoopVar         bool
}

func newParallelAnalyzer() *parallelAnalyzer {
	a := &parallelAnalyzer{}

	var flags flag.FlagSet
	flags.BoolVar(&a.ignoreMissing, "i", false, "ignore missing calls to t.Parallel")
	flags.BoolVar(&a.ignoreMissingSubtests, "ignoremissingsubtests", false, "ignore missing calls to t.Parallel in subtests")
	flags.BoolVar(&a.ignoreLoopVar, "ignoreloopVar", false, "ignore loop variable detection")

	a.analyzer = &analysis.Analyzer{
		Name:  "paralleltest",
		Doc:   Doc,
		Run:   a.run,
		Flags: flags,
	}
	return a
}

func (a *parallelAnalyzer) run(pass *analysis.Pass) (interface{}, error) {
	inspector := inspector.New(pass.Files)

	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}

	inspector.Preorder(nodeFilter, func(node ast.Node) {
		funcDecl := node.(*ast.FuncDecl)
		var funcHasParallelMethod,
			funcCantParallelMethod,
			rangeStatementOverTestCasesExists,
			rangeStatementHasParallelMethod,
			rangeStatementCantParallelMethod bool
		var loopVariableUsedInRun *string
		var numberOfTestRun int
		var positionOfTestRunNode []ast.Node
		var rangeNode ast.Node

		// Check runs for test functions only
		isTest, testVar := isTestFunction(funcDecl)
		if !isTest {
			return
		}

		for _, l := range funcDecl.Body.List {
			switch v := l.(type) {

			case *ast.ExprStmt:
				ast.Inspect(v, func(n ast.Node) bool {
					// Check if the test method is calling t.Parallel
					if !funcHasParallelMethod {
						funcHasParallelMethod = methodParallelIsCalledInTestFunction(n, testVar)
					}

					// Check if the test calls t.Setenv, cannot be used in parallel tests or tests with parallel ancestors
					if !funcCantParallelMethod {
						funcCantParallelMethod = methodSetenvIsCalledInTestFunction(n, testVar)
					}

					// Check if the t.Run within the test function is calling t.Parallel
					if methodRunIsCalledInTestFunction(n, testVar) {
						// n is a call to t.Run; find out the name of the subtest's *testing.T parameter.
						innerTestVar := getRunCallbackParameterName(n)

						hasParallel := false
						cantParallel := false
						numberOfTestRun++
						ast.Inspect(v, func(p ast.Node) bool {
							if !hasParallel {
								hasParallel = methodParallelIsCalledInTestFunction(p, innerTestVar)
							}
							if !cantParallel {
								cantParallel = methodSetenvIsCalledInTestFunction(p, innerTestVar)
							}
							return true
						})
						if !hasParallel && !cantParallel {
							positionOfTestRunNode = append(positionOfTestRunNode, n)
						}
					}
					return true
				})

			// Check if the range over testcases is calling t.Parallel
			case *ast.RangeStmt:
				rangeNode = v

				var loopVars []types.Object
				for _, expr := range []ast.Expr{v.Key, v.Value} {
					if id, ok := expr.(*ast.Ident); ok {
						loopVars = append(loopVars, pass.TypesInfo.ObjectOf(id))
					}
				}

				ast.Inspect(v, func(n ast.Node) bool {
					// nolint: gocritic
					switch r := n.(type) {
					case *ast.ExprStmt:
						if methodRunIsCalledInRangeStatement(r.X, testVar) {
							// r.X is a call to t.Run; find out the name of the subtest's *testing.T parameter.
							innerTestVar := getRunCallbackParameterName(r.X)

							rangeStatementOverTestCasesExists = true

							if !rangeStatementHasParallelMethod {
								rangeStatementHasParallelMethod = methodParallelIsCalledInMethodRun(r.X, innerTestVar)
							}

							if !rangeStatementCantParallelMethod {
								rangeStatementCantParallelMethod = methodSetenvIsCalledInMethodRun(r.X, innerTestVar)
							}

							if !a.ignoreLoopVar && loopVariableUsedInRun == nil {
								if run, ok := r.X.(*ast.CallExpr); ok {
									loopVariableUsedInRun = loopVarReferencedInRun(run, loopVars, pass.TypesInfo)
								}
							}
						}
					}
					return true
				})
			}
		}

		// Descendents which call Setenv, also prevent tests from calling Parallel
		if rangeStatementCantParallelMethod {
			funcCantParallelMethod = true
		}

		if !a.ignoreMissing && !funcHasParallelMethod && !funcCantParallelMethod {
			pass.Reportf(node.Pos(), "Function %s missing the call to method parallel\n", funcDecl.Name.Name)
		}

		if rangeStatementOverTestCasesExists && rangeNode != nil {
			if !rangeStatementHasParallelMethod && !rangeStatementCantParallelMethod {
				if !a.ignoreMissing && !a.ignoreMissingSubtests {
					pass.Reportf(rangeNode.Pos(), "Range statement for test %s missing the call to method parallel in test Run\n", funcDecl.Name.Name)
				}
			} else if loopVariableUsedInRun != nil {
				pass.Reportf(rangeNode.Pos(), "Range statement for test %s does not reinitialise the variable %s\n", funcDecl.Name.Name, *loopVariableUsedInRun)
			}
		}

		// Check if the t.Run is more than one as there is no point making one test parallel
		if !a.ignoreMissing && !a.ignoreMissingSubtests {
			if numberOfTestRun > 1 && len(positionOfTestRunNode) > 0 {
				for _, n := range positionOfTestRunNode {
					pass.Reportf(n.Pos(), "Function %s missing the call to method parallel in the test run\n", funcDecl.Name.Name)
				}
			}
		}
	})

	return nil, nil
}

func methodParallelIsCalledInMethodRun(node ast.Node, testVar string) bool {
	return targetMethodIsCalledInMethodRun(node, testVar, "Parallel")
}

func methodSetenvIsCalledInMethodRun(node ast.Node, testVar string) bool {
	return targetMethodIsCalledInMethodRun(node, testVar, "Setenv")
}

func targetMethodIsCalledInMethodRun(node ast.Node, testVar, targetMethod string) bool {
	var called bool
	// nolint: gocritic
	switch callExp := node.(type) {
	case *ast.CallExpr:
		for _, arg := range callExp.Args {
			if !called {
				ast.Inspect(arg, func(n ast.Node) bool {
					if !called {
						called = exprCallHasMethod(n, testVar, targetMethod)
						return true
					}
					return false
				})
			}
		}
	}
	return called
}

func methodParallelIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Parallel")
}

func methodRunIsCalledInRangeStatement(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Run")
}

func methodRunIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Run")
}

func methodSetenvIsCalledInTestFunction(node ast.Node, testVar string) bool {
	return exprCallHasMethod(node, testVar, "Setenv")
}

func exprCallHasMethod(node ast.Node, receiverName, methodName string) bool {
	// nolint: gocritic
	switch n := node.(type) {
	case *ast.CallExpr:
		if fun, ok := n.Fun.(*ast.SelectorExpr); ok {
			if receiver, ok := fun.X.(*ast.Ident); ok {
				return receiver.Name == receiverName && fun.Sel.Name == methodName
			}
		}
	}
	return false
}

// In an expression of the form t.Run(x, func(q *testing.T) {...}), return the
// value "q". In _most_ code, the name is probably t, but we shouldn't just
// assume.
func getRunCallbackParameterName(node ast.Node) string {
	if n, ok := node.(*ast.CallExpr); ok {
		if len(n.Args) < 2 {
			// We want argument #2, but this call doesn't have two
			// arguments. Maybe it's not really t.Run.
			return ""
		}
		funcArg := n.Args[1]
		if fun, ok := funcArg.(*ast.FuncLit); ok {
			if len(fun.Type.Params.List) < 1 {
				// Subtest function doesn't have any parameters.
				return ""
			}
			firstArg := fun.Type.Params.List[0]
			// We'll assume firstArg.Type is *testing.T.
			if len(firstArg.Names) < 1 {
				return ""
			}
			return firstArg.Names[0].Name
		}
	}
	return ""
}

// Checks if the function has the param type *testing.T; if it does, then the
// parameter name is returned, too.
func isTestFunction(funcDecl *ast.FuncDecl) (bool, string) {
	testMethodPackageType := "testing"
	testMethodStruct := "T"
	testPrefix := "Test"

	if !strings.HasPrefix(funcDecl.Name.Name, testPrefix) {
		return false, ""
	}

	if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) != 1 {
		return false, ""
	}

	param := funcDecl.Type.Params.List[0]
	if starExp, ok := param.Type.(*ast.StarExpr); ok {
		if selectExpr, ok := starExp.X.(*ast.SelectorExpr); ok {
			if selectExpr.Sel.Name == testMethodStruct {
				if s, ok := selectExpr.X.(*ast.Ident); ok {
					if len(param.Names) > 0 {
						return s.Name == testMethodPackageType, param.Names[0].Name
					}
				}
			}
		}
	}

	return false, ""
}

func loopVarReferencedInRun(call *ast.CallExpr, vars []types.Object, typeInfo *types.Info) (found *string) {
	if len(call.Args) != 2 {
		return
	}

	ast.Inspect(call.Args[1], func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		for _, o := range vars {
			if typeInfo.ObjectOf(ident) == o {
				found = &ident.Name
			}
		}
		return true
	})

	return
}
