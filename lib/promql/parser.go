package promql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Panicf controls how this package reports a runtime error indicative of a
// bug in the implementation.
var Panicf func(format string, args ...interface{}) = func(format string, args ...interface{}) {
	panic(fmt.Errorf(format, args...))
}

// A Parser is a thread-safe object that can be used to parse Extended PromQL
// into a parsed tree of objects
type Parser struct {
	compileRegexpAnchored func(re string) (*regexp.Regexp, error)
}

// NewParser constructs a new Parser
func NewParser(
	compileRegexpAnchored func(re string) (*regexp.Regexp, error),
) *Parser {
	return &Parser{
		compileRegexpAnchored: compileRegexpAnchored,
	}
}

// ParsePromQL parses an extended PromQL string into an Expr object
func (p *Parser) ParsePromQL(s string) (Expr, error) {
	e, err := p.ParseRawPromQL(s)
	if err != nil {
		return nil, err
	}
	was := p.getDefaultWithArgExprs()
	if e, err = p.expandWithExpr(was, e); err != nil {
		return nil, fmt.Errorf(`cannot expand WITH expressions: %s`, err)
	}
	e = removeParensExpr(e)
	return e, nil
}

// ParseRawPromQL parses an extended PromQL string into an Expr object, without
// rewriting or expanding with clauses
func (p *Parser) ParseRawPromQL(s string) (Expr, error) {
	var ps parseState
	ps.lex.Init(s)
	if err := ps.lex.Next(); err != nil {
		return nil, fmt.Errorf(`cannot find the first token: %s`, err)
	}
	e, err := ps.parseExpr()
	if err != nil {
		return nil, fmt.Errorf(`%s; unparsed data: %q`, err, ps.lex.Context())
	}
	if !isEOF(ps.lex.Token) {
		return nil, fmt.Errorf(`unparsed data left: %q`, ps.lex.Context())
	}
	return e, nil
}

