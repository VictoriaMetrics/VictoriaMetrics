package logstorage

import (
	"math"
	"reflect"
	"testing"
	"time"
)

func TestLexer(t *testing.T) {
	f := func(s string, tokensExpected []string) {
		t.Helper()
		lex := newLexer(s)
		for _, tokenExpected := range tokensExpected {
			lex.nextToken()
			if lex.token != tokenExpected {
				t.Fatalf("unexpected token; got %q; want %q", lex.token, tokenExpected)
			}
		}
		lex.nextToken()
		if lex.token != "" {
			t.Fatalf("unexpected tail token: %q", lex.token)
		}
	}

	f("", nil)
	f("  ", nil)
	f("foo", []string{"foo"})
	f("тест123", []string{"тест123"})
	f("foo:bar", []string{"foo", ":", "bar"})
	f(` re   (  "тест(\":"  )  `, []string{"re", "(", `тест(":`, ")"})
	f(" `foo, bar`* AND baz:(abc or 'd\\'\"ЙЦУК `'*)", []string{"foo, bar", "*", "AND", "baz", ":", "(", "abc", "or", `d'"ЙЦУК ` + "`", "*", ")"})
	f(`_stream:{foo="bar",a=~"baz", b != 'cd',"d,}a"!~abc}`,
		[]string{"_stream", ":", "{", "foo", "=", "bar", ",", "a", "=~", "baz", ",", "b", "!=", "cd", ",", "d,}a", "!~", "abc", "}"})
}

func TestNewStreamFilterSuccess(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		sf, err := newStreamFilter(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := sf.String()
		if result != resultExpected {
			t.Fatalf("unexpected StreamFilter; got %s; want %s", result, resultExpected)
		}
	}

	f("{}", "{}")
	f(`{foo="bar"}`, `{foo="bar"}`)
	f(`{ "foo" =~ "bar.+" , baz!="a" or x="y"}`, `{foo=~"bar.+",baz!="a" or x="y"}`)
	f(`{"a b"='c}"d' OR de="aaa"}`, `{"a b"="c}\"d" or de="aaa"}`)
	f(`{a="b", c="d" or x="y"}`, `{a="b",c="d" or x="y"}`)
}

func TestNewStreamFilterFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		sf, err := newStreamFilter(s)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if sf != nil {
			t.Fatalf("expecting nil sf; got %v", sf)
		}
	}

	f("")
	f("}")
	f("{")
	f("{foo")
	f("{foo}")
	f("{'foo")
	f("{foo=")
	f("{foo or bar}")
	f("{foo=bar")
	f("{foo=bar baz}")
	f("{foo='bar' baz='x'}")
}

func TestParseTimeDuration(t *testing.T) {
	f := func(s string, durationExpected time.Duration) {
		t.Helper()
		q, err := ParseQuery("_time:" + s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		tf, ok := q.f.(*timeFilter)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *timeFilter; filter: %s", q.f, q.f)
		}
		if tf.stringRepr != s {
			t.Fatalf("unexpected string represenation for timeFilter; got %q; want %q", tf.stringRepr, s)
		}
		duration := time.Duration(tf.maxTimestamp - tf.minTimestamp)
		if duration != durationExpected {
			t.Fatalf("unexpected duration; got %s; want %s", duration, durationExpected)
		}
	}
	f("5m", 5*time.Minute)
	f("5m offset 1h", 5*time.Minute)
	f("5m offset -3.5h5m45s", 5*time.Minute)
	f("-5.5m", 5*time.Minute+30*time.Second)
	f("-5.5m offset 1d5m", 5*time.Minute+30*time.Second)
	f("3d2h12m34s45ms", 3*24*time.Hour+2*time.Hour+12*time.Minute+34*time.Second+45*time.Millisecond)
	f("3d2h12m34s45ms offset 10ms", 3*24*time.Hour+2*time.Hour+12*time.Minute+34*time.Second+45*time.Millisecond)
}

