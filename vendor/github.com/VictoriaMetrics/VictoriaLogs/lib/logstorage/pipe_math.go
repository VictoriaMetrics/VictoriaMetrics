package logstorage

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/valyala/fastrand"

	"github.com/VictoriaMetrics/VictoriaLogs/lib/prefixfilter"
)

// pipeMath processes '| math ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe
type pipeMath struct {
	entries []*mathEntry
}

type mathEntry struct {
	// The calculated expr result is stored in resultField.
	resultField string

	// expr is the expression to calculate.
	expr *mathExpr
}

type mathExpr struct {
	// if isConst is set, then the given mathExpr returns the given constValue.
	isConst    bool
	constValue float64

	// constValueStr is the original string representation of constValue.
	//
	// It is used in String() method for returning the original representation of the given constValue.
	constValueStr string

	// if fieldName isn't empty, then the given mathExpr fetches numeric values from the given fieldName.
	fieldName string

	// args are args for the given mathExpr.
	args []*mathExpr

	// op is the operation name (aka function name) for the given mathExpr.
	op string

	// f is the function for calculating results for the given mathExpr.
	f mathFunc

	// whether the mathExpr was wrapped in parens.
	wrappedInParens bool
}

// mathFunc must fill result with calculated results based on the given args.
type mathFunc func(result []float64, args [][]float64)

func (pm *pipeMath) String() string {
	s := "math"
	a := make([]string, len(pm.entries))
	for i, e := range pm.entries {
		a[i] = e.String()
	}
	s += " " + strings.Join(a, ", ")
	return s
}

func (pm *pipeMath) splitToRemoteAndLocal(_ int64) (pipe, []pipe) {
	return pm, nil
}

func (pm *pipeMath) canLiveTail() bool {
	return true
}

func (me *mathEntry) String() string {
	s := me.expr.String()
	if isMathBinaryOp(me.expr.op) {
		s = "(" + s + ")"
	}
	s += " as " + quoteTokenIfNeeded(me.resultField)
	return s
}

func (me *mathExpr) String() string {
	if me.isConst {
		return me.constValueStr
	}
	if me.fieldName != "" {
		return quoteTokenIfNeeded(me.fieldName)
	}

	args := me.args
	if isMathBinaryOp(me.op) {
		opPriority := getMathBinaryOpPriority(me.op)
		left := me.args[0]
		right := me.args[1]
		leftStr := left.String()
		rightStr := right.String()
		if isMathBinaryOp(left.op) && getMathBinaryOpPriority(left.op) > opPriority {
			leftStr = "(" + leftStr + ")"
		}
		if isMathBinaryOp(right.op) && getMathBinaryOpPriority(right.op) >= opPriority {
			rightStr = "(" + rightStr + ")"
		}
		return fmt.Sprintf("%s %s %s", leftStr, me.op, rightStr)
	}

	if me.op == "unary_minus" {
		argStr := args[0].String()
		if isMathBinaryOp(args[0].op) {
			argStr = "(" + argStr + ")"
		}
		return "-" + argStr
	}

	a := make([]string, len(args))
	for i, arg := range args {
		a[i] = arg.String()
	}
	argsStr := strings.Join(a, ", ")
	return fmt.Sprintf("%s(%s)", me.op, argsStr)
}

func isMathBinaryOp(op string) bool {
	_, ok := mathBinaryOps[op]
	return ok
}

func getMathBinaryOpPriority(op string) int {
	bo, ok := mathBinaryOps[op]
	if !ok {
		logger.Panicf("BUG: unexpected binary op: %q", op)
	}
	return bo.priority
}

func getMathFuncForBinaryOp(op string) (mathFunc, error) {
	bo, ok := mathBinaryOps[op]
	if !ok {
		return nil, fmt.Errorf("unsupported binary operation: %q", op)
	}
	return bo.f, nil
}