func (p *Parser) expandWithExpr(was []*WithArgExpr, e Expr) (Expr, error) {
	switch t := e.(type) {
	case *BinaryOpExpr:
		left, err := p.expandWithExpr(was, t.Left)
		if err != nil {
			return nil, err
		}
		right, err := p.expandWithExpr(was, t.Right)
		if err != nil {
			return nil, err
		}
		groupModifierArgs, err := p.expandModifierArgs(was, t.GroupModifier.Args)
		if err != nil {
			return nil, err
		}
		joinModifierArgs, err := p.expandModifierArgs(was, t.JoinModifier.Args)
		if err != nil {
			return nil, err
		}
		if t.Op == "+" {
			lse, lok := left.(*StringExpr)
			rse, rok := right.(*StringExpr)
			if lok && rok {
				se := &StringExpr{
					S: lse.S + rse.S,
				}
				return se, nil
			}
		}
		be := &BinaryOpExpr{
			Op:            t.Op,
			Bool:          t.Bool,
			GroupModifier: t.GroupModifier,
			JoinModifier:  t.JoinModifier,
			Left:          left,
			Right:         right,
		}
		be.GroupModifier.Args = groupModifierArgs
		be.JoinModifier.Args = joinModifierArgs
		pe := ParensExpr{be}
		return &pe, nil
	case *FuncExpr:
		args, err := p.expandWithArgs(was, t.Args)
		if err != nil {
			return nil, err
		}
		wa := getWithArgExpr(was, t.Name)
		if wa == nil {
			fe := &FuncExpr{
				Name: t.Name,
				Args: args,
			}
			return fe, nil
		}
		return p.expandWithExprExt(was, wa, args)
	case *AggrFuncExpr:
		args, err := p.expandWithArgs(was, t.Args)
		if err != nil {
			return nil, err
		}
		modifierArgs, err := p.expandModifierArgs(was, t.Modifier.Args)
		if err != nil {
			return nil, err
		}
		ae := &AggrFuncExpr{
			Name:     t.Name,
			Args:     args,
			Modifier: t.Modifier,
		}
		ae.Modifier.Args = modifierArgs
		return ae, nil
	case *ParensExpr:
		exprs, err := p.expandWithArgs(was, *t)
		if err != nil {
			return nil, err
		}
		pe := ParensExpr(exprs)
		return &pe, nil
	case *StringTemplateExpr:
		var b []byte
		for _, token := range t.Tokens {
			if !token.Ident {
				b = append(b, token.S...)
				continue
			}
			wa := getWithArgExpr(was, token.S)
			if wa == nil {
				return nil, fmt.Errorf("missing %q value inside stringExpr", token.S)
			}
			eNew, err := p.expandWithExprExt(was, wa, nil)
			if err != nil {
				return nil, err
			}
			seSrc, ok := eNew.(*StringExpr)
			if !ok {
				return nil, fmt.Errorf("%q must be string expression; got %q", token.S, eNew.AppendString(nil))
			}
			b = append(b, seSrc.S...)
		}
		se := &StringExpr{
			S: string(b),
		}
		return se, nil
	case *RollupExpr:
		eNew, err := p.expandWithExpr(was, t.Expr)
		if err != nil {
			return nil, err
		}
		re := *t
		re.Expr = eNew
		return &re, nil
	case *WithExpr:
		wasNew := make([]*WithArgExpr, 0, len(was)+len(t.Was))
		wasNew = append(wasNew, was...)
		wasNew = append(wasNew, t.Was...)
		eNew, err := p.expandWithExpr(wasNew, t.Expr)
		if err != nil {
			return nil, err
		}
		return eNew, nil
	case *MetricTemplateExpr:
		var newMe MetricExpr
		// Populate converted tag filters
		for _, tfe := range t.TagFilters {
			if tfe.Value == nil {
				// Expand tfe.Key into TagFilters.
				wa := getWithArgExpr(was, tfe.Key)
				if wa == nil {
					return nil, fmt.Errorf("missing %q value inside %q", tfe.Key, t.AppendString(nil))
				}
				eNew, err := p.expandWithExprExt(was, wa, nil)
				if err != nil {
					return nil, err
				}
				wme, ok := eNew.(*MetricExpr)
				if !ok || wme.HasNonEmptyMetricGroup() {
					return nil, fmt.Errorf("%q must be filters expression inside %q; got %q", tfe.Key, t.AppendString(nil), eNew.AppendString(nil))
				}
				newMe.TagFilters = append(newMe.TagFilters, wme.TagFilters...)
				continue
			}

			// convert tfe to TagFilter.
			se, err := p.expandWithExpr(was, tfe.Value)
			if err != nil {
				return nil, err
			}
			tf, err := p.createTagFilter(tfe.Key, se.(*StringExpr).S, tfe.IsRegexp, tfe.IsNegative)
			if err != nil {
				return nil, err
			}
			newMe.TagFilters = append(newMe.TagFilters, *tf)
		}
		newMe.TagFilters = p.removeDuplicateTagFilters(newMe.TagFilters)
		if !newMe.HasNonEmptyMetricGroup() {
			return &newMe, nil
		}
		k := string(appendEscapedIdent(nil, newMe.TagFilters[0].Value))
		wa := getWithArgExpr(was, k)
		if wa == nil {
			return &newMe, nil
		}
		eNew, err := p.expandWithExprExt(was, wa, nil)
		if err != nil {
			return nil, err
		}
		var wme *MetricExpr
		re, _ := eNew.(*RollupExpr)
		if re != nil {
			wme, _ = re.Expr.(*MetricExpr)
		} else {
			wme, _ = eNew.(*MetricExpr)
		}
		if wme == nil {
			if !newMe.IsOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot expand %q to non-metric expression %q", t.AppendString(nil), eNew.AppendString(nil))
			}
			return eNew, nil
		}

		rest := newMe.TagFilters[1:]
		newMe.TagFilters = append(make([]TagFilter, 0, len(wme.TagFilters)+len(rest)), wme.TagFilters...)
		newMe.TagFilters = append(newMe.TagFilters, rest...)
		newMe.TagFilters = p.removeDuplicateTagFilters(newMe.TagFilters)

		if re == nil {
			return &newMe, nil
		}
		reNew := *re
		reNew.Expr = &newMe
		return &reNew, nil
	default:
		return e, nil
	}
}

func (p *Parser) expandWithArgs(was []*WithArgExpr, args []Expr) ([]Expr, error) {
	dstArgs := make([]Expr, len(args))
	for i, arg := range args {
		dstArg, err := p.expandWithExpr(was, arg)
		if err != nil {
			return nil, err
		}
		dstArgs[i] = dstArg
	}
	return dstArgs, nil
}

func (p *Parser) expandModifierArgs(was []*WithArgExpr, args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, nil
	}
	dstArgs := make([]string, 0, len(args))
	for _, arg := range args {
		wa := getWithArgExpr(was, arg)
		if wa == nil {
			// Leave the arg as is.
			dstArgs = append(dstArgs, arg)
			continue
		}
		if len(wa.Args) > 0 {
			// Template funcs cannot be used inside modifier list. Leave the arg as is.
			dstArgs = append(dstArgs, arg)
			continue
		}
		me, ok := wa.Expr.(*MetricExpr)
		if ok {
			if !me.IsOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot use %q instead of %q in %s", me.AppendString(nil), arg, args)
			}
			dstArg := string(me.TagFilters[0].Value)
			dstArgs = append(dstArgs, dstArg)
			continue
		}
		pe, ok := wa.Expr.(*ParensExpr)
		if ok {
			for _, pArg := range *pe {
				me, ok := pArg.(*MetricExpr)
				if !ok || !me.IsOnlyMetricGroup() {
					return nil, fmt.Errorf("cannot use %q instead of %q in %s", pe.AppendString(nil), arg, args)
				}
				dstArg := string(me.TagFilters[0].Value)
				dstArgs = append(dstArgs, dstArg)
			}
			continue
		}
		return nil, fmt.Errorf("cannot use %q instead of %q in %s", wa.Expr.AppendString(nil), arg, args)
	}

	// Remove duplicate args from dstArgs
	m := make(map[string]bool, len(dstArgs))
	filteredArgs := dstArgs[:0]
	for _, arg := range dstArgs {
		if !m[arg] {
			filteredArgs = append(filteredArgs, arg)
			m[arg] = true
		}
	}
	return filteredArgs, nil
}