func TestParseTimeRange(t *testing.T) {
	f := func(s string, minTimestampExpected, maxTimestampExpected int64) {
		t.Helper()
		q, err := ParseQuery("_time:" + s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		tf, ok := q.f.(*timeFilter)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *timeFilter; filter: %s", q.f, q.f)
		}
		if tf.stringRepr != s {
			t.Fatalf("unexpected string represenation for timeFilter; got %q; want %q", tf.stringRepr, s)
		}
		if tf.minTimestamp != minTimestampExpected {
			t.Fatalf("unexpected minTimestamp; got %s; want %s", timestampToString(tf.minTimestamp), timestampToString(minTimestampExpected))
		}
		if tf.maxTimestamp != maxTimestampExpected {
			t.Fatalf("unexpected maxTimestamp; got %s; want %s", timestampToString(tf.maxTimestamp), timestampToString(maxTimestampExpected))
		}
	}

	var minTimestamp, maxTimestamp int64

	// _time:YYYY -> _time:[YYYY, YYYY+1)
	minTimestamp = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023", minTimestamp, maxTimestamp)
	f("2023Z", minTimestamp, maxTimestamp)

	// _time:YYYY-hh:mm -> _time:[YYYY-hh:mm, (YYYY+1)-hh:mm)
	minTimestamp = time.Date(2023, time.January, 1, 2, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.January, 1, 2, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02:00", minTimestamp, maxTimestamp)

	// _time:YYYY+hh:mm -> _time:[YYYY+hh:mm, (YYYY+1)+hh:mm)
	minTimestamp = time.Date(2022, time.December, 31, 22, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.December, 31, 22, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023+02:00", minTimestamp, maxTimestamp)

	// _time:YYYY-MM -> _time:[YYYY-MM, YYYY-MM+1)
	minTimestamp = time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02", minTimestamp, maxTimestamp)
	f("2023-02Z", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-hh:mm -> _time:[YYYY-MM-hh:mm, (YYYY-MM+1)-hh:mm)
	minTimestamp = time.Date(2023, time.February, 1, 2, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 2, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-02:00", minTimestamp, maxTimestamp)
	// March
	minTimestamp = time.Date(2023, time.March, 1, 2, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.April, 1, 2, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03-02:00", minTimestamp, maxTimestamp)

	// _time:YYYY-MM+hh:mm -> _time:[YYYY-MM+hh:mm, (YYYY-MM+1)+hh:mm)
	minTimestamp = time.Date(2023, time.February, 28, 21, 35, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 31, 21, 35, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03+02:25", minTimestamp, maxTimestamp)
	// February with timezone offset
	minTimestamp = time.Date(2023, time.January, 31, 21, 35, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 28, 21, 35, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02+02:25", minTimestamp, maxTimestamp)
	// February with timezone offset at leap year
	minTimestamp = time.Date(2024, time.January, 31, 21, 35, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.February, 29, 21, 35, 0, 0, time.UTC).UnixNano() - 1
	f("2024-02+02:25", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DD
	minTimestamp = time.Date(2023, time.February, 12, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 13, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-12", minTimestamp, maxTimestamp)
	f("2023-02-12Z", minTimestamp, maxTimestamp)
	// February 28
	minTimestamp = time.Date(2023, time.February, 28, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28", minTimestamp, maxTimestamp)
	// January 31
	minTimestamp = time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-01-31", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DD-hh:mm
	minTimestamp = time.Date(2023, time.January, 31, 2, 25, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 1, 2, 25, 0, 0, time.UTC).UnixNano() - 1
	f("2023-01-31-02:25", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DD+hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 21, 35, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 21, 35, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03-01+02:25", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH
	minTimestamp = time.Date(2023, time.February, 28, 23, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28T23", minTimestamp, maxTimestamp)
	f("2023-02-28T23Z", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH-hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 01, 25, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 28, 02, 25, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-27T23-02:25", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH+hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 35, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 00, 35, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03-01T02+02:25", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28T23:59", minTimestamp, maxTimestamp)
	f("2023-02-28T23:59Z", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM-hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28T22:59-01:00", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM+hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03-01T00:59+01:00", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM:SS-hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 59, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28T23:59:59", minTimestamp, maxTimestamp)
	f("2023-02-28T23:59:59Z", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM:SS-hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 59, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28T22:59:59-01:00", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM:SS+hh:mm
	minTimestamp = time.Date(2023, time.February, 28, 23, 59, 59, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-03-01T00:59:59+01:00", minTimestamp, maxTimestamp)

	// _time:(start, end)
	minTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() + 1
	maxTimestamp = time.Date(2023, time.April, 6, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`(2023-03-01,2023-04-06)`, minTimestamp, maxTimestamp)

	// _time:[start, end)
	minTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.April, 6, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`[2023-03-01,2023-04-06)`, minTimestamp, maxTimestamp)

	// _time:(start, end]
	minTimestamp = time.Date(2023, time.March, 1, 21, 20, 0, 0, time.UTC).UnixNano() + 1
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`(2023-03-01T21:20,2023-04-06]`, minTimestamp, maxTimestamp)

	// _time:[start, end] with timezone
	minTimestamp = time.Date(2023, time.February, 28, 21, 40, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`[2023-03-01+02:20,2023-04-06T23]`, minTimestamp, maxTimestamp)

	// _time:[start, end] with timezone and offset
	offset := int64(30*time.Minute + 5*time.Second)
	minTimestamp = time.Date(2023, time.February, 28, 21, 40, 0, 0, time.UTC).UnixNano() - offset
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1 - offset
	f(`[2023-03-01+02:20,2023-04-06T23] offset 30m5s`, minTimestamp, maxTimestamp)
}

func TestParseSequenceFilter(t *testing.T) {
	f := func(s, fieldNameExpected string, phrasesExpected []string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		sf, ok := q.f.(*sequenceFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *sequenceFilter; filter: %s", q.f, q.f)
		}
		if sf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", sf.fieldName, fieldNameExpected)
		}
		if !reflect.DeepEqual(sf.phrases, phrasesExpected) {
			t.Fatalf("unexpected phrases\ngot\n%q\nwant\n%q", sf.phrases, phrasesExpected)
		}
	}

	f(`seq()`, ``, nil)
	f(`foo:seq(foo)`, `foo`, []string{"foo"})
	f(`_msg:seq("foo bar,baz")`, `_msg`, []string{"foo bar,baz"})
	f(`seq(foo,bar-baz.aa"bb","c,)d")`, ``, []string{"foo", `bar-baz.aa"bb"`, "c,)d"})
}

func TestParseInFilter(t *testing.T) {
	f := func(s, fieldNameExpected string, valuesExpected []string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		f, ok := q.f.(*inFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *inFilter; filter: %s", q.f, q.f)
		}
		if f.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", f.fieldName, fieldNameExpected)
		}
		if !reflect.DeepEqual(f.values, valuesExpected) {
			t.Fatalf("unexpected values\ngot\n%q\nwant\n%q", f.values, valuesExpected)
		}
	}

	f(`in()`, ``, nil)
	f(`foo:in(foo)`, `foo`, []string{"foo"})
	f(`:in("foo bar,baz")`, ``, []string{"foo bar,baz"})
	f(`ip:in(1.2.3.4, 5.6.7.8, 9.10.11.12)`, `ip`, []string{"1.2.3.4", "5.6.7.8", "9.10.11.12"})
	f(`foo-bar:in(foo,bar-baz.aa"bb","c,)d")`, `foo-bar`, []string{"foo", `bar-baz.aa"bb"`, "c,)d"})
}

func TestParseIPv4RangeFilter(t *testing.T) {
	f := func(s, fieldNameExpected string, minValueExpected, maxValueExpected uint32) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		rf, ok := q.f.(*ipv4RangeFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *ipv4RangeFilter; filter: %s", q.f, q.f)
		}
		if rf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", rf.fieldName, fieldNameExpected)
		}
		if rf.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %08x; want %08x", rf.minValue, minValueExpected)
		}
		if rf.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %08x; want %08x", rf.maxValue, maxValueExpected)
		}
	}

	f(`ipv4_range(1.2.3.4, 5.6.7.8)`, ``, 0x01020304, 0x05060708)
	f(`_msg:ipv4_range("0.0.0.0", 255.255.255.255)`, `_msg`, 0, 0xffffffff)
	f(`ip:ipv4_range(1.2.3.0/24)`, `ip`, 0x01020300, 0x010203ff)
	f(`:ipv4_range("1.2.3.34/24")`, ``, 0x01020300, 0x010203ff)
	f(`ipv4_range("1.2.3.34/20")`, ``, 0x01020000, 0x01020fff)
	f(`ipv4_range("1.2.3.15/32")`, ``, 0x0102030f, 0x0102030f)
	f(`ipv4_range(1.2.3.34/0)`, ``, 0, 0xffffffff)
}

func TestParseStringRangeFilter(t *testing.T) {
	f := func(s, fieldNameExpected, minValueExpected, maxValueExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		rf, ok := q.f.(*stringRangeFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *stringRangeFilter; filter: %s", q.f, q.f)
		}
		if rf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", rf.fieldName, fieldNameExpected)
		}
		if rf.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %q; want %q", rf.minValue, minValueExpected)
		}
		if rf.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %q; want %q", rf.maxValue, maxValueExpected)
		}
	}

	f("string_range(foo, bar)", ``, "foo", "bar")
	f(`abc:string_range("foo,bar", "baz) !")`, `abc`, `foo,bar`, `baz) !`)
}

func TestParseRegexpFilter(t *testing.T) {
	f := func(s, reExpected string) {
		t.Helper()
		q, err := ParseQuery("re(" + s + ")")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		rf, ok := q.f.(*regexpFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *regexpFilter; filter: %s", q.f, q.f)
		}
		if reString := rf.re.String(); reString != reExpected {
			t.Fatalf("unexpected regexp; got %q; want %q", reString, reExpected)
		}
	}

	f(`""`, ``)
	f(`foo`, `foo`)
	f(`"foo.+|bar.*"`, `foo.+|bar.*`)
	f(`"foo(bar|baz),x[y]"`, `foo(bar|baz),x[y]`)
}

func TestParseAnyCasePhraseFilter(t *testing.T) {
	f := func(s, fieldNameExpected, phraseExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		pf, ok := q.f.(*anyCasePhraseFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *anyCasePhraseFilter; filter: %s", q.f, q.f)
		}
		if pf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", pf.fieldName, fieldNameExpected)
		}
		if pf.phrase != phraseExpected {
			t.Fatalf("unexpected phrase; got %q; want %q", pf.phrase, phraseExpected)
		}
	}

	f(`i("")`, ``, ``)
	f(`i(foo)`, ``, `foo`)
	f(`abc-de.fg:i(foo-bar+baz)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar+baz")`, `abc-de.fg`, `foo-bar+baz`)
}

func TestParseAnyCasePrefixFilter(t *testing.T) {
	f := func(s, fieldNameExpected, prefixExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		pf, ok := q.f.(*anyCasePrefixFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *anyCasePrefixFilter; filter: %s", q.f, q.f)
		}
		if pf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", pf.fieldName, fieldNameExpected)
		}
		if pf.prefix != prefixExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", pf.prefix, prefixExpected)
		}
	}

	f(`i(*)`, ``, ``)
	f(`i(""*)`, ``, ``)
	f(`i(foo*)`, ``, `foo`)
	f(`abc-de.fg:i(foo-bar+baz*)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar+baz"*)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar*baz *"*)`, `abc-de.fg`, `foo-bar*baz *`)
}

func TestParsePhraseFilter(t *testing.T) {
	f := func(s, fieldNameExpected, phraseExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		pf, ok := q.f.(*phraseFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *phraseFilter; filter: %s", q.f, q.f)
		}
		if pf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", pf.fieldName, fieldNameExpected)
		}
		if pf.phrase != phraseExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", pf.phrase, phraseExpected)
		}
	}

	f(`""`, ``, ``)
	f(`foo`, ``, `foo`)
	f(`abc-de.fg:foo-bar+baz`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar+baz"`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar*baz *"`, `abc-de.fg`, `foo-bar*baz *`)
	f(`"foo:bar*,( baz"`, ``, `foo:bar*,( baz`)
}

func TestParsePrefixFilter(t *testing.T) {
	f := func(s, fieldNameExpected, prefixExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		pf, ok := q.f.(*prefixFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *prefixFilter; filter: %s", q.f, q.f)
		}
		if pf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", pf.fieldName, fieldNameExpected)
		}
		if pf.prefix != prefixExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", pf.prefix, prefixExpected)
		}
	}

	f(`*`, ``, ``)
	f(`""*`, ``, ``)
	f(`foo*`, ``, `foo`)
	f(`abc-de.fg:foo-bar+baz*`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar+baz"*`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar*baz *"*`, `abc-de.fg`, `foo-bar*baz *`)
}

func TestParseRangeFilter(t *testing.T) {
	f := func(s, fieldNameExpected string, minValueExpected, maxValueExpected float64) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		rf, ok := q.f.(*rangeFilter)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *ipv4RangeFilter; filter: %s", q.f, q.f)
		}
		if rf.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", rf.fieldName, fieldNameExpected)
		}
		if rf.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %v; want %v", rf.minValue, minValueExpected)
		}
		if rf.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %v; want %v", rf.maxValue, maxValueExpected)
		}
	}

	f(`range[-1.234, +2e5]`, ``, -1.234, 2e5)
	f(`foo:range[-1.234e-5, 2e5]`, `foo`, -1.234e-5, 2e5)
	f(`range:range["-1.234e5", "-2e-5"]`, `range`, -1.234e5, -2e-5)

	f(`_msg:range[1, 2]`, `_msg`, 1, 2)
	f(`:range(1, 2)`, ``, math.Nextafter(1, math.Inf(1)), math.Nextafter(2, math.Inf(-1)))
	f(`range[1, 2)`, ``, 1, math.Nextafter(2, math.Inf(-1)))
	f(`range("1", 2]`, ``, math.Nextafter(1, math.Inf(1)), 2)
}

