// Package sloglint implements the sloglint analyzer.
package sloglint

import (
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"github.com/ettle/strcase"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Options are options for the sloglint analyzer.
type Options struct {
	NoMixedArgs    bool     // Enforce not mixing key-value pairs and attributes (default true).
	KVOnly         bool     // Enforce using key-value pairs only (overrides NoMixedArgs, incompatible with AttrOnly).
	AttrOnly       bool     // Enforce using attributes only (overrides NoMixedArgs, incompatible with KVOnly).
	NoGlobal       string   // Enforce not using global loggers ("all" or "default").
	ContextOnly    string   // Enforce using methods that accept a context ("all" or "scope").
	StaticMsg      bool     // Enforce using static log messages.
	NoRawKeys      bool     // Enforce using constants instead of raw keys.
	KeyNamingCase  string   // Enforce a single key naming convention ("snake", "kebab", "camel", or "pascal").
	ForbiddenKeys  []string // Enforce not using specific keys.
	ArgsOnSepLines bool     // Enforce putting arguments on separate lines.
}

// New creates a new sloglint analyzer.
func New(opts *Options) *analysis.Analyzer {
	if opts == nil {
		opts = &Options{NoMixedArgs: true}
	}

	return &analysis.Analyzer{
		Name:     "sloglint",
		Doc:      "ensure consistent code style when using log/slog",
		Flags:    flags(opts),
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			if opts.KVOnly && opts.AttrOnly {
				return nil, fmt.Errorf("sloglint: Options.KVOnly and Options.AttrOnly: %w", errIncompatible)
			}

			switch opts.NoGlobal {
			case "", "all", "default":
			default:
				return nil, fmt.Errorf("sloglint: Options.NoGlobal=%s: %w", opts.NoGlobal, errInvalidValue)
			}

			switch opts.ContextOnly {
			case "", "all", "scope":
			default:
				return nil, fmt.Errorf("sloglint: Options.ContextOnly=%s: %w", opts.ContextOnly, errInvalidValue)
			}

			switch opts.KeyNamingCase {
			case "", snakeCase, kebabCase, camelCase, pascalCase:
			default:
				return nil, fmt.Errorf("sloglint: Options.KeyNamingCase=%s: %w", opts.KeyNamingCase, errInvalidValue)
			}

			run(pass, opts)
			return nil, nil
		},
	}
}

var (
	errIncompatible = errors.New("incompatible options")
	errInvalidValue = errors.New("invalid value")
)

func flags(opts *Options) flag.FlagSet {
	fset := flag.NewFlagSet("sloglint", flag.ContinueOnError)

	boolVar := func(value *bool, name, usage string) {
		fset.Func(name, usage, func(s string) error {
			v, err := strconv.ParseBool(s)
			*value = v
			return err
		})
	}

	strVar := func(value *string, name, usage string) {
		fset.Func(name, usage, func(s string) error {
			*value = s
			return nil
		})
	}

	boolVar(&opts.NoMixedArgs, "no-mixed-args", "enforce not mixing key-value pairs and attributes (default true)")
	boolVar(&opts.KVOnly, "kv-only", "enforce using key-value pairs only (overrides -no-mixed-args, incompatible with -attr-only)")
	boolVar(&opts.AttrOnly, "attr-only", "enforce using attributes only (overrides -no-mixed-args, incompatible with -kv-only)")
	strVar(&opts.NoGlobal, "no-global", "enforce not using global loggers (all|default)")
	strVar(&opts.ContextOnly, "context-only", "enforce using methods that accept a context (all|scope)")
	boolVar(&opts.StaticMsg, "static-msg", "enforce using static log messages")
	boolVar(&opts.NoRawKeys, "no-raw-keys", "enforce using constants instead of raw keys")
	strVar(&opts.KeyNamingCase, "key-naming-case", "enforce a single key naming convention (snake|kebab|camel|pascal)")
	boolVar(&opts.ArgsOnSepLines, "args-on-sep-lines", "enforce putting arguments on separate lines")

	fset.Func("forbidden-keys", "enforce not using specific keys (comma-separated)", func(s string) error {
		opts.ForbiddenKeys = append(opts.ForbiddenKeys, strings.Split(s, ",")...)
		return nil
	})

	return *fset
}

