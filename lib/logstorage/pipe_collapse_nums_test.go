package logstorage

import (
	"testing"
)

func TestParsePipeCollapseNumsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`collapse_nums`)
	f(`collapse_nums at x`)
	f(`collapse_nums if (x:y)`)
	f(`collapse_nums if (x:y) at a`)
	f(`collapse_nums prettify`)
	f(`collapse_nums at x prettify`)
	f(`collapse_nums if (error) prettify`)
	f(`collapse_nums if (error) at x prettify`)
}

func TestParsePipeCollapseNumsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`collapse_nums foo`)
	f(`collapse_nums at`)
	f(`collapse_nums if`)
	f(`collapse_nums prettify at x`)
	f(`collapse_nums prettify if (error)`)
}

func TestPipeCollapseNums(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// collapse_nums without if and at
	f(`collapse_nums prettify`, [][]Field{
		{
			{"_msg", `2004-10-12T43:23:12Z abc:345`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
		{
			{"_msg", `1234`},
		},
	}, [][]Field{
		{
			{"_msg", `<DATETIME> abc:<N>`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
		{
			{"_msg", `<N>`},
		},
	})

	// collapse_nums with at
	f(`collapse_nums at bar prettify`, [][]Field{
		{
			{"_msg", `2004-10-12T43:23:12Z abc:345`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
			{"bar", "ip: 12.34.56.78"},
		},
		{
			{"_msg", `1234`},
		},
	}, [][]Field{
		{
			{"_msg", `2004-10-12T43:23:12Z abc:345`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
			{"bar", "ip: <IP4>"},
		},
		{
			{"_msg", `1234`},
			{"bar", ""},
		},
	})

	// collapse_nums with if
	f(`collapse_nums if (-abc)`, [][]Field{
		{
			{"_msg", `2004-10-12T43:23:12Z abc:345`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
		{
			{"_msg", `1234`},
		},
	}, [][]Field{
		{
			{"_msg", `2004-10-12T43:23:12Z abc:345`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
		{
			{"_msg", `<N>`},
		},
	})
}

func TestPipeCollapseNumsUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`collapse_nums`, "*", "", "*", "")
	f(`collaps_nums if (f1:q) at x`, "*", "", "*", "")

	// unneeded fields do not intersect with at field
	f(`collapse_nums at x`, "*", "f1,f2", "*", "f1,f2")
	f(`collapse_nums if (f3:q) at x`, "*", "f1,f2", "*", "f1,f2")
	f(`collapse_nums if (f2:q) at x`, "*", "f1,f2", "*", "f1")

	// unneeded fields intersect with at field
	f(`collapse_nums at x`, "*", "x,y", "*", "x,y")
	f(`collapse_nums if (f1:q) at x`, "*", "x,y", "*", "x,y")
	f(`collapse_nums if (x:q) at x`, "*", "x,y", "*", "x,y")
	f(`collapse_nums if (y:q) at x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with at field
	f(`collapse_nums at x`, "f2,y", "", "f2,y", "")
	f(`collapse_nums if (f1:q) at x`, "f2,y", "", "f2,y", "")

	// needed fields intersect with at field
	f(`collapse_nums at y`, "f2,y", "", "f2,y", "")
	f(`collapse_nums if (f1:q) at y`, "f2,y", "", "f1,f2,y", "")
}

func TestAppendCollapseNums(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		result := appendCollapseNums(nil, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for %s\ngot\n%s\nwant\n%s", s, result, resultExpected)
		}
	}

	f("", "")
	f("foo", "foo")
	f("ad", "ad")
	f("abc", "abc")
	f("deadbeef", "<N>")
	f("a b c d e f ad be:eac,dead beef ab", "a b c d e f ad be:eac,<N> <N> ab")
	f("ыва", "ыва")
	f("0", "<N>")
	f("1234567890", "<N>")
	f("1foo", "1foo")
	f("1 foo", "<N> foo")
	f("a1foo2bar34", "a1foo2bar34")
	f("a.1Zfoo.2Tbar:34", "a.<N>Zfoo.<N>Tbar:<N>")
	f("ЫВА123bar45.78", "ЫВА123bar45.<N>")
	f("ЫВА.123.bar.45.78", "ЫВА.<N>.bar.<N>.<N>")
	f("1.23.45.67", "<N>.<N>.<N>.<N>")
	f("2024-12-25T10:20:30Z foo", "<N>-<N>-<N>T<N>:<N>:<N>Z foo")
	f("2024-12-25T10:20:30.123324+05:00 foo", "<N>-<N>-<N>T<N>:<N>:<N>.<N>+<N>:<N> foo")
	f("release v1.2.3", "release v<N>.<N>.<N>")
	f("2004-10-12T43:23:12Z abc:345", "<N>-<N>-<N>T<N>:<N>:<N>Z abc:<N>")
	f("123.43s", "<N>.<N>s")
	f("123ms 2us 3h5m6s43ms43μs324ns", "<N>ms <N>us <N>h<N>m<N>s<N>ms<N>μs<N>ns")
	f("0x1234 0XFEAD12", "0x<N> 0X<N>")
}

func TestAppendCollapseNums_Prettified(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		data := appendCollapseNums(nil, s)
		result := appendPrettifyCollapsedNums(data[:0], data)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for %s\ngot\n%s\nwant\n%s", s, result, resultExpected)
		}
	}

	f("", "")
	f("foo", "foo")
	f("35.191.193.225:51648 - 2edfed59-3e98-4073-bbb2-28d321ca71a7 - - [2024/12/08 15:21:02] 10.71.20.32 GET /foo 200", "<IP4>:<N> - <UUID> - - [<DATETIME>] <IP4> GET /foo <N>")
	f("E1208 15:21:02.748877 62 metric_reporter.go:182", "E1208 <TIME> <N> metric_reporter.go:<N>")
	f("2024-12-08T15:22:32.342Z error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
	f("2024-12-08 15:22:32Z error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
	f("2024-12-08 15:22:32,123 error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
	f("2024-12-08 15:22:32.123+10:30 error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
	f("2024-12-08 15:22:32.123-10:30 error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
	f("2024/12/08T15:22:32-10:30 error exporterhelper/queued_retry.go:101", "<DATETIME> error exporterhelper/queued_retry.go:<N>")
}