func TestParseQuerySuccess(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		result := q.String()
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f("foo", "foo")
	f(":foo", "foo")
	f(`"":foo`, "foo")
	f(`"" bar`, `"" bar`)
	f(`!''`, `!""`)
	f(`foo:""`, `foo:""`)
	f(`!foo:""`, `!foo:""`)
	f(`not foo:""`, `!foo:""`)
	f(`not(foo)`, `!foo`)
	f(`not (foo)`, `!foo`)
	f(`not ( foo or bar )`, `!(foo or bar)`)
	f(`foo:!""`, `!foo:""`)
	f("_msg:foo", "foo")
	f("'foo:bar'", `"foo:bar"`)
	f("'!foo'", `"!foo"`)
	f("foo 'and' and bar", `foo "and" bar`)
	f("foo bar", "foo bar")
	f("foo and bar", "foo bar")
	f("foo AND bar", "foo bar")
	f("foo or bar", "foo or bar")
	f("foo OR bar", "foo or bar")
	f("not foo", "!foo")
	f("! foo", "!foo")
	f("not !`foo bar`", `"foo bar"`)
	f("foo or bar and not baz", "foo or bar !baz")
	f("'foo bar' !baz", `"foo bar" !baz`)
	f("foo:!bar", `!foo:bar`)
	f(`foo and bar and baz or x or y or z and zz`, `foo bar baz or x or y or z zz`)
	f(`foo and bar and (baz or x or y or z) and zz`, `foo bar (baz or x or y or z) zz`)
	f(`(foo or bar or baz) and x and y and (z or zz)`, `(foo or bar or baz) x y (z or zz)`)
	f(`(foo or bar or baz) and x and y and not (z or zz)`, `(foo or bar or baz) x y !(z or zz)`)
	f(`NOT foo AND bar OR baz`, `!foo bar or baz`)
	f(`NOT (foo AND bar) OR baz`, `!(foo bar) or baz`)
	f(`foo OR bar AND baz`, `foo or bar baz`)
	f(`(foo OR bar) AND baz`, `(foo or bar) baz`)

	// parens
	f(`foo:(bar baz or not :xxx)`, `foo:bar foo:baz or !foo:xxx`)
	f(`(foo:bar and (foo:baz or aa:bb) and xx) and y`, `foo:bar (foo:baz or aa:bb) xx y`)
	f("level:error and _msg:(a or b)", "level:error (a or b)")
	f("level: ( ((error or warn*) and re(foo))) (not (bar))", `(level:error or level:warn*) level:re("foo") !bar`)
	f("!(foo bar or baz and not aa*)", `!(foo bar or baz !aa*)`)

	// prefix search
	f(`'foo'* and (a:x* and x:* or y:i(""*)) and i("abc def"*)`, `foo* (a:x* x:* or y:i(*)) i("abc def"*)`)

	// This isn't a prefix search - it equals to `foo AND *`
	f(`foo *`, `foo *`)
	f(`"foo" *`, `foo *`)

	// empty filter
	f(`"" or foo:"" and not bar:""`, `"" or foo:"" !bar:""`)

	// _stream filters
	f(`_stream:{}`, ``)
	f(`_stream:{foo="bar", baz=~"x" OR or!="b", "x=},"="d}{"}`, `_stream:{foo="bar",baz=~"x" or "or"!="b","x=},"="d}{"}`)
	f(`_stream:{or=a or ","="b"}`, `_stream:{"or"="a" or ","="b"}`)
	f("_stream : { foo =  bar , }  ", `_stream:{foo="bar"}`)

	// _time filters
	f(`_time:[-5m,now)`, `_time:[-5m,now)`)
	f(`_time:(  now-1h  , now-5m34s5ms]`, `_time:(now-1h,now-5m34s5ms]`)
	f(`_time:[2023, 2023-01)`, `_time:[2023,2023-01)`)
	f(`_time:[2023-01-02, 2023-02-03T04)`, `_time:[2023-01-02,2023-02-03T04)`)
	f(`_time:[2023-01-02T04:05, 2023-02-03T04:05:06)`, `_time:[2023-01-02T04:05,2023-02-03T04:05:06)`)
	f(`_time:[2023-01-02T04:05:06Z, 2023-02-03T04:05:06.234Z)`, `_time:[2023-01-02T04:05:06Z,2023-02-03T04:05:06.234Z)`)
	f(`_time:[2023-01-02T04:05:06+02:30, 2023-02-03T04:05:06.234-02:45)`, `_time:[2023-01-02T04:05:06+02:30,2023-02-03T04:05:06.234-02:45)`)
	f(`_time:[2023-06-07T23:56:34.3456-02:30, now)`, `_time:[2023-06-07T23:56:34.3456-02:30,now)`)
	f(`_time:("2024-01-02+02:00", now)`, `_time:(2024-01-02+02:00,now)`)
	f(`_time:now`, `_time:now`)
	f(`_time:"now"`, `_time:now`)
	f(`_time:2024Z`, `_time:2024Z`)
	f(`_time:2024-02:30`, `_time:2024-02:30`)
	f(`_time:2024-01-02:30`, `_time:2024-01-02:30`)
	f(`_time:2024-01-02:30`, `_time:2024-01-02:30`)
	f(`_time:2024-01-02+03:30`, `_time:2024-01-02+03:30`)
	f(`_time:2024-01-02T10+03:30`, `_time:2024-01-02T10+03:30`)
	f(`_time:2024-01-02T10:20+03:30`, `_time:2024-01-02T10:20+03:30`)
	f(`_time:2024-01-02T10:20:40+03:30`, `_time:2024-01-02T10:20:40+03:30`)
	f(`_time:2024-01-02T10:20:40-03:30`, `_time:2024-01-02T10:20:40-03:30`)
	f(`_time:"2024-01-02T10:20:40Z"`, `_time:2024-01-02T10:20:40Z`)
	f(`_time:2023-01-02T04:05:06.789Z`, `_time:2023-01-02T04:05:06.789Z`)
	f(`_time:2023-01-02T04:05:06.789-02:30`, `_time:2023-01-02T04:05:06.789-02:30`)
	f(`_time:2023-01-02T04:05:06.789+02:30`, `_time:2023-01-02T04:05:06.789+02:30`)
	f(`_time:[1234567890, 1400000000]`, `_time:[1234567890,1400000000]`)
	f(`_time:2d3h5.5m3s45ms`, `_time:2d3h5.5m3s45ms`)
	f(`_time:2023-01-05 OFFSET 5m`, `_time:2023-01-05 offset 5m`)
	f(`_time:[2023-01-05, 2023-01-06] OFFset 5m`, `_time:[2023-01-05,2023-01-06] offset 5m`)
	f(`_time:[2023-01-05, 2023-01-06) OFFset 5m`, `_time:[2023-01-05,2023-01-06) offset 5m`)
	f(`_time:(2023-01-05, 2023-01-06] OFFset 5m`, `_time:(2023-01-05,2023-01-06] offset 5m`)
	f(`_time:(2023-01-05, 2023-01-06) OFFset 5m`, `_time:(2023-01-05,2023-01-06) offset 5m`)
	f(`_time:1h offset 5m`, `_time:1h offset 5m`)
	f(`_time:1h "offSet"`, `_time:1h "offSet"`) // "offset" is a search word, since it is quoted
	f(`_time:1h (Offset)`, `_time:1h "Offset"`) // "offset" is a search word, since it is in parens
	f(`_time:1h "and"`, `_time:1h "and"`)       // "and" is a search word, since it is quoted

	// reserved keywords
	f("and", `"and"`)
	f("and and or", `"and" "or"`)
	f("AnD", `"AnD"`)
	f("or", `"or"`)
	f("re 'and' `or` 'not'", `"re" "and" "or" "not"`)
	f("foo:and", `foo:"and"`)
	f("'re':or or x", `"re":"or" or x`)
	f(`"-"`, `"-"`)
	f(`"!"`, `"!"`)
	f(`"not"`, `"not"`)
	f(`''`, `""`)

	// reserved functions
	f("exact", `"exact"`)
	f("exact:a", `"exact":a`)
	f("exact-foo", `exact-foo`)
	f("a:exact", `a:"exact"`)
	f("a:exact-foo", `a:exact-foo`)
	f("exact-foo:b", `exact-foo:b`)
	f("i", `"i"`)
	f("i-foo", `i-foo`)
	f("a:i-foo", `a:i-foo`)
	f("i-foo:b", `i-foo:b`)
	f("in", `"in"`)
	f("in:a", `"in":a`)
	f("in-foo", `in-foo`)
	f("a:in", `a:"in"`)
	f("a:in-foo", `a:in-foo`)
	f("in-foo:b", `in-foo:b`)
	f("ipv4_range", `"ipv4_range"`)
	f("ipv4_range:a", `"ipv4_range":a`)
	f("ipv4_range-foo", `ipv4_range-foo`)
	f("a:ipv4_range", `a:"ipv4_range"`)
	f("a:ipv4_range-foo", `a:ipv4_range-foo`)
	f("ipv4_range-foo:b", `ipv4_range-foo:b`)
	f("len_range", `"len_range"`)
	f("len_range:a", `"len_range":a`)
	f("len_range-foo", `len_range-foo`)
	f("a:len_range", `a:"len_range"`)
	f("a:len_range-foo", `a:len_range-foo`)
	f("len_range-foo:b", `len_range-foo:b`)
	f("range", `"range"`)
	f("range:a", `"range":a`)
	f("range-foo", `range-foo`)
	f("a:range", `a:"range"`)
	f("a:range-foo", `a:range-foo`)
	f("range-foo:b", `range-foo:b`)
	f("re", `"re"`)
	f("re-bar", `re-bar`)
	f("a:re-bar", `a:re-bar`)
	f("re-bar:a", `re-bar:a`)
	f("seq", `"seq"`)
	f("seq-a", `seq-a`)
	f("x:seq-a", `x:seq-a`)
	f("seq-a:x", `seq-a:x`)
	f("string_range", `"string_range"`)
	f("string_range-a", `string_range-a`)
	f("x:string_range-a", `x:string_range-a`)
	f("string_range-a:x", `string_range-a:x`)

	// exact filter
	f("exact(foo)", `exact(foo)`)
	f("exact(foo*)", `exact(foo*)`)
	f("exact('foo bar),|baz')", `exact("foo bar),|baz")`)
	f("exact('foo bar),|baz'*)", `exact("foo bar),|baz"*)`)
	f(`exact(foo|b:ar)`, `exact("foo|b:ar")`)
	f(`foo:exact(foo|b:ar*)`, `foo:exact("foo|b:ar"*)`)

	// i filter
	f("i(foo)", `i(foo)`)
	f("i(foo*)", `i(foo*)`)
	f("i(`foo`* )", `i(foo*)`)
	f("i(' foo ) bar')", `i(" foo ) bar")`)
	f("i('foo bar'*)", `i("foo bar"*)`)
	f(`foo:i(foo:bar-baz|aa+bb)`, `foo:i("foo:bar-baz|aa+bb")`)

	// in filter
	f(`in()`, `in()`)
	f(`in(foo)`, `in(foo)`)
	f(`in(foo, bar)`, `in(foo,bar)`)
	f(`in("foo bar", baz)`, `in("foo bar",baz)`)
	f(`foo:in(foo-bar|baz)`, `foo:in("foo-bar|baz")`)

	// ipv4_range filter
	f(`ipv4_range(1.2.3.4, "5.6.7.8")`, `ipv4_range(1.2.3.4, 5.6.7.8)`)
	f(`foo:ipv4_range(1.2.3.4, "5.6.7.8" , )`, `foo:ipv4_range(1.2.3.4, 5.6.7.8)`)
	f(`ipv4_range(1.2.3.4)`, `ipv4_range(1.2.3.4, 1.2.3.4)`)
	f(`ipv4_range(1.2.3.4/20)`, `ipv4_range(1.2.0.0, 1.2.15.255)`)
	f(`ipv4_range(1.2.3.4,)`, `ipv4_range(1.2.3.4, 1.2.3.4)`)

	// len_range filter
	f(`len_range(10, 20)`, `len_range(10,20)`)
	f(`foo:len_range("10", 20, )`, `foo:len_range(10,20)`)

	// range filter
	f(`range(1.234, 5656.43454)`, `range(1.234,5656.43454)`)
	f(`foo:range(-2343.344, 2343.4343)`, `foo:range(-2343.344,2343.4343)`)
	f(`range(-1.234e-5  , 2.34E+3)`, `range(-1.234e-5,2.34E+3)`)
	f(`range[123, 456)`, `range[123,456)`)
	f(`range(123, 445]`, `range(123,445]`)
	f(`range("1.234e-4", -23)`, `range(1.234e-4,-23)`)

	// re filter
	f("re('foo|ba(r.+)')", `re("foo|ba(r.+)")`)
	f("re(foo)", `re("foo")`)
	f(`foo:re(foo-bar|baz.)`, `foo:re("foo-bar|baz.")`)

	// seq filter
	f(`seq()`, `seq()`)
	f(`seq(foo)`, `seq(foo)`)
	f(`seq("foo, bar", baz, abc)`, `seq("foo, bar",baz,abc)`)
	f(`foo:seq(foo"bar-baz+aa, b)`, `foo:seq("foo\"bar-baz+aa",b)`)

	// string_range filter
	f(`string_range(foo, bar)`, `string_range(foo, bar)`)
	f(`foo:string_range("foo, bar", baz)`, `foo:string_range("foo, bar", baz)`)

	// reserved field names
	f(`"_stream"`, `_stream`)
	f(`"_time"`, `_time`)
	f(`"_msg"`, `_msg`)
	f(`_stream and _time or _msg`, `_stream _time or _msg`)

	// invalid rune
	f("\xff", `"\xff"`)

	// ip addresses in the query
	f("1.2.3.4 or ip:5.6.7.9", "1.2.3.4 or ip:5.6.7.9")

	// '-' and '.' chars in field name and search phrase
	f("trace-id.foo.bar:baz", `trace-id.foo.bar:baz`)
	f(`custom-Time:2024-01-02T03:04:05+08:00    fooBar OR !baz:xxx`, `custom-Time:"2024-01-02T03:04:05+08:00" fooBar or !baz:xxx`)
	f("foo-bar+baz*", `"foo-bar+baz"*`)
	f("foo- bar", `foo- bar`)
	f("foo -bar", `foo -bar`)
	f("foo!bar", `"foo!bar"`)
	f("foo:aa!bb:cc", `foo:"aa!bb:cc"`)
	f(`foo:bar:baz`, `foo:"bar:baz"`)
	f(`foo:(bar baz:xxx)`, `foo:bar foo:"baz:xxx"`)
	f(`foo:(_time:abc or not z)`, `foo:"_time:abc" or !foo:z`)
	f(`foo:(_msg:a :x _stream:{c="d"})`, `foo:"_msg:a" foo:x foo:"_stream:{c=\"d\"}"`)
	f(`:(_msg:a:b c)`, `"a:b" c`)
	f(`"foo"bar baz:"a'b"c`, `"\"foo\"bar" baz:"\"a'b\"c"`)

	// complex queries
	f(`_time:[-1h, now] _stream:{job="foo",env=~"prod|staging"} level:(error or warn*) and not "connection reset by peer"`,
		`_time:[-1h,now] _stream:{job="foo",env=~"prod|staging"} (level:error or level:warn*) !"connection reset by peer"`)
	f(`(_time:(2023-04-20, now] or _time:[-10m, -1m))
		and (_stream:{job="a"} or _stream:{instance!="b"})
		and (err* or ip:(ipv4_range(1.2.3.0, 1.2.3.255) and not 1.2.3.4))`,
		`(_time:(2023-04-20,now] or _time:[-10m,-1m)) (_stream:{job="a"} or _stream:{instance!="b"}) (err* or ip:ipv4_range(1.2.3.0, 1.2.3.255) !ip:1.2.3.4)`)
}

func TestParseQueryFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		q, err := ParseQuery(s)
		if q != nil {
			t.Fatalf("expecting nil result; got %s", q)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("")
	f("|")
	f("foo|")
	f("foo|bar")
	f("foo and")
	f("foo OR ")
	f("not")
	f("NOT")
	f("not (abc")
	f("!")

	// invalid parens
	f("(")
	f("foo (bar ")
	f("(foo:'bar")

	// missing filter
	f(":")
	f(":  ")
	f("foo:  ")
	f("_msg :   ")
	f(`"":   `)

	// invalid quoted strings
	f(`"foo`)
	f(`'foo`)
	f("`foo")

	// invalid _stream filters
	f("_stream:")
	f("_stream:{")
	f("_stream:(")
	f("_stream:{foo")
	f("_stream:{foo}")
	f("_stream:{foo=")
	f("_stream:{foo='bar")
	f("_stream:{foo='bar}")
	f("_stream:{foo=bar or")
	f("_stream:{foo=bar or}")
	f("_stream:{foo=bar or baz}")
	f("_stream:{foo=bar baz x=y}")
	f("_stream:{foo=bar,")
	f("_stream:{foo=bar")
	f("_stream:foo")
	f("_stream:(foo)")
	f("_stream:[foo]")

	// invalid _time filters
	f("_time:")
	f("_time:[")
	f("_time:foo")
	f("_time:{}")
	f("_time:[foo,bar)")
	f("_time:(now)")
	f("_time:[now,")
	f("_time:(now, not now]")
	f("_time:(-5m, -1m}")
	f("_time:[-")
	f("_time:[now-foo,-bar]")
	f("_time:[2023-ab,2023]")
	f("_time:[fooo-02,2023]")
	f("_time:[2023-01-02T04:05:06+12,2023]")
	f("_time:[2023-01-02T04:05:06-12,2023]")
	f("_time:2023-01-02T04:05:06.789")
	f("_time:234foo")
	f("_time:5m offset")
	f("_time:10m offset foobar")

	// long query with error
	f(`very long query with error aaa ffdfd fdfdfd fdfd:( ffdfdfdfdfd`)

	// query with unexpected tail
	f(`foo | bar`)

	// unexpected comma
	f(`foo,bar`)
	f(`foo, bar`)
	f(`foo ,bar`)

	// unexpected token
	f(`[foo`)
	f(`foo]bar`)
	f(`foo] bar`)
	f(`foo ]bar`)
	f(`) foo`)
	f(`foo)bar`)

	// unknown function
	f(`unknown_function(foo)`)

	// invalid exact
	f(`exact(`)
	f(`exact(f, b)`)
	f(`exact(foo`)
	f(`exact(foo,`)
	f(`exact(foo bar)`)
	f(`exact(foo, bar`)
	f(`exact(foo,)`)

	// invalid i
	f(`i(`)
	f(`i(aa`)
	f(`i(aa, bb)`)
	f(`i(*`)
	f(`i(aaa*`)
	f(`i(a**)`)
	f(`i("foo`)
	f(`i(foo bar)`)

	// invalid in
	f(`in(`)
	f(`in(,)`)
	f(`in(f, b c)`)
	f(`in(foo`)
	f(`in(foo,`)
	f(`in(foo*)`)
	f(`in(foo, "bar baz"*)`)
	f(`in(foo, "bar baz"*, abc)`)
	f(`in(foo bar)`)
	f(`in(foo, bar`)

	// invalid ipv4_range
	f(`ipv4_range(`)
	f(`ipv4_range(foo,bar)`)
	f(`ipv4_range(1.2.3.4*)`)
	f(`ipv4_range("1.2.3.4"*)`)
	f(`ipv4_range(1.2.3.4`)
	f(`ipv4_range(1.2.3.4,`)
	f(`ipv4_range(1.2.3.4, 5.6.7)`)
	f(`ipv4_range(1.2.3.4, 5.6.7.8`)
	f(`ipv4_range(1.2.3.4, 5.6.7.8,`)
	f(`ipv4_range(1.2.3.4, 5.6.7.8,,`)
	f(`ipv4_range(1.2.3.4, 5.6.7.8,5.3.2.1)`)

	// invalid len_range
	f(`len_range(`)
	f(`len_range(1)`)
	f(`len_range(foo, bar)`)
	f(`len_range(1, bar)`)
	f(`len_range(1, 2`)
	f(`len_range(1.2, 3.4)`)

	// invalid range
	f(`range(`)
	f(`range(foo,bar)`)
	f(`range(1"`)
	f(`range(1,`)
	f(`range(1)`)
	f(`range(1,)`)
	f(`range(1,2,`)
	f(`range[1,foo)`)
	f(`range[1,2,3)`)
	f(`range(1)`)

	// invalid re
	f("re(")
	f("re(a, b)")
	f("foo:re(bar")
	f("re(`ab(`)")
	f(`re(a b)`)

	// invalid seq
	f(`seq(`)
	f(`seq(,)`)
	f(`seq(foo`)
	f(`seq(foo,`)
	f(`seq(foo*)`)
	f(`seq(foo*, bar)`)
	f(`seq(foo bar)`)
	f(`seq(foo, bar`)

	// invalid string_range
	f(`string_range(`)
	f(`string_range(,)`)
	f(`string_range(foo`)
	f(`string_range(foo,`)
	f(`string_range(foo*)`)
	f(`string_range(foo bar)`)
	f(`string_range(foo, bar`)
	f(`string_range(foo)`)
	f(`string_range(foo, bar, baz)`)
}