var mathBinaryOps = map[string]mathBinaryOp{
	"^": {
		priority: 1,
		f:        mathFuncPow,
	},
	"*": {
		priority: 2,
		f:        mathFuncMul,
	},
	"/": {
		priority: 2,
		f:        mathFuncDiv,
	},
	"%": {
		priority: 2,
		f:        mathFuncMod,
	},
	"+": {
		priority: 3,
		f:        mathFuncPlus,
	},
	"-": {
		priority: 3,
		f:        mathFuncMinus,
	},
	"&": {
		priority: 4,
		f:        mathFuncAnd,
	},
	"xor": {
		priority: 5,
		f:        mathFuncXor,
	},
	"or": {
		priority: 6,
		f:        mathFuncOr,
	},
	"default": {
		priority: 10,
		f:        mathFuncDefault,
	},
}

type mathBinaryOp struct {
	priority int
	f        mathFunc
}

func (pm *pipeMath) updateNeededFields(pf *prefixfilter.Filter) {
	for i := len(pm.entries) - 1; i >= 0; i-- {
		e := pm.entries[i]
		if pf.MatchString(e.resultField) {
			pf.AddDenyFilter(e.resultField)
			e.expr.updateNeededFields(pf)
		}
	}
}

func (me *mathExpr) updateNeededFields(pf *prefixfilter.Filter) {
	if me.isConst {
		return
	}
	if me.fieldName != "" {
		pf.AddAllowFilter(me.fieldName)
		return
	}
	for _, arg := range me.args {
		arg.updateNeededFields(pf)
	}
}

func (pm *pipeMath) hasFilterInWithQuery() bool {
	return false
}

func (pm *pipeMath) initFilterInValues(_ *inValuesCache, _ getFieldValuesFunc, _ bool) (pipe, error) {
	return pm, nil
}

func (pm *pipeMath) visitSubqueries(_ func(q *Query)) {
	// nothing to do
}

func (pm *pipeMath) newPipeProcessor(_ int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pmp := &pipeMathProcessor{
		pm:     pm,
		ppNext: ppNext,
	}
	return pmp
}

type pipeMathProcessor struct {
	pm     *pipeMath
	ppNext pipeProcessor

	shards atomicutil.Slice[pipeMathProcessorShard]
}

type pipeMathProcessorShard struct {
	// a holds all the data for rcs.
	a arena

	// rcs is used for storing calculated results before they are written to ppNext.
	rcs []resultColumn

	// rs is storage for temporary results
	rs [][]float64

	// rsBuf is backing storage for rs slices
	rsBuf []float64
}

func (shard *pipeMathProcessorShard) executeMathEntry(e *mathEntry, rc *resultColumn, br *blockResult) (float64, float64) {
	clear(shard.rs)
	shard.rs = shard.rs[:0]
	shard.rsBuf = shard.rsBuf[:0]

	shard.executeExpr(e.expr, br)
	r := shard.rs[0]
	if len(r) == 0 {
		return nan, nan
	}

	b := shard.a.b
	minValue := nan
	maxValue := nan
	for _, f := range r {
		if math.IsNaN(minValue) {
			minValue = f
			maxValue = f
		} else if f < minValue {
			minValue = f
		} else if f > maxValue {
			maxValue = f
		}

		bLen := len(b)
		b = marshalFloat64(b, f)
		v := bytesutil.ToUnsafeString(b[bLen:])
		rc.addValue(v)
	}
	shard.a.b = b

	return minValue, maxValue
}

func (shard *pipeMathProcessorShard) executeExpr(me *mathExpr, br *blockResult) {
	rIdx := len(shard.rs)
	shard.rs = slicesutil.SetLength(shard.rs, len(shard.rs)+1)

	shard.rsBuf = slicesutil.SetLength(shard.rsBuf, len(shard.rsBuf)+br.rowsLen)
	shard.rs[rIdx] = shard.rsBuf[len(shard.rsBuf)-br.rowsLen:]

	if me.isConst {
		r := shard.rs[rIdx]
		for i := 0; i < br.rowsLen; i++ {
			r[i] = me.constValue
		}
		return
	}
	if me.fieldName != "" {
		r := shard.rs[rIdx]
		c := br.getColumnByName(me.fieldName)
		shard.loadArgValuesFromColumn(r, br, c)
		return
	}

	rsBufLen := len(shard.rsBuf)
	for _, arg := range me.args {
		shard.executeExpr(arg, br)
	}

	result := shard.rs[rIdx]
	args := shard.rs[rIdx+1:]
	me.f(result, args)

	shard.rs = shard.rs[:rIdx+1]
	shard.rsBuf = shard.rsBuf[:rsBufLen]
}

