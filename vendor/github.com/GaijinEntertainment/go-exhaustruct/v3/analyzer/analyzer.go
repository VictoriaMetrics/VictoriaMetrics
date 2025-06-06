package analyzer

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/GaijinEntertainment/go-exhaustruct/v3/internal/comment"
	"github.com/GaijinEntertainment/go-exhaustruct/v3/internal/pattern"
	"github.com/GaijinEntertainment/go-exhaustruct/v3/internal/structure"
)

type analyzer struct {
	include pattern.List `exhaustruct:"optional"`
	exclude pattern.List `exhaustruct:"optional"`

	structFields structure.FieldsCache `exhaustruct:"optional"`
	comments     comment.Cache         `exhaustruct:"optional"`

	typeProcessingNeed   map[string]bool
	typeProcessingNeedMu sync.RWMutex `exhaustruct:"optional"`
}

func NewAnalyzer(include, exclude []string) (*analysis.Analyzer, error) {
	a := analyzer{
		typeProcessingNeed: make(map[string]bool),
		comments:           comment.Cache{},
	}

	var err error

	a.include, err = pattern.NewList(include...)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	a.exclude, err = pattern.NewList(exclude...)
	if err != nil {
		return nil, err //nolint:wrapcheck
	}

	return &analysis.Analyzer{ //nolint:exhaustruct
		Name:     "exhaustruct",
		Doc:      "Checks if all structure fields are initialized",
		Run:      a.run,
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Flags:    a.newFlagSet(),
	}, nil
}

func (a *analyzer) newFlagSet() flag.FlagSet {
	fs := flag.NewFlagSet("", flag.PanicOnError)

	fs.Var(&a.include, "i", `Regular expression to match type names, can receive multiple flags.
Anonymous structs can be matched by '<anonymous>' alias.
4ex: 
	github.com/GaijinEntertainment/go-exhaustruct/v3/analyzer\.<anonymous>
	github.com/GaijinEntertainment/go-exhaustruct/v3/analyzer\.TypeInfo`)
	fs.Var(&a.exclude, "e", `Regular expression to exclude type names, can receive multiple flags.
Anonymous structs can be matched by '<anonymous>' alias.
4ex: 
	github.com/GaijinEntertainment/go-exhaustruct/v3/analyzer\.<anonymous>
	github.com/GaijinEntertainment/go-exhaustruct/v3/analyzer\.TypeInfo`)

	return *fs
}

func (a *analyzer) run(pass *analysis.Pass) (any, error) {
	insp := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector) //nolint:forcetypeassert

	insp.WithStack([]ast.Node{(*ast.CompositeLit)(nil)}, a.newVisitor(pass))

	return nil, nil //nolint:nilnil
}

// newVisitor returns visitor that only expects [ast.CompositeLit] nodes.
func (a *analyzer) newVisitor(pass *analysis.Pass) func(n ast.Node, push bool, stack []ast.Node) bool {
	return func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}

		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			// this should never happen, but better be prepared
			return true
		}

		structTyp, typeInfo, ok := getStructType(pass, lit)
		if !ok {
			return true
		}

		if len(lit.Elts) == 0 {
			if ret, ok := stackParentIsReturn(stack); ok {
				if returnContainsNonNilError(pass, ret, n) {
					// it is okay to return uninitialized structure in case struct's direct parent is
					// a return statement containing non-nil error
					return true
				}
			}
		}

		file := a.comments.Get(pass.Fset, stack[0].(*ast.File)) //nolint:forcetypeassert
		rc := getCompositeLitRelatedComments(stack, file)
		pos, msg := a.processStruct(pass, lit, structTyp, typeInfo, rc)

		if pos != nil {
			pass.Reportf(*pos, msg)
		}

		return true
	}
}

// getCompositeLitRelatedComments returns all comments that are related to checked node. We
// have to traverse the stack manually as ast do not associate comments with
// [ast.CompositeLit].
func getCompositeLitRelatedComments(stack []ast.Node, cm ast.CommentMap) []*ast.CommentGroup {
	comments := make([]*ast.CommentGroup, 0)

	for i := len(stack) - 1; i >= 0; i-- {
		node := stack[i]

		switch node.(type) {
		case *ast.CompositeLit, // stack[len(stack)-1]
			*ast.ReturnStmt, // return ...
			*ast.IndexExpr,  // map[enum]...{...}[key]
			*ast.CallExpr,   // myfunc(map...)
			*ast.UnaryExpr,  // &map...
			*ast.AssignStmt, // variable assignment (without var keyword)
			*ast.DeclStmt,   // var declaration, parent of *ast.GenDecl
			*ast.GenDecl,    // var declaration, parent of *ast.ValueSpec
			*ast.ValueSpec:  // var declaration
			comments = append(comments, cm[node]...)

		default:
			return comments
		}
	}

	return comments
}