var slogFuncs = map[string]struct {
	argsPos          int
	skipContextCheck bool
}{
	"log/slog.With":                   {argsPos: 0, skipContextCheck: true},
	"log/slog.Log":                    {argsPos: 3},
	"log/slog.LogAttrs":               {argsPos: 3},
	"log/slog.Debug":                  {argsPos: 1},
	"log/slog.Info":                   {argsPos: 1},
	"log/slog.Warn":                   {argsPos: 1},
	"log/slog.Error":                  {argsPos: 1},
	"log/slog.DebugContext":           {argsPos: 2},
	"log/slog.InfoContext":            {argsPos: 2},
	"log/slog.WarnContext":            {argsPos: 2},
	"log/slog.ErrorContext":           {argsPos: 2},
	"(*log/slog.Logger).With":         {argsPos: 0, skipContextCheck: true},
	"(*log/slog.Logger).Log":          {argsPos: 3},
	"(*log/slog.Logger).LogAttrs":     {argsPos: 3},
	"(*log/slog.Logger).Debug":        {argsPos: 1},
	"(*log/slog.Logger).Info":         {argsPos: 1},
	"(*log/slog.Logger).Warn":         {argsPos: 1},
	"(*log/slog.Logger).Error":        {argsPos: 1},
	"(*log/slog.Logger).DebugContext": {argsPos: 2},
	"(*log/slog.Logger).InfoContext":  {argsPos: 2},
	"(*log/slog.Logger).WarnContext":  {argsPos: 2},
	"(*log/slog.Logger).ErrorContext": {argsPos: 2},
}

var attrFuncs = map[string]struct{}{
	"log/slog.String":   {},
	"log/slog.Int64":    {},
	"log/slog.Int":      {},
	"log/slog.Uint64":   {},
	"log/slog.Float64":  {},
	"log/slog.Bool":     {},
	"log/slog.Time":     {},
	"log/slog.Duration": {},
	"log/slog.Group":    {},
	"log/slog.Any":      {},
}

const (
	snakeCase  = "snake"
	kebabCase  = "kebab"
	camelCase  = "camel"
	pascalCase = "pascal"
)

func run(pass *analysis.Pass, opts *Options) {
	visitor := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	filter := []ast.Node{(*ast.CallExpr)(nil)}

	// WithStack is ~2x slower than Preorder, use it only when stack is needed.
	if opts.ContextOnly == "scope" {
		visitor.WithStack(filter, func(node ast.Node, _ bool, stack []ast.Node) bool {
			visit(pass, opts, node, stack)
			return false
		})
		return
	}

	visitor.Preorder(filter, func(node ast.Node) {
		visit(pass, opts, node, nil)
	})
}