func (shard *pipeMathProcessorShard) loadArgValuesFromColumn(dst []float64, br *blockResult, c *blockResultColumn) {
	if c.isConst {
		v := c.valuesEncoded[0]
		f := parseMathNumber(v)
		for i := range dst {
			dst[i] = f
		}
		return
	}
	if c.isTime {
		timestamps := br.getTimestamps()
		for i, ts := range timestamps {
			dst[i] = float64(ts)
		}
		return
	}

	switch c.valueType {
	case valueTypeDict:
		a := encoding.GetFloat64s(len(c.dictValues))
		fs := a.A
		for i, v := range c.dictValues {
			fs[i] = parseMathNumber(v)
		}
		values := c.getValuesEncoded(br)
		for i, v := range values {
			idx := v[0]
			dst[i] = fs[idx]
		}
		encoding.PutFloat64s(a)
	case valueTypeUint8:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalUint8(v))
		}
	case valueTypeUint16:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalUint16(v))
		}
	case valueTypeUint32:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalUint32(v))
		}
	case valueTypeUint64:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalUint64(v))
		}
	case valueTypeInt64:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalInt64(v))
		}
	case valueTypeFloat64:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = unmarshalFloat64(v)
		}
	case valueTypeIPv4:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalIPv4(v))
		}
	case valueTypeTimestampISO8601:
		for i, v := range c.getValuesEncoded(br) {
			dst[i] = float64(unmarshalTimestampISO8601(v))
		}
	default:
		values := c.getValues(br)
		var f float64
		for i, v := range values {
			if i == 0 || v != values[i-1] {
				f = parseMathNumber(v)
			}
			dst[i] = f
		}
	}
}

func (pmp *pipeMathProcessor) writeBlock(workerID uint, br *blockResult) {
	if br.rowsLen == 0 {
		return
	}

	shard := pmp.shards.Get(workerID)
	entries := pmp.pm.entries

	shard.rcs = slicesutil.SetLength(shard.rcs, len(entries))
	rcs := shard.rcs
	for i, e := range entries {
		rc := &rcs[i]
		rc.name = e.resultField
		minValue, maxValue := shard.executeMathEntry(e, rc, br)
		br.addResultColumnFloat64(*rc, minValue, maxValue)
	}

	pmp.ppNext.writeBlock(workerID, br)

	for i := range rcs {
		rcs[i].resetValues()
	}
	shard.a.reset()
}

func (pmp *pipeMathProcessor) flush() error {
	return nil
}

func parsePipeMath(lex *lexer) (pipe, error) {
	if !lex.isKeyword("math", "eval") {
		return nil, fmt.Errorf("unexpected token: %q; want 'math' or 'eval'", lex.token)
	}
	lex.nextToken()

	var mes []*mathEntry
	for {
		me, err := parseMathEntry(lex)
		if err != nil {
			return nil, err
		}
		mes = append(mes, me)

		switch {
		case lex.isKeyword(","):
			lex.nextToken()
		case lex.isKeyword("|", ")", ""):
			if len(mes) == 0 {
				return nil, fmt.Errorf("missing 'math' expressions")
			}
			pm := &pipeMath{
				entries: mes,
			}
			return pm, nil
		default:
			return nil, fmt.Errorf("unexpected token after 'math' expression [%s]: %q; expecting ',', '|' or ')'", mes[len(mes)-1], lex.token)
		}
	}
}

func parseMathEntry(lex *lexer) (*mathEntry, error) {
	me, err := parseMathExpr(lex)
	if err != nil {
		return nil, err
	}

	resultField := ""
	if lex.isKeyword(",", "|", ")", "") {
		resultField = me.String()
	} else {
		if lex.isKeyword("as") {
			// skip optional 'as'
			lex.nextToken()
		}

		fieldName, err := parseFieldName(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse result name for [%s]: %w", me, err)
		}
		resultField = fieldName
	}

	e := &mathEntry{
		resultField: resultField,
		expr:        me,
	}
	return e, nil
}

