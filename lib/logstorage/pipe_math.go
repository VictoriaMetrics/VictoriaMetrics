package logstorage

import (
	"fmt"
	"math"
	"strings"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// pipeMath processes '| math ...' pipe.
//
// See https://docs.victoriametrics.com/victorialogs/logsql/#math-pipe
type pipeMath struct {
	entries []*mathEntry
}

type mathEntry struct {
	resultField  string
	be           *mathBinaryExpr
	neededFields []string
}

func (pm *pipeMath) String() string {
	s := "math"
	a := make([]string, len(pm.entries))
	for i, e := range pm.entries {
		a[i] = e.String()
	}
	s += " " + strings.Join(a, ", ")
	return s
}

func (me *mathEntry) String() string {
	return quoteTokenIfNeeded(me.resultField) + " = " + me.be.String()
}

func (pm *pipeMath) updateNeededFields(neededFields, unneededFields fieldsSet) {
	for i := len(pm.entries) - 1; i >= 0; i-- {
		e := pm.entries[i]
		if neededFields.contains("*") {
			if !unneededFields.contains(e.resultField) {
				unneededFields.add(e.resultField)
				unneededFields.removeFields(e.neededFields)
			}
		} else {
			if neededFields.contains(e.resultField) {
				neededFields.remove(e.resultField)
				neededFields.addFields(e.neededFields)
			}
		}
	}
}

func (pm *pipeMath) optimize() {
	// nothing to do
}

func (pm *pipeMath) hasFilterInWithQuery() bool {
	return false
}

func (pm *pipeMath) initFilterInValues(_ map[string][]string, _ getFieldValuesFunc) (pipe, error) {
	return pm, nil
}

func (pm *pipeMath) newPipeProcessor(workersCount int, _ <-chan struct{}, _ func(), ppNext pipeProcessor) pipeProcessor {
	pmp := &pipeMathProcessor{
		pm:     pm,
		ppNext: ppNext,

		shards: make([]pipeMathProcessorShard, workersCount),
	}
	return pmp
}

type pipeMathProcessor struct {
	pm     *pipeMath
	ppNext pipeProcessor

	shards []pipeMathProcessorShard
}

type pipeMathProcessorShard struct {
	pipeMathProcessorShardNopad

	// The padding prevents false sharing on widespread platforms with 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(pipeMathProcessorShardNopad{})%128]byte
}

type pipeMathProcessorShardNopad struct {
	a   arena
	rcs []resultColumn

	rs [][]float64
}

func (shard *pipeMathProcessorShard) executeMathEntry(e *mathEntry, rc *resultColumn, br *blockResult) {
	shard.rs = shard.rs[:0]
	shard.executeBinaryExpr(e.be, br)
	r := shard.rs[0]

	b := shard.a.b
	for _, f := range r {
		bLen := len(b)
		b = marshalFloat64String(b, f)
		v := bytesutil.ToUnsafeString(b[bLen:])
		rc.addValue(v)
	}
	shard.a.b = b
}

func (shard *pipeMathProcessorShard) executeBinaryExpr(expr *mathBinaryExpr, br *blockResult) {
	rIdx := len(shard.rs)
	shard.rs = slicesutil.SetLength(shard.rs, len(shard.rs)+1)
	shard.rs[rIdx] = slicesutil.SetLength(shard.rs[rIdx], len(br.timestamps))

	if expr.isConst {
		r := shard.rs[rIdx]
		for i := range br.timestamps {
			r[i] = expr.constValue
		}
		return
	}
	if expr.fieldName != "" {
		c := br.getColumnByName(expr.fieldName)
		values := c.getValues(br)
		r := shard.rs[rIdx]
		var f float64
		for i, v := range values {
			if i == 0 || v != values[i-1] {
				var ok bool
				f, ok = tryParseFloat64(v)
				if !ok {
					f = nan
				}
			}
			r[i] = f
		}
		return
	}

	shard.executeBinaryExpr(expr.left, br)
	shard.executeBinaryExpr(expr.right, br)

	r := shard.rs[rIdx]
	rLeft := shard.rs[rIdx+1]
	rRight := shard.rs[rIdx+2]

	mathFunc := expr.mathFunc
	for i := range r {
		r[i] = mathFunc(rLeft[i], rRight[i])
	}

	shard.rs = shard.rs[:rIdx+1]
}

func (pmp *pipeMathProcessor) writeBlock(workerID uint, br *blockResult) {
	if len(br.timestamps) == 0 {
		return
	}

	shard := &pmp.shards[workerID]
	entries := pmp.pm.entries

	shard.rcs = slicesutil.SetLength(shard.rcs, len(entries))
	rcs := shard.rcs
	for i, e := range entries {
		rc := &rcs[i]
		rc.name = e.resultField
		shard.executeMathEntry(e, rc, br)
		br.addResultColumn(rc)
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

func parsePipeMath(lex *lexer, needMathKeyword bool) (*pipeMath, error) {
	if needMathKeyword {
		if !lex.isKeyword("math") {
			return nil, fmt.Errorf("unexpected token: %q; want %q", lex.token, "math")
		}
		lex.nextToken()
	}

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
			return nil, fmt.Errorf("unexpected token after 'math' expression [%s]: %q; expacting ',', '|' or ')'", mes[len(mes)-1], lex.token)
		}
	}
}

func parseMathEntry(lex *lexer) (*mathEntry, error) {
	resultField, err := parseFieldName(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse result name: %w", err)
	}

	if !lex.isKeyword("=") {
		return nil, fmt.Errorf("missing '=' after %q", resultField)
	}
	lex.nextToken()

	be, err := parseMathBinaryExpr(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse expression after '%q=': %w", resultField, err)
	}

	neededFields := newFieldsSet()
	be.updateNeededFields(neededFields)

	me := &mathEntry{
		resultField:  resultField,
		be:           be,
		neededFields: neededFields.getAll(),
	}
	return me, nil
}

type mathBinaryExpr struct {
	isConst       bool
	constValue    float64
	constValueStr string

	fieldName string

	left  *mathBinaryExpr
	right *mathBinaryExpr
	op    string

	mathFunc func(a, b float64) float64
}

func (be *mathBinaryExpr) String() string {
	if be.isConst {
		return be.constValueStr
	}
	if be.fieldName != "" {
		return quoteTokenIfNeeded(be.fieldName)
	}

	leftStr := be.left.String()
	rightStr := be.right.String()

	if binaryOpPriority(be.op) > binaryOpPriority(be.left.op) {
		leftStr = "(" + leftStr + ")"
	}
	if binaryOpPriority(be.op) > binaryOpPriority(be.right.op) {
		rightStr = "(" + rightStr + ")"
	}
	if be.op == "unary_minus" {
		// Unary minus
		return "-" + rightStr
	}

	return leftStr + " " + be.op + " " + rightStr
}

func (be *mathBinaryExpr) updateNeededFields(neededFields fieldsSet) {
	if be.isConst {
		return
	}
	if be.fieldName != "" {
		neededFields.add(be.fieldName)
		return
	}
	be.left.updateNeededFields(neededFields)
	be.right.updateNeededFields(neededFields)
}

func parseMathBinaryExpr(lex *lexer) (*mathBinaryExpr, error) {
	// parse left operand
	leftParens := lex.isKeyword("(")
	left, err := parseMathBinaryExprOperand(lex)
	if err != nil {
		return nil, err
	}

	if lex.isKeyword("|", ")", ",", "") {
		// There is no right operand
		return left, nil
	}

again:
	// parse operator
	op := lex.token
	lex.nextToken()

	mathFunc, err := getMathFuncForOp(op)
	if err != nil {
		return nil, err
	}

	// parse right operand
	right, err := parseMathBinaryExprOperand(lex)
	if err != nil {
		return nil, fmt.Errorf("cannot parse right operand after [%s %s]: %w", left, op, err)
	}

	be := &mathBinaryExpr{
		left:     left,
		right:    right,
		op:       op,
		mathFunc: mathFunc,
	}

	// balance operands according to their priority
	if !leftParens && binaryOpPriority(op) > binaryOpPriority(left.op) {
		be.left = left.right
		left.right = be
		be = left
	}

	if !lex.isKeyword("|", ")", ",", "") {
		left = be
		goto again
	}

	return be, nil
}

func getMathFuncForOp(op string) (func(a, b float64) float64, error) {
	switch op {
	case "+":
		return mathFuncPlus, nil
	case "-":
		return mathFuncMinus, nil
	case "*":
		return mathFuncMul, nil
	case "/":
		return mathFuncDiv, nil
	case "%":
		return mathFuncMod, nil
	case "^":
		return mathFuncPow, nil
	default:
		return nil, fmt.Errorf("unsupported math operator: %q; supported operators: '+', '-', '*', '/'", op)
	}
}

func mathFuncPlus(a, b float64) float64 {
	return a + b
}

func mathFuncMinus(a, b float64) float64 {
	return a - b
}

func mathFuncMul(a, b float64) float64 {
	return a * b
}

func mathFuncDiv(a, b float64) float64 {
	return a / b
}

func mathFuncMod(a, b float64) float64 {
	return math.Mod(a, b)
}

func mathFuncPow(a, b float64) float64 {
	return math.Pow(a, b)
}

func binaryOpPriority(op string) int {
	switch op {
	case "+", "-":
		return 10
	case "*", "/", "%":
		return 20
	case "^":
		return 30
	case "unary_minus":
		return 40
	case "":
		return 100
	default:
		logger.Panicf("BUG: unexpected binary operation %q", op)
		return 0
	}
}

func parseMathBinaryExprInParens(lex *lexer) (*mathBinaryExpr, error) {
	if !lex.isKeyword("(") {
		return nil, fmt.Errorf("missing '('")
	}
	lex.nextToken()

	be, err := parseMathBinaryExpr(lex)
	if err != nil {
		return nil, err
	}

	if !lex.isKeyword(")") {
		return nil, fmt.Errorf("missing ')'; got %q instead", lex.token)
	}
	lex.nextToken()
	return be, nil
}

func parseMathBinaryExprOperand(lex *lexer) (*mathBinaryExpr, error) {
	if lex.isKeyword("(") {
		return parseMathBinaryExprInParens(lex)
	}

	if lex.isKeyword("-") {
		lex.nextToken()
		be, err := parseMathBinaryExprOperand(lex)
		if err != nil {
			return nil, err
		}
		be = &mathBinaryExpr{
			left: &mathBinaryExpr{
				isConst: true,
			},
			right:    be,
			op:       "unary_minus",
			mathFunc: mathFuncMinus,
		}
		return be, nil
	}

	if lex.isNumber() {
		numStr, err := getCompoundMathToken(lex)
		if err != nil {
			return nil, fmt.Errorf("cannot parse number: %w", err)
		}
		f, ok := tryParseNumber(numStr)
		if !ok {
			return nil, fmt.Errorf("cannot parse number from %q", numStr)
		}
		be := &mathBinaryExpr{
			isConst:       true,
			constValue:    f,
			constValueStr: numStr,
		}
		return be, nil
	}

	fieldName, err := getCompoundMathToken(lex)
	if err != nil {
		return nil, err
	}
	fieldName = getCanonicalColumnName(fieldName)
	be := &mathBinaryExpr{
		fieldName: fieldName,
	}
	return be, nil
}

func getCompoundMathToken(lex *lexer) (string, error) {
	stopTokens := []string{"+", "*", "/", "%", "^", ",", ")", "|", ""}
	if lex.isKeyword(stopTokens...) {
		return "", fmt.Errorf("compound token cannot start with '%s'", lex.token)
	}

	s := lex.token
	rawS := lex.rawToken
	lex.nextToken()
	suffix := ""
	stopTokens = append(stopTokens, "-")
	for !lex.isSkippedSpace && !lex.isKeyword(stopTokens...) {
		s += lex.token
		lex.nextToken()
	}
	if suffix == "" {
		return s, nil
	}
	return rawS + suffix, nil
}