// NOTE: stack is nil if Preorder is used.
func visit(pass *analysis.Pass, opts *Options, node ast.Node, stack []ast.Node) {
	call := node.(*ast.CallExpr)

	fn := typeutil.StaticCallee(pass.TypesInfo, call)
	if fn == nil {
		return
	}

	name := fn.FullName()
	funcInfo, ok := slogFuncs[name]
	if !ok {
		return
	}

	switch opts.NoGlobal {
	case "all":
		if strings.HasPrefix(name, "log/slog.") || isGlobalLoggerUsed(pass.TypesInfo, call.Fun) {
			pass.Reportf(call.Pos(), "global logger should not be used")
		}
	case "default":
		if strings.HasPrefix(name, "log/slog.") {
			pass.Reportf(call.Pos(), "default logger should not be used")
		}
	}

	// NOTE: "With" functions are not checked for context.Context.
	if !funcInfo.skipContextCheck {
		switch opts.ContextOnly {
		case "all":
			typ := pass.TypesInfo.TypeOf(call.Args[0])
			if typ != nil && typ.String() != "context.Context" {
				pass.Reportf(call.Pos(), "%sContext should be used instead", fn.Name())
			}
		case "scope":
			typ := pass.TypesInfo.TypeOf(call.Args[0])
			if typ != nil && typ.String() != "context.Context" && isContextInScope(pass.TypesInfo, stack) {
				pass.Reportf(call.Pos(), "%sContext should be used instead", fn.Name())
			}
		}
	}

	msgPos := funcInfo.argsPos - 1
	// NOTE: "With" functions have no message argument and must be skipped.
	if opts.StaticMsg && msgPos >= 0 && !isStaticMsg(call.Args[msgPos]) {
		pass.Reportf(call.Pos(), "message should be a string literal or a constant")
	}

	// NOTE: we assume that the arguments have already been validated by govet.
	args := call.Args[funcInfo.argsPos:]
	if len(args) == 0 {
		return
	}

	var keys []ast.Expr
	var attrs []ast.Expr

	for i := 0; i < len(args); i++ {
		typ := pass.TypesInfo.TypeOf(args[i])
		if typ == nil {
			continue
		}

		switch typ.String() {
		case "string":
			keys = append(keys, args[i])
			i++ // skip the value.
		case "log/slog.Attr":
			attrs = append(attrs, args[i])
		case "[]any", "[]log/slog.Attr":
			continue // the last argument may be an unpacked slice, skip it.
		}
	}

	switch {
	case opts.KVOnly && len(attrs) > 0:
		pass.Reportf(call.Pos(), "attributes should not be used")
	case opts.AttrOnly && len(keys) > 0:
		pass.Reportf(call.Pos(), "key-value pairs should not be used")
	case opts.NoMixedArgs && len(attrs) > 0 && len(keys) > 0:
		pass.Reportf(call.Pos(), "key-value pairs and attributes should not be mixed")
	}

	if opts.NoRawKeys {
		forEachKey(pass.TypesInfo, keys, attrs, func(key ast.Expr) {
			if selector, ok := key.(*ast.SelectorExpr); ok {
				key = selector.Sel // the key is defined in another package, e.g. pkg.ConstKey.
			}
			isConst := false
			if ident, ok := key.(*ast.Ident); ok {
				if obj := pass.TypesInfo.ObjectOf(ident); obj != nil {
					if _, ok := obj.(*types.Const); ok {
						isConst = true
					}
				}
			}
			if !isConst {
				pass.Reportf(call.Pos(), "raw keys should not be used")
			}
		})
	}

	checkKeyNamingCase := func(caseFn func(string) string, caseName string) {
		forEachKey(pass.TypesInfo, keys, attrs, func(key ast.Expr) {
			if name, ok := getKeyName(key); ok && name != caseFn(name) {
				pass.Reportf(call.Pos(), "keys should be written in %s", caseName)
			}
		})
	}

	switch opts.KeyNamingCase {
	case snakeCase:
		checkKeyNamingCase(strcase.ToSnake, "snake_case")
	case kebabCase:
		checkKeyNamingCase(strcase.ToKebab, "kebab-case")
	case camelCase:
		checkKeyNamingCase(strcase.ToCamel, "camelCase")
	case pascalCase:
		checkKeyNamingCase(strcase.ToPascal, "PascalCase")
	}

	if len(opts.ForbiddenKeys) > 0 {
		forEachKey(pass.TypesInfo, keys, attrs, func(key ast.Expr) {
			if name, ok := getKeyName(key); ok && slices.Contains(opts.ForbiddenKeys, name) {
				pass.Reportf(call.Pos(), "%q key is forbidden and should not be used", name)
			}
		})
	}

	if opts.ArgsOnSepLines && areArgsOnSameLine(pass.Fset, call, keys, attrs) {
		pass.Reportf(call.Pos(), "arguments should be put on separate lines")
	}
}

