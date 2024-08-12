package graphiteql

import (
	"fmt"
	"strconv"
	"strings"
)

type parser struct {
	lex lexer
}

// Expr is Graphite expression for render API.
type Expr interface {
	// AppendString appends Expr contents to dst and returns the result.
	AppendString(dst []byte) []byte
}

// Parse parses Graphite render API target expression.
//
// See https://graphite.readthedocs.io/en/stable/render_api.html
func Parse(s string) (Expr, error) {
	var p parser
	p.lex.Init(s)
	if err := p.lex.Next(); err != nil {
		return nil, fmt.Errorf("cannot parse target expression: %w; context: %q", err, p.lex.Context())
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("cannot parse target expression: %w; context: %q", err, p.lex.Context())
	}
	if !isEOF(p.lex.Token) {
		return nil, fmt.Errorf("unexpected tail left after parsing %q; context: %q", expr.AppendString(nil), p.lex.Context())
	}
	return expr, nil
}

func (p *parser) parseExpr() (Expr, error) {
	var expr Expr
	var err error
	token := p.lex.Token
	switch {
	case isPositiveNumberPrefix(token) || token == "+" || token == "-":
		expr, err = p.parseNumber()
		if err != nil {
			return nil, err
		}
	case isStringPrefix(token):
		expr, err = p.parseString()
		if err != nil {
			return nil, err
		}
	case isIdentPrefix(token):
		expr, err = p.parseMetricExprOrFuncCall()
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unexpected token when parsing expression: %q", token)
	}

	for {
		switch p.lex.Token {
		case "|":
			// Chained function call. For example, `metric|func`
			firstArg := &ArgExpr{
				Expr: expr,
			}
			expr, err = p.parseChainedFunc(firstArg)
			if err != nil {
				return nil, err
			}
			continue
		default:
			return expr, nil
		}
	}
}

func (p *parser) parseNumber() (*NumberExpr, error) {
	token := p.lex.Token
	isMinus := false
	if token == "-" || token == "+" {
		if err := p.lex.Next(); err != nil {
			return nil, fmt.Errorf("cannot find number after %q token: %w", token, err)
		}
		isMinus = token == "-"
		token = p.lex.Token
	}
	var n float64
	if isSpecialIntegerPrefix(token) {
		d, err := strconv.ParseInt(token, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse integer %q: %w", token, err)
		}
		n = float64(d)
	} else {
		f, err := strconv.ParseFloat(token, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse floating-point number %q: %w", token, err)
		}
		n = f
	}
	if isMinus {
		n = -n
	}
	if err := p.lex.Next(); err != nil {
		return nil, fmt.Errorf("cannot find next token after %q: %w", token, err)
	}
	ne := &NumberExpr{
		N: n,
	}
	return ne, nil
}

// NoneExpr contains None value
type NoneExpr struct{}

// AppendString appends string representation of nne to dst and returns the result.
func (nne *NoneExpr) AppendString(dst []byte) []byte {
	return append(dst, "None"...)
}

// BoolExpr contains bool value (True or False).
type BoolExpr struct {
	// B is bool value
	B bool
}

// AppendString appends string representation of be to dst and returns the result.
func (be *BoolExpr) AppendString(dst []byte) []byte {
	if be.B {
		return append(dst, "True"...)
	}
	return append(dst, "False"...)
}

// NumberExpr contains float64 constant.
type NumberExpr struct {
	// N is float64 constant
	N float64
}

// AppendString appends string representation of ne to dst and returns the result.
func (ne *NumberExpr) AppendString(dst []byte) []byte {
	return strconv.AppendFloat(dst, ne.N, 'g', -1, 64)
}