func (p *Parser) expandWithExprExt(was []*WithArgExpr, wa *WithArgExpr, args []Expr) (Expr, error) {
	if len(wa.Args) != len(args) {
		if args == nil {
			// Just return metricExpr with the wa.Name name.
			return newMetricExpr(wa.Name), nil
		}
		return nil, fmt.Errorf("invalid number of args for %q; got %d; want %d", wa.Name, len(args), len(wa.Args))
	}
	wasNew := make([]*WithArgExpr, 0, len(was)+len(args))
	for _, waTmp := range was {
		if waTmp == wa {
			break
		}
		wasNew = append(wasNew, waTmp)
	}
	for i, arg := range args {
		wasNew = append(wasNew, &WithArgExpr{
			Name: wa.Args[i],
			Expr: arg,
		})
	}
	return p.expandWithExpr(wasNew, wa.Expr)
}

func (p *Parser) removeDuplicateTagFilters(tfs []TagFilter) []TagFilter {
	tfsm := make(map[string]bool, len(tfs))
	tfsNew := tfs[:0]
	var bb []byte
	for i := range tfs {
		tf := &tfs[i]
		bb = appendStringTagFilter(bb[:0], tf)
		if tfsm[string(bb)] {
			continue
		}
		tfsm[string(bb)] = true
		tfsNew = append(tfsNew, *tf)
	}
	return tfsNew
}

func (p *Parser) createTagFilter(key, value string, isRegexp, isNegative bool) (*TagFilter, error) {
	var tf TagFilter
	tf.Key = []byte(unescapeIdent(key))
	if len(key) == 0 {
		tf.Value = []byte(unescapeIdent(value))
	} else {
		tf.Value = []byte(value)
	}
	if string(tf.Key) == "__name__" {
		// This is required for storage.Search
		tf.Key = nil
	}
	tf.IsRegexp = isRegexp
	tf.IsNegative = isNegative
	if !tf.IsRegexp {
		return &tf, nil
	}

	// Verify regexp.
	if _, err := p.compileRegexpAnchored(value); err != nil {
		return nil, fmt.Errorf("invalid regexp in %s=%q: %s", tf.Key, tf.Value, err)
	}
	return &tf, nil
}

func (p *Parser) getDefaultWithArgExprs() []*WithArgExpr {
	defaultWithArgExprsOnce.Do(func() {
		defaultWithArgExprs = p.prepareWithArgExprs([]string{
			// ru - resource utilization
			`ru(freev, maxv) = clamp_min(maxv - clamp_min(freev, 0), 0) / clamp_min(maxv, 0) * 100`,

			// ttf - time to fuckup
			`ttf(freev) = smooth_exponential(
				clamp_max(clamp_max(-freev, 0) / clamp_max(deriv_fast(freev), 0), 365*24*3600),
				clamp_max(step()/300, 1)
			)`,

			`median_over_time(m) = quantile_over_time(0.5, m)`,
			`range_median(q) = range_quantile(0.5, q)`,
			`alias(q, name) = label_set(q, "__name__", name)`,
		})
	})
	return defaultWithArgExprs
}

var (
	defaultWithArgExprs     []*WithArgExpr
	defaultWithArgExprsOnce sync.Once
)

func (p *Parser) prepareWithArgExprs(ss []string) []*WithArgExpr {
	was := make([]*WithArgExpr, len(ss))
	for i, s := range ss {
		was[i] = p.mustParseWithArgExpr(s)
	}
	if err := p.checkDuplicateWithArgNames(was); err != nil {
		Panicf("BUG: %s", err)
	}
	return was
}

func (p *Parser) checkDuplicateWithArgNames(was []*WithArgExpr) error {
	m := make(map[string]*WithArgExpr, len(was))
	for _, wa := range was {
		if waOld := m[wa.Name]; waOld != nil {
			return fmt.Errorf("duplicate `with` arg name for: %s; previous one: %s", wa, waOld.AppendString(nil))
		}
		m[wa.Name] = wa
	}
	return nil
}

