package metricsql

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
)

// Parse parses MetricsQL query s.
//
// All the `WITH` expressions are expanded in the returned Expr.
//
// MetricsQL is backwards-compatible with PromQL.
func Parse(s string) (Expr, error) {
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

// Expr holds any of *Expr types.
type Expr interface {
	// AppendString appends string representation of Expr to dst.
	AppendString(dst []byte) []byte
}

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
		panic(fmt.Errorf("BUG: %s", err))
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
		panic(fmt.Errorf("BUG: cannot find the first token in %q: %s", s, err))
	}
	wa, err := p.parseWithArgExpr()
	if err != nil {
		panic(fmt.Errorf("BUG: cannot parse %q: %s; unparsed data: %q", s, err, p.lex.Context()))
	}
	return wa
}

// removeParensExpr removes parensExpr for (Expr) case.
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
	if pe, ok := e.(*parensExpr); ok {
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

func simplifyConstants(e Expr) Expr {
	if re, ok := e.(*RollupExpr); ok {
		re.Expr = simplifyConstants(re.Expr)
		return re
	}
	if ae, ok := e.(*AggrFuncExpr); ok {
		simplifyConstantsInplace(ae.Args)
		return ae
	}
	if fe, ok := e.(*FuncExpr); ok {
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
	be, ok := e.(*BinaryOpExpr)
	if !ok {
		return e
	}

	be.Left = simplifyConstants(be.Left)
	be.Right = simplifyConstants(be.Right)

	lne, ok := be.Left.(*NumberExpr)
	if !ok {
		return be
	}
	rne, ok := be.Right.(*NumberExpr)
	if !ok {
		return be
	}
	n := binaryOpEval(be.Op, lne.N, rne.N, be.Bool)
	ne := &NumberExpr{
		N: n,
	}
	return ne
}

func simplifyConstantsInplace(args []Expr) {
	for i, arg := range args {
		args[i] = simplifyConstants(arg)
	}
}

// parser parses MetricsQL expression.
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
	if isAggrFunc(wa.Name) || IsRollupFunc(wa.Name) || IsTransformFunc(wa.Name) || isWith(wa.Name) {
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

func (p *parser) parseExpr() (Expr, error) {
	e, err := p.parseSingleExpr()
	if err != nil {
		return nil, err
	}
	for {
		if !isBinaryOp(p.lex.Token) {
			return e, nil
		}

		var be BinaryOpExpr
		be.Op = strings.ToLower(p.lex.Token)
		be.Left = e
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if isBinaryOpBoolModifier(p.lex.Token) {
			if !IsBinaryOpCmp(be.Op) {
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
func (p *parser) parseSingleExpr() (Expr, error) {
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

func (p *parser) parseSingleExprWithoutRollupSuffix() (Expr, error) {
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
		be := &BinaryOpExpr{
			Op: "-",
			Left: &NumberExpr{
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

func (p *parser) parsePositiveNumberExpr() (*NumberExpr, error) {
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
	ne := &NumberExpr{
		N: n,
	}
	return ne, nil
}

func (p *parser) parseStringExpr() (*StringExpr, error) {
	var se StringExpr

	for {
		switch {
		case isStringPrefix(p.lex.Token) || isIdentPrefix(p.lex.Token):
			se.tokens = append(se.tokens, p.lex.Token)
		default:
			return nil, fmt.Errorf(`StringExpr: unexpected token %q; want "string"`, p.lex.Token)
		}
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token != "+" {
			return &se, nil
		}

		// composite StringExpr like `"s1" + "s2"`, `"s" + m()` or `"s" + m{}` or `"s" + unknownToken`.
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
	var exprs []Expr
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

func (p *parser) parseAggrFuncExpr() (*AggrFuncExpr, error) {
	if !isAggrFunc(p.lex.Token) {
		return nil, fmt.Errorf(`AggrFuncExpr: unexpected token %q; want aggregate func`, p.lex.Token)
	}

	var ae AggrFuncExpr
	ae.Name = strings.ToLower(p.lex.Token)
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if isIdentPrefix(p.lex.Token) {
		goto funcPrefixLabel
	}
	if p.lex.Token == "(" {
		goto funcArgsLabel
	}
	return nil, fmt.Errorf(`AggrFuncExpr: unexpected token %q; want "("`, p.lex.Token)

funcPrefixLabel:
	{
		if !isAggrFuncModifier(p.lex.Token) {
			return nil, fmt.Errorf(`AggrFuncExpr: unexpected token %q; want aggregate func modifier`, p.lex.Token)
		}
		if err := p.parseModifierExpr(&ae.Modifier); err != nil {
			return nil, err
		}
	}

funcArgsLabel:
	{
		args, err := p.parseArgListExpr()
		if err != nil {
			return nil, err
		}
		ae.Args = args

		// Verify whether func suffix exists.
		if ae.Modifier.Op == "" && isAggrFuncModifier(p.lex.Token) {
			if err := p.parseModifierExpr(&ae.Modifier); err != nil {
				return nil, err
			}
		}

		// Check for optional limit.
		if strings.ToLower(p.lex.Token) == "limit" {
			if err := p.lex.Next(); err != nil {
				return nil, err
			}
			limit, err := strconv.Atoi(p.lex.Token)
			if err != nil {
				return nil, fmt.Errorf("cannot parse limit %q: %s", p.lex.Token, err)
			}
			if err := p.lex.Next(); err != nil {
				return nil, err
			}
			ae.Limit = limit
		}
		return &ae, nil
	}
}

func expandWithExpr(was []*withArgExpr, e Expr) (Expr, error) {
	switch t := e.(type) {
	case *BinaryOpExpr:
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
		pe := parensExpr{be}
		return &pe, nil
	case *FuncExpr:
		args, err := expandWithArgs(was, t.Args)
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
		return expandWithExprExt(was, wa, args)
	case *AggrFuncExpr:
		args, err := expandWithArgs(was, t.Args)
		if err != nil {
			return nil, err
		}
		modifierArgs, err := expandModifierArgs(was, t.Modifier.Args)
		if err != nil {
			return nil, err
		}
		ae := &AggrFuncExpr{
			Name:     t.Name,
			Args:     args,
			Modifier: t.Modifier,
			Limit:    t.Limit,
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
	case *StringExpr:
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
				return nil, fmt.Errorf("missing %q value inside StringExpr", token)
			}
			eNew, err := expandWithExprExt(was, wa, nil)
			if err != nil {
				return nil, err
			}
			seSrc, ok := eNew.(*StringExpr)
			if !ok {
				return nil, fmt.Errorf("%q must be string expression; got %q", token, eNew.AppendString(nil))
			}
			if len(seSrc.tokens) > 0 {
				panic(fmt.Errorf("BUG: seSrc.tokens must be empty; got %q", seSrc.tokens))
			}
			b = append(b, seSrc.S...)
		}
		se := &StringExpr{
			S: string(b),
		}
		return se, nil
	case *RollupExpr:
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
	case *MetricExpr:
		if len(t.LabelFilters) > 0 {
			// Already expanded.
			return t, nil
		}
		{
			var me MetricExpr
			// Populate me.LabelFilters
			for _, lfe := range t.labelFilters {
				if lfe.Value == nil {
					// Expand lfe.Label into []LabelFilter.
					wa := getWithArgExpr(was, lfe.Label)
					if wa == nil {
						return nil, fmt.Errorf("missing %q value inside %q", lfe.Label, t.AppendString(nil))
					}
					eNew, err := expandWithExprExt(was, wa, nil)
					if err != nil {
						return nil, err
					}
					wme, ok := eNew.(*MetricExpr)
					if !ok || wme.hasNonEmptyMetricGroup() {
						return nil, fmt.Errorf("%q must be filters expression inside %q; got %q", lfe.Label, t.AppendString(nil), eNew.AppendString(nil))
					}
					if len(wme.labelFilters) > 0 {
						panic(fmt.Errorf("BUG: wme.labelFilters must be empty; got %s", wme.labelFilters))
					}
					me.LabelFilters = append(me.LabelFilters, wme.LabelFilters...)
					continue
				}

				// convert lfe to LabelFilter.
				se, err := expandWithExpr(was, lfe.Value)
				if err != nil {
					return nil, err
				}
				var lfeNew labelFilterExpr
				lfeNew.Label = lfe.Label
				lfeNew.Value = se.(*StringExpr)
				lfeNew.IsNegative = lfe.IsNegative
				lfeNew.IsRegexp = lfe.IsRegexp
				lf, err := lfeNew.toLabelFilter()
				if err != nil {
					return nil, err
				}
				me.LabelFilters = append(me.LabelFilters, *lf)
			}
			me.LabelFilters = removeDuplicateLabelFilters(me.LabelFilters)
			t = &me
		}
		if !t.hasNonEmptyMetricGroup() {
			return t, nil
		}
		k := string(appendEscapedIdent(nil, t.LabelFilters[0].Value))
		wa := getWithArgExpr(was, k)
		if wa == nil {
			return t, nil
		}
		eNew, err := expandWithExprExt(was, wa, nil)
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
			if !t.isOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot expand %q to non-metric expression %q", t.AppendString(nil), eNew.AppendString(nil))
			}
			return eNew, nil
		}
		if len(wme.labelFilters) > 0 {
			panic(fmt.Errorf("BUG: wme.labelFilters must be empty; got %s", wme.labelFilters))
		}

		var me MetricExpr
		me.LabelFilters = append(me.LabelFilters, wme.LabelFilters...)
		me.LabelFilters = append(me.LabelFilters, t.LabelFilters[1:]...)
		me.LabelFilters = removeDuplicateLabelFilters(me.LabelFilters)

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

func expandWithArgs(was []*withArgExpr, args []Expr) ([]Expr, error) {
	dstArgs := make([]Expr, len(args))
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
		me, ok := wa.Expr.(*MetricExpr)
		if ok {
			if !me.isOnlyMetricGroup() {
				return nil, fmt.Errorf("cannot use %q instead of %q in %s", me.AppendString(nil), arg, args)
			}
			dstArg := me.LabelFilters[0].Value
			dstArgs = append(dstArgs, dstArg)
			continue
		}
		pe, ok := wa.Expr.(*parensExpr)
		if ok {
			for _, pArg := range *pe {
				me, ok := pArg.(*MetricExpr)
				if !ok || !me.isOnlyMetricGroup() {
					return nil, fmt.Errorf("cannot use %q instead of %q in %s", pe.AppendString(nil), arg, args)
				}
				dstArg := me.LabelFilters[0].Value
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

func expandWithExprExt(was []*withArgExpr, wa *withArgExpr, args []Expr) (Expr, error) {
	if len(wa.Args) != len(args) {
		if args == nil {
			// Just return MetricExpr with the wa.Name name.
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

func newMetricExpr(name string) *MetricExpr {
	return &MetricExpr{
		LabelFilters: []LabelFilter{{
			Label: "__name__",
			Value: name,
		}},
	}
}

func extractStringValue(token string) (string, error) {
	if !isStringPrefix(token) {
		return "", fmt.Errorf(`StringExpr must contain only string literals; got %q`, token)
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

func removeDuplicateLabelFilters(lfs []LabelFilter) []LabelFilter {
	lfsm := make(map[string]bool, len(lfs))
	lfsNew := lfs[:0]
	var buf []byte
	for i := range lfs {
		lf := &lfs[i]
		buf = lf.AppendString(buf[:0])
		if lfsm[string(buf)] {
			continue
		}
		lfsm[string(buf)] = true
		lfsNew = append(lfsNew, *lf)
	}
	return lfsNew
}

func (p *parser) parseFuncExpr() (*FuncExpr, error) {
	if !isIdentPrefix(p.lex.Token) {
		return nil, fmt.Errorf(`FuncExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}

	var fe FuncExpr
	fe.Name = p.lex.Token
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`FuncExpr; unexpected token %q; want "("`, p.lex.Token)
	}
	args, err := p.parseArgListExpr()
	if err != nil {
		return nil, err
	}
	fe.Args = args
	return &fe, nil
}

func (p *parser) parseModifierExpr(me *ModifierExpr) error {
	if !isIdentPrefix(p.lex.Token) {
		return fmt.Errorf(`ModifierExpr: unexpected token %q; want "ident"`, p.lex.Token)
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

func (p *parser) parseArgListExpr() ([]Expr, error) {
	if p.lex.Token != "(" {
		return nil, fmt.Errorf(`argList: unexpected token %q; want "("`, p.lex.Token)
	}
	var args []Expr
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

func (p *parser) parseLabelFilters() ([]*labelFilterExpr, error) {
	if p.lex.Token != "{" {
		return nil, fmt.Errorf(`labelFilters: unexpected token %q; want "{"`, p.lex.Token)
	}

	var lfes []*labelFilterExpr
	for {
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token == "}" {
			goto closeBracesLabel
		}
		lfe, err := p.parseLabelFilterExpr()
		if err != nil {
			return nil, err
		}
		lfes = append(lfes, lfe)
		switch p.lex.Token {
		case ",":
			continue
		case "}":
			goto closeBracesLabel
		default:
			return nil, fmt.Errorf(`labelFilters: unexpected token %q; want ",", "}"`, p.lex.Token)
		}
	}

closeBracesLabel:
	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	return lfes, nil
}

func (p *parser) parseLabelFilterExpr() (*labelFilterExpr, error) {
	if !isIdentPrefix(p.lex.Token) {
		return nil, fmt.Errorf(`labelFilterExpr: unexpected token %q; want "ident"`, p.lex.Token)
	}
	var lfe labelFilterExpr
	lfe.Label = p.lex.Token
	if err := p.lex.Next(); err != nil {
		return nil, err
	}

	switch p.lex.Token {
	case "=":
		// Nothing to do.
	case "!=":
		lfe.IsNegative = true
	case "=~":
		lfe.IsRegexp = true
	case "!~":
		lfe.IsNegative = true
		lfe.IsRegexp = true
	case ",", "}":
		return &lfe, nil
	default:
		return nil, fmt.Errorf(`labelFilterExpr: unexpected token %q; want "=", "!=", "=~", "!~", ",", "}"`, p.lex.Token)
	}

	if err := p.lex.Next(); err != nil {
		return nil, err
	}
	se, err := p.parseStringExpr()
	if err != nil {
		return nil, err
	}
	lfe.Value = se
	return &lfe, nil
}

// labelFilterExpr represents `foo <op> "bar"` expression, where <op> is `=`, `!=`, `=~` or `!~`.
//
// This type isn't exported.
type labelFilterExpr struct {
	Label      string
	Value      *StringExpr
	IsRegexp   bool
	IsNegative bool
}

func (lfe *labelFilterExpr) String() string {
	return fmt.Sprintf("[label=%q, value=%+v, isRegexp=%v, isNegative=%v]", lfe.Label, lfe.Value, lfe.IsRegexp, lfe.IsNegative)
}

func (lfe *labelFilterExpr) toLabelFilter() (*LabelFilter, error) {
	if lfe.Value == nil || len(lfe.Value.tokens) > 0 {
		panic(fmt.Errorf("BUG: lfe.Value must be already expanded; got %v", lfe.Value))
	}

	var lf LabelFilter
	lf.Label = unescapeIdent(lfe.Label)
	if lf.Label == "__name__" {
		lf.Value = unescapeIdent(lfe.Value.S)
	} else {
		lf.Value = lfe.Value.S
	}
	lf.IsRegexp = lfe.IsRegexp
	lf.IsNegative = lfe.IsNegative
	if !lf.IsRegexp {
		return &lf, nil
	}

	// Verify regexp.
	if _, err := CompileRegexpAnchored(lfe.Value.S); err != nil {
		return nil, fmt.Errorf("invalid regexp in %s=%q: %s", lf.Label, lf.Value, err)
	}
	return &lf, nil
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
		window, err = p.parsePositiveDuration()
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
			step, err = p.parsePositiveDuration()
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
	isNegative := false
	if p.lex.Token == "-" {
		isNegative = true
		if err := p.lex.Next(); err != nil {
			return "", err
		}
	}
	if !isPositiveDuration(p.lex.Token) {
		return "", fmt.Errorf(`duration: unexpected token %q; want "duration"`, p.lex.Token)
	}
	d := p.lex.Token
	if err := p.lex.Next(); err != nil {
		return "", err
	}
	if isNegative {
		d = "-" + d
	}
	return d, nil
}

func (p *parser) parsePositiveDuration() (string, error) {
	d, err := p.parseDuration()
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(d, "-") {
		return "", fmt.Errorf("positiveDuration: expecting positive duration; got %q", d)
	}
	return d, nil
}

// parseIdentExpr parses expressions starting with `ident` token.
func (p *parser) parseIdentExpr() (Expr, error) {
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

func (p *parser) parseMetricExpr() (*MetricExpr, error) {
	var me MetricExpr
	if isIdentPrefix(p.lex.Token) {
		var lfe labelFilterExpr
		lfe.Label = "__name__"
		lfe.Value = &StringExpr{
			tokens: []string{strconv.Quote(p.lex.Token)},
		}
		me.labelFilters = append(me.labelFilters[:0], &lfe)
		if err := p.lex.Next(); err != nil {
			return nil, err
		}
		if p.lex.Token != "{" {
			return &me, nil
		}
	}
	lfes, err := p.parseLabelFilters()
	if err != nil {
		return nil, err
	}
	me.labelFilters = append(me.labelFilters, lfes...)
	return &me, nil
}

func (p *parser) parseRollupExpr(arg Expr) (Expr, error) {
	var re RollupExpr
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

// StringExpr represents string expression.
type StringExpr struct {
	// S contains unquoted value for string expression.
	S string

	// Composite string has non-empty tokens.
	// They must be converted into S by expandWithExpr.
	tokens []string
}

// AppendString appends string representation of se to dst and returns the result.
func (se *StringExpr) AppendString(dst []byte) []byte {
	return strconv.AppendQuote(dst, se.S)
}

// NumberExpr represents number expression.
type NumberExpr struct {
	// N is the parsed number, i.e. `1.23`, `-234`, etc.
	N float64
}

// AppendString appends string representation of ne to dst and returns the result.
func (ne *NumberExpr) AppendString(dst []byte) []byte {
	return strconv.AppendFloat(dst, ne.N, 'g', -1, 64)
}

// parensExpr represents `(...)`.
//
// It isn't exported.
type parensExpr []Expr

// AppendString appends string representation of pe to dst and returns the result.
func (pe parensExpr) AppendString(dst []byte) []byte {
	return appendStringArgListExpr(dst, pe)
}

// BinaryOpExpr represents binary operation.
type BinaryOpExpr struct {
	// Op is the operation itself, i.e. `+`, `-`, `*`, etc.
	Op string

	// Bool indicates whether `bool` modifier is present.
	// For example, `foo >bool bar`.
	Bool bool

	// GroupModifier contains modifier such as "on" or "ignoring".
	GroupModifier ModifierExpr

	// JoinModifier contains modifier such as "group_left" or "group_right".
	JoinModifier ModifierExpr

	// Left contains left arg for the `left op right` expression.
	Left Expr

	// Right contains right arg for the `left op right` epxression.
	Right Expr
}

// AppendString appends string representation of be to dst and returns the result.
func (be *BinaryOpExpr) AppendString(dst []byte) []byte {
	if _, ok := be.Left.(*BinaryOpExpr); ok {
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
	if _, ok := be.Right.(*BinaryOpExpr); ok {
		dst = append(dst, '(')
		dst = be.Right.AppendString(dst)
		dst = append(dst, ')')
	} else {
		dst = be.Right.AppendString(dst)
	}
	return dst
}

// ModifierExpr represents MetricsQL modifier such as `<op> (...)`
type ModifierExpr struct {
	// Op is modifier operation.
	Op string

	// Args contains modifier args from parens.
	Args []string
}

// AppendString appends string representation of me to dst and returns the result.
func (me *ModifierExpr) AppendString(dst []byte) []byte {
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

func appendStringArgListExpr(dst []byte, args []Expr) []byte {
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

// FuncExpr represetns MetricsQL function such as `foo(...)`
type FuncExpr struct {
	// Name is function name.
	Name string

	// Args contains function args.
	Args []Expr
}

// AppendString appends string representation of fe to dst and returns the result.
func (fe *FuncExpr) AppendString(dst []byte) []byte {
	dst = append(dst, fe.Name...)
	dst = appendStringArgListExpr(dst, fe.Args)
	return dst
}

// AggrFuncExpr represents aggregate function such as `sum(...) by (...)`
type AggrFuncExpr struct {
	// Name is the function name.
	Name string

	// Args is the function args.
	Args []Expr

	// Modifier is optional modifier such as `by (...)` or `without (...)`.
	Modifier ModifierExpr

	// Optional limit for the number of output time series.
	// This is MetricsQL extension.
	//
	// Example: `sum(...) by (...) limit 10` would return maximum 10 time series.
	Limit int
}

// AppendString appends string representation of ae to dst and returns the result.
func (ae *AggrFuncExpr) AppendString(dst []byte) []byte {
	dst = append(dst, ae.Name...)
	dst = appendStringArgListExpr(dst, ae.Args)
	if ae.Modifier.Op != "" {
		dst = append(dst, ' ')
		dst = ae.Modifier.AppendString(dst)
	}
	if ae.Limit > 0 {
		dst = append(dst, " limit "...)
		dst = strconv.AppendInt(dst, int64(ae.Limit), 10)
	}
	return dst
}

// withExpr represents `with (...)` extension from MetricsQL.
//
// It isn't exported.
type withExpr struct {
	Was  []*withArgExpr
	Expr Expr
}

// AppendString appends string representation of we to dst and returns the result.
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

// withArgExpr represents a single entry from WITH expression.
//
// It isn't exported.
type withArgExpr struct {
	Name string
	Args []string
	Expr Expr
}

// AppendString appends string representation of wa to dst and returns the result.
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

// RollupExpr represents MetricsQL expression, which contains at least `offset` or `[...]` part.
type RollupExpr struct {
	// The expression for the rollup. Usually it is MetricExpr, but may be arbitrary expr
	// if subquery is used. https://prometheus.io/blog/2019/01/28/subquery-support/
	Expr Expr

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

// ForSubquery returns true if re represents subquery.
func (re *RollupExpr) ForSubquery() bool {
	return len(re.Step) > 0 || re.InheritStep
}

// AppendString appends string representation of re to dst and returns the result.
func (re *RollupExpr) AppendString(dst []byte) []byte {
	needParens := func() bool {
		if _, ok := re.Expr.(*RollupExpr); ok {
			return true
		}
		if _, ok := re.Expr.(*BinaryOpExpr); ok {
			return true
		}
		if ae, ok := re.Expr.(*AggrFuncExpr); ok && ae.Modifier.Op != "" {
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

// LabelFilter represents MetricsQL label filter like `foo="bar"`.
type LabelFilter struct {
	// Label contains label name for the filter.
	Label string

	// Value contains unquoted value for the filter.
	Value string

	// IsNegative reperesents whether the filter is negative, i.e. '!=' or '!~'.
	IsNegative bool

	// IsRegexp represents whether the filter is regesp, i.e. `=~` or `!~`.
	IsRegexp bool
}

// AppendString appends string representation of me to dst and returns the result.
func (lf *LabelFilter) AppendString(dst []byte) []byte {
	dst = appendEscapedIdent(dst, lf.Label)
	var op string
	if lf.IsNegative {
		if lf.IsRegexp {
			op = "!~"
		} else {
			op = "!="
		}
	} else {
		if lf.IsRegexp {
			op = "=~"
		} else {
			op = "="
		}
	}
	dst = append(dst, op...)
	dst = strconv.AppendQuote(dst, lf.Value)
	return dst
}

// MetricExpr represents MetricsQL metric with optional filters, i.e. `foo{...}`.
type MetricExpr struct {
	// LabelFilters contains a list of label filters from curly braces.
	// Metric name if present must be the first.
	LabelFilters []LabelFilter

	// labelFilters must be expanded to LabelFilters by expandWithExpr.
	labelFilters []*labelFilterExpr
}

// AppendString appends string representation of me to dst and returns the result.
func (me *MetricExpr) AppendString(dst []byte) []byte {
	lfs := me.LabelFilters
	if len(lfs) > 0 {
		lf := &lfs[0]
		if lf.Label == "__name__" && !lf.IsNegative && !lf.IsRegexp {
			dst = appendEscapedIdent(dst, lf.Value)
			lfs = lfs[1:]
		}
	}
	if len(lfs) > 0 {
		dst = append(dst, '{')
		for i := range lfs {
			dst = lfs[i].AppendString(dst)
			if i+1 < len(lfs) {
				dst = append(dst, ", "...)
			}
		}
		dst = append(dst, '}')
	} else if len(me.LabelFilters) == 0 {
		dst = append(dst, "{}"...)
	}
	return dst
}

// IsEmpty returns true of me equals to `{}`.
func (me *MetricExpr) IsEmpty() bool {
	return len(me.LabelFilters) == 0
}

func (me *MetricExpr) isOnlyMetricGroup() bool {
	if !me.hasNonEmptyMetricGroup() {
		return false
	}
	return len(me.LabelFilters) == 1
}

func (me *MetricExpr) hasNonEmptyMetricGroup() bool {
	if len(me.LabelFilters) == 0 {
		return false
	}
	lf := &me.LabelFilters[0]
	return lf.Label == "__name__" && !lf.IsNegative && !lf.IsRegexp
}