func getStructType(pass *analysis.Pass, lit *ast.CompositeLit) (*types.Struct, *TypeInfo, bool) {
	switch typ := types.Unalias(pass.TypesInfo.TypeOf(lit)).(type) {
	case *types.Named: // named type
		if structTyp, ok := typ.Underlying().(*types.Struct); ok {
			pkg := typ.Obj().Pkg()
			ti := TypeInfo{
				Name:        typ.Obj().Name(),
				PackageName: pkg.Name(),
				PackagePath: pkg.Path(),
			}

			return structTyp, &ti, true
		}

		return nil, nil, false

	case *types.Struct: // anonymous struct
		ti := TypeInfo{
			Name:        "<anonymous>",
			PackageName: pass.Pkg.Name(),
			PackagePath: pass.Pkg.Path(),
		}

		return typ, &ti, true

	default:
		return nil, nil, false
	}
}

func stackParentIsReturn(stack []ast.Node) (*ast.ReturnStmt, bool) {
	// it is safe to skip boundary check, since stack always has at least one element
	// we also have no reason to check the first element, since it is always a file
	for i := len(stack) - 2; i > 0; i-- {
		switch st := stack[i].(type) {
		case *ast.ReturnStmt:
			return st, true

		case *ast.UnaryExpr:
			// in case we're dealing with pointers - it is still viable to check pointer's
			// parent for return statement
			continue

		default:
			return nil, false
		}
	}

	return nil, false
}

// errorIface is a type that represents [error] interface and all types will be
// compared against.
var errorIface = types.Universe.Lookup("error").Type().Underlying().(*types.Interface)

func returnContainsNonNilError(pass *analysis.Pass, ret *ast.ReturnStmt, except ast.Node) bool {
	// errors are mostly located at the end of return statement, so we're starting
	// from the end.
	for i := len(ret.Results) - 1; i >= 0; i-- {
		ri := ret.Results[i]

		// skip current node
		if ri == except {
			continue
		}

		if un, ok := ri.(*ast.UnaryExpr); ok {
			if un.X == except {
				continue
			}
		}

		if types.Implements(pass.TypesInfo.TypeOf(ri), errorIface) {
			return true
		}
	}

	return false
}

func (a *analyzer) processStruct(
	pass *analysis.Pass,
	lit *ast.CompositeLit,
	structTyp *types.Struct,
	info *TypeInfo,
	comments []*ast.CommentGroup,
) (*token.Pos, string) {
	shouldProcess := a.shouldProcessType(info)

	if shouldProcess && comment.HasDirective(comments, comment.DirectiveIgnore) {
		return nil, ""
	}

	if !shouldProcess && !comment.HasDirective(comments, comment.DirectiveEnforce) {
		return nil, ""
	}

	// unnamed structures are only defined in same package, along with types that has
	// prefix identical to current package name.
	isSamePackage := info.PackagePath == pass.Pkg.Path()

	if f := a.litSkippedFields(lit, structTyp, !isSamePackage); len(f) > 0 {
		pos := lit.Pos()

		if len(f) == 1 {
			return &pos, fmt.Sprintf("%s is missing field %s", info.ShortString(), f.String())
		}

		return &pos, fmt.Sprintf("%s is missing fields %s", info.ShortString(), f.String())
	}

	return nil, ""
}

// shouldProcessType returns true if type should be processed basing off include
// and exclude patterns, defined though constructor and\or flags.
func (a *analyzer) shouldProcessType(info *TypeInfo) bool {
	if len(a.include) == 0 && len(a.exclude) == 0 {
		return true
	}

	name := info.String()

	a.typeProcessingNeedMu.RLock()
	res, ok := a.typeProcessingNeed[name]
	a.typeProcessingNeedMu.RUnlock()

	if !ok {
		a.typeProcessingNeedMu.Lock()
		res = true

		if a.include != nil && !a.include.MatchFullString(name) {
			res = false
		}

		if res && a.exclude != nil && a.exclude.MatchFullString(name) {
			res = false
		}

		a.typeProcessingNeed[name] = res
		a.typeProcessingNeedMu.Unlock()
	}

	return res
}

func (a *analyzer) litSkippedFields(
	lit *ast.CompositeLit,
	typ *types.Struct,
	onlyExported bool,
) structure.Fields {
	return a.structFields.Get(typ).Skipped(lit, onlyExported)
}

type TypeInfo struct {
	Name        string
	PackageName string
	PackagePath string
}

func (t TypeInfo) String() string {
	return t.PackagePath + "." + t.Name
}

func (t TypeInfo) ShortString() string {
	return t.PackageName + "." + t.Name
}