func (p *Parser) mustParseWithArgExpr(s string) *WithArgExpr {
	var ps parseState
	ps.lex.Init(s)
	if err := ps.lex.Next(); err != nil {
		Panicf("BUG: cannot find the first token in %q: %s", s, err)
	}
	wa, err := ps.parseWithArgExpr()
	if err != nil {
		Panicf("BUG: cannot parse %q: %s; unparsed data: %q", s, err, ps.lex.Context())
	}
	return wa
}

// removeParensExpr removes parensExpr for (expr) case.
func removeParensExpr(e Expr) Expr {
	if re, ok := e.(*RollupExpr); ok {
		re.Expr = removeParensExpr(re.Expr)
		return re
	}
	if be, ok := e.(*BinaryOpExpr); ok {
		be.Left = removeParensExpr(be.Left)
		be.Right = removeParensExpr(be.Right)
		return be
	}
	if ae, ok := e.(*AggrFuncExpr); ok {
		for i, arg := range ae.Args {
			ae.Args[i] = removeParensExpr(arg)
		}
		return ae
	}
	if fe, ok := e.(*FuncExpr); ok {
		for i, arg := range fe.Args {
			fe.Args[i] = removeParensExpr(arg)
		}
		return fe
	}
	if pe, ok := e.(*ParensExpr); ok {
		args := *pe
		for i, arg := range args {
			args[i] = removeParensExpr(arg)
		}
		if len(*pe) == 1 {
			return args[0]
		}
		// Treat parensExpr as a function with empty name, i.e. union()
		fe := &FuncExpr{
			Name: "",
			Args: args,
		}
		return fe
	}
	return e
}

// parseState parses PromQL expression.
//
// preconditions for all parseState.parse* funcs:
// - p.lex.Token should point to the first token to parse.
//
// postconditions for all parseState.parse* funcs:
// - p.lex.Token should point to the next token after the parsed token.
type parseState struct {
	parser *Parser
	lex    lexer
}

func isWith(s string) bool {
	s = strings.ToLower(s)
	return s == "with"
}

