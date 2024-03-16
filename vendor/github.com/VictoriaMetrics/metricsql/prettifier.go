package metricsql

// Prettify returns prettified representation of MetricsQL query q.
func Prettify(q string) (string, error) {
	e, err := parseInternal(q)
	if err != nil {
		return "", err
	}
	e = removeParensExpr(e)
	b := appendPrettifiedExpr(nil, e, 0, false)
	return string(b), nil
}

// maxPrettifiedLineLen is the maximum length of a single line returned by Prettify().
//
// Actual lines may exceed the maximum length in some cases.
const maxPrettifiedLineLen = 80

func appendPrettifiedExpr(dst []byte, e Expr, indent int, needParens bool) []byte {
	dstLen := len(dst)

	// Try appending e to dst and check whether its length exceeds the maximum allowed line length.
	dst = appendIndent(dst, indent)
	if needParens {
		dst = append(dst, '(')
	}
	dst = e.AppendString(dst)
	if needParens {
		dst = append(dst, ')')
	}
	if len(dst)-dstLen <= maxPrettifiedLineLen {
		// There is no need in splitting the e string representation, since its' length doesn't exceed maxPrettifiedLineLen.
		return dst
	}

	// The e string representation exceeds maxPrettifiedLineLen. Split it into multiple lines.
	dst = dst[:dstLen]
	if needParens {
		dst = appendIndent(dst, indent)
		dst = append(dst, "(\n"...)
		indent++
	}
	switch t := e.(type) {
	case *withExpr:
		// Put every WITH expression on a separate line
		dst = appendIndent(dst, indent)
		dst = append(dst, "WITH (\n"...)
		indent++
		for _, wa := range t.Was {
			dst = appendPrettifiedExpr(dst, wa, indent, false)
			dst = append(dst, ",\n"...)
		}
		indent--
		dst = appendIndent(dst, indent)
		dst = append(dst, ")\n"...)
		dst = appendPrettifiedExpr(dst, t.Expr, indent, false)
	case *withArgExpr:
		// Wrap long withArgExpr into `(...)`
		dst = appendIndent(dst, indent)
		dst = appendEscapedIdent(dst, t.Name)
		if len(t.Args) > 0 {
			dst = append(dst, '(')
			dst = appendEscapedIdent(dst, t.Args[0])
			for _, arg := range t.Args[1:] {
				dst = append(dst, ", "...)
				dst = appendEscapedIdent(dst, arg)
			}
			dst = append(dst, ')')
		}
		dst = append(dst, " = (\n"...)
		dst = appendPrettifiedExpr(dst, t.Expr, indent+1, false)
		dst = append(dst, '\n')
		dst = appendIndent(dst, indent)
		dst = append(dst, ')')
	case *BinaryOpExpr:
		// Split:
		//
		//   a op b
		//
		// into:
		//
		//   foo
		//     op
		//   bar
		if t.KeepMetricNames {
			dst = appendIndent(dst, indent)
			dst = append(dst, "(\n"...)
			indent++
		}
		dst = appendPrettifiedExpr(dst, t.Left, indent, t.needLeftParens())
		dst = append(dst, '\n')
		dst = appendIndent(dst, indent+1)
		dst = t.appendModifiers(dst)
		dst = append(dst, '\n')
		dst = appendPrettifiedExpr(dst, t.Right, indent, t.needRightParens())
		if t.KeepMetricNames {
			indent--
			dst = append(dst, '\n')
			dst = appendIndent(dst, indent)
			dst = append(dst, ") keep_metric_names"...)
		}
	case *RollupExpr:
		// Split:
		//
		//   q[d:s] offset off @ x
		//
		// into:
		//
		//   (
		//     q
		//   )[d:s] offset off @ x
		dst = appendPrettifiedExpr(dst, t.Expr, indent, t.needParens())
		dst = t.appendModifiers(dst)
	case *AggrFuncExpr:
		// Split:
		//
		//   aggr_func(arg1, ..., argN) modifiers
		//
		// into:
		//
		//   aggr_func(
		//     arg1,
		//     ...
		//     argN
		//   ) modifiers
		dst = appendIndent(dst, indent)
		dst = appendEscapedIdent(dst, t.Name)
		dst = appendPrettifiedFuncArgs(dst, indent, t.Args)
		dst = t.appendModifiers(dst)
	case *FuncExpr:
		// Split:
		//
		//   func(arg1, ..., argN) modifiers
		//
		// into:
		//
		//   func(
		//     arg1,
		//     ...
		//     argN
		//   ) modifiers
		dst = appendIndent(dst, indent)
		dst = appendEscapedIdent(dst, t.Name)
		dst = appendPrettifiedFuncArgs(dst, indent, t.Args)
		dst = t.appendModifiers(dst)
	case *MetricExpr:
		// Split:
		//
		//   metric{filters1 or ... or filtersN}
		//
		// into:
		//
		//   metric{
		//     filters1
		//       or
		//     ...
		//       or
		//     filtersN
		//   }
		lfss := t.labelFilterss
		offset := 0
		metricName := getMetricNameFromLabelFilterss(lfss)
		if metricName != "" {
			offset = 1
		}
		dst = appendIndent(dst, indent)
		dst = appendEscapedIdent(dst, metricName)
		if !isOnlyMetricNameInLabelFilterss(lfss) {
			dst = append(dst, "{\n"...)
			for i, lfs := range lfss {
				lfs = lfs[offset:]
				if len(lfs) == 0 {
					continue
				}
				dst = appendPrettifiedLabelFilters(dst, indent+1, lfs)
				dst = append(dst, '\n')
				if i+1 < len(lfss) && len(lfss[i+1]) > offset {
					dst = appendIndent(dst, indent+2)
					dst = append(dst, "or\n"...)
				}
			}
			dst = appendIndent(dst, indent)
			dst = append(dst, '}')
		}
	default:
		// marshal other expressions as is
		dst = t.AppendString(dst)
	}
	if needParens {
		indent--
		dst = append(dst, '\n')
		dst = appendIndent(dst, indent)
		dst = append(dst, ')')
	}
	return dst
}

func appendPrettifiedFuncArgs(dst []byte, indent int, args []Expr) []byte {
	dst = append(dst, "(\n"...)
	for i, arg := range args {
		dst = appendPrettifiedExpr(dst, arg, indent+1, false)
		if i+1 < len(args) {
			dst = append(dst, ',')
		}
		dst = append(dst, '\n')
	}
	dst = appendIndent(dst, indent)
	dst = append(dst, ')')
	return dst
}

func appendPrettifiedLabelFilters(dst []byte, indent int, lfs []*labelFilterExpr) []byte {
	dstLen := len(dst)

	// Try marshaling lfs into a single line
	dst = appendIndent(dst, indent)
	dst = appendLabelFilterExprs(dst, lfs)
	if len(dst)-dstLen <= maxPrettifiedLineLen {
		return dst
	}

	// Too long line - split it into multiple lines
	dst = dst[:dstLen]
	for i := range lfs {
		dst = appendIndent(dst, indent)
		dst = lfs[i].AppendString(dst)
		if i+1 < len(lfs) {
			dst = append(dst, ",\n"...)
		}
	}
	return dst
}

func appendIndent(dst []byte, indent int) []byte {
	for i := 0; i < indent; i++ {
		dst = append(dst, "  "...)
	}
	return dst
}
