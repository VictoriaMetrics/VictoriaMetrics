package promql

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func getDefaultWithArgExprs() []*withArgExpr {
	defaultWithArgExprsOnce.Do(func() {
		defaultWithArgExprs = prepareWithArgExprs([]string{
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
	defaultWithArgExprs     []*withArgExpr
	defaultWithArgExprsOnce sync.Once
)

func prepareWithArgExprs(ss []string) []*withArgExpr {
	was := make([]*withArgExpr, len(ss))
	for i, s := range ss {
		was[i] = mustParseWithArgExpr(s)
	}
	if err := checkDuplicateWithArgNames(was); err != nil {
		logger.Panicf("BUG: %s", err)
	}
	return was
}

func checkDuplicateWithArgNames(was []*withArgExpr) error {
	m := make(map[string]*withArgExpr, len(was))
	for _, wa := range was {
		if waOld := m[wa.Name]; waOld != nil {
			return fmt.Errorf("duplicate `with` arg name for: %s; previous one: %s", wa, waOld.AppendString(nil))
		}
		m[wa.Name] = wa
	}
	return nil
}

func mustParseWithArgExpr(s string) *withArgExpr {
	var p parser
	p.lex.Init(s)
	if err := p.lex.Next(); err != nil {
		logger.Panicf("BUG: cannot find the first token in %q: %s", s, err)
	}
	wa, err := p.parseWithArgExpr()
	if err != nil {
		logger.Panicf("BUG: cannot parse %q: %s; unparsed data: %q", s, err, p.lex.Context())
	}
	return wa
}

func parsePromQL(s string) (expr, error) {
	var p parser
	p.lex.Init(s)
	if err := p.lex.Next(); err != nil {
		return nil, fmt.Errorf(`cannot find the first token: %s`, err)
	}
	e, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf(`%s; unparsed data: %q`, err, p.lex.Context())
	}
	if !isEOF(p.lex.Token) {
		return nil, fmt.Errorf(`unparsed data left: %q`, p.lex.Context())
	}
	was := getDefaultWithArgExprs()
	if e, err = expandWithExpr(was, e); err != nil {
		return nil, fmt.Errorf(`cannot expand WITH expressions: %s`, err)
	}
	e = removeParensExpr(e)
	e = simplifyConstants(e)
	return e, nil
}

// removeParensExpr removes parensExpr for (expr) case.
func removeParensExpr(e expr) expr {
	if re, ok := e.(*rollupExpr); ok {
		re.Expr = removeParensExpr(re.Expr)
		return re
	}
	if be, ok := e.(*binaryOpExpr); ok {
		be.Left = removeParensExpr(be.Left)
		be.Right = removeParensExpr(be.Right)
		return be
	}
	if ae, ok := e.(*aggrFuncExpr); ok {
		for i, arg := range ae.Args {
			ae.Args[i] = removeParensExpr(arg)
		}
		return ae
	}
	if fe, ok := e.(*funcExpr); ok {
		for i, arg := range fe.Args {
			fe.Args[i] = removeParensExpr(arg)
		}
		return fe
	}
	if pe, ok := e.(*parensExpr); ok {
		if len(*pe) == 1 {
			return removeParensExpr((*pe)[0])
		}
		// Treat parensExpr as a function with empty name, i.e. union()
		fe := &funcExpr{
			Name: "",
			Args: *pe,
		}
		return fe
	}
	return e
}

func simplifyConstants(e expr) expr {
	if re, ok := e.(*rollupExpr); ok {
		re.Expr = simplifyConstants(re.Expr)
		return re
	}
	if ae, ok := e.(*aggrFuncExpr); ok {
		simplifyConstantsInplace(ae.Args)
		return ae
	}
	if fe, ok := e.(*funcExpr); ok {
		simplifyConstantsInplace(fe.Args)
		return fe
	}
	if pe, ok := e.(*parensExpr); ok {
		if len(*pe) == 1 {
			return simplifyConstants((*pe)[0])
		}
		simplifyConstantsInplace(*pe)
		return pe
	}
	be, ok := e.(*binaryOpExpr)
	if !ok {
		return e
	}

	be.Left = simplifyConstants(be.Left)
	be.Right = simplifyConstants(be.Right)

	lne, ok := be.Left.(*numberExpr)
	if !ok {
		return be
	}
	rne, ok := be.Right.(*numberExpr)
	if !ok {
		return be
	}
	n := binaryOpConstants(be.Op, lne.N, rne.N, be.Bool)
	ne := &numberExpr{
		N: n,
	}
	return ne
}

func simplifyConstantsInplace(args []expr) {
	for i, arg := range args {
		args[i] = simplifyConstants(arg)
	}
}

// parser parses PromQL expression.
//
// preconditions for all parser.parse* funcs:
// - p.lex.Token should point to the first token to parse.
//
// postconditions for all parser.parse* funcs:
// - p.lex.Token should point to the next token after the parsed token.
type parser struct {
	lex lexer
}

func isWith(s string) bool {
	s = strings.ToLower(s)
	return s == "with"
}

// parseWithExpr parses `WITH (withArgExpr...) expr`.
func (p *parser) parseWithExpr() (*withExpr, error) {
	var we withExpr
	if !isWith(p.lex.Token) {
		return nil, fmt.Errorf("withExpr: unexpected token %q; want `WITH`", p.lex.Token)
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`withExpr: unexpected token %q; want "("`, p.lex.Token)
	}
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == ")" {
			goto end
		}
		wa, err := p.parseWithArgExpr()
		if err != nil {
			return nil, err
		}
		we.Was = append(we.Was, wa)
		switch p.lex.Token {
		case ",":
			continue
		case ")":
			goto end
		default:
			return nil, fmt.Errorf(`withExpr: unexpected token %q; want ",", ")"`, p.lex.Token)
		}
	}

end:
	if err := checkDuplicateWithArgNames(we.Was); err != nil {
		return nil, err
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	we.Expr = e
	return &we, nil
}

func (p *parser) parseWithArgExpr() (*withArgExpr, error) {
	var wa withArgExpr
	if !isIdentPrefix(p.lex.Token) {
		return nil, fmt.Errorf(`withArgExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}
	wa.Name = p.lex.Token
	if isAggrFunc(wa.Name) || isRollupFunc(wa.Name) || isTransformFunc(wa.Name) || isWith(wa.Name) {
		return nil, fmt.Errorf(`withArgExpr: cannot use reserved name %q`, wa.Name)
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if p.lex.Token == "(" {
		// Parse func args.
		args, err := p.parseIdentList()
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
	if p.lex.Token != "=" {
		return nil, fmt.Errorf(`withArgExpr: unexpected token %q; want "="`, p.lex.Token)
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	e, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf(`withArgExpr: cannot parse %q: %s`, wa.Name, err)
	}
	wa.Expr = e
	return &wa, nil
}

// parseExpr parses promql expr
func (p *parser) parseExpr() (expr, error) {
	e, err := p.parseSingleExpr()
	if err != nil {
		return nil, err
	}
	for {
		if !isBinaryOp(p.lex.Token) {
			return e, nil
		}

		var be binaryOpExpr
		be.Op = strings.ToLower(p.lex.Token)
		be.Left = e
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if isBinaryOpBoolModifier(p.lex.Token) {
			if !isBinaryOpCmp(be.Op) {
				return nil, fmt.Errorf(`bool modifier cannot be applied to %q`, be.Op)
			}
			be.Bool = true
			if err := p.lex.Next(); err != nil {
				return nil, err
			}
		}
		if isBinaryOpGroupModifier(p.lex.Token) {
			if err := p.parseModifierExpr(&be.GroupModifier); err != nil {
				return nil, err
			}
			if isBinaryOpJoinModifier(p.lex.Token) {
				if isBinaryOpLogicalSet(be.Op) {
					return nil, fmt.Errorf(`modifier %q cannot be applied to %q`, p.lex.Token, be.Op)
				}
				if err := p.parseModifierExpr(&be.JoinModifier); err != nil {
					return nil, err
				}
			}
		}
		e2, err := p.parseSingleExpr()
		if err != nil {
			return nil, err
		}
		be.Right = e2
		e = balanceBinaryOp(&be)
	}
}

func balanceBinaryOp(be *binaryOpExpr) expr {
	bel, ok := be.Left.(*binaryOpExpr)
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
func (p *parser) parseSingleExpr() (expr, error) {
	if isWith(p.lex.Token) {
		err := p.lex.Next()
		nextToken := p.lex.Token
		p.lex.Prev()
		if err == nil && nextToken == "(" {
			return p.parseWithExpr()
		}
	}
	e, err := p.parseSingleExprWithoutRollupSuffix()
	if err != nil {
		return nil, err
	}
	if p.lex.Token != "[" && !isOffset(p.lex.Token) {
		// There is no rollup expression.
		return e, nil
	}
	return p.parseRollupExpr(e)
}

func (p *parser) parseSingleExprWithoutRollupSuffix() (expr, error) {
	if isPositiveNumberPrefix(p.lex.Token) || isInfOrNaN(p.lex.Token) {
		return p.parsePositiveNumberExpr()
	}
	if isStringPrefix(p.lex.Token) {
		return p.parseStringExpr()
	}
	if isIdentPrefix(p.lex.Token) {
		return p.parseIdentExpr()
	}
	switch p.lex.Token {
	case "(":
		return p.parseParensExpr()
	case "{":
		return p.parseMetricExpr()
	case "-":
		// Unary minus. Substitute -expr with (0 - expr)
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		e, err := p.parseSingleExpr()
		if err != nil {
			return nil, err
		}
		be := &binaryOpExpr{
			Op: "-",
			Left: &numberExpr{
				N: 0,
			},
			Right: e,
		}
		pe := parensExpr{be}
		return &pe, nil
	case "+":
		// Unary plus
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		return p.parseSingleExpr()
	default:
		return nil, fmt.Errorf(`singleExpr: unexpected token %q; want "(", "{", "-", "+"`, p.lex.Token)
	}
}

func (p *parser) parsePositiveNumberExpr() (*numberExpr, error) {
	if !isPositiveNumberPrefix(p.lex.Token) && !isInfOrNaN(p.lex.Token) {
		return nil, fmt.Errorf(`positiveNumberExpr: unexpected token %q; want "number"`, p.lex.Token)
	}

	n, err := strconv.ParseFloat(p.lex.Token, 64)
	if err != nil {
		return nil, fmt.Errorf(`positiveNumberExpr: cannot parse %q: %s`, p.lex.Token, err)
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	ne := &numberExpr{
		N: n,
	}
	return ne, nil
}

func (p *parser) parseStringExpr() (*stringExpr, error) {
	var se stringExpr

	for {
		switch {
		case isStringPrefix(p.lex.Token) || isIdentPrefix(p.lex.Token):
			se.tokens = append(se.tokens, p.lex.Token)
		default:
			return nil, fmt.Errorf(`stringExpr: unexpected token %q; want "string"`, p.lex.Token)
		}
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token != "+" {
			return &se, nil
		}

		// composite stringExpr like `"s1" + "s2"`, `"s" + m()` or `"s" + m{}` or `"s" + unknownToken`.
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if isStringPrefix(p.lex.Token) {
			// "s1" + "s2"
			continue
		}
		if !isIdentPrefix(p.lex.Token) {
			// "s" + unknownToken
			p.lex.Prev()
			return &se, nil
		}
		// Look after ident
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == "(" || p.lex.Token == "{" {
			// `"s" + m(` or `"s" + m{`
			p.lex.Prev()
			p.lex.Prev()
			return &se, nil
		}
		// "s" + ident
		p.lex.Prev()
	}
}

func (p *parser) parseParensExpr() (*parensExpr, error) {
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`parensExpr: unexpected token %q; want "("`, p.lex.Token)
	}
	var exprs []expr
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == ")" {
			break
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)
		if p.lex.Token == "," {
			continue
		}
		if p.lex.Token == ")" {
			break
		}
		return nil, fmt.Errorf(`parensExpr: unexpected token %q; want "," or ")"`, p.lex.Token)
	}
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	pe := parensExpr(exprs)
	return &pe, nil
}

func (p *parser) parseAggrFuncExpr() (*aggrFuncExpr, error) {
	if !isAggrFunc(p.lex.Token) {
		return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want aggregate func`, p.lex.Token)
	}

	var ae aggrFuncExpr
	ae.Name = strings.ToLower(p.lex.Token)
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if isIdentPrefix(p.lex.Token) {
		goto funcPrefixLabel
	}
	switch p.lex.Token {
	case "(":
		goto funcArgsLabel
	default:
		return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want "("`, p.lex.Token)
	}

funcPrefixLabel:
	{
		if !isAggrFuncModifier(p.lex.Token) {
			return nil, fmt.Errorf(`aggrFuncExpr: unexpected token %q; want aggregate func modifier`, p.lex.Token)
		}
		if err := p.parseModifierExpr(&ae.Modifier); err != nil {
			return nil, err
		}
		goto funcArgsLabel
	}

funcArgsLabel:
	{
		args, err := p.parseArgListExpr()
		if err != nil {
			return nil, err
		}
		ae.Args = args

		// Verify whether func suffix exists.
		if ae.Modifier.Op != "" || !isAggrFuncModifier(p.lex.Token) {
			return &ae, nil
		}
		if err := p.parseModifierExpr(&ae.Modifier); err != nil {
			return nil, err
		}
		return &ae, nil
	}
}

func expandWithExpr(was []*withArgExpr, e expr) (expr, error) {
	switch t := e.(type) {
	case *binaryOpExpr:
		left, err := expandWithExpr(was, t.Left)
		if err != nil {
			return nil, err
		}
		right, err := expandWithExpr(was, t.Right)
		if err != nil {
			return nil, err
		}
		groupModifierArgs, err := expandModifierArgs(was, t.GroupModifier.Args)
		if err != nil {
			return nil, err
		}
		joinModifierArgs, err := expandModifierArgs(was, t.JoinModifier.Args)
		if err != nil {
			return nil, err
		}
		if t.Op == "+" {
			lse, lok := left.(*stringExpr)
			rse, rok := right.(*stringExpr)
			if lok && rok {
				se := &stringExpr{
					S: lse.S + rse.S,
				}
				return se, nil
			}
		}
		be := &binaryOpExpr{
			Op:            t.Op,
			Bool:          t.Bool,
			GroupModifier: t.GroupModifier,
			JoinModifier:  t.JoinModifier,
			Left:          left,
			Right:         right,
		}
		be.GroupModifier.Args = groupModifierArgs
		be.JoinModifier.Args = joinModifierArgs
		pe := parensExpr{be}
		return &pe, nil
	case *funcExpr:
		args, err := expandWithArgs(was, t.Args)
		if err != nil {
			return nil, err
		}
		wa := getWithArgExpr(was, t.Name)
		if wa == nil {
			fe := &funcExpr{
				Name: t.Name,
				Args: args,
			}
			return fe, nil
		}
		return expandWithExprExt(was, wa, args)
	case *aggrFuncExpr:
		args, err := expandWithArgs(was, t.Args)
		if err != nil {
			return nil, err
		}
		modifierArgs, err := expandModifierArgs(was, t.Modifier.Args)
		if err != nil {
			return nil, err
		}
		ae := &aggrFuncExpr{
			Name:     t.Name,
			Args:     args,
			Modifier: t.Modifier,
		}
		ae.Modifier.Args = modifierArgs
		return ae, nil
	case *parensExpr:
		exprs, err := expandWithArgs(was, *t)
		if err != nil {
			return nil, err
		}
		pe := parensExpr(exprs)
		return &pe, nil
	case *stringExpr:
		if len(t.S) > 0 {
			// Already expanded.
			return t, nil
		}
		var b []byte
		for _, token := range t.tokens {
			if isStringPrefix(token) {
				s, err := extractStringValue(token)
				if err != nil {
					return nil, err
				}
				b = append(b, s...)
				continue
			}
			wa := getWithArgExpr(was, token)
			if wa == nil {
				return nil, fmt.Errorf("missing %q value inside stringExpr", token)
			}
			eNew, err := expandWithExprExt(was, wa, nil)
			if err != nil {
				return nil, err
			}
			seSrc, ok := eNew.(*stringExpr)
			if !ok {
				return nil, fmt.Errorf("%q must be string expression; got %q", token, eNew.AppendString(nil))
			}
			if len(seSrc.tokens) > 0 {
				logger.Panicf("BUG: seSrc.tokens must be empty; got %q", seSrc.tokens)
			}
			b = append(b, seSrc.S...)
		}
		se := &stringExpr{
			S: string(b),
		}
		return se, nil
	case *rollupExpr:
		eNew, err := expandWithExpr(was, t.Expr)
		if err != nil {
			return nil, err
		}
		re := *t
		re.Expr = eNew
		return &re, nil
	case *withExpr:
		wasNew := make([]*withArgExpr, 0, len(was)+len(t.Was))
		wasNew = append(wasNew, was...)
		wasNew = append(wasNew, t.Was...)
		eNew, err := expandWithExpr(wasNew, t.Expr)
		if err != nil {
			return nil, err
		}
		return eNew, nil
	case *metricExpr:
		if len(t.TagFilters) > 0 {
			// Already expanded.
			return t, nil
		}
		{
			var me metricExpr
			// Populate me.TagFilters
			for _, tfe := range t.tagFilters {
				if tfe.Value == nil {
					// Expand tfe.Key into storage.TagFilters.
					wa := getWithArgExpr(was, tfe.Key)
					if wa == nil {
						return nil, fmt.Errorf("missing %q value inside %q", tfe.Key, t.AppendString(nil))
					}
					eNew, err := expandWithExprExt(was, wa, nil)
					if err != nil {
						return nil, err
					}
					wme, ok := eNew.(*metricExpr)
					if !ok || wme.HasNonEmptyMetricGroup() {
						return nil, fmt.Errorf("%q must be filters expression inside %q; got %q", tfe.Key, t.AppendString(nil), eNew.AppendString(nil))
					}
					if len(wme.tagFilters) > 0 {
						logger.Panicf("BUG: wme.tagFilters must be empty; got %s", wme.tagFilters)
					}
					me.TagFilters = append(me.TagFilters, wme.TagFilters...)
					continue
				}

				// convert tfe to storage.TagFilter.
				se, err := expandWithExpr(was, tfe.Value)
				if err != nil {
					return nil, err
				}
				var tfeNew tagFilterExpr
				tfeNew.Key = tfe.Key
				tfeNew.Value = se.(*stringExpr)
				tfeNew.IsNegative = tfe.IsNegative
				tfeNew.IsRegexp = tfe.IsRegexp
				tf, err := tfeNew.toTagFilter()
				if err != nil {
					return nil, err
				}
				me.TagFilters = append(me.TagFilters, *tf)
			}
			me.TagFilters = removeDuplicateTagFilters(me.TagFilters)
			t = &me
		}
		if !t.HasNonEmptyMetricGroup() {
			return t, nil
		}
		k := string(appendEscapedIdent(nil, t.TagFilters[0].Value))
		wa := getWithArgExpr(was, k)
		if wa == nil {
			return t, nil
		}
		eNew, err := expandWithExprExt(was, wa, nil)
		if err != nil {
			return nil, err
		}
		var wme *metricExpr
		re, _ := eNew.(*rollupExpr)
		if re != nil {
			wme, _ = re.Expr.(*metricExpr)
		} else {
			wme, _ = eNew.(*metricExpr)
		}
		if wme == nil {
			if !t.IsOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot expand %q to non-metric expression %q", t.AppendString(nil), eNew.AppendString(nil))
			}
			return eNew, nil
		}
		if len(wme.tagFilters) > 0 {
			logger.Panicf("BUG: wme.tagFilters must be empty; got %s", wme.tagFilters)
		}

		var me metricExpr
		me.TagFilters = append(me.TagFilters, wme.TagFilters...)
		me.TagFilters = append(me.TagFilters, t.TagFilters[1:]...)
		me.TagFilters = removeDuplicateTagFilters(me.TagFilters)

		if re == nil {
			return &me, nil
		}
		reNew := *re
		reNew.Expr = &me
		return &reNew, nil
	default:
		return e, nil
	}
}

func expandWithArgs(was []*withArgExpr, args []expr) ([]expr, error) {
	dstArgs := make([]expr, len(args))
	for i, arg := range args {
		dstArg, err := expandWithExpr(was, arg)
		if err != nil {
			return nil, err
		}
		dstArgs[i] = dstArg
	}
	return dstArgs, nil
}

func expandModifierArgs(was []*withArgExpr, args []string) ([]string, error) {
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
		me, ok := wa.Expr.(*metricExpr)
		if ok {
			if !me.IsOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot use %q instead of %q in %s", me.AppendString(nil), arg, args)
			}
			dstArg := string(me.TagFilters[0].Value)
			dstArgs = append(dstArgs, dstArg)
			continue
		}
		pe, ok := wa.Expr.(*parensExpr)
		if ok {
			for _, pArg := range *pe {
				me, ok := pArg.(*metricExpr)
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

func expandWithExprExt(was []*withArgExpr, wa *withArgExpr, args []expr) (expr, error) {
	if len(wa.Args) != len(args) {
		if args == nil {
			// Just return metricExpr with the wa.Name name.
			return newMetricExpr(wa.Name), nil
		}
		return nil, fmt.Errorf("invalid number of args for %q; got %d; want %d", wa.Name, len(args), len(wa.Args))
	}
	wasNew := make([]*withArgExpr, 0, len(was)+len(args))
	for _, waTmp := range was {
		if waTmp == wa {
			break
		}
		wasNew = append(wasNew, waTmp)
	}
	for i, arg := range args {
		wasNew = append(wasNew, &withArgExpr{
			Name: wa.Args[i],
			Expr: arg,
		})
	}
	return expandWithExpr(wasNew, wa.Expr)
}

func newMetricExpr(name string) *metricExpr {
	return &metricExpr{
		TagFilters: []storage.TagFilter{{
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

func removeDuplicateTagFilters(tfs []storage.TagFilter) []storage.TagFilter {
	tfsm := make(map[string]bool, len(tfs))
	tfsNew := tfs[:0]
	bb := bbPool.Get()
	for i := range tfs {
		tf := &tfs[i]
		bb.B = appendStringTagFilter(bb.B[:0], tf)
		if tfsm[string(bb.B)] {
			continue
		}
		tfsm[string(bb.B)] = true
		tfsNew = append(tfsNew, *tf)
	}
	bbPool.Put(bb)
	return tfsNew
}

func (p *parser) parseFuncExpr() (*funcExpr, error) {
	if !isIdentPrefix(p.lex.Token) {
		return nil, fmt.Errorf(`funcExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}

	var fe funcExpr
	fe.Name = p.lex.Token
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`funcExpr; unexpected token %q; want "("`, p.lex.Token)
	}
	args, err := p.parseArgListExpr()
	if err != nil {
		return nil, err
	}
	fe.Args = args
	return &fe, nil
}

func (p *parser) parseModifierExpr(me *modifierExpr) error {
	if !isIdentPrefix(p.lex.Token) {
		return fmt.Errorf(`modifierExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}

	me.Op = strings.ToLower(p.lex.Token)

	if err := p.lex.Next(); err != nil {
		return err
	}
	if isBinaryOpJoinModifier(me.Op) && p.lex.Token != "(" {
		// join modifier may miss ident list.
		return nil
	}
	args, err := p.parseIdentList()
	if err != nil {
		return err
	}
	me.Args = args
	return nil
}

func (p *parser) parseIdentList() ([]string, error) {
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`identList: unexpected token %q; want "("`, p.lex.Token)
	}
	var idents []string
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == ")" {
			goto closeParensLabel
		}
		if !isIdentPrefix(p.lex.Token) {
			return nil, fmt.Errorf(`identList: unexpected token %q; want "ident"`, p.lex.Token)
		}
		idents = append(idents, p.lex.Token)
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		switch p.lex.Token {
		case ",":
			continue
		case ")":
			goto closeParensLabel
		default:
			return nil, fmt.Errorf(`identList: unexpected token %q; want ",", ")"`, p.lex.Token)
		}
	}

closeParensLabel:
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	return idents, nil
}

func (p *parser) parseArgListExpr() ([]expr, error) {
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`argList: unexpected token %q; want "("`, p.lex.Token)
	}
	var args []expr
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == ")" {
			goto closeParensLabel
		}
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		args = append(args, expr)
		switch p.lex.Token {
		case ",":
			continue
		case ")":
			goto closeParensLabel
		default:
			return nil, fmt.Errorf(`argList: unexpected token %q; want ",", ")"`, p.lex.Token)
		}
	}

closeParensLabel:
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	return args, nil
}

func getWithArgExpr(was []*withArgExpr, name string) *withArgExpr {
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

func (p *parser) parseTagFilters() ([]*tagFilterExpr, error) {
	if p.lex.Token != "{" {
		return nil, fmt.Errorf(`tagFilters: unexpected token %q; want "{"`, p.lex.Token)
	}

	var tfes []*tagFilterExpr
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == "}" {
			goto closeBracesLabel
		}
		tfe, err := p.parseTagFilterExpr()
		if err != nil {
			return nil, err
		}
		tfes = append(tfes, tfe)
		switch p.lex.Token {
		case ",":
			continue
		case "}":
			goto closeBracesLabel
		default:
			return nil, fmt.Errorf(`tagFilters: unexpected token %q; want ",", "}"`, p.lex.Token)
		}
	}

closeBracesLabel:
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	return tfes, nil
}

func (p *parser) parseTagFilterExpr() (*tagFilterExpr, error) {
	if !isIdentPrefix(p.lex.Token) {
		return nil, fmt.Errorf(`tagFilterExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}
	var tfe tagFilterExpr
	tfe.Key = p.lex.Token
	if err := p.lex.Next(); err != nil {
		return nil, err
	}

	switch p.lex.Token {
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
		return nil, fmt.Errorf(`tagFilterExpr: unexpected token %q; want "=", "!=", "=~", "!~", ",", "}"`, p.lex.Token)
	}

	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	se, err := p.parseStringExpr()
	if err != nil {
		return nil, err
	}
	tfe.Value = se
	return &tfe, nil
}

type tagFilterExpr struct {
	Key        string
	Value      *stringExpr
	IsRegexp   bool
	IsNegative bool
}

func (tfe *tagFilterExpr) String() string {
	return fmt.Sprintf("[key=%q, value=%+v, isRegexp=%v, isNegative=%v]", tfe.Key, tfe.Value, tfe.IsRegexp, tfe.IsNegative)
}

func (tfe *tagFilterExpr) toTagFilter() (*storage.TagFilter, error) {
	if tfe.Value == nil || len(tfe.Value.tokens) > 0 {
		logger.Panicf("BUG: tfe.Value must be already expanded; got %v", tfe.Value)
	}

	var tf storage.TagFilter
	tf.Key = []byte(unescapeIdent(tfe.Key))
	if len(tfe.Key) == 0 {
		tf.Value = []byte(unescapeIdent(tfe.Value.S))
	} else {
		tf.Value = []byte(tfe.Value.S)
	}
	if string(tf.Key) == "__name__" {
		// This is required for storage.Search
		tf.Key = nil
	}
	tf.IsRegexp = tfe.IsRegexp
	tf.IsNegative = tfe.IsNegative
	if !tf.IsRegexp {
		return &tf, nil
	}

	// Verify regexp.
	if _, err := compileRegexpAnchored(tfe.Value.S); err != nil {
		return nil, fmt.Errorf("invalid regexp in %s=%q: %s", tf.Key, tf.Value, err)
	}
	return &tf, nil
}

func (p *parser) parseWindowAndStep() (string, string, bool, error) {
	if p.lex.Token != "[" {
		return "", "", false, fmt.Errorf(`windowAndStep: unexpected token %q; want "["`, p.lex.Token)
	}
	err := p.lex.Next()
	if err != nil {
		return "", "", false, err
	}
	var window string
	if !strings.HasPrefix(p.lex.Token, ":") {
		window, err = p.parseDuration()
		if err != nil {
			return "", "", false, err
		}
	}
	var step string
	inheritStep := false
	if strings.HasPrefix(p.lex.Token, ":") {
		// Parse step
		p.lex.Token = p.lex.Token[1:]
		if p.lex.Token == "" {
			if err := p.lex.Next(); err != nil {
				return "", "", false, err
			}
			if p.lex.Token == "]" {
				inheritStep = true
			}
		}
		if p.lex.Token != "]" {
			step, err = p.parseDuration()
			if err != nil {
				return "", "", false, err
			}
		}
	}
	if p.lex.Token != "]" {
		return "", "", false, fmt.Errorf(`windowAndStep: unexpected token %q; want "]"`, p.lex.Token)
	}
	if err := p.lex.Next(); err != nil {
		return "", "", false, err
	}
	return window, step, inheritStep, nil
}

func (p *parser) parseOffset() (string, error) {
	if !isOffset(p.lex.Token) {
		return "", fmt.Errorf(`offset: unexpected token %q; want "offset"`, p.lex.Token)
	}
	if err := p.lex.Next(); err != nil {
		return "", err
	}
	d, err := p.parseDuration()
	if err != nil {
		return "", err
	}
	return d, nil
}

func (p *parser) parseDuration() (string, error) {
	if !isDuration(p.lex.Token) {
		return "", fmt.Errorf(`duration: unexpected token %q; want "duration"`, p.lex.Token)
	}
	d := p.lex.Token
	if err := p.lex.Next(); err != nil {
		return "", err
	}
	return d, nil
}

// parseIdentExpr parses expressions starting with `ident` token.
func (p *parser) parseIdentExpr() (expr, error) {
	// Look into the next-next token in order to determine how to parse
	// the current expression.
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if isEOF(p.lex.Token) || isOffset(p.lex.Token) {
		p.lex.Prev()
		return p.parseMetricExpr()
	}
	if isIdentPrefix(p.lex.Token) {
		p.lex.Prev()
		if isAggrFunc(p.lex.Token) {
			return p.parseAggrFuncExpr()
		}
		return p.parseMetricExpr()
	}
	if isBinaryOp(p.lex.Token) {
		p.lex.Prev()
		return p.parseMetricExpr()
	}
	switch p.lex.Token {
	case "(":
		p.lex.Prev()
		if isAggrFunc(p.lex.Token) {
			return p.parseAggrFuncExpr()
		}
		return p.parseFuncExpr()
	case "{", "[", ")", ",":
		p.lex.Prev()
		return p.parseMetricExpr()
	default:
		return nil, fmt.Errorf(`identExpr: unexpected token %q; want "(", "{", "[", ")", ","`, p.lex.Token)
	}
}

// IsMetricSelectorWithRollup verifies whether s contains PromQL metric selector
// wrapped into rollup.
//
// It returns the wrapped query with the corresponding window with offset.
func IsMetricSelectorWithRollup(s string) (childQuery string, window, offset string) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return
	}
	re, ok := expr.(*rollupExpr)
	if !ok || len(re.Window) == 0 || len(re.Step) > 0 {
		return
	}
	me, ok := re.Expr.(*metricExpr)
	if !ok || len(me.TagFilters) == 0 {
		return
	}
	wrappedQuery := me.AppendString(nil)
	return string(wrappedQuery), re.Window, re.Offset
}

// ParseMetricSelector parses s containing PromQL metric selector
// and returns the corresponding TagFilters.
func ParseMetricSelector(s string) ([]storage.TagFilter, error) {
	expr, err := parsePromQLWithCache(s)
	if err != nil {
		return nil, err
	}
	me, ok := expr.(*metricExpr)
	if !ok {
		return nil, fmt.Errorf("expecting metricSelector; got %q", expr.AppendString(nil))
	}
	if len(me.TagFilters) == 0 {
		return nil, fmt.Errorf("tagFilters cannot be empty")
	}
	return me.TagFilters, nil
}

func (p *parser) parseMetricExpr() (*metricExpr, error) {
	var me metricExpr
	if isIdentPrefix(p.lex.Token) {
		var tfe tagFilterExpr
		tfe.Value = &stringExpr{
			tokens: []string{strconv.Quote(p.lex.Token)},
		}
		me.tagFilters = append(me.tagFilters[:0], &tfe)
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token != "{" {
			return &me, nil
		}
	}
	tfes, err := p.parseTagFilters()
	if err != nil {
		return nil, err
	}
	me.tagFilters = append(me.tagFilters, tfes...)
	return &me, nil
}

func (p *parser) parseRollupExpr(arg expr) (expr, error) {
	var re rollupExpr
	re.Expr = arg
	if p.lex.Token == "[" {
		window, step, inheritStep, err := p.parseWindowAndStep()
		if err != nil {
			return nil, err
		}
		re.Window = window
		re.Step = step
		re.InheritStep = inheritStep
		if !isOffset(p.lex.Token) {
			return &re, nil
		}
	}
	offset, err := p.parseOffset()
	if err != nil {
		return nil, err
	}
	re.Offset = offset
	return &re, nil
}

type expr interface {
	// AppendString appends string representation of expr to dst.
	AppendString(dst []byte) []byte
}

type stringExpr struct {
	S string

	// Composite string has non-empty tokens.
	// They must be converted into S by expandWithExpr.
	tokens []string
}

func (se *stringExpr) AppendString(dst []byte) []byte {
	return strconv.AppendQuote(dst, se.S)
}

type numberExpr struct {
	N float64
}

func (ne *numberExpr) AppendString(dst []byte) []byte {
	return strconv.AppendFloat(dst, ne.N, 'g', -1, 64)
}

type parensExpr []expr

func (pe parensExpr) AppendString(dst []byte) []byte {
	return appendStringArgListExpr(dst, pe)
}

type binaryOpExpr struct {
	Op string

	Bool          bool
	GroupModifier modifierExpr
	JoinModifier  modifierExpr

	Left  expr
	Right expr
}

func (be *binaryOpExpr) AppendString(dst []byte) []byte {
	if _, ok := be.Left.(*binaryOpExpr); ok {
		dst = append(dst, '(')
		dst = be.Left.AppendString(dst)
		dst = append(dst, ')')
	} else {
		dst = be.Left.AppendString(dst)
	}
	dst = append(dst, ' ')
	dst = append(dst, be.Op...)
	if be.Bool {
		dst = append(dst, " bool"...)
	}
	if be.GroupModifier.Op != "" {
		dst = append(dst, ' ')
		dst = be.GroupModifier.AppendString(dst)
	}
	if be.JoinModifier.Op != "" {
		dst = append(dst, ' ')
		dst = be.JoinModifier.AppendString(dst)
	}
	dst = append(dst, ' ')
	if _, ok := be.Right.(*binaryOpExpr); ok {
		dst = append(dst, '(')
		dst = be.Right.AppendString(dst)
		dst = append(dst, ')')
	} else {
		dst = be.Right.AppendString(dst)
	}
	return dst
}

type modifierExpr struct {
	Op string

	Args []string
}

func (me *modifierExpr) AppendString(dst []byte) []byte {
	dst = append(dst, me.Op...)
	dst = append(dst, " ("...)
	for i, arg := range me.Args {
		dst = append(dst, arg...)
		if i+1 < len(me.Args) {
			dst = append(dst, ", "...)
		}
	}
	dst = append(dst, ')')
	return dst
}

func appendStringArgListExpr(dst []byte, args []expr) []byte {
	dst = append(dst, '(')
	for i, arg := range args {
		dst = arg.AppendString(dst)
		if i+1 < len(args) {
			dst = append(dst, ", "...)
		}
	}
	dst = append(dst, ')')
	return dst
}

type funcExpr struct {
	Name string

	Args []expr
}

func (fe *funcExpr) AppendString(dst []byte) []byte {
	dst = append(dst, fe.Name...)
	dst = appendStringArgListExpr(dst, fe.Args)
	return dst
}

type aggrFuncExpr struct {
	Name string

	Args []expr

	Modifier modifierExpr
}

func (ae *aggrFuncExpr) AppendString(dst []byte) []byte {
	dst = append(dst, ae.Name...)
	dst = appendStringArgListExpr(dst, ae.Args)
	if ae.Modifier.Op != "" {
		dst = append(dst, ' ')
		dst = ae.Modifier.AppendString(dst)
	}
	return dst
}

type withExpr struct {
	Was  []*withArgExpr
	Expr expr
}

func (we *withExpr) AppendString(dst []byte) []byte {
	dst = append(dst, "WITH ("...)
	for i, wa := range we.Was {
		dst = wa.AppendString(dst)
		if i+1 < len(we.Was) {
			dst = append(dst, ',')
		}
	}
	dst = append(dst, ") "...)
	dst = we.Expr.AppendString(dst)
	return dst
}

type withArgExpr struct {
	Name string
	Args []string
	Expr expr
}

func (wa *withArgExpr) AppendString(dst []byte) []byte {
	dst = append(dst, wa.Name...)
	if len(wa.Args) > 0 {
		dst = append(dst, '(')
		for i, arg := range wa.Args {
			dst = append(dst, arg...)
			if i+1 < len(wa.Args) {
				dst = append(dst, ',')
			}
		}
		dst = append(dst, ')')
	}
	dst = append(dst, " = "...)
	dst = wa.Expr.AppendString(dst)
	return dst
}

type rollupExpr struct {
	// The expression for the rollup. Usually it is metricExpr, but may be arbitrary expr
	// if subquery is used. https://prometheus.io/blog/2019/01/28/subquery-support/
	Expr expr

	// Window contains optional window value from square brackets
	//
	// For example, `http_requests_total[5m]` will have Window value `5m`.
	Window string

	// Offset contains optional value from `offset` part.
	//
	// For example, `foobar{baz="aa"} offset 5m` will have Offset value `5m`.
	Offset string

	// Step contains optional step value from square brackets.
	//
	// For example, `foobar[1h:3m]` will have Step value '3m'.
	Step string

	// If set to true, then `foo[1h:]` would print the same
	// instead of `foo[1h]`.
	InheritStep bool
}

func (re *rollupExpr) ForSubquery() bool {
	return len(re.Step) > 0 || re.InheritStep
}

func (re *rollupExpr) AppendString(dst []byte) []byte {
	needParens := func() bool {
		if _, ok := re.Expr.(*rollupExpr); ok {
			return true
		}
		if _, ok := re.Expr.(*binaryOpExpr); ok {
			return true
		}
		if ae, ok := re.Expr.(*aggrFuncExpr); ok && ae.Modifier.Op != "" {
			return true
		}
		return false
	}()
	if needParens {
		dst = append(dst, '(')
	}
	dst = re.Expr.AppendString(dst)
	if needParens {
		dst = append(dst, ')')
	}
	if len(re.Window) > 0 || re.InheritStep || len(re.Step) > 0 {
		dst = append(dst, '[')
		if len(re.Window) > 0 {
			dst = append(dst, re.Window...)
		}
		if len(re.Step) > 0 {
			dst = append(dst, ':')
			dst = append(dst, re.Step...)
		} else if re.InheritStep {
			dst = append(dst, ':')
		}
		dst = append(dst, ']')
	}
	if len(re.Offset) > 0 {
		dst = append(dst, " offset "...)
		dst = append(dst, re.Offset...)
	}
	return dst
}

type metricExpr struct {
	// TagFilters contains a list of tag filters from curly braces.
	// The first item may be the metric name.
	TagFilters []storage.TagFilter

	// tagFilters must be expanded to TagFilters by expandWithExpr.
	tagFilters []*tagFilterExpr
}

func (me *metricExpr) AppendString(dst []byte) []byte {
	tfs := me.TagFilters
	if len(tfs) > 0 {
		tf := &tfs[0]
		if len(tf.Key) == 0 && !tf.IsNegative && !tf.IsRegexp {
			dst = appendEscapedIdent(dst, tf.Value)
			tfs = tfs[1:]
		}
	}
	if len(tfs) > 0 {
		dst = append(dst, '{')
		for i := range tfs {
			dst = appendStringTagFilter(dst, &tfs[i])
			if i+1 < len(tfs) {
				dst = append(dst, ", "...)
			}
		}
		dst = append(dst, '}')
	} else if len(me.TagFilters) == 0 {
		dst = append(dst, "{}"...)
	}
	return dst
}

func (me *metricExpr) IsEmpty() bool {
	return len(me.TagFilters) == 0
}

func (me *metricExpr) IsOnlyMetricGroup() bool {
	if !me.HasNonEmptyMetricGroup() {
		return false
	}
	return len(me.TagFilters) == 1
}

func (me *metricExpr) HasNonEmptyMetricGroup() bool {
	if len(me.TagFilters) == 0 {
		return false
	}
	tf := &me.TagFilters[0]
	return len(tf.Key) == 0 && !tf.IsNegative && !tf.IsRegexp
}

func appendStringTagFilter(dst []byte, tf *storage.TagFilter) []byte {
	if len(tf.Key) == 0 {
		dst = append(dst, "__name__"...)
	} else {
		dst = appendEscapedIdent(dst, tf.Key)
	}
	var op string
	if tf.IsNegative {
		if tf.IsRegexp {
			op = "!~"
		} else {
			op = "!="
		}
	} else {
		if tf.IsRegexp {
			op = "=~"
		} else {
			op = "="
		}
	}
	dst = append(dst, op...)
	dst = strconv.AppendQuote(dst, string(tf.Value))
	return dst
}