func isGlobalLoggerUsed(info *types.Info, call ast.Expr) bool {
	selector, ok := call.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	obj := info.ObjectOf(ident)
	return obj.Parent() == obj.Pkg().Scope()
}

func isContextInScope(info *types.Info, stack []ast.Node) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		decl, ok := stack[i].(*ast.FuncDecl)
		if !ok {
			continue
		}
		params := decl.Type.Params
		if len(params.List) == 0 || len(params.List[0].Names) == 0 {
			continue
		}
		typ := info.TypeOf(params.List[0].Names[0])
		if typ != nil && typ.String() == "context.Context" {
			return true
		}
	}
	return false
}

func isStaticMsg(msg ast.Expr) bool {
	switch msg := msg.(type) {
	case *ast.BasicLit: // e.g. slog.Info("msg")
		return msg.Kind == token.STRING
	case *ast.Ident: // e.g. const msg = "msg"; slog.Info(msg)
		return msg.Obj != nil && msg.Obj.Kind == ast.Con
	default:
		return false
	}
}

func forEachKey(info *types.Info, keys, attrs []ast.Expr, fn func(key ast.Expr)) {
	for _, key := range keys {
		fn(key)
	}

	for _, attr := range attrs {
		switch attr := attr.(type) {
		case *ast.CallExpr: // e.g. slog.Int()
			callee := typeutil.StaticCallee(info, attr)
			if callee == nil {
				continue
			}
			if _, ok := attrFuncs[callee.FullName()]; !ok {
				continue
			}
			fn(attr.Args[0])

		case *ast.CompositeLit: // slog.Attr{}
			switch len(attr.Elts) {
			case 1: // slog.Attr{Key: ...} | slog.Attr{Value: ...}
				if kv := attr.Elts[0].(*ast.KeyValueExpr); kv.Key.(*ast.Ident).Name == "Key" {
					fn(kv.Value)
				}
			case 2: // slog.Attr{Key: ..., Value: ...} | slog.Attr{Value: ..., Key: ...} | slog.Attr{..., ...}
				if kv, ok := attr.Elts[0].(*ast.KeyValueExpr); ok && kv.Key.(*ast.Ident).Name == "Key" {
					fn(kv.Value)
				} else if kv, ok := attr.Elts[1].(*ast.KeyValueExpr); ok && kv.Key.(*ast.Ident).Name == "Key" {
					fn(kv.Value)
				} else {
					fn(attr.Elts[0])
				}
			}
		}
	}
}

func getKeyName(key ast.Expr) (string, bool) {
	if ident, ok := key.(*ast.Ident); ok {
		if ident.Obj == nil || ident.Obj.Decl == nil || ident.Obj.Kind != ast.Con {
			return "", false
		}
		if spec, ok := ident.Obj.Decl.(*ast.ValueSpec); ok && len(spec.Values) > 0 {
			// TODO: support len(spec.Values) > 1; e.g. const foo, bar = 1, 2
			key = spec.Values[0]
		}
	}
	if lit, ok := key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// string literals are always quoted.
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			panic("unreachable")
		}
		return value, true
	}
	return "", false
}

func areArgsOnSameLine(fset *token.FileSet, call ast.Expr, keys, attrs []ast.Expr) bool {
	if len(keys)+len(attrs) <= 1 {
		return false // special case: slog.Info("msg", "key", "value") is ok.
	}

	l := len(keys) + len(attrs) + 1
	args := make([]ast.Expr, 0, l)
	args = append(args, call)
	args = append(args, keys...)
	args = append(args, attrs...)

	lines := make(map[int]struct{}, l)
	for _, arg := range args {
		line := fset.Position(arg.Pos()).Line
		if _, ok := lines[line]; ok {
			return true
		}
		lines[line] = struct{}{}
	}

	return false
}