func parseMathExpr(lex *lexer) (*mathExpr, error) {
	// parse left operand
	left, err := parseMathExprOperand(lex)
	if err != nil {
		return nil, err
	}

	for {
		if !isMathBinaryOp(lex.token) {
			// There is no right operand
			return left, nil
		}

		// parse operator
		op := lex.token
		lex.nextToken()

		f, err := getMathFuncForBinaryOp(op)
		if err != nil {
			return nil, fmt.Errorf("cannot parse operator after [%s]: %w", left, err)
		}

		// parse right operand
		right, err := parseMathExprOperand(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse operand after [%s %s]: %w", left, op, err)
		}

		me := &mathExpr{
			args: []*mathExpr{left, right},
			op:   op,
			f:    f,
		}

		// balance operands according to their priority
		if !left.wrappedInParens && isMathBinaryOp(left.op) && getMathBinaryOpPriority(left.op) > getMathBinaryOpPriority(op) {
			me.args[0] = left.args[1]
			left.args[1] = me
			me = left
		}

		left = me
	}
}

func parseMathExprInParens(lex *lexer) (*mathExpr, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	me, err := parseMathExpr(lex)
	if err != nil {
		return nil, err
	}
	me.wrappedInParens = true

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')'; got %q instead", lex.token)
	}
	lex.nextToken()
	return me, nil
}

func parseMathExprOperand(lex *lexer) (*mathExpr, error) {
	if lex.isKeyword("(") {
		return parseMathExprInParens(lex)
	}

	switch {
	case lex.isKeyword("abs"):
		return parseMathExprAbs(lex)
	case lex.isKeyword("exp"):
		return parseMathExprExp(lex)
	case lex.isKeyword("ln"):
		return parseMathExprLn(lex)
	case lex.isKeyword("max"):
		return parseMathExprMax(lex)
	case lex.isKeyword("min"):
		return parseMathExprMin(lex)
	case lex.isKeyword("now"):
		return parseMathExprNow(lex)
	case lex.isKeyword("rand"):
		return parseMathExprRand(lex)
	case lex.isKeyword("round"):
		return parseMathExprRound(lex)
	case lex.isKeyword("ceil"):
		return parseMathExprCeil(lex)
	case lex.isKeyword("floor"):
		return parseMathExprFloor(lex)
	case lex.isKeyword("-"):
		return parseMathExprUnaryMinus(lex)
	case lex.isKeyword("+"):
		// just skip unary plus
		lex.nextToken()
		return parseMathExprOperand(lex)
	case isNumberPrefix(lex.token):
		return parseMathExprConstNumber(lex)
	default:
		return parseMathExprFieldName(lex)
	}
}