// parseWithExpr parses `WITH (withArgExpr...) expr`.
func (ps *parseState) parseWithExpr() (*WithExpr, error) {
	var we WithExpr
	if !isWith(ps.lex.Token) {
		return nil, fmt.Errorf("withExpr: unexpected token %q; want `WITH`", ps.lex.Token)
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	if ps.lex.Token != "(" {
		return nil, fmt.Errorf(`withExpr: unexpected token %q; want "("`, ps.lex.Token)
	}
	for {
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == ")" {
			goto end
		}
		wa, err := ps.parseWithArgExpr()
		if err != nil {
			return nil, err
		}
		we.Was = append(we.Was, wa)
		switch ps.lex.Token {
		case ",":
			continue
		case ")":
			goto end
		default:
			return nil, fmt.Errorf(`withExpr: unexpected token %q; want ",", ")"`, ps.lex.Token)
		}
	}

end:
	if err := ps.parser.checkDuplicateWithArgNames(we.Was); err != nil {
		return nil, err
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	e, err := ps.parseExpr()
	if err != nil {
		return nil, err
	}
	we.Expr = e
	return &we, nil
}

func (ps *parseState) parseWithArgExpr() (*WithArgExpr, error) {
	var wa WithArgExpr
	if !isIdentPrefix(ps.lex.Token) {
		return nil, fmt.Errorf(`withArgExpr: unexpected token %q; want "ident"`, ps.lex.Token)
	}
	wa.Name = ps.lex.Token
	if isAggrFunc(wa.Name) || isRollupFunc(wa.Name) || isTransformFunc(wa.Name) || isWith(wa.Name) {
		return nil, fmt.Errorf(`withArgExpr: cannot use reserved name %q`, wa.Name)
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	if ps.lex.Token == "(" {
		// Parse func args.
		args, err := ps.parseIdentList()
		if err != nil {
			return nil, fmt.Errorf(`withArgExpr: cannot parse args for %q: %s`, wa.Name, err)
		}
		// Make sure all the args have different names
		m := make(map[string]bool, len(args))
		for _, arg := range args {
			if m[arg] {
				return nil, fmt.Errorf(`withArgExpr: duplicate func arg found in %q: %q`, wa.Name, arg)
			}
			m[arg] = true
		}
		wa.Args = args
	}
	if ps.lex.Token != "=" {
		return nil, fmt.Errorf(`withArgExpr: unexpected token %q; want "="`, ps.lex.Token)
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	e, err := ps.parseExpr()
	if err != nil {
		return nil, fmt.Errorf(`withArgExpr: cannot parse %q: %s`, wa.Name, err)
	}
	wa.Expr = e
	return &wa, nil
}

// parseExpr parses promql expr
func (ps *parseState) parseExpr() (Expr, error) {
	e, err := ps.parseSingleExpr()
	if err != nil {
		return nil, err
	}
	for {
		if !isBinaryOp(ps.lex.Token) {
			return e, nil
		}

		var be BinaryOpExpr
		be.Op = strings.ToLower(ps.lex.Token)
		be.Left = e
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if isBinaryOpBoolModifier(ps.lex.Token) {
			if !isBinaryOpCmp(be.Op) {
				return nil, fmt.Errorf(`bool modifier cannot be applied to %q`, be.Op)
			}
			be.Bool = true
			if err := ps.lex.Next(); err != nil {
				return nil, err
			}
		}
		if isBinaryOpGroupModifier(ps.lex.Token) {
			if err := ps.parseModifierExpr(&be.GroupModifier); err != nil {
				return nil, err
			}
			if isBinaryOpJoinModifier(ps.lex.Token) {
				if isBinaryOpLogicalSet(be.Op) {
					return nil, fmt.Errorf(`modifier %q cannot be applied to %q`, ps.lex.Token, be.Op)
				}
				if err := ps.parseModifierExpr(&be.JoinModifier); err != nil {
					return nil, err
				}
			}
		}
		e2, err := ps.parseSingleExpr()
		if err != nil {
			return nil, err
		}
		be.Right = e2
		e = balanceBinaryOp(&be)
	}
}

func balanceBinaryOp(be *BinaryOpExpr) Expr {
	bel, ok := be.Left.(*BinaryOpExpr)
	if !ok {
		return be
	}
	lp := binaryOpPriority(bel.Op)
	rp := binaryOpPriority(be.Op)
	if rp < lp {
		return be
	}
	if rp == lp && !isRightAssociativeBinaryOp(be.Op) {
		return be
	}
	be.Left = bel.Right
	bel.Right = balanceBinaryOp(be)
	return bel
}

// parseSingleExpr parses non-binaryOp expressions.
func (ps *parseState) parseSingleExpr() (Expr, error) {
	if isWith(ps.lex.Token) {
		err := ps.lex.Next()
		nextToken := ps.lex.Token
		ps.lex.Prev()
		if err == nil && nextToken == "(" {
			return ps.parseWithExpr()
		}
	}
	e, err := ps.parseSingleExprWithoutRollupSuffix()
	if err != nil {
		return nil, err
	}
	if ps.lex.Token != "[" && !isOffset(ps.lex.Token) {
		// There is no rollup expression.
		return e, nil
	}
	return ps.parseRollupExpr(e)
}

func (ps *parseState) parseSingleExprWithoutRollupSuffix() (Expr, error) {
	if isPositiveNumberPrefix(ps.lex.Token) || isInfOrNaN(ps.lex.Token) {
		return ps.parsePositiveNumberExpr()
	}
	if isStringPrefix(ps.lex.Token) {
		return ps.parseStringTemplateExpr()
	}
	if isIdentPrefix(ps.lex.Token) {
		return ps.parseIdentExpr()
	}
	switch ps.lex.Token {
	case "(":
		return ps.parseParensExpr()
	case "{":
		return ps.parseMetricTemplateExpr()
	case "-":
		// Unary minus. Substitute -expr with (0 - expr)
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		e, err := ps.parseSingleExpr()
		if err != nil {
			return nil, err
		}

		// Fall back in the simple -<number> case to a negative number
		if ne, ok := e.(*NumberExpr); ok {
			ne.N *= -1
			return ne, nil
		}

		be := &BinaryOpExpr{
			Op: "-",
			Left: &NumberExpr{
				N: 0,
			},
			Right: e,
		}
		pe := ParensExpr{be}
		return &pe, nil
	case "+":
		// Unary plus
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		return ps.parseSingleExpr()
	default:
		return nil, fmt.Errorf(`singleExpr: unexpected token %q; want "(", "{", "-", "+"`, ps.lex.Token)
	}
}

func (ps *parseState) parsePositiveNumberExpr() (*NumberExpr, error) {
	if !isPositiveNumberPrefix(ps.lex.Token) && !isInfOrNaN(ps.lex.Token) {
		return nil, fmt.Errorf(`positiveNumberExpr: unexpected token %q; want "number"`, ps.lex.Token)
	}

	n, err := strconv.ParseFloat(ps.lex.Token, 64)
	if err != nil {
		return nil, fmt.Errorf(`positiveNumberExpr: cannot parse %q: %s`, ps.lex.Token, err)
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	ne := &NumberExpr{
		N: n,
	}
	return ne, nil
}

func (ps *parseState) parseStringTemplateExpr() (*StringTemplateExpr, error) {
	var se StringTemplateExpr

	for {
		switch {
		case isStringPrefix(ps.lex.Token):
			s, err := extractStringValue(ps.lex.Token)
			if err != nil {
				return nil, err
			}
			se.Tokens = append(se.Tokens, StringToken{Ident: false, S: s})
		case isIdentPrefix(ps.lex.Token):
			se.Tokens = append(se.Tokens, StringToken{Ident: true, S: ps.lex.Token})
		default:
			return nil, fmt.Errorf(`stringExpr: unexpected token %q; want "string"`, ps.lex.Token)
		}
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token != "+" {
			return &se, nil
		}

		// composite stringExpr like `"s1" + "s2"`, `"s" + m()` or `"s" + m{}` or `"s" + unknownToken`.
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if isStringPrefix(ps.lex.Token) {
			// "s1" + "s2"
			continue
		}
		if !isIdentPrefix(ps.lex.Token) {
			// "s" + unknownToken
			ps.lex.Prev()
			return &se, nil
		}
		// Look after ident
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == "(" || ps.lex.Token == "{" {
			// `"s" + m(` or `"s" + m{`
			ps.lex.Prev()
			ps.lex.Prev()
			return &se, nil
		}
		// "s" + ident
		ps.lex.Prev()
	}
}

func (ps *parseState) parseParensExpr() (*ParensExpr, error) {
	if ps.lex.Token != "(" {
		return nil, fmt.Errorf(`parensExpr: unexpected token %q; want "("`, ps.lex.Token)
	}
	var exprs []Expr
	for {
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == ")" {
			break
		}
		expr, err := ps.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		if ps.lex.Token == "," {
			continue
		}
		if ps.lex.Token == ")" {
			break
		}
		return nil, fmt.Errorf(`parensExpr: unexpected token %q; want "," or ")"`, ps.lex.Token)
	}
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	pe := ParensExpr(exprs)
	return &pe, nil
}

func (ps *parseState) parseAggrFuncExpr() (*AggrFuncExpr, error) {
	if !isAggrFunc(ps.lex.Token) {
		return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want aggregate func`, ps.lex.Token)
	}

	var ae AggrFuncExpr
	ae.Name = strings.ToLower(ps.lex.Token)
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	if isIdentPrefix(ps.lex.Token) {
		goto funcPrefixLabel
	}
	switch ps.lex.Token {
	case "(":
		goto funcArgsLabel
	default:
		return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want "("`, ps.lex.Token)
	}

funcPrefixLabel:
	{
		if !isAggrFuncModifier(ps.lex.Token) {
			return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want aggregate func modifier`, ps.lex.Token)
		}
		if err := ps.parseModifierExpr(&ae.Modifier); err != nil {
			return nil, err
		}
		goto funcArgsLabel
	}

funcArgsLabel:
	{
		args, err := ps.parseArgListExpr()
		if err != nil {
			return nil, err
		}
		ae.Args = args

		// Verify whether func suffix exists.
		if ae.Modifier.Op != "" || !isAggrFuncModifier(ps.lex.Token) {
			return &ae, nil
		}
		if err := ps.parseModifierExpr(&ae.Modifier); err != nil {
			return nil, err
		}
		return &ae, nil
	}
}

func newMetricExpr(name string) *MetricExpr {
	return &MetricExpr{
		TagFilters: []TagFilter{{
			Value: []byte(name),
		}},
	}
}

func extractStringValue(token string) (string, error) {
	if !isStringPrefix(token) {
		return "", fmt.Errorf(`stringExpr must contain only string literals; got %q`, token)
	}

	// See https://prometheus.io/docs/prometheus/latest/querying/basics/#string-literals
	if token[0] == '\'' {
		if len(token) < 2 || token[len(token)-1] != '\'' {
			return "", fmt.Errorf(`string literal contains unexpected trailing char; got %q`, token)
		}
		token = token[1 : len(token)-1]
		token = strings.Replace(token, "\\'", "'", -1)
		token = strings.Replace(token, `"`, `\"`, -1)
		token = `"` + token + `"`
	}
	s, err := strconv.Unquote(token)
	if err != nil {
		return "", fmt.Errorf(`cannot parse string literal %q: %s`, token, err)
	}
	return s, nil
}

func (ps *parseState) parseFuncExpr() (*FuncExpr, error) {
	if !isIdentPrefix(ps.lex.Token) {
		return nil, fmt.Errorf(`funcExpr: unexpected token %q; want "ident"`, ps.lex.Token)
	}

	var fe FuncExpr
	fe.Name = ps.lex.Token
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	if ps.lex.Token != "(" {
		return nil, fmt.Errorf(`funcExpr; unexpected token %q; want "("`, ps.lex.Token)
	}
	args, err := ps.parseArgListExpr()
	if err != nil {
		return nil, err
	}
	fe.Args = args
	return &fe, nil
}

func (ps *parseState) parseModifierExpr(me *ModifierExpr) error {
	if !isIdentPrefix(ps.lex.Token) {
		return fmt.Errorf(`modifierExpr: unexpected token %q; want "ident"`, ps.lex.Token)
	}

	me.Op = strings.ToLower(ps.lex.Token)

	if err := ps.lex.Next(); err != nil {
		return err
	}
	if isBinaryOpJoinModifier(me.Op) && ps.lex.Token != "(" {
		// join modifier may miss ident list.
		return nil
	}
	args, err := ps.parseIdentList()
	if err != nil {
		return err
	}
	me.Args = args
	return nil
}

func (ps *parseState) parseIdentList() ([]string, error) {
	if ps.lex.Token != "(" {
		return nil, fmt.Errorf(`identList: unexpected token %q; want "("`, ps.lex.Token)
	}
	var idents []string
	for {
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == ")" {
			goto closeParensLabel
		}
		if !isIdentPrefix(ps.lex.Token) {
			return nil, fmt.Errorf(`identList: unexpected token %q; want "ident"`, ps.lex.Token)
		}
		idents = append(idents, ps.lex.Token)
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		switch ps.lex.Token {
		case ",":
			continue
		case ")":
			goto closeParensLabel
		default:
			return nil, fmt.Errorf(`identList: unexpected token %q; want ",", ")"`, ps.lex.Token)
		}
	}

closeParensLabel:
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	return idents, nil
}

func (ps *parseState) parseArgListExpr() ([]Expr, error) {
	if ps.lex.Token != "(" {
		return nil, fmt.Errorf(`argList: unexpected token %q; want "("`, ps.lex.Token)
	}
	var args []Expr
	for {
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == ")" {
			goto closeParensLabel
		}
		expr, err := ps.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		switch ps.lex.Token {
		case ",":
			continue
		case ")":
			goto closeParensLabel
		default:
			return nil, fmt.Errorf(`argList: unexpected token %q; want ",", ")"`, ps.lex.Token)
		}
	}

closeParensLabel:
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	return args, nil
}

func getWithArgExpr(was []*WithArgExpr, name string) *WithArgExpr {
	// Scan wes backwards, since certain expressions may override
	// previously defined expressions
	for i := len(was) - 1; i >= 0; i-- {
		wa := was[i]
		if wa.Name == name {
			return wa
		}
	}
	return nil
}

func (ps *parseState) parseTagFilters() ([]*TagFilterExpr, error) {
	if ps.lex.Token != "{" {
		return nil, fmt.Errorf(`tagFilters: unexpected token %q; want "{"`, ps.lex.Token)
	}

	var tfes []*TagFilterExpr
	for {
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token == "}" {
			goto closeBracesLabel
		}
		tfe, err := ps.parseTagFilterExpr()
		if err != nil {
			return nil, err
		}
		tfes = append(tfes, tfe)
		switch ps.lex.Token {
		case ",":
			continue
		case "}":
			goto closeBracesLabel
		default:
			return nil, fmt.Errorf(`tagFilters: unexpected token %q; want ",", "}"`, ps.lex.Token)
		}
	}

closeBracesLabel:
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	return tfes, nil
}

func (ps *parseState) parseTagFilterExpr() (*TagFilterExpr, error) {
	if !isIdentPrefix(ps.lex.Token) {
		return nil, fmt.Errorf(`tagFilterExpr: unexpected token %q; want "ident"`, ps.lex.Token)
	}
	var tfe TagFilterExpr
	tfe.Key = ps.lex.Token
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}

	switch ps.lex.Token {
	case "=":
		// Nothing to do.
	case "!=":
		tfe.IsNegative = true
	case "=~":
		tfe.IsRegexp = true
	case "!~":
		tfe.IsNegative = true
		tfe.IsRegexp = true
	case ",", "}":
		return &tfe, nil
	default:
		return nil, fmt.Errorf(`tagFilterExpr: unexpected token %q; want "=", "!=", "=~", "!~", ",", "}"`, ps.lex.Token)
	}

	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	se, err := ps.parseStringTemplateExpr()
	if err != nil {
		return nil, err
	}
	tfe.Value = se
	return &tfe, nil
}

func (ps *parseState) parseWindowAndStep() (string, string, bool, error) {
	if ps.lex.Token != "[" {
		return "", "", false, fmt.Errorf(`windowAndStep: unexpected token %q; want "["`, ps.lex.Token)
	}
	err := ps.lex.Next()
	if err != nil {
		return "", "", false, err
	}
	var window string
	if !strings.HasPrefix(ps.lex.Token, ":") {
		window, err = ps.parseDuration()
		if err != nil {
			return "", "", false, err
		}
	}
	var step string
	inheritStep := false
	if strings.HasPrefix(ps.lex.Token, ":") {
		// Parse step
		ps.lex.Token = ps.lex.Token[1:]
		if ps.lex.Token == "" {
			if err := ps.lex.Next(); err != nil {
				return "", "", false, err
			}
			if ps.lex.Token == "]" {
				inheritStep = true
			}
		}
		if ps.lex.Token != "]" {
			step, err = ps.parseDuration()
			if err != nil {
				return "", "", false, err
			}
		}
	}
	if ps.lex.Token != "]" {
		return "", "", false, fmt.Errorf(`windowAndStep: unexpected token %q; want "]"`, ps.lex.Token)
	}
	if err := ps.lex.Next(); err != nil {
		return "", "", false, err
	}
	return window, step, inheritStep, nil
}

func (ps *parseState) parseOffset() (string, error) {
	if !isOffset(ps.lex.Token) {
		return "", fmt.Errorf(`offset: unexpected token %q; want "offset"`, ps.lex.Token)
	}
	if err := ps.lex.Next(); err != nil {
		return "", err
	}
	d, err := ps.parseDuration()
	if err != nil {
		return "", err
	}
	return d, nil
}

func (ps *parseState) parseDuration() (string, error) {
	if !isDuration(ps.lex.Token) {
		return "", fmt.Errorf(`duration: unexpected token %q; want "duration"`, ps.lex.Token)
	}
	d := ps.lex.Token
	if err := ps.lex.Next(); err != nil {
		return "", err
	}
	return d, nil
}

// parseIdentExpr parses expressions starting with `ident` token.
func (ps *parseState) parseIdentExpr() (Expr, error) {
	// Look into the next-next token in order to determine how to parse
	// the current expression.
	if err := ps.lex.Next(); err != nil {
		return nil, err
	}
	if isEOF(ps.lex.Token) || isOffset(ps.lex.Token) {
		ps.lex.Prev()
		return ps.parseMetricTemplateExpr()
	}
	if isIdentPrefix(ps.lex.Token) {
		ps.lex.Prev()
		if isAggrFunc(ps.lex.Token) {
			return ps.parseAggrFuncExpr()
		}
		return ps.parseMetricTemplateExpr()
	}
	if isBinaryOp(ps.lex.Token) {
		ps.lex.Prev()
		return ps.parseMetricTemplateExpr()
	}
	switch ps.lex.Token {
	case "(":
		ps.lex.Prev()
		if isAggrFunc(ps.lex.Token) {
			return ps.parseAggrFuncExpr()
		}
		return ps.parseFuncExpr()
	case "{", "[", ")", ",":
		ps.lex.Prev()
		return ps.parseMetricTemplateExpr()
	default:
		return nil, fmt.Errorf(`identExpr: unexpected token %q; want "(", "{", "[", ")", ","`, ps.lex.Token)
	}
}

func (ps *parseState) parseMetricTemplateExpr() (*MetricTemplateExpr, error) {
	var me MetricTemplateExpr
	if isIdentPrefix(ps.lex.Token) {
		var tfe TagFilterExpr
		tfe.Value = &StringTemplateExpr{
			Tokens: []StringToken{
				{
					Ident: false,
					S:     ps.lex.Token,
				},
			},
		}
		me.TagFilters = append(me.TagFilters[:0], &tfe)
		if err := ps.lex.Next(); err != nil {
			return nil, err
		}
		if ps.lex.Token != "{" {
			return &me, nil
		}
	}
	tfes, err := ps.parseTagFilters()
	if err != nil {
		return nil, err
	}
	me.TagFilters = append(me.TagFilters, tfes...)
	return &me, nil
}

func (ps *parseState) parseRollupExpr(arg Expr) (Expr, error) {
	var re RollupExpr
	re.Expr = arg
	if ps.lex.Token == "[" {
		window, step, inheritStep, err := ps.parseWindowAndStep()
		if err != nil {
			return nil, err
		}
		re.Window = window
		re.Step = step
		re.InheritStep = inheritStep
		if !isOffset(ps.lex.Token) {
			return &re, nil
		}
	}
	offset, err := ps.parseOffset()
	if err != nil {
		return nil, err
	}
	re.Offset = offset
	return &re, nil
}

func appendStringTagFilter(dst []byte, tf *TagFilter) []byte {
	if len(tf.Key) == 0 {
		dst = append(dst, "__name__"...)
	} else {
		dst = appendEscapedIdent(dst, []byte(tf.Key))
	}
	dst = appendStringTagFilterOp(dst, tf.IsRegexp, tf.IsNegative)
	return strconv.AppendQuote(dst, string(tf.Value))
}

func appendStringTagFilterOp(dst []byte, isRegexp, isNegative bool) []byte {
	var op string
	if isNegative {
		if isRegexp {
			op = "!~"
		} else {
			op = "!="
		}
	} else {
		if isRegexp {
			op = "=~"
		} else {
			op = "="
		}
	}
	return append(dst, op...)
}
