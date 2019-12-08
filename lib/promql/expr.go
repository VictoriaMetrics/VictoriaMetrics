package promql

import (
	"fmt"
	"strconv"
)

// An Expr represents a parsed Extended PromQL expression
type Expr interface {
	// AppendString appends string representation of expr to dst.
	AppendString(dst []byte) []byte
}

// A StringExpr represents a string
type StringExpr struct {
	S string
}

// AppendString appends string representation of expr to dst.
func (se *StringExpr) AppendString(dst []byte) []byte {
	return strconv.AppendQuote(dst, se.S)
}

// A StringTemplateExpr represents a string prior to applying a With clause
type StringTemplateExpr struct {
	// Composite string has non-empty tokens.
	Tokens []StringToken
}

// AppendString appends string representation of expr to dst.
func (ste *StringTemplateExpr) AppendString(dst []byte) []byte {
	if ste == nil {
		return dst
	}
	for i, tok := range ste.Tokens {
		if i > 0 {
			dst = append(dst, " + "...)
		}
		dst = tok.AppendString(dst)
	}
	return dst
}

// A StringToken represents a portion of a string expression
type StringToken struct {
	Ident bool
	S     string
}

// AppendString appends string representation of st to dst.
func (st *StringToken) AppendString(dst []byte) []byte {
	if st.Ident {
		return appendEscapedIdent(dst, []byte(st.S))
	}
	return strconv.AppendQuote(dst, st.S)
}

// A NumberExpr represents a number
type NumberExpr struct {
	N float64
}

// AppendString appends string representation of expr to dst.
func (ne *NumberExpr) AppendString(dst []byte) []byte {
	return strconv.AppendFloat(dst, ne.N, 'g', -1, 64)
}

// A ParensExpr represents a parens expression
type ParensExpr []Expr

// AppendString appends string representation of expr to dst.
func (pe ParensExpr) AppendString(dst []byte) []byte {
	return appendStringArgListExpr(dst, pe)
}

// A BinaryOpExpr represents a binary operator
type BinaryOpExpr struct {
	Op string

	Bool          bool
	GroupModifier ModifierExpr
	JoinModifier  ModifierExpr

	Left  Expr
	Right Expr
}

// AppendString appends string representation of expr to dst.
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

// A ModifierExpr represents a modifier attached to a parent expression
type ModifierExpr struct {
	Op string

	Args []string
}

// AppendString appends string representation of expr to dst.
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

// A FuncExpr represents a function invocation
type FuncExpr struct {
	Name string

	Args []Expr
}

// AppendString appends string representation of expr to dst.
func (fe *FuncExpr) AppendString(dst []byte) []byte {
	dst = append(dst, fe.Name...)
	dst = appendStringArgListExpr(dst, fe.Args)
	return dst
}

// An AggrFuncExpr represents the invocation of an aggregate function
type AggrFuncExpr struct {
	Name string

	Args []Expr

	Modifier ModifierExpr
}

// AppendString appends string representation of expr to dst.
func (ae *AggrFuncExpr) AppendString(dst []byte) []byte {
	dst = append(dst, ae.Name...)
	dst = appendStringArgListExpr(dst, ae.Args)
	if ae.Modifier.Op != "" {
		dst = append(dst, ' ')
		dst = ae.Modifier.AppendString(dst)
	}
	return dst
}

// A WithExpr represents a With expression
type WithExpr struct {
	Was  []*WithArgExpr
	Expr Expr
}

// AppendString appends string representation of expr to dst.
func (we *WithExpr) AppendString(dst []byte) []byte {
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

// A WithArgExpr represents an arg in a With expression
type WithArgExpr struct {
	Name string
	Args []string
	Expr Expr
}

// AppendString appends string representation of expr to dst.
func (wa *WithArgExpr) AppendString(dst []byte) []byte {
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

// A RollupExpr represents a rollup expression
type RollupExpr struct {
	// The expression for the rollup. Usually it is metricExpr, but may be arbitrary expr
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

// ForSubquery returns whether is rollup is for a subquery
func (re *RollupExpr) ForSubquery() bool {
	return len(re.Step) > 0 || re.InheritStep
}

// AppendString appends string representation of expr to dst.
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

// A MetricExpr represents a metric expression
type MetricExpr struct {
	// TagFilters contains a list of tag filters from curly braces.
	// The first item may be the metric name.
	TagFilters []TagFilter
}

// AppendString appends string representation of expr to dst.
func (me *MetricExpr) AppendString(dst []byte) []byte {
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
			tf := &tfs[i]
			dst = appendStringTagFilter(dst, tf)
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

// IsEmpty returns whether this is an empty metric expression
func (me *MetricExpr) IsEmpty() bool {
	return len(me.TagFilters) == 0
}

// IsOnlyMetricGroup returns whether this is a metric group only
func (me *MetricExpr) IsOnlyMetricGroup() bool {
	if !me.HasNonEmptyMetricGroup() {
		return false
	}
	return len(me.TagFilters) == 1
}

// HasNonEmptyMetricGroup returns whether this has a non-empty metric group
func (me *MetricExpr) HasNonEmptyMetricGroup() bool {
	if len(me.TagFilters) == 0 {
		return false
	}
	tf := &me.TagFilters[0]
	return len(tf.Key) == 0 && !tf.IsNegative && !tf.IsRegexp
}

// A TagFilter is a single key <op> value filter tag in a metric filter
//
// Note that this should exactly match the definition in the stroage package
type TagFilter struct {
	Key        []byte
	Value      []byte
	IsNegative bool
	IsRegexp   bool
}

// A MetricTemplateExpr represents a metric expression prior to expansion via
// a with clause
type MetricTemplateExpr struct {
	TagFilters []*TagFilterExpr
}

// AppendString appends string representation of expr to dst.
func (mte *MetricTemplateExpr) AppendString(dst []byte) []byte {
	tfs := mte.TagFilters
	if len(tfs) > 0 {
		tf := tfs[0]
		if len(tf.Key) == 0 && !tf.IsNegative && !tf.IsRegexp && len(tf.Value.Tokens) == 1 && !tf.Value.Tokens[0].Ident {
			dst = appendEscapedIdent(dst, []byte(tf.Value.Tokens[0].S))
			tfs = tfs[1:]
		}
	}
	if len(tfs) > 0 {
		dst = append(dst, '{')
		for i := range tfs {
			tf := tfs[i]
			dst = tf.AppendString(dst)
			if i+1 < len(tfs) {
				dst = append(dst, ", "...)
			}
		}
		dst = append(dst, '}')
	} else if len(mte.TagFilters) == 0 {
		dst = append(dst, "{}"...)
	}
	return dst
}

// A TagFilterExpr represents a tag filter
type TagFilterExpr struct {
	Key        string
	Value      *StringTemplateExpr
	IsRegexp   bool
	IsNegative bool
}

func (tfe *TagFilterExpr) String() string {
	return fmt.Sprintf("[key=%q, value=%+v, isRegexp=%v, isNegative=%v]", tfe.Key, tfe.Value, tfe.IsRegexp, tfe.IsNegative)
}

// AppendString appends string representation of expr to dst.
func (tfe *TagFilterExpr) AppendString(dst []byte) []byte {
	if len(tfe.Key) == 0 {
		dst = append(dst, "__name__"...)
	} else {
		dst = append(dst, tfe.Key...)
	}
	dst = appendStringTagFilterOp(dst, tfe.IsRegexp, tfe.IsNegative)
	return tfe.Value.AppendString(dst)
}