func (p *parser) parseString() (*StringExpr, error) {
	token := p.lex.Token
	if len(token) < 2 || token[0] != token[len(token)-1] {
		return nil, fmt.Errorf(`string literal contains unexpected trailing char; got %q`, token)
	}
	quote := string(append([]byte{}, token[0]))
	s := token[1 : len(token)-1]
	s = strings.ReplaceAll(s, `\`+quote, quote)
	s = strings.ReplaceAll(s, `\\`, `\`)
	if err := p.lex.Next(); err != nil {
		return nil, fmt.Errorf("cannot find next token after %s: %w", token, err)
	}
	se := &StringExpr{
		S: s,
	}
	return se, nil
}

// StringExpr represents string constant.
type StringExpr struct {
	// S contains unquoted string contents.
	S string
}

// AppendString appends se to dst and returns the result.
func (se *StringExpr) AppendString(dst []byte) []byte {
	dst = append(dst, '\'')
	s := strings.ReplaceAll(se.S, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	dst = append(dst, s...)
	dst = append(dst, '\'')
	return dst
}

// QuoteString quotes s, so it could be used in Graphite queries.
func QuoteString(s string) string {
	se := &StringExpr{
		S: s,
	}
	return string(se.AppendString(nil))
}

func (p *parser) parseMetricExprOrFuncCall() (Expr, error) {
	token := p.lex.Token
	ident := unescapeIdent(token)
	if err := p.lex.Next(); err != nil {
		return nil, fmt.Errorf("cannot find next token after %q: %w", token, err)
	}
	token = p.lex.Token
	switch token {
	case "(":
		// Function call. For example, `func(foo,bar)`
		funcName := ident
		args, err := p.parseArgs()
		if err != nil {
			return nil, fmt.Errorf("cannot parse args for function %q: %w", funcName, err)
		}
		fe := &FuncExpr{
			FuncName:   funcName,
			Args:       args,
			printState: printStateNormal,
		}
		return fe, nil
	default:
		// Metric epxression or bool expression or None.
		if isBool(ident) {
			be := &BoolExpr{
				B: strings.EqualFold(ident, "true"),
			}
			return be, nil
		}
		if strings.EqualFold(ident, "none") {
			nne := &NoneExpr{}
			return nne, nil
		}
		me := &MetricExpr{
			Query: ident,
		}
		return me, nil
	}
}

func (p *parser) parseChainedFunc(firstArg *ArgExpr) (*FuncExpr, error) {
	for {
		if err := p.lex.Next(); err != nil {
			return nil, fmt.Errorf("cannot find function name after %q|: %w", firstArg.AppendString(nil), err)
		}
		if !isIdentPrefix(p.lex.Token) {
			return nil, fmt.Errorf("expecting function name after %q|, got %q", firstArg.AppendString(nil), p.lex.Token)
		}
		funcName := unescapeIdent(p.lex.Token)
		if err := p.lex.Next(); err != nil {
			return nil, fmt.Errorf("cannot find next token after %q|%q: %w", firstArg.AppendString(nil), funcName, err)
		}
		fe := &FuncExpr{
			FuncName:   funcName,
			printState: printStateChained,
		}
		if p.lex.Token != "(" {
			fe.Args = []*ArgExpr{firstArg}
		} else {
			args, err := p.parseArgs()
			if err != nil {
				return nil, fmt.Errorf("cannot parse args for %q|%q: %w", firstArg.AppendString(nil), funcName, err)
			}
			fe.Args = append([]*ArgExpr{firstArg}, args...)
		}
		if p.lex.Token != "|" {
			return fe, nil
		}
		firstArg = &ArgExpr{
			Expr: fe,
		}
	}
}

func (p *parser) parseArgs() ([]*ArgExpr, error) {
	var args []*ArgExpr
	for {
		if err := p.lex.Next(); err != nil {
			return nil, fmt.Errorf("cannot find arg #%d: %w", len(args), err)
		}
		if p.lex.Token == ")" {
			if err := p.lex.Next(); err != nil {
				return nil, fmt.Errorf("cannot find next token after function args: %w", err)
			}
			return args, nil
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, fmt.Errorf("cannot parse arg #%d: %w", len(args), err)
		}
		if p.lex.Token == "=" {
			// Named expression
			me, ok := expr.(*MetricExpr)
			if !ok {
				return nil, fmt.Errorf("expecting a name for named expression; got %q", expr.AppendString(nil))
			}
			argName := me.Query
			if err := p.lex.Next(); err != nil {
				return nil, fmt.Errorf("cannot find named value for %q: %w", argName, err)
			}
			argValue, err := p.parseExpr()
			if err != nil {
				return nil, fmt.Errorf("cannot parse named value for %q: %w", argName, err)
			}
			args = append(args, &ArgExpr{
				Name: argName,
				Expr: argValue,
			})
		} else {
			args = append(args, &ArgExpr{
				Expr: expr,
			})
		}
		switch p.lex.Token {
		case ",":
			// Continue parsing args
		case ")":
			// End of args
			if err := p.lex.Next(); err != nil {
				return nil, fmt.Errorf("cannot find next token after func args: %w", err)
			}
			return args, nil
		default:
			return nil, fmt.Errorf("unexpected token detected in func args: %q", p.lex.Token)
		}
	}
}

// ArgExpr represents function arg (which may be named).
type ArgExpr struct {
	// Name is named arg name. It is empty for positional arg.
	Name string

	// Expr arg expression.
	Expr Expr
}

// AppendString appends string representation of ae to dst and returns the result.
func (ae *ArgExpr) AppendString(dst []byte) []byte {
	if ae.Name != "" {
		dst = appendEscapedIdent(dst, ae.Name)
		dst = append(dst, '=')
	}
	dst = ae.Expr.AppendString(dst)
	return dst
}

// FuncExpr represents function call.
type FuncExpr struct {
	// FuncName is the function name
	FuncName string

	// Args is function args.
	Args []*ArgExpr

	printState funcPrintState
}

type funcPrintState int

const (
	// Normal func call: `func(arg1, ..., argN)`
	printStateNormal = funcPrintState(0)

	// Chained func call: `arg1|func(arg2, ..., argN)`
	printStateChained = funcPrintState(1)
)

// AppendString appends string representation of fe to dst and returns the result.
func (fe *FuncExpr) AppendString(dst []byte) []byte {
	switch fe.printState {
	case printStateNormal:
		dst = appendEscapedIdent(dst, fe.FuncName)
		dst = appendArgsString(dst, fe.Args)
	case printStateChained:
		if len(fe.Args) == 0 {
			panic("BUG: chained func call must have at least a single arg")
		}
		firstArg := fe.Args[0]
		tailArgs := fe.Args[1:]
		if firstArg.Name != "" {
			panic("BUG: the first chained arg must have no name")
		}
		dst = firstArg.AppendString(dst)
		dst = append(dst, '|')
		dst = appendEscapedIdent(dst, fe.FuncName)
		if len(tailArgs) > 0 {
			dst = appendArgsString(dst, tailArgs)
		}
	default:
		panic(fmt.Sprintf("BUG: unexpected printState=%d", fe.printState))
	}
	return dst
}

// MetricExpr represents metric expression.
type MetricExpr struct {
	// Query is the query for fetching metrics.
	Query string
}

// AppendString append string representation of me to dst and returns the result.
func (me *MetricExpr) AppendString(dst []byte) []byte {
	return appendEscapedIdent(dst, me.Query)
}

func appendArgsString(dst []byte, args []*ArgExpr) []byte {
	dst = append(dst, '(')
	for i, arg := range args {
		dst = arg.AppendString(dst)
		if i+1 < len(args) {
			dst = append(dst, ',')
		}
	}
	dst = append(dst, ')')
	return dst
}