func parseMathExprAbs(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "abs", mathFuncAbs)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 {
		return nil, fmt.Errorf("'abs' function accepts only one arg; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprExp(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "exp", mathFuncExp)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 {
		return nil, fmt.Errorf("'exp' function accepts only one arg; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprLn(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "ln", mathFuncLn)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 {
		return nil, fmt.Errorf("'ln' function accepts only one arg; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprMax(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "max", mathFuncMax)
	if err != nil {
		return nil, err
	}
	if len(me.args) < 2 {
		return nil, fmt.Errorf("'max' function needs at least 2 args; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprMin(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "min", mathFuncMin)
	if err != nil {
		return nil, err
	}
	if len(me.args) < 2 {
		return nil, fmt.Errorf("'min' function needs at least 2 args; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprNow(lex *lexer) (*mathExpr, error) {
	if !lex.isKeyword("now") {
		return nil, fmt.Errorf("missing 'now' keyword")
	}
	lex.nextToken()

	args, err := parseMathFuncArgs(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse args for 'now' function: %w", err)
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("'now' function must have no args; got %d args", len(args))
	}
	me := &mathExpr{
		op: "now",
		f:  mathFuncNow,
	}
	return me, nil
}

func parseMathExprRand(lex *lexer) (*mathExpr, error) {
	if !lex.isKeyword("rand") {
		return nil, fmt.Errorf("missing 'rand' keyword")
	}
	lex.nextToken()

	args, err := parseMathFuncArgs(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse args for 'rand' function: %w", err)
	}
	if len(args) != 0 {
		return nil, fmt.Errorf("'rand' function must have no args; got %d args", len(args))
	}
	me := &mathExpr{
		op: "rand",
		f:  mathFuncRand,
	}
	return me, nil
}

func parseMathExprRound(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "round", mathFuncRound)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 && len(me.args) != 2 {
		return nil, fmt.Errorf("'round' function needs 1 or 2 args; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprCeil(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "ceil", mathFuncCeil)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 {
		return nil, fmt.Errorf("'ceil' function needs one arg; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprFloor(lex *lexer) (*mathExpr, error) {
	me, err := parseMathExprGenericFunc(lex, "floor", mathFuncFloor)
	if err != nil {
		return nil, err
	}
	if len(me.args) != 1 {
		return nil, fmt.Errorf("'floor' function needs one arg; got %d args: [%s]", len(me.args), me)
	}
	return me, nil
}

func parseMathExprGenericFunc(lex *lexer, funcName string, f mathFunc) (*mathExpr, error) {
	if !lex.isKeyword(funcName) {
		return nil, fmt.Errorf("missing %q keyword", funcName)
	}
	lex.nextToken()

	args, err := parseMathFuncArgs(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse args for %q function: %w", funcName, err)
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("%q function needs at least one org", funcName)
	}
	me := &mathExpr{
		args: args,
		op:   funcName,
		f:    f,
	}
	return me, nil
}

func parseMathFuncArgs(lex *lexer) ([]*mathExpr, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	var args []*mathExpr
	for {
		if lex.isKeyword(")") {
			lex.nextToken()
			return args, nil
		}

		me, err := parseMathExpr(lex)
		if err != nil {
			return nil, err
		}
		args = append(args, me)

		switch {
		case lex.isKeyword(")"):
		case lex.isKeyword(","):
			lex.nextToken()
		default:
			return nil, fmt.Errorf("unexpected token after [%s]: %q; want ',' or ')'", me, lex.token)
		}
	}
}

func parseMathExprUnaryMinus(lex *lexer) (*mathExpr, error) {
	if !lex.isKeyword("-") {
		return nil, fmt.Errorf("missing '-'")
	}
	lex.nextToken()

	expr, err := parseMathExprOperand(lex)
	if err != nil {
		return nil, err
	}
	me := &mathExpr{
		args: []*mathExpr{expr},
		op:   "unary_minus",
		f:    mathFuncUnaryMinus,
	}
	return me, nil
}

func parseMathExprConstNumber(lex *lexer) (*mathExpr, error) {
	if !isNumberPrefix(lex.token) {
		return nil, fmt.Errorf("cannot parse number from %q", lex.token)
	}
	numStr, err := getCompoundMathToken(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse number: %w", err)
	}
	f := parseMathNumber(numStr)
	if math.IsNaN(f) {
		return nil, fmt.Errorf("cannot parse number from %q", numStr)
	}
	me := &mathExpr{
		isConst:       true,
		constValue:    f,
		constValueStr: numStr,
	}
	return me, nil
}

func parseMathExprFieldName(lex *lexer) (*mathExpr, error) {
	fieldName, err := getCompoundMathToken(lex)
	if err != nil {
		return nil, err
	}
	fieldName = getCanonicalColumnName(fieldName)
	me := &mathExpr{
		fieldName: fieldName,
	}
	return me, nil
}

func getCompoundMathToken(lex *lexer) (string, error) {
	if err := lex.isInvalidQuotedString(); err != nil {
		return "", err
	}

	stopTokens := []string{"=", "+", "-", "*", "/", "%", "^", ",", ")", "|", "!", ""}
	if lex.isKeyword(stopTokens...) {
		return "", fmt.Errorf("compound token cannot start with '%s'", lex.token)
	}

	s := lex.token
	rawS := lex.rawToken
	lex.nextToken()
	suffix := ""
	for !lex.isSkippedSpace && !lex.isKeyword(stopTokens...) && !lex.isEnd() {
		suffix += lex.rawToken
		lex.nextToken()
	}
	if suffix == "" {
		return s, nil
	}
	return rawS + suffix, nil
}

func mathFuncAnd(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		if math.IsNaN(a[i]) || math.IsNaN(b[i]) {
			result[i] = nan
		} else {
			result[i] = float64(uint64(a[i]) & uint64(b[i]))
		}
	}
}

func mathFuncOr(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		if math.IsNaN(a[i]) || math.IsNaN(b[i]) {
			result[i] = nan
		} else {
			result[i] = float64(uint64(a[i]) | uint64(b[i]))
		}
	}
}

func mathFuncXor(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		if math.IsNaN(a[i]) || math.IsNaN(b[i]) {
			result[i] = nan
		} else {
			result[i] = float64(uint64(a[i]) ^ uint64(b[i]))
		}
	}
}

func mathFuncPlus(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		result[i] = a[i] + b[i]
	}
}

func mathFuncMinus(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		result[i] = a[i] - b[i]
	}
}

func mathFuncMul(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		result[i] = a[i] * b[i]
	}
}

func mathFuncDiv(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		result[i] = a[i] / b[i]
	}
}

func mathFuncMod(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		x := a[i]
		y := b[i]
		xInt := int64(x)
		yInt := int64(y)
		if float64(xInt) == x && float64(yInt) == y {
			// Fast path - integer modulo
			result[i] = float64(xInt % yInt)
		} else {
			// Slow path - floating point modulo
			result[i] = math.Mod(x, y)
		}
	}
}

func mathFuncPow(result []float64, args [][]float64) {
	a := args[0]
	b := args[1]
	for i := range result {
		result[i] = math.Pow(a[i], b[i])
	}
}

func mathFuncDefault(result []float64, args [][]float64) {
	values := args[0]
	defaultValues := args[1]
	for i := range result {
		f := values[i]
		if math.IsNaN(f) {
			f = defaultValues[i]
		}
		result[i] = f
	}
}

func mathFuncAbs(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = math.Abs(arg[i])
	}
}

func mathFuncExp(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = math.Exp(arg[i])
	}
}

func mathFuncLn(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = math.Log(arg[i])
	}
}

func mathFuncUnaryMinus(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = -arg[i]
	}
}

func mathFuncMax(result []float64, args [][]float64) {
	for i := range result {
		f := nan
		for _, arg := range args {
			if math.IsNaN(f) || arg[i] > f {
				f = arg[i]
			}
		}
		result[i] = f
	}
}

func mathFuncMin(result []float64, args [][]float64) {
	for i := range result {
		f := nan
		for _, arg := range args {
			if math.IsNaN(f) || arg[i] < f {
				f = arg[i]
			}
		}
		result[i] = f
	}
}

func mathFuncCeil(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = math.Ceil(arg[i])
	}
}

func mathFuncFloor(result []float64, args [][]float64) {
	arg := args[0]
	for i := range result {
		result[i] = math.Floor(arg[i])
	}
}

func mathFuncRand(result []float64, _ [][]float64) {
	for i := range result {
		n := fastrand.Uint32()
		result[i] = float64(n) / (1 << 32)
	}
}

func mathFuncNow(result []float64, _ [][]float64) {
	nowNanos := float64(time.Now().UnixNano())
	for i := range result {
		result[i] = nowNanos
	}
}

func mathFuncRound(result []float64, args [][]float64) {
	arg := args[0]
	if len(args) == 1 {
		// Round to integer
		for i := range result {
			result[i] = math.Round(arg[i])
		}
		return
	}

	// Round to nearest
	nearest := args[1]
	var f float64
	for i := range result {
		if i == 0 || arg[i-1] != arg[i] || nearest[i-1] != nearest[i] {
			f = round(arg[i], nearest[i])
		}
		result[i] = f
	}
}

func round(f, nearest float64) float64 {
	_, e := decimal.FromFloat(nearest)
	p10 := math.Pow10(int(-e))
	f += 0.5 * math.Copysign(nearest, f)
	f -= math.Mod(f, nearest)
	f, _ = math.Modf(f * p10)
	return f / p10
}

func parseMathNumber(s string) float64 {
	f, ok := tryParseNumber(s)
	if ok {
		return f
	}
	nsecs, ok := TryParseTimestampRFC3339Nano(s)
	if ok {
		return float64(nsecs)
	}
	ipNum, ok := tryParseIPv4(s)
	if ok {
		return float64(ipNum)
	}
	return nan
}
