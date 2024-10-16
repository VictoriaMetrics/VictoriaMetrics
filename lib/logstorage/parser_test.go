package logstorage

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestLexer(t *testing.T) {
	f := func(s string, tokensExpected []string) {
		t.Helper()
		lex := newLexer(s)
		for _, tokenExpected := range tokensExpected {
			if lex.token != tokenExpected {
				t.Fatalf("unexpected token; got %q; want %q", lex.token, tokenExpected)
			}
			lex.nextToken()
		}
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
	f(`{foo="bar",a=~"baz", b != 'cd',"d,}a"!~abc} def`,
		[]string{"{", "foo", "=", "bar", ",", "a", "=~", "baz", ",", "b", "!=", "cd", ",", "d,}a", "!~", "abc", "}", "def"})
	f(`_stream:{foo="bar",a=~"baz", b != 'cd',"d,}a"!~abc}`,
		[]string{"_stream", ":", "{", "foo", "=", "bar", ",", "a", "=~", "baz", ",", "b", "!=", "cd", ",", "d,}a", "!~", "abc", "}"})
}

func TestParseDayRange(t *testing.T) {
	f := func(s string, startExpected, endExpected, offsetExpected int64) {
		t.Helper()
		q, err := ParseQuery("_time:day_range" + s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fr, ok := q.f.(*filterDayRange)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *filterDayRange; filter: %s", q.f, q.f)
		}
		if fr.stringRepr != s {
			t.Fatalf("unexpected string representation for filterDayRange; got %q; want %q", fr.stringRepr, s)
		}
		if fr.start != startExpected {
			t.Fatalf("unexpected start; got %d; want %d", fr.start, startExpected)
		}
		if fr.end != endExpected {
			t.Fatalf("unexpected end; got %d; want %d", fr.end, endExpected)
		}
		if fr.offset != offsetExpected {
			t.Fatalf("unexpected offset; got %d; want %d", fr.offset, offsetExpected)
		}
	}

	f("[00:00, 24:00]", 0, nsecsPerDay-1, 0)
	f("[10:20, 125:00]", 10*nsecsPerHour+20*nsecsPerMinute, nsecsPerDay-1, 0)
	f("(00:00, 24:00)", 1, nsecsPerDay-2, 0)
	f("[08:00, 18:00)", 8*nsecsPerHour, 18*nsecsPerHour-1, 0)
	f("[08:00, 18:00) offset 2h", 8*nsecsPerHour, 18*nsecsPerHour-1, 2*nsecsPerHour)
	f("[08:00, 18:00) offset -2h", 8*nsecsPerHour, 18*nsecsPerHour-1, -2*nsecsPerHour)
}

func TestParseWeekRange(t *testing.T) {
	f := func(s string, startDayExpected, endDayExpected time.Weekday, offsetExpected int64) {
		t.Helper()
		q, err := ParseQuery("_time:week_range" + s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fr, ok := q.f.(*filterWeekRange)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *filterWeekRange; filter: %s", q.f, q.f)
		}
		if fr.stringRepr != s {
			t.Fatalf("unexpected string representation for filterWeekRange; got %q; want %q", fr.stringRepr, s)
		}
		if fr.startDay != startDayExpected {
			t.Fatalf("unexpected start; got %s; want %s", fr.startDay, startDayExpected)
		}
		if fr.endDay != endDayExpected {
			t.Fatalf("unexpected end; got %s; want %s", fr.endDay, endDayExpected)
		}
		if fr.offset != offsetExpected {
			t.Fatalf("unexpected offset; got %d; want %d", fr.offset, offsetExpected)
		}
	}

	f("[Sun, Sat]", time.Sunday, time.Saturday, 0)
	f("(Sun, Sat]", time.Monday, time.Saturday, 0)
	f("(Sun, Sat)", time.Monday, time.Friday, 0)
	f("[Sun, Sat)", time.Sunday, time.Friday, 0)

	f(`[Mon, Tue]`, time.Monday, time.Tuesday, 0)
	f(`[Wed, Thu]`, time.Wednesday, time.Thursday, 0)
	f(`[Fri, Sat]`, time.Friday, time.Saturday, 0)

	f(`[Mon, Fri] offset 2h`, time.Monday, time.Friday, 2*nsecsPerHour)
	f(`[Mon, Fri] offset -2h`, time.Monday, time.Friday, -2*nsecsPerHour)
}

func TestParseTimeDuration(t *testing.T) {
	f := func(s string, durationExpected time.Duration) {
		t.Helper()
		q, err := ParseQuery("_time:" + s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		ft, ok := q.f.(*filterTime)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *filterTime; filter: %s", q.f, q.f)
		}
		if ft.stringRepr != s {
			t.Fatalf("unexpected string represenation for filterTime; got %q; want %q", ft.stringRepr, s)
		}
		duration := time.Duration(ft.maxTimestamp - ft.minTimestamp)
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
		ft, ok := q.f.(*filterTime)
		if !ok {
			t.Fatalf("unexpected filter; got %T; want *filterTime; filter: %s", q.f, q.f)
		}
		if ft.stringRepr != s {
			t.Fatalf("unexpected string represenation for filterTime; got %q; want %q", ft.stringRepr, s)
		}
		if ft.minTimestamp != minTimestampExpected {
			t.Fatalf("unexpected minTimestamp; got %s; want %s", timestampToString(ft.minTimestamp), timestampToString(minTimestampExpected))
		}
		if ft.maxTimestamp != maxTimestampExpected {
			t.Fatalf("unexpected maxTimestamp; got %s; want %s", timestampToString(ft.maxTimestamp), timestampToString(maxTimestampExpected))
		}
	}

	var minTimestamp, maxTimestamp int64

	// _time:YYYY -> _time:[YYYY, YYYY+1)
	minTimestamp = time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
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
	f("2023-02-12Z", minTimestamp, maxTimestamp)
	// February 28
	minTimestamp = time.Date(2023, time.February, 28, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-02-28Z", minTimestamp, maxTimestamp)
	// January 31
	minTimestamp = time.Date(2023, time.January, 31, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f("2023-01-31Z", minTimestamp, maxTimestamp)

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
	f("2023-02-28T23:59:59Z", minTimestamp, maxTimestamp)

	// _time:[YYYY-MM-DDTHH:MM:SS.sss, YYYY-MM-DDTHH:MM:SS.sss)
	minTimestamp = time.Date(2024, time.May, 12, 0, 0, 0, 333000000, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.May, 12, 0, 0, 0, 555000000, time.UTC).UnixNano() - 1
	f("[2024-05-12T00:00:00.333+00:00,2024-05-12T00:00:00.555+00:00)", minTimestamp, maxTimestamp)

	// _time:[YYYY-MM-DDTHH:MM:SS.sss, YYYY-MM-DDTHH:MM:SS.sss]
	minTimestamp = time.Date(2024, time.May, 12, 0, 0, 0, 333000000, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.May, 12, 0, 0, 0, 556000000, time.UTC).UnixNano() - 1
	f("[2024-05-12T00:00:00.333+00:00,2024-05-12T00:00:00.555+00:00]", minTimestamp, maxTimestamp)

	// _time:YYYY-MM-DDTHH:MM:SS.sss
	minTimestamp = time.Date(2024, time.May, 14, 13, 54, 59, 134000000, time.UTC).UnixNano()
	maxTimestamp = time.Date(2024, time.May, 14, 13, 54, 59, 135000000, time.UTC).UnixNano() - 1
	f("2024-05-14T13:54:59.134Z", minTimestamp, maxTimestamp)

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
	f(`(2023-03-01Z,2023-04-06Z)`, minTimestamp, maxTimestamp)

	// _time:[start, end)
	minTimestamp = time.Date(2023, time.March, 1, 0, 0, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.April, 6, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`[2023-03-01Z,2023-04-06Z)`, minTimestamp, maxTimestamp)

	// _time:(start, end]
	minTimestamp = time.Date(2023, time.March, 1, 21, 20, 0, 0, time.UTC).UnixNano() + 1
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`(2023-03-01T21:20Z,2023-04-06Z]`, minTimestamp, maxTimestamp)

	// _time:[start, end] with timezone
	minTimestamp = time.Date(2023, time.February, 28, 21, 40, 0, 0, time.UTC).UnixNano()
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1
	f(`[2023-03-01+02:20,2023-04-06T23Z]`, minTimestamp, maxTimestamp)

	// _time:[start, end] with timezone and offset
	offset := int64(30*time.Minute + 5*time.Second)
	minTimestamp = time.Date(2023, time.February, 28, 21, 40, 0, 0, time.UTC).UnixNano() - offset
	maxTimestamp = time.Date(2023, time.April, 7, 0, 0, 0, 0, time.UTC).UnixNano() - 1 - offset
	f(`[2023-03-01+02:20,2023-04-06T23Z] offset 30m5s`, minTimestamp, maxTimestamp)
}

func TestParseFilterSequence(t *testing.T) {
	f := func(s, fieldNameExpected string, phrasesExpected []string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fs, ok := q.f.(*filterSequence)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterSequence; filter: %s", q.f, q.f)
		}
		if fs.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fs.fieldName, fieldNameExpected)
		}
		if !reflect.DeepEqual(fs.phrases, phrasesExpected) {
			t.Fatalf("unexpected phrases\ngot\n%q\nwant\n%q", fs.phrases, phrasesExpected)
		}
	}

	f(`seq()`, ``, nil)
	f(`foo:seq(foo)`, `foo`, []string{"foo"})
	f(`_msg:seq("foo bar,baz")`, `_msg`, []string{"foo bar,baz"})
	f(`seq(foo,bar-baz.aa"bb","c,)d")`, ``, []string{"foo", `bar-baz.aa"bb"`, "c,)d"})
}

func TestParseFilterIn(t *testing.T) {
	f := func(s, fieldNameExpected string, valuesExpected []string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		f, ok := q.f.(*filterIn)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterIn; filter: %s", q.f, q.f)
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

	// verify `in(query)` - it shouldn't set values
	f(`in(x|fields foo)`, ``, nil)
	f(`a:in(* | fields bar)`, `a`, nil)
}

func TestParseFilterIPv4Range(t *testing.T) {
	f := func(s, fieldNameExpected string, minValueExpected, maxValueExpected uint32) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fr, ok := q.f.(*filterIPv4Range)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterIPv4Range; filter: %s", q.f, q.f)
		}
		if fr.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fr.fieldName, fieldNameExpected)
		}
		if fr.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %08x; want %08x", fr.minValue, minValueExpected)
		}
		if fr.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %08x; want %08x", fr.maxValue, maxValueExpected)
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

func TestParseFilterStringRange(t *testing.T) {
	f := func(s, fieldNameExpected, minValueExpected, maxValueExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fr, ok := q.f.(*filterStringRange)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterStringRange; filter: %s", q.f, q.f)
		}
		if fr.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fr.fieldName, fieldNameExpected)
		}
		if fr.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %q; want %q", fr.minValue, minValueExpected)
		}
		if fr.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %q; want %q", fr.maxValue, maxValueExpected)
		}
	}

	f("string_range(foo, bar)", ``, "foo", "bar")
	f(`abc:string_range("foo,bar", "baz) !")`, `abc`, `foo,bar`, `baz) !`)
	f(">foo", ``, "foo\x00", maxStringRangeValue)
	f("x:>=foo", `x`, "foo", maxStringRangeValue)
	f("x:<foo", `x`, ``, `foo`)
	f(`<="123.456.789"`, ``, ``, "123.456.789\x00")
}

func TestParseFilterRegexp(t *testing.T) {
	f := func(s, reExpected string) {
		t.Helper()
		q, err := ParseQuery("re(" + s + ")")
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fr, ok := q.f.(*filterRegexp)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterRegexp; filter: %s", q.f, q.f)
		}
		if reString := fr.re.String(); reString != reExpected {
			t.Fatalf("unexpected regexp; got %q; want %q", reString, reExpected)
		}
	}

	f(`""`, ``)
	f(`foo`, `foo`)
	f(`"foo.+|bar.*"`, `foo.+|bar.*`)
	f(`"foo(bar|baz),x[y]"`, `foo(bar|baz),x[y]`)
}

func TestParseAnyCaseFilterPhrase(t *testing.T) {
	f := func(s, fieldNameExpected, phraseExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fp, ok := q.f.(*filterAnyCasePhrase)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterAnyCasePhrase; filter: %s", q.f, q.f)
		}
		if fp.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fp.fieldName, fieldNameExpected)
		}
		if fp.phrase != phraseExpected {
			t.Fatalf("unexpected phrase; got %q; want %q", fp.phrase, phraseExpected)
		}
	}

	f(`i("")`, ``, ``)
	f(`i(foo)`, ``, `foo`)
	f(`abc-de.fg:i(foo-bar+baz)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar+baz")`, `abc-de.fg`, `foo-bar+baz`)
}

func TestParseAnyCaseFilterPrefix(t *testing.T) {
	f := func(s, fieldNameExpected, prefixExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fp, ok := q.f.(*filterAnyCasePrefix)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterAnyCasePrefix; filter: %s", q.f, q.f)
		}
		if fp.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fp.fieldName, fieldNameExpected)
		}
		if fp.prefix != prefixExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", fp.prefix, prefixExpected)
		}
	}

	f(`i(*)`, ``, ``)
	f(`i(""*)`, ``, ``)
	f(`i(foo*)`, ``, `foo`)
	f(`abc-de.fg:i(foo-bar+baz*)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar+baz"*)`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":i("foo-bar*baz *"*)`, `abc-de.fg`, `foo-bar*baz *`)
}

func TestParseFilterPhrase(t *testing.T) {
	f := func(s, fieldNameExpected, phraseExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fp, ok := q.f.(*filterPhrase)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterPhrase; filter: %s", q.f, q.f)
		}
		if fp.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fp.fieldName, fieldNameExpected)
		}
		if fp.phrase != phraseExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", fp.phrase, phraseExpected)
		}
	}

	f(`""`, ``, ``)
	f(`foo`, ``, `foo`)
	f(`abc-de.fg:foo-bar+baz`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar+baz"`, `abc-de.fg`, `foo-bar+baz`)
	f(`"abc-de.fg":"foo-bar*baz *"`, `abc-de.fg`, `foo-bar*baz *`)
	f(`"foo:bar*,( baz"`, ``, `foo:bar*,( baz`)
}

func TestParseFilterPrefix(t *testing.T) {
	f := func(s, fieldNameExpected, prefixExpected string) {
		t.Helper()
		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		fp, ok := q.f.(*filterPrefix)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterPrefix; filter: %s", q.f, q.f)
		}
		if fp.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fp.fieldName, fieldNameExpected)
		}
		if fp.prefix != prefixExpected {
			t.Fatalf("unexpected prefix; got %q; want %q", fp.prefix, prefixExpected)
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
		fr, ok := q.f.(*filterRange)
		if !ok {
			t.Fatalf("unexpected filter type; got %T; want *filterRange; filter: %s", q.f, q.f)
		}
		if fr.fieldName != fieldNameExpected {
			t.Fatalf("unexpected fieldName; got %q; want %q", fr.fieldName, fieldNameExpected)
		}
		if fr.minValue != minValueExpected {
			t.Fatalf("unexpected minValue; got %v; want %v", fr.minValue, minValueExpected)
		}
		if fr.maxValue != maxValueExpected {
			t.Fatalf("unexpected maxValue; got %v; want %v", fr.maxValue, maxValueExpected)
		}
	}

	f(`range[-1.234, +2e5]`, ``, -1.234, 2e5)
	f(`foo:range[-1.234e-5, 2e5]`, `foo`, -1.234e-5, 2e5)
	f(`range:range["-1.234e5", "-2e-5"]`, `range`, -1.234e5, -2e-5)

	f(`_msg:range[1, 2]`, `_msg`, 1, 2)
	f(`:range(1, 2)`, ``, nextafter(1, inf), nextafter(2, -inf))
	f(`range[1, 2)`, ``, 1, nextafter(2, -inf))
	f(`range("1", 2]`, ``, nextafter(1, inf), 2)

	f(`response_size:range[1KB, 10MiB]`, `response_size`, 1_000, 10*(1<<20))
	f(`response_size:range[1G, 10Ti]`, `response_size`, 1_000_000_000, 10*(1<<40))
	f(`response_size:range[10, inf]`, `response_size`, 10, inf)

	f(`duration:range[100ns, 1y2w2.5m3s5ms]`, `duration`, 100, 1*nsecsPerYear+2*nsecsPerWeek+2.5*nsecsPerMinute+3*nsecsPerSecond+5*nsecsPerMillisecond)

	f(`>=10`, ``, 10, inf)
	f(`<=10`, ``, -inf, 10)
	f(`foo:>10.43`, `foo`, nextafter(10.43, inf), inf)
	f(`foo: > -10.43`, `foo`, nextafter(-10.43, inf), inf)
	f(`foo:>=10.43`, `foo`, 10.43, inf)
	f(`foo: >= -10.43`, `foo`, -10.43, inf)

	f(`foo:<10.43K`, `foo`, -inf, nextafter(10_430, -inf))
	f(`foo: < -10.43`, `foo`, -inf, nextafter(-10.43, -inf))
	f(`foo:<=10.43ms`, `foo`, -inf, 10_430_000)
	f(`foo: <= 10.43`, `foo`, -inf, 10.43)

	f(`foo:<=1.2.3.4`, `foo`, -inf, 16909060)
	f(`foo:<='1.2.3.4'`, `foo`, -inf, 16909060)
	f(`foo:>=0xffffffff`, `foo`, (1<<32)-1, inf)
	f(`foo:>=1_234e3`, `foo`, 1234000, inf)
	f(`foo:>=1_234e-3`, `foo`, 1.234, inf)
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

		// verify that the marshaled query is parsed to the same query
		qParsed, err := ParseQuery(result)
		if err != nil {
			t.Fatalf("cannot parse marshaled query: %s", err)
		}
		qStr := qParsed.String()
		if qStr != result {
			t.Fatalf("unexpected marshaled query\ngot\n%s\nwant\n%s", qStr, result)
		}
	}

	f("foo", "foo")
	f(":foo", "foo")
	f(`"":foo`, "foo")
	f(`"" bar`, `"" bar`)
	f(`!''`, `!""`)
	f(`-''`, `!""`)
	f(`foo:""`, `foo:""`)
	f(`-foo:""`, `!foo:""`)
	f(`!foo:""`, `!foo:""`)
	f(`not foo:""`, `!foo:""`)
	f(`not(foo)`, `!foo`)
	f(`not (foo)`, `!foo`)
	f(`not ( foo or bar )`, `!(foo or bar)`)
	f(`!(foo or bar)`, `!(foo or bar)`)
	f(`-(foo or bar)`, `!(foo or bar)`)
	f(`foo:!""`, `!foo:""`)
	f("_msg:foo", "foo")
	f("'foo:bar'", `"foo:bar"`)
	f("'!foo'", `"!foo"`)
	f("'-foo'", `"-foo"`)
	f(`'{a="b"}'`, `"{a=\"b\"}"`)
	f("foo 'and' and bar", `foo "and" bar`)
	f("foo bar", "foo bar")
	f("foo and bar", "foo bar")
	f("foo AND bar", "foo bar")
	f("foo or bar", "foo or bar")
	f("foo OR bar", "foo or bar")
	f("not foo", "!foo")
	f("! foo", "!foo")
	f("- foo", "!foo")
	f("not !`foo bar`", `"foo bar"`)
	f("not -`foo bar`", `"foo bar"`)
	f("foo or bar and not baz", "foo or bar !baz")
	f("'foo bar' !baz", `"foo bar" !baz`)
	f("foo:!bar", `!foo:bar`)
	f("foo:-bar", `!foo:bar`)
	f(`foo and bar and baz or x or y or z and zz`, `foo bar baz or x or y or z zz`)
	f(`foo and bar and (baz or x or y or z) and zz`, `foo bar (baz or x or y or z) zz`)
	f(`(foo or bar or baz) and x and y and (z or zz)`, `(foo or bar or baz) x y (z or zz)`)
	f(`(foo or bar or baz) and x and y and not (z or zz)`, `(foo or bar or baz) x y !(z or zz)`)
	f(`NOT foo AND bar OR baz`, `!foo bar or baz`)
	f(`NOT (foo AND bar) OR baz`, `!(foo bar) or baz`)
	f(`foo OR bar AND baz`, `foo or bar baz`)
	f(`foo bar or baz xyz`, `foo bar or baz xyz`)
	f(`foo (bar or baz) xyz`, `foo (bar or baz) xyz`)
	f(`foo or bar baz or xyz`, `foo or bar baz or xyz`)
	f(`(foo or bar) (baz or xyz)`, `(foo or bar) (baz or xyz)`)
	f(`(foo OR bar) AND baz`, `(foo or bar) baz`)
	f(`'stats' foo`, `"stats" foo`)
	f(`"filter" bar copy fields avg baz`, `"filter" bar "copy" "fields" "avg" baz`)

	// parens
	f(`foo:(bar baz or not :xxx)`, `foo:bar foo:baz or !foo:xxx`)
	f(`(foo:bar and (foo:baz or aa:bb) and xx) and y`, `foo:bar (foo:baz or aa:bb) xx y`)
	f("level:error and _msg:(a or b)", "level:error (a or b)")
	f("level: ( ((error or warn*) and re(foo))) (not (bar))", `(level:error or level:warn*) level:~foo !bar`)
	f("!(foo bar or baz and not aa*)", `!(foo bar or baz !aa*)`)

	// prefix search
	f(`'foo'* and (a:x* and x:* or y:i(""*)) and i("abc def"*)`, `foo* (a:x* x:* or y:i(*)) i("abc def"*)`)

	// This isn't a prefix search - it equals to `foo AND *`
	f(`foo *`, `foo *`)
	f(`"foo" *`, `foo *`)

	// empty filter
	f(`"" or foo:"" and not bar:""`, `"" or foo:"" !bar:""`)

	// _stream_id filter
	f(`_stream_id:0000007b000001c8302bc96e02e54e5524b3a68ec271e55e`, `_stream_id:0000007b000001c8302bc96e02e54e5524b3a68ec271e55e`)
	f(`_stream_id:"0000007b000001c8302bc96e02e54e5524b3a68ec271e55e"`, `_stream_id:0000007b000001c8302bc96e02e54e5524b3a68ec271e55e`)
	f(`_stream_id:in()`, `_stream_id:in()`)
	f(`_stream_id:in(0000007b000001c8302bc96e02e54e5524b3a68ec271e55e)`, `_stream_id:0000007b000001c8302bc96e02e54e5524b3a68ec271e55e`)
	f(`_stream_id:in(0000007b000001c8302bc96e02e54e5524b3a68ec271e55e, "0000007b000001c850d9950ea6196b1a4812081265faa1c7")`,
		`_stream_id:in(0000007b000001c8302bc96e02e54e5524b3a68ec271e55e,0000007b000001c850d9950ea6196b1a4812081265faa1c7)`)
	f(`_stream_id:in(_time:5m | fields _stream_id)`, `_stream_id:in(_time:5m | fields _stream_id)`)

	// _stream filters
	f(`_stream:{}`, `{}`)
	f(`_stream:{foo="bar", baz=~"x" OR or!="b", "x=},"="d}{"}`, `{foo="bar",baz=~"x" or "or"!="b","x=},"="d}{"}`)
	f(`_stream:{or=a or ","="b"}`, `{"or"="a" or ","="b"}`)
	f("_stream : { foo =  bar , }  ", `{foo="bar"}`)

	// _stream filter without _stream prefix
	f(`{}`, `{}`)
	f(`{foo="bar", baz=~"x" OR or!="b", "x=},"="d}{"}`, `{foo="bar",baz=~"x" or "or"!="b","x=},"="d}{"}`)

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

	// dayRange filters
	f(`_time:day_range[08:00, 20:30)`, `_time:day_range[08:00, 20:30)`)
	f(`_time:day_range(08:00, 20:30)`, `_time:day_range(08:00, 20:30)`)
	f(`_time:day_range(08:00, 20:30]`, `_time:day_range(08:00, 20:30]`)
	f(`_time:day_range[08:00, 20:30]`, `_time:day_range[08:00, 20:30]`)
	f(`_time:day_range[08:00, 20:30] offset 2.5h`, `_time:day_range[08:00, 20:30] offset 2.5h`)
	f(`_time:day_range[08:00, 20:30] offset -2.5h`, `_time:day_range[08:00, 20:30] offset -2.5h`)

	// weekRange filters
	f(`_time:week_range[Mon, Fri]`, `_time:week_range[Mon, Fri]`)
	f(`_time:week_range(Monday, Friday] offset 2.5h`, `_time:week_range(Monday, Friday] offset 2.5h`)
	f(`_time:week_range[monday, friday) offset -2.5h`, `_time:week_range[monday, friday) offset -2.5h`)
	f(`_time:week_range(mon, fri]`, `_time:week_range(mon, fri]`)

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
	f("exact-foo", `"exact-foo"`)
	f("a:exact", `a:"exact"`)
	f("a:exact-foo", `a:"exact-foo"`)
	f("exact-foo:b", `"exact-foo":b`)
	f("i", `"i"`)
	f("i-foo", `"i-foo"`)
	f("a:i-foo", `a:"i-foo"`)
	f("i-foo:b", `"i-foo":b`)
	f("in", `"in"`)
	f("in:a", `"in":a`)
	f("in-foo", `"in-foo"`)
	f("a:in", `a:"in"`)
	f("a:in-foo", `a:"in-foo"`)
	f("in-foo:b", `"in-foo":b`)
	f("ipv4_range", `"ipv4_range"`)
	f("ipv4_range:a", `"ipv4_range":a`)
	f("ipv4_range-foo", `"ipv4_range-foo"`)
	f("a:ipv4_range", `a:"ipv4_range"`)
	f("a:ipv4_range-foo", `a:"ipv4_range-foo"`)
	f("ipv4_range-foo:b", `"ipv4_range-foo":b`)
	f("len_range", `"len_range"`)
	f("len_range:a", `"len_range":a`)
	f("len_range-foo", `"len_range-foo"`)
	f("a:len_range", `a:"len_range"`)
	f("a:len_range-foo", `a:"len_range-foo"`)
	f("len_range-foo:b", `"len_range-foo":b`)
	f("range", `"range"`)
	f("range:a", `"range":a`)
	f("range-foo", `"range-foo"`)
	f("a:range", `a:"range"`)
	f("a:range-foo", `a:"range-foo"`)
	f("range-foo:b", `"range-foo":b`)
	f("re", `"re"`)
	f("re-bar", `"re-bar"`)
	f("a:re-bar", `a:"re-bar"`)
	f("re-bar:a", `"re-bar":a`)
	f("seq", `"seq"`)
	f("seq-a", `"seq-a"`)
	f("x:seq-a", `x:"seq-a"`)
	f("seq-a:x", `"seq-a":x`)
	f("string_range", `"string_range"`)
	f("string_range-a", `"string_range-a"`)
	f("x:string_range-a", `x:"string_range-a"`)
	f("string_range-a:x", `"string_range-a":x`)

	// exact filter
	f("exact(foo)", `=foo`)
	f("exact(foo*)", `=foo*`)
	f("exact('foo bar),|baz')", `="foo bar),|baz"`)
	f("exact('foo bar),|baz'*)", `="foo bar),|baz"*`)
	f(`exact(foo/b:ar)`, `="foo/b:ar"`)
	f(`foo:exact(foo/b:ar*)`, `foo:="foo/b:ar"*`)
	f(`exact("foo/bar")`, `="foo/bar"`)
	f(`exact('foo/bar')`, `="foo/bar"`)
	f(`="foo/bar"`, `="foo/bar"`)
	f("=foo=bar !=b<=a>z foo:!='abc'*", `="foo=bar" !="b<=a>z" !foo:=abc*`)
	f("==foo =>=bar x : ( = =a<b*='c*' >=20)", `="=foo" =">=bar" x:="=a<b"* x:="c*" x:>=20`)

	// i filter
	f("i(foo)", `i(foo)`)
	f("i(foo*)", `i(foo*)`)
	f("i(`foo`* )", `i(foo*)`)
	f("i(' foo ) bar')", `i(" foo ) bar")`)
	f("i('foo bar'*)", `i("foo bar"*)`)
	f(`foo:i(foo:bar-baz/aa+bb)`, `foo:i("foo:bar-baz/aa+bb")`)

	// in filter with values
	f(`in()`, `in()`)
	f(`in(foo)`, `in(foo)`)
	f(`in(foo, bar)`, `in(foo,bar)`)
	f(`in("foo bar", baz)`, `in("foo bar",baz)`)
	f(`foo:in(foo-bar/baz)`, `foo:in("foo-bar/baz")`)

	// in filter with query
	f(`in(err|fields x)`, `in(err | fields x)`)
	f(`ip:in(foo and user:in(admin, moderator)|fields ip)`, `ip:in(foo user:in(admin,moderator) | fields ip)`)
	f(`x:in(_time:5m y:in(*|fields z) | stats by (q) count() rows|fields q)`, `x:in(_time:5m y:in(* | fields z) | stats by (q) count(*) as rows | fields q)`)
	f(`in(bar:in(1,2,3) | uniq (x)) | stats count() rows`, `in(bar:in(1,2,3) | uniq by (x)) | stats count(*) as rows`)
	f(`in((1) | fields z) | stats count() rows`, `in(1 | fields z) | stats count(*) as rows`)

	// ipv4_range filter
	f(`ipv4_range(1.2.3.4, "5.6.7.8")`, `ipv4_range(1.2.3.4, 5.6.7.8)`)
	f(`foo:ipv4_range(1.2.3.4, "5.6.7.8" , )`, `foo:ipv4_range(1.2.3.4, 5.6.7.8)`)
	f(`ipv4_range(1.2.3.4)`, `ipv4_range(1.2.3.4, 1.2.3.4)`)
	f(`ipv4_range(1.2.3.4/20)`, `ipv4_range(1.2.0.0, 1.2.15.255)`)
	f(`ipv4_range(1.2.3.4,)`, `ipv4_range(1.2.3.4, 1.2.3.4)`)

	// len_range filter
	f(`len_range(10, 20)`, `len_range(10, 20)`)
	f(`foo:len_range("10", 20, )`, `foo:len_range(10, 20)`)
	f(`len_RANGe(10, inf)`, `len_range(10, inf)`)
	f(`len_range(10, +InF)`, `len_range(10, +InF)`)
	f(`len_range(10, 1_000_000)`, `len_range(10, 1_000_000)`)
	f(`len_range(0x10,0b100101)`, `len_range(0x10, 0b100101)`)
	f(`len_range(1.5KB, 22MB100KB)`, `len_range(1.5KB, 22MB100KB)`)

	// range filter
	f(`range(1.234, 5656.43454)`, `range(1.234, 5656.43454)`)
	f(`foo:range(-2343.344, 2343.4343)`, `foo:range(-2343.344, 2343.4343)`)
	f(`range(-1.234e-5  , 2.34E+3)`, `range(-1.234e-5, 2.34E+3)`)
	f(`range[123, 456)`, `range[123, 456)`)
	f(`range(123, 445]`, `range(123, 445]`)
	f(`range("1.234e-4", -23)`, `range(1.234e-4, -23)`)
	f(`range(1_000, 0o7532)`, `range(1_000, 0o7532)`)
	f(`range(0x1ff, inf)`, `range(0x1ff, inf)`)
	f(`range(-INF,+inF)`, `range(-INF, +inF)`)
	f(`range(1.5K, 22.5GiB)`, `range(1.5K, 22.5GiB)`)
	f(`foo:range(5,inf)`, `foo:range(5, inf)`)

	// >,  >=, < and <= filter
	f(`foo: > 10.5M`, `foo:>10.5M`)
	f(`foo: >= 10.5M`, `foo:>=10.5M`)
	f(`foo: < 10.5M`, `foo:<10.5M`)
	f(`foo: <= 10.5M`, `foo:<=10.5M`)
	f(`foo:(>10 !<=20)`, `foo:>10 !foo:<=20`)
	f(`>=10 !<20`, `>=10 !<20`)

	// re filter
	f("re('foo|ba(r.+)')", `~"foo|ba(r.+)"`)
	f("re(foo)", `~foo`)
	f(`foo:re(foo-bar/baz.)`, `foo:~"foo-bar/baz."`)
	f(`~foo.bar.baz !~bar`, `~foo.bar.baz !~bar`)
	f(`foo:~~foo~ba/ba>z`, `foo:~"~foo~ba/ba>z"`)
	f(`foo:~'.*'`, `foo:~".*"`)

	// seq filter
	f(`seq()`, `seq()`)
	f(`seq(foo)`, `seq(foo)`)
	f(`seq("foo, bar", baz, abc)`, `seq("foo, bar",baz,abc)`)
	f(`foo:seq(foo"bar-baz+aa, b)`, `foo:seq("foo\"bar-baz+aa",b)`)

	// string_range filter
	f(`string_range(foo, bar)`, `string_range(foo, bar)`)
	f(`foo:string_range("foo, bar", baz)`, `foo:string_range("foo, bar", baz)`)
	f(`foo:>bar`, `foo:>bar`)
	f(`foo:>"1234"`, `foo:>1234`)
	f(`>="abc"`, `>=abc`)
	f(`foo:<bar`, `foo:<bar`)
	f(`foo:<"-12.34"`, `foo:<-12.34`)
	f(`<="abc < de"`, `<="abc < de"`)

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
	f("trace-id.foo.bar:baz", `"trace-id.foo.bar":baz`)
	f(`custom-Time:2024-01-02T03:04:05+08:00    fooBar OR !baz:xxx`, `"custom-Time":"2024-01-02T03:04:05+08:00" fooBar or !baz:xxx`)
	f("foo-bar+baz*", `"foo-bar+baz"*`)
	f("foo- bar", `"foo-" bar`)
	f("foo -bar", `foo !bar`)
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
		`_time:[-1h,now] {job="foo",env=~"prod|staging"} (level:error or level:warn*) !"connection reset by peer"`)
	f(`(_time:(2023-04-20, now] or _time:[-10m, -1m))
		and (_stream:{job="a"} or _stream:{instance!="b"})
		and (err* or ip:(ipv4_range(1.2.3.0, 1.2.3.255) and not 1.2.3.4))`,
		`(_time:(2023-04-20,now] or _time:[-10m,-1m)) ({job="a"} or {instance!="b"}) (err* or ip:ipv4_range(1.2.3.0, 1.2.3.255) !ip:1.2.3.4)`)

	// fields pipe
	f(`foo|fields *`, `foo | fields *`)
	f(`foo | fields bar`, `foo | fields bar`)
	f(`foo|FIELDS bar,Baz  , "a,b|c"`, `foo | fields bar, Baz, "a,b|c"`)
	f(`foo | Fields   x.y, "abc:z/a", _b$c`, `foo | fields x.y, "abc:z/a", "_b$c"`)
	f(`foo | fields "", a`, `foo | fields _msg, a`)

	// multiple fields pipes
	f(`foo | fields bar | fields baz, abc`, `foo | fields bar | fields baz, abc`)

	// field_names pipe
	f(`foo | field_names as x`, `foo | field_names as x`)
	f(`foo | field_names y`, `foo | field_names as y`)
	f(`foo | field_names`, `foo | field_names`)

	// field_values pipe
	f(`* | field_values x`, `* | field_values x`)
	f(`* | field_values (x)`, `* | field_values x`)

	// blocks_count pipe
	f(`foo | blocks_count as x`, `foo | blocks_count as x`)
	f(`foo | blocks_count y`, `foo | blocks_count as y`)
	f(`foo | blocks_count`, `foo | blocks_count`)

	// copy and cp pipe
	f(`* | copy foo as bar`, `* | copy foo as bar`)
	f(`* | cp foo bar`, `* | copy foo as bar`)
	f(`* | COPY foo as bar, x y | Copy a as b`, `* | copy foo as bar, x as y | copy a as b`)

	// rename and mv pipe
	f(`* | rename foo as bar`, `* | rename foo as bar`)
	f(`* | mv foo bar`, `* | rename foo as bar`)
	f(`* | RENAME foo AS bar, x y | Rename a as b`, `* | rename foo as bar, x as y | rename a as b`)

	// delete, del and rm pipe
	f(`* | delete foo`, `* | delete foo`)
	f(`* | del foo`, `* | delete foo`)
	f(`* | rm foo`, `* | delete foo`)
	f(`* | DELETE foo, bar`, `* | delete foo, bar`)

	// len pipe
	f(`* | len(x)`, `* | len(x)`)
	f(`* | len(x) as _msg`, `* | len(x)`)
	f(`* | len(x) y`, `* | len(x) as y`)
	f(`* | len  ( x ) as y`, `* | len(x) as y`)
	f(`* | len x y`, `* | len(x) as y`)
	f(`* | len x as y`, `* | len(x) as y`)

	// limit and head pipe
	f(`foo | limit`, `foo | limit 10`)
	f(`foo | head`, `foo | limit 10`)
	f(`foo | limit 20`, `foo | limit 20`)
	f(`foo | head 20`, `foo | limit 20`)
	f(`foo | HEAD 1_123_432`, `foo | limit 1123432`)
	f(`foo | head 10K`, `foo | limit 10000`)

	// multiple limit pipes
	f(`foo | limit 100 | limit 10 | limit 234`, `foo | limit 100 | limit 10 | limit 234`)

	// offset and skip pipe
	f(`foo | skip 10`, `foo | offset 10`)
	f(`foo | offset 10`, `foo | offset 10`)
	f(`foo | skip 12_345M`, `foo | offset 12345000000`)

	// multiple offset pipes
	f(`foo | offset 10 | offset 100`, `foo | offset 10 | offset 100`)

	// stats pipe count
	f(`* | STATS bY (foo, b.a/r, "b az",) count(*) XYz`, `* | stats by (foo, "b.a/r", "b az") count(*) as XYz`)
	f(`* | stats by() COUNT(x, 'a).b,c|d',) as qwert`, `* | stats count(x, "a).b,c|d") as qwert`)
	f(`* | stats count() x`, `* | stats count(*) as x`)
	f(`* | stats count(*) x`, `* | stats count(*) as x`)
	f(`* | stats count(foo,*,bar) x`, `* | stats count(*) as x`)
	f(`* | stats count('') foo`, `* | stats count(_msg) as foo`)
	f(`* | stats count(foo) ''`, `* | stats count(foo) as _msg`)
	f(`* | count()`, `* | stats count(*) as "count(*)"`)
	f(`* | count(), count() if (foo)`, `* | stats count(*) as "count(*)", count(*) if (foo) as "count(*) if (foo)"`)

	// stats pipe count_empty
	f(`* | stats count_empty() x`, `* | stats count_empty(*) as x`)
	f(`* | stats by (x, y) count_empty(a,b,c) z`, `* | stats by (x, y) count_empty(a, b, c) as z`)
	f(`* | count_empty()`, `* | stats count_empty(*) as "count_empty(*)"`)

	// stats pipe sum
	f(`* | stats Sum(foo) bar`, `* | stats sum(foo) as bar`)
	f(`* | stats BY(x, y, ) SUM(foo,bar,) bar`, `* | stats by (x, y) sum(foo, bar) as bar`)
	f(`* | stats sum() x`, `* | stats sum(*) as x`)
	f(`* | sum()`, `* | stats sum(*) as "sum(*)"`)
	f(`* | stats sum(*) x`, `* | stats sum(*) as x`)
	f(`* | stats sum(foo,*,bar) x`, `* | stats sum(*) as x`)

	// stats pipe max
	f(`* | stats Max(foo) bar`, `* | stats max(foo) as bar`)
	f(`* | stats BY(x, y, ) MAX(foo,bar,) bar`, `* | stats by (x, y) max(foo, bar) as bar`)
	f(`* | stats max() x`, `* | stats max(*) as x`)
	f(`* | max()`, `* | stats max(*) as "max(*)"`)
	f(`* | stats max(*) x`, `* | stats max(*) as x`)
	f(`* | stats max(foo,*,bar) x`, `* | stats max(*) as x`)

	// stats pipe min
	f(`* | stats Min(foo) bar`, `* | stats min(foo) as bar`)
	f(`* | stats BY(x, y, ) MIN(foo,bar,) bar`, `* | stats by (x, y) min(foo, bar) as bar`)
	f(`* | stats min() x`, `* | stats min(*) as x`)
	f(`* | min()`, `* | stats min(*) as "min(*)"`)
	f(`* | stats min(*) x`, `* | stats min(*) as x`)
	f(`* | stats min(foo,*,bar) x`, `* | stats min(*) as x`)

	// stats pipe row_min
	f(`* | stats row_Min(foo) bar`, `* | stats row_min(foo) as bar`)
	f(`* | row_Min(foo)`, `* | stats row_min(foo) as "row_min(foo)"`)
	f(`* | stats BY(x, y, ) row_MIN(foo,bar,) bar`, `* | stats by (x, y) row_min(foo, bar) as bar`)

	// stats pipe avg
	f(`* | stats Avg(foo) bar`, `* | stats avg(foo) as bar`)
	f(`* | stats BY(x, y, ) AVG(foo,bar,) bar`, `* | stats by (x, y) avg(foo, bar) as bar`)
	f(`* | stats avg() x`, `* | stats avg(*) as x`)
	f(`* | avg()`, `* | stats avg(*) as "avg(*)"`)
	f(`* | stats avg(*) x`, `* | stats avg(*) as x`)
	f(`* | stats avg(foo,*,bar) x`, `* | stats avg(*) as x`)

	// stats pipe count_uniq
	f(`* | stats count_uniq(foo) bar`, `* | stats count_uniq(foo) as bar`)
	f(`* | count_uniq(foo)`, `* | stats count_uniq(foo) as "count_uniq(foo)"`)
	f(`* | stats by(x, y) count_uniq(foo,bar) LiMit 10 As baz`, `* | stats by (x, y) count_uniq(foo, bar) limit 10 as baz`)
	f(`* | stats by(x) count_uniq(*) z`, `* | stats by (x) count_uniq(*) as z`)
	f(`* | stats by(x) count_uniq() z`, `* | stats by (x) count_uniq(*) as z`)
	f(`* | stats by(x) count_uniq(a,*,b) z`, `* | stats by (x) count_uniq(*) as z`)

	// stats pipe uniq_values
	f(`* | stats uniq_values(foo) bar`, `* | stats uniq_values(foo) as bar`)
	f(`* | uniq_values(foo)`, `* | stats uniq_values(foo) as "uniq_values(foo)"`)
	f(`* | stats uniq_values(foo) limit 10 bar`, `* | stats uniq_values(foo) limit 10 as bar`)
	f(`* | stats by(x, y) uniq_values(foo, bar) as baz`, `* | stats by (x, y) uniq_values(foo, bar) as baz`)
	f(`* | stats by(x) uniq_values(*) y`, `* | stats by (x) uniq_values(*) as y`)
	f(`* | stats by(x) uniq_values() limit 1_000 AS y`, `* | stats by (x) uniq_values(*) limit 1000 as y`)
	f(`* | stats by(x) uniq_values(a,*,b) y`, `* | stats by (x) uniq_values(*) as y`)

	// stats pipe values
	f(`* | stats values(foo) bar`, `* | stats values(foo) as bar`)
	f(`* | values(foo)`, `* | stats values(foo) as "values(foo)"`)
	f(`* | stats values(foo) limit 10 bar`, `* | stats values(foo) limit 10 as bar`)
	f(`* | stats by(x, y) values(foo, bar) as baz`, `* | stats by (x, y) values(foo, bar) as baz`)
	f(`* | stats by(x) values(*) y`, `* | stats by (x) values(*) as y`)
	f(`* | stats by(x) values() limit 1_000 AS y`, `* | stats by (x) values(*) limit 1000 as y`)
	f(`* | stats by(x) values(a,*,b) y`, `* | stats by (x) values(*) as y`)

	// stats pipe sum_len
	f(`* | stats Sum_len(foo) bar`, `* | stats sum_len(foo) as bar`)
	f(`* | stats BY(x, y, ) SUM_Len(foo,bar,) bar`, `* | stats by (x, y) sum_len(foo, bar) as bar`)
	f(`* | stats sum_len() x`, `* | stats sum_len(*) as x`)
	f(`* | sum_len()`, `* | stats sum_len(*) as "sum_len(*)"`)
	f(`* | stats sum_len(*) x`, `* | stats sum_len(*) as x`)
	f(`* | stats sum_len(foo,*,bar) x`, `* | stats sum_len(*) as x`)

	// stats pipe quantile
	f(`* | stats quantile(0, foo) bar`, `* | stats quantile(0, foo) as bar`)
	f(`* | stats quantile(1, foo) bar`, `* | stats quantile(1, foo) as bar`)
	f(`* | stats quantile(0.5, a, b, c) bar`, `* | stats quantile(0.5, a, b, c) as bar`)
	f(`* | stats quantile(0.99) bar`, `* | stats quantile(0.99) as bar`)
	f(`* | quantile(0.99)`, `* | stats quantile(0.99) as "quantile(0.99)"`)
	f(`* | stats quantile(0.99, a, *, b) bar`, `* | stats quantile(0.99) as bar`)

	// stats pipe median
	f(`* | stats Median(foo) bar`, `* | stats median(foo) as bar`)
	f(`* | stats BY(x, y, ) MEDIAN(foo,bar,) bar`, `* | stats by (x, y) median(foo, bar) as bar`)
	f(`* | stats median() x`, `* | stats median(*) as x`)
	f(`* | median()`, `* | stats median(*) as "median(*)"`)
	f(`* | stats median(*) x`, `* | stats median(*) as x`)
	f(`* | stats median(foo,*,bar) x`, `* | stats median(*) as x`)

	// stats pipe multiple funcs
	f(`* | stats count() "foo.bar:baz", count_uniq(a) bar`, `* | stats count(*) as "foo.bar:baz", count_uniq(a) as bar`)
	f(`* | stats by (x, y) count(*) foo, count_uniq(a,b) bar`, `* | stats by (x, y) count(*) as foo, count_uniq(a, b) as bar`)

	// stats pipe with grouping buckets
	f(`* | stats by(_time:1d, response_size:1_000KiB, request_duration:5s, foo) count() as bar`, `* | stats by (_time:1d, response_size:1_000KiB, request_duration:5s, foo) count(*) as bar`)
	f(`*|stats by(client_ip:/24, server_ip:/16) count() foo`, `* | stats by (client_ip:/24, server_ip:/16) count(*) as foo`)
	f(`* | stats by(_time:1d offset 2h) count() as foo`, `* | stats by (_time:1d offset 2h) count(*) as foo`)
	f(`* | stats by(_time:1d offset -2.5h5m) count() as foo`, `* | stats by (_time:1d offset -2.5h5m) count(*) as foo`)
	f(`* | stats by (_time:nanosecond) count() foo`, `* | stats by (_time:nanosecond) count(*) as foo`)
	f(`* | stats by (_time:microsecond) count() foo`, `* | stats by (_time:microsecond) count(*) as foo`)
	f(`* | stats by (_time:millisecond) count() foo`, `* | stats by (_time:millisecond) count(*) as foo`)
	f(`* | stats by (_time:second) count() foo`, `* | stats by (_time:second) count(*) as foo`)
	f(`* | stats by (_time:minute) count() foo`, `* | stats by (_time:minute) count(*) as foo`)
	f(`* | stats by (_time:hour) count() foo`, `* | stats by (_time:hour) count(*) as foo`)
	f(`* | stats by (_time:day) count() foo`, `* | stats by (_time:day) count(*) as foo`)
	f(`* | stats by (_time:week) count() foo`, `* | stats by (_time:week) count(*) as foo`)
	f(`* | stats by (_time:month) count() foo`, `* | stats by (_time:month) count(*) as foo`)
	f(`* | stats by (_time:year offset 6.5h) count() foo`, `* | stats by (_time:year offset 6.5h) count(*) as foo`)
	f(`* | stats (_time:year offset 6.5h) count() foo`, `* | stats by (_time:year offset 6.5h) count(*) as foo`)

	// stats pipe with per-func filters
	f(`* | stats count() if (foo bar) rows`, `* | stats count(*) if (foo bar) as rows`)
	f(`* | stats by (_time:1d offset -2h, f2)
	   count() if (is_admin:true or _msg:"foo bar"*) as foo,
	   sum(duration) if (host:in('foo.com', 'bar.com') and path:/foobar) as bar`,
		`* | stats by (_time:1d offset -2h, f2) count(*) if (is_admin:true or "foo bar"*) as foo, sum(duration) if (host:in(foo.com,bar.com) path:"/foobar") as bar`)
	f(`* | stats count(x) if (error ip:in(_time:1d | fields ip)) rows`, `* | stats count(x) if (error ip:in(_time:1d | fields ip)) as rows`)
	f(`* | stats count() if () rows`, `* | stats count(*) if (*) as rows`)

	// sort pipe
	f(`* | sort`, `* | sort`)
	f(`* | order`, `* | sort`)
	f(`* | sort desc`, `* | sort desc`)
	f(`* | sort by()`, `* | sort`)
	f(`* | sort bY (foo)`, `* | sort by (foo)`)
	f(`* | ORDer bY (foo)`, `* | sort by (foo)`)
	f(`* | sORt bY (_time, _stream DEsc, host)`, `* | sort by (_time, _stream desc, host)`)
	f(`* | sort bY (foo desc, bar,) desc`, `* | sort by (foo desc, bar) desc`)
	f(`* | sort limit 10`, `* | sort limit 10`)
	f(`* | sort offset 20 limit 10`, `* | sort offset 20 limit 10`)
	f(`* | sort desc limit 10`, `* | sort desc limit 10`)
	f(`* | sort desc offset 20 limit 10`, `* | sort desc offset 20 limit 10`)
	f(`* | sort by (foo desc, bar) limit 10`, `* | sort by (foo desc, bar) limit 10`)
	f(`* | sort by (foo desc, bar) oFFset 20 limit 10`, `* | sort by (foo desc, bar) offset 20 limit 10`)
	f(`* | sort by (foo desc, bar) desc limit 10`, `* | sort by (foo desc, bar) desc limit 10`)
	f(`* | sort by (foo desc, bar) desc OFFSET 30 limit 10`, `* | sort by (foo desc, bar) desc offset 30 limit 10`)
	f(`* | sort by (foo desc, bar) desc limit 10 OFFSET 30`, `* | sort by (foo desc, bar) desc offset 30 limit 10`)
	f(`* | sort (foo desc, bar) desc limit 10 OFFSET 30`, `* | sort by (foo desc, bar) desc offset 30 limit 10`)

	// uniq pipe
	f(`* | uniq`, `* | uniq`)
	f(`* | uniq by()`, `* | uniq`)
	f(`* | uniq by(*)`, `* | uniq`)
	f(`* | uniq by(foo,*,bar)`, `* | uniq`)
	f(`* | uniq by(f1,f2)`, `* | uniq by (f1, f2)`)
	f(`* | uniq by(f1,f2) limit 10`, `* | uniq by (f1, f2) limit 10`)
	f(`* | uniq (f1,f2) limit 10`, `* | uniq by (f1, f2) limit 10`)
	f(`* | uniq limit 10`, `* | uniq limit 10`)

	// filter pipe
	f(`* | filter error ip:12.3.4.5 or warn`, `* | filter error ip:12.3.4.5 or warn`)
	f(`foo | stats by (host) count() logs | filter logs:>50 | sort by (logs desc) | limit 10`, `foo | stats by (host) count(*) as logs | filter logs:>50 | sort by (logs desc) | limit 10`)
	f(`* | error`, `* | filter error`)
	f(`* | "by"`, `* | filter "by"`)
	f(`* | "stats"`, `* | filter "stats"`)
	f(`* | "count"`, `* | filter "count"`)
	f(`* | foo:bar AND baz:<10`, `* | filter foo:bar baz:<10`)

	// extract pipe
	f(`* | extract "foo<bar>baz"`, `* | extract "foo<bar>baz"`)
	f(`* | extract "foo<bar>baz" from _msg`, `* | extract "foo<bar>baz"`)
	f(`* | extract 'foo<bar>baz' from ''`, `* | extract "foo<bar>baz"`)
	f("* | extract `foo<bar>baz` from x", `* | extract "foo<bar>baz" from x`)
	f("* | extract foo<bar>baz from x", `* | extract "foo<bar>baz" from x`)
	f("* | extract if (a:b) foo<bar>baz from x", `* | extract if (a:b) "foo<bar>baz" from x`)

	// unpack_json pipe
	f(`* | unpack_json`, `* | unpack_json`)
	f(`* | unpack_json result_prefix y`, `* | unpack_json result_prefix y`)
	f(`* | unpack_json from x`, `* | unpack_json from x`)
	f(`* | unpack_json from x result_prefix y`, `* | unpack_json from x result_prefix y`)

	// unpack_logfmt pipe
	f(`* | unpack_logfmt`, `* | unpack_logfmt`)
	f(`* | unpack_logfmt result_prefix y`, `* | unpack_logfmt result_prefix y`)
	f(`* | unpack_logfmt from x`, `* | unpack_logfmt from x`)
	f(`* | unpack_logfmt from x result_prefix y`, `* | unpack_logfmt from x result_prefix y`)

	// multiple different pipes
	f(`* | fields foo, bar | limit 100 | stats by(foo,bar) count(baz) as qwert`, `* | fields foo, bar | limit 100 | stats by (foo, bar) count(baz) as qwert`)
	f(`* | skip 100 | head 20 | skip 10`, `* | offset 100 | limit 20 | offset 10`)

	// comments
	f(`* # some comment | foo bar`, `*`)
	f(`foo | # some comment | foo bar
	  fields x # another comment
	  |filter "foo#this#isn't a comment"#this is comment`, `foo | fields x | filter "foo#this#isn't a comment"`)

	// skip 'stats' and 'filter' prefixes
	f(`* | by (host) count() rows | rows:>10`, `* | stats by (host) count(*) as rows | filter rows:>10`)
	f(`* | (host) count() rows, count() if (error) errors | rows:>10`, `* | stats by (host) count(*) as rows, count(*) if (error) as errors | filter rows:>10`)
}

func TestParseQueryFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		q, err := ParseQuery(s)
		if q != nil {
			t.Fatalf("expecting nil result; got [%s]", q)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("")
	f("|")
	f("foo|")
	f("foo|bar(")
	f("foo and")
	f("foo OR ")
	f("not")
	f("NOT")
	f("not (abc")
	f("!")

	// pipe names without quoutes
	f(`filter foo:bar`)
	f(`stats count()`)
	f(`count()`)

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

	// invalid _stream_id filters
	f("_stream_id:foo")
	f("_stream_id:()")
	f("_stream_id:in(foo)")
	f("_stream_id:in(foo | bar)")
	f("_stream_id:in(* | stats by (x) count() y)")

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

	// invalid _stream filters without _stream: prefix
	f("{")
	f(`{foo`)
	f(`{foo}`)
	f(`{foo=`)
	f(`{foo=}`)
	f(`{foo="bar`)
	f(`{foo='bar`)
	f(`{foo="bar}`)
	f(`{foo='bar}`)

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

	// invalid day_range filters
	f("_time:day_range")
	f("_time:day_range[")
	f("_time:day_range[foo")
	f("_time:day_range[00:00,")
	f("_time:day_range[00:00,bar")
	f("_time:day_range[00:00,08:00")
	f("_time:day_range[00:00,08:00] offset")

	// invalid week_range filters
	f("_time:week_range")
	f("_time:week_range[")
	f("_time:week_range[foo")
	f("_time:week_range[Mon,")
	f("_time:week_range[Mon,bar")
	f("_time:week_range[Mon,Fri")
	f("_time:week_range[Mon,Fri] offset")

	// long query with error
	f(`very long query with error aaa ffdfd fdfdfd fdfd:( ffdfdfdfdfd`)

	// query with unexpected tail
	f(`foo | bar(`)

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
	f(`in(foo|bar)`)
	f(`in(|foo`)
	f(`in(x | limit 10)`)
	f(`in(x | fields a,b)`)

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
	f(`>(`)

	// missing filter
	f(`| fields *`)

	// missing pipe keyword
	f(`foo |`)

	// invlaid pipe
	f(`foo | bar(`)
	f(`foo | fields bar | baz(`)

	// missing field in fields pipe
	f(`foo | fields`)
	f(`foo | fields ,`)
	f(`foo | fields bar,`)
	f(`foo | fields bar,,`)

	// invalid field_names
	f(`foo | field_names |`)
	f(`foo | field_names (`)
	f(`foo | field_names )`)
	f(`foo | field_names ,`)
	f(`foo | field_names ()`)
	f(`foo | field_names (x)`)
	f(`foo | field_names (x,y)`)
	f(`foo | field_names x y`)
	f(`foo | field_names x, y`)

	// invalid blocks_count
	f(`foo | blocks_count |`)
	f(`foo | blocks_count (`)
	f(`foo | blocks_count )`)
	f(`foo | blocks_count ,`)
	f(`foo | blocks_count ()`)
	f(`foo | blocks_count (x)`)
	f(`foo | blocks_count (x,y)`)
	f(`foo | blocks_count x y`)
	f(`foo | blocks_count x, y`)

	// invalid copy and cp pipe
	f(`foo | copy`)
	f(`foo | cp`)
	f(`foo | copy foo`)
	f(`foo | copy foo,`)
	f(`foo | copy foo,,`)

	// invalid rename and mv pipe
	f(`foo | rename`)
	f(`foo | mv`)
	f(`foo | rename foo`)
	f(`foo | rename foo,`)
	f(`foo | rename foo,,`)

	// invalid delete pipe
	f(`foo | delete`)
	f(`foo | del`)
	f(`foo | rm`)
	f(`foo | delete foo,`)
	f(`foo | delete foo,,`)

	// invalid len pipe
	f(`foo | len`)
	f(`foo | len(`)
	f(`foo | len()`)
	f(`foo | len (x) y z`)

	// invalid limit pipe value
	f(`foo | limit bar`)
	f(`foo | limit -123`)

	// missing offset and skip pipe value
	f(`foo | offset`)
	f(`foo | skip`)

	// invalid offset pipe value
	f(`foo | offset bar`)
	f(`foo | offset -10`)

	// missing stats
	f(`foo | stats`)

	// invalid stats
	f(`foo | stats bar`)

	// invalid stats count
	f(`foo | stats count`)
	f(`foo | stats count(`)
	f(`foo | stats count bar`)
	f(`foo | stats count(bar`)
	f(`foo | stats count() as`)
	f(`foo | stats count() as |`)

	// invalid stats count_empty
	f(`foo | stats count_empty`)
	f(`foo | stats count_empty() as`)
	f(`foo | stats count_empty() as |`)

	// invalid stats sum
	f(`foo | stats sum`)

	// invalid stats max
	f(`foo | stats max`)

	// invalid stats min
	f(`foo | stats min`)

	// invalid stats min
	f(`foo | stats row_min`)

	// invalid stats avg
	f(`foo | stats avg`)

	// invalid stats count_uniq
	f(`foo | stats count_uniq`)
	f(`foo | stats count_uniq() limit`)
	f(`foo | stats count_uniq() limit foo`)
	f(`foo | stats count_uniq() limit 0.5`)
	f(`foo | stats count_uniq() limit -1`)

	// invalid stats uniq_values
	f(`foo | stats uniq_values`)
	f(`foo | stats uniq_values() limit`)
	f(`foo | stats uniq_values(a) limit foo`)
	f(`foo | stats uniq_values(a) limit 0.5`)
	f(`foo | stats uniq_values(a) limit -1`)

	// invalid stats values
	f(`foo | stats values`)
	f(`foo | stats values() limit`)
	f(`foo | stats values(a) limit foo`)
	f(`foo | stats values(a) limit 0.5`)
	f(`foo | stats values(a) limit -1`)

	// invalid stats sum_len
	f(`foo | stats sum_len`)

	// invalid stats quantile
	f(`foo | stats quantile`)
	f(`foo | stats quantile() foo`)
	f(`foo | stats quantile(bar, baz) foo`)
	f(`foo | stats quantile(-1, x) foo`)
	f(`foo | stats quantile(10, x) foo`)

	// invalid stats grouping fields
	f(`foo | stats by(foo:bar) count() baz`)
	f(`foo | stats by(foo:/bar) count() baz`)
	f(`foo | stats by(foo:-1h) count() baz`)
	f(`foo | stats by (foo:1h offset) count() baz`)
	f(`foo | stats by (foo:1h offset bar) count() baz`)

	// invalid stats by clause
	f(`foo | stats by`)
	f(`foo | stats by bar`)
	f(`foo | stats by(`)
	f(`foo | stats by(bar`)
	f(`foo | stats by(bar,`)
	f(`foo | stats by(bar)`)

	// duplicate stats result names
	f(`foo | stats min() x, max() x`)

	// stats result names identical to by fields
	f(`foo | stats by (x) count() x`)

	// missing stats function
	f(`foo | by (bar)`)

	// invalid sort pipe
	f(`foo | sort bar`)
	f(`foo | sort by`)
	f(`foo | sort by(`)
	f(`foo | sort by(baz`)
	f(`foo | sort by(baz,`)
	f(`foo | sort by(bar) foo`)
	f(`foo | sort by(bar) limit`)
	f(`foo | sort by(bar) limit foo`)
	f(`foo | sort by(bar) limit -1234`)
	f(`foo | sort by(bar) limit 12.34`)
	f(`foo | sort by(bar) limit 10 limit 20`)
	f(`foo | sort by(bar) offset`)
	f(`foo | sort by(bar) offset limit`)
	f(`foo | sort by(bar) offset -1234`)
	f(`foo | sort by(bar) offset 12.34`)
	f(`foo | sort by(bar) offset 10 offset 20`)

	// invalid uniq pipe
	f(`foo | uniq bar`)
	f(`foo | uniq limit`)
	f(`foo | uniq by(`)
	f(`foo | uniq by(a`)
	f(`foo | uniq by(a,`)
	f(`foo | uniq by(a) bar`)
	f(`foo | uniq by(a) limit -10`)
	f(`foo | uniq by(a) limit foo`)

	// invalid filter pipe
	f(`foo | filter`)
	f(`foo | filter | sort by (x)`)
	f(`foo | filter (`)
	f(`foo | filter )`)

	f(`foo | filter stats`)
	f(`foo | filter fields`)
	f(`foo | filter by`)
	f(`foo | count`)
	f(`foo | filter count`)
	f(`foo | (`)
	f(`foo | )`)

	// invalid extract pipe
	f(`foo | extract`)
	f(`foo | extract bar`)
	f(`foo | extract "xy"`)
	f(`foo | extract "<>"`)
	f(`foo | extract "foo<>foo"`)
	f(`foo | extract "foo<>foo<_>bar<*>asdf"`)
	f(`foo | extract from`)
	f(`foo | extract from x`)
	f(`foo | extract from x "abc"`)
	f(`foo | extract from x "<abc`)
	f(`foo | extract from x "<abc>" de`)

	// invalid unpack_json pipe
	f(`foo | unpack_json bar`)
	f(`foo | unpack_json from`)
	f(`foo | unpack_json result_prefix`)
	f(`foo | unpack_json result_prefix x from y`)
	f(`foo | unpack_json from x result_prefix`)

	// invalid unpack_logfmt pipe
	f(`foo | unpack_logfmt bar`)
	f(`foo | unpack_logfmt from`)
	f(`foo | unpack_logfmt result_prefix`)
	f(`foo | unpack_logfmt result_prefix x from y`)
	f(`foo | unpack_logfmt from x result_prefix`)
}

func TestQueryGetNeededColumns(t *testing.T) {
	f := func(s, neededColumnsExpected, unneededColumnsExpected string) {
		t.Helper()

		q, err := ParseQuery(s)
		if err != nil {
			t.Fatalf("cannot parse query [%s]: %s", s, err)
		}
		q.Optimize()

		needed, unneeded := q.getNeededColumns()
		neededColumns := strings.Join(needed, ",")
		unneededColumns := strings.Join(unneeded, ",")

		if neededColumns != neededColumnsExpected {
			t.Fatalf("unexpected neededColumns for [%s]; got %q; want %q", s, neededColumns, neededColumnsExpected)
		}
		if unneededColumns != unneededColumnsExpected {
			t.Fatalf("unexpected unneededColumns for [%s]; got %q; want %q", s, unneededColumns, unneededColumnsExpected)
		}
	}

	f(`*`, `*`, ``)
	f(`foo bar`, `*`, ``)
	f(`foo:bar _time:5m baz`, `*`, ``)

	f(`* | fields *`, `*`, ``)
	f(`* | fields * | offset 10`, `*`, ``)
	f(`* | fields * | offset 10 | limit 20`, `*`, ``)
	f(`* | fields foo`, `foo`, ``)
	f(`* | fields foo, bar`, `bar,foo`, ``)
	f(`* | fields foo, bar | fields baz, bar`, `bar`, ``)
	f(`* | fields foo, bar | fields baz, a`, ``, ``)
	f(`* | fields f1, f2 | rm f3, f4`, `f1,f2`, ``)
	f(`* | fields f1, f2 | rm f2, f3`, `f1`, ``)
	f(`* | fields f1, f2 | rm f1, f2, f3`, ``, ``)
	f(`* | fields f1, f2 | cp f1 f2, f3 f4`, `f1`, ``)
	f(`* | fields f1, f2 | cp f1 f3, f4 f5`, `f1,f2`, ``)
	f(`* | fields f1, f2 | cp f2 f3, f4 f5`, `f1,f2`, ``)
	f(`* | fields f1, f2 | cp f2 f3, f4 f1`, `f2`, ``)
	f(`* | fields f1, f2 | mv f1 f2, f3 f4`, `f1`, ``)
	f(`* | fields f1, f2 | mv f1 f3, f4 f5`, `f1,f2`, ``)
	f(`* | fields f1, f2 | mv f2 f3, f4 f5`, `f1,f2`, ``)
	f(`* | fields f1, f2 | mv f2 f3, f4 f1`, `f2`, ``)
	f(`* | fields f1, f2 | stats count_uniq() r1`, `f1,f2`, ``)
	f(`* | fields f1, f2 | stats count(f1) r1`, `f1`, ``)
	f(`* | fields f1, f2 | stats count(f1,f2,f3) r1`, `f1,f2`, ``)
	f(`* | fields f1, f2 | stats by(b1) count() r1`, ``, ``)
	f(`* | fields f1, f2 | stats by(b1,f1) count() r1`, `f1`, ``)
	f(`* | fields f1, f2 | stats by(b1,f1) count(f1) r1`, `f1`, ``)
	f(`* | fields f1, f2 | stats by(b1,f1) count(f1,f2,f3) r1`, `f1,f2`, ``)
	f(`* | fields f1, f2 | sort by(f3)`, `f1,f2`, ``)
	f(`* | fields f1, f2 | sort by(f1,f3)`, `f1,f2`, ``)
	f(`* | fields f1, f2 | sort by(f3) | stats count() r1`, ``, ``)
	f(`* | fields f1, f2 | sort by(f1) | stats count() r1`, ``, ``)
	f(`* | fields f1, f2 | sort by(f1) | stats count(f2,f3) r1`, `f1,f2`, ``)
	f(`* | fields f1, f2 | sort by(f3) | fields f2`, `f2`, ``)
	f(`* | fields f1, f2 | sort by(f1,f3) | fields f2`, `f1,f2`, ``)

	f(`* | cp foo bar`, `*`, `bar`)
	f(`* | cp foo bar, baz a`, `*`, `a,bar`)
	f(`* | cp foo bar, baz a | fields foo,a,b`, `b,baz,foo`, ``)
	f(`* | cp foo bar, baz a | fields bar,a,b`, `b,baz,foo`, ``)
	f(`* | cp foo bar, baz a | fields baz,a,b`, `b,baz`, ``)
	f(`* | cp foo bar | fields bar,a`, `a,foo`, ``)
	f(`* | cp foo bar | fields baz,a`, `a,baz`, ``)
	f(`* | cp foo bar | fields foo,a`, `a,foo`, ``)
	f(`* | cp f1 f2 | rm f1`, `*`, `f2`)
	f(`* | cp f1 f2 | rm f2`, `*`, `f2`)
	f(`* | cp f1 f2 | rm f3`, `*`, `f2,f3`)

	f(`* | mv foo bar`, `*`, `bar`)
	f(`* | mv foo bar, baz a`, `*`, `a,bar`)
	f(`* | mv foo bar, baz a | fields foo,a,b`, `b,baz`, ``)
	f(`* | mv foo bar, baz a | fields bar,a,b`, `b,baz,foo`, ``)
	f(`* | mv foo bar, baz a | fields baz,a,b`, `b,baz`, ``)
	f(`* | mv foo bar, baz a | fields baz,foo,b`, `b`, ``)
	f(`* | mv foo bar | fields bar,a`, `a,foo`, ``)
	f(`* | mv foo bar | fields baz,a`, `a,baz`, ``)
	f(`* | mv foo bar | fields foo,a`, `a`, ``)
	f(`* | mv f1 f2 | rm f1`, `*`, `f2`)
	f(`* | mv f1 f2 | rm f2,f3`, `*`, `f1,f2,f3`)
	f(`* | mv f1 f2 | rm f3`, `*`, `f2,f3`)

	f(`* | sort by (f1)`, `*`, ``)
	f(`* | sort by (f1) | fields f2`, `f1,f2`, ``)
	f(`_time:5m | sort by (_time) | fields foo`, `_time,foo`, ``)
	f(`* | sort by (f1) | fields *`, `*`, ``)
	f(`* | sort by (f1) | sort by (f2,f3 desc) desc`, `*`, ``)
	f(`* | sort by (f1) | sort by (f2,f3 desc) desc | fields f4`, `f1,f2,f3,f4`, ``)
	f(`* | sort by (f1) | sort by (f2,f3 desc) desc | fields f4 | rm f1,f2,f5`, `f1,f2,f3,f4`, ``)

	f(`* | stats by(f1) count(f2) r1, count(f3,f4) r2`, `f1,f2,f3,f4`, ``)
	f(`* | stats by(f1) count(f2) r1, count(f3,f4) r2 | fields f5,f6`, `f1`, ``)
	f(`* | stats by(f1) count(f2) r1, count(f3,f4) r2 | fields f1,f5`, `f1`, ``)
	f(`* | stats by(f1) count(f2) r1, count(f3,f4) r2 | fields r1`, `f1,f2`, ``)
	f(`* | stats by(f1) count(f2) r1, count(f3,f4) r2 | fields r2,r3`, `f1,f3,f4`, ``)
	f(`* | stats count(f1) r1 | stats count() r1`, ``, ``)
	f(`* | stats count(f1) r1 | stats count() r2`, ``, ``)
	f(`* | stats count(f1) r1 | stats count(r1) r2`, `f1`, ``)
	f(`* | stats count(f1) r1 | stats count(f1) r2`, ``, ``)
	f(`* | stats count(f1) r1 | stats count(f1,r1) r1`, `f1`, ``)
	f(`* | stats count(f1,f2) r1 | stats count(f2) r1, count(r1) r2`, `f1,f2`, ``)
	f(`* | stats count(f1,f2) r1 | stats count(f2) r1, count(r1) r2 | fields r1`, ``, ``)
	f(`* | stats count(f1,f2) r1 | stats count(f2) r1, count(r1) r2 | fields r2`, `f1,f2`, ``)
	f(`* | stats by(f3,f4) count(f1,f2) r1 | stats count(f2) r1, count(r1) r2 | fields r2`, `f1,f2,f3,f4`, ``)
	f(`* | stats by(f3,f4) count(f1,f2) r1 | stats count(f3) r1, count(r1) r2 | fields r1`, `f3,f4`, ``)

	f(`* | stats avg() q`, `*`, ``)
	f(`* | stats avg(*) q`, `*`, ``)
	f(`* | stats avg(x) q`, `x`, ``)
	f(`* | stats count_empty() q`, `*`, ``)
	f(`* | stats count_empty(*) q`, `*`, ``)
	f(`* | stats count_empty(x) q`, `x`, ``)
	f(`* | stats count() q`, ``, ``)
	f(`* | stats count(*) q`, ``, ``)
	f(`* | stats count(x) q`, `x`, ``)
	f(`* | stats count_uniq() q`, `*`, ``)
	f(`* | stats count_uniq(*) q`, `*`, ``)
	f(`* | stats count_uniq(x) q`, `x`, ``)
	f(`* | stats row_max(a) q`, `*`, ``)
	f(`* | stats row_max(a, *) q`, `*`, ``)
	f(`* | stats row_max(a, x) q`, `a,x`, ``)
	f(`* | stats row_min(a) q`, `*`, ``)
	f(`* | stats row_min(a, *) q`, `*`, ``)
	f(`* | stats row_min(a, x) q`, `a,x`, ``)
	f(`* | stats min() q`, `*`, ``)
	f(`* | stats min(*) q`, `*`, ``)
	f(`* | stats min(x) q`, `x`, ``)
	f(`* | stats median() q`, `*`, ``)
	f(`* | stats median(*) q`, `*`, ``)
	f(`* | stats median(x) q`, `x`, ``)
	f(`* | stats max() q`, `*`, ``)
	f(`* | stats max(*) q`, `*`, ``)
	f(`* | stats max(x) q`, `x`, ``)
	f(`* | stats quantile(0.5) q`, `*`, ``)
	f(`* | stats quantile(0.5, *) q`, `*`, ``)
	f(`* | stats quantile(0.5, x) q`, `x`, ``)
	f(`* | stats sum() q`, `*`, ``)
	f(`* | stats sum(*) q`, `*`, ``)
	f(`* | stats sum(x) q`, `x`, ``)
	f(`* | stats sum_len() q`, `*`, ``)
	f(`* | stats sum_len(*) q`, `*`, ``)
	f(`* | stats sum_len(x) q`, `x`, ``)
	f(`* | stats uniq_values() q`, `*`, ``)
	f(`* | stats uniq_values(*) q`, `*`, ``)
	f(`* | stats uniq_values(x) q`, `x`, ``)
	f(`* | stats values() q`, `*`, ``)
	f(`* | stats values(*) q`, `*`, ``)
	f(`* | stats values(x) q`, `x`, ``)

	f(`_time:5m | stats by(_time:day) count() r1 | stats values(_time) r2`, `_time`, ``)
	f(`_time:1y | stats (_time:1w) count() r1 | stats count() r2`, `_time`, ``)

	f(`* | uniq`, `*`, ``)
	f(`* | uniq by (f1,f2)`, `f1,f2`, ``)
	f(`* | uniq by (f1,f2) | fields f1,f3`, `f1,f2`, ``)
	f(`* | uniq by (f1,f2) | rm f1,f3`, `f1,f2`, ``)
	f(`* | uniq by (f1,f2) | fields f3`, `f1,f2`, ``)

	f(`* | filter foo f1:bar`, `*`, ``)
	f(`* | filter foo f1:bar | fields f2`, `f2`, ``)
	f(`* | limit 10 | filter foo f1:bar | fields f2`, `_msg,f1,f2`, ``)
	f(`* | filter foo f1:bar | fields f1`, `f1`, ``)
	f(`* | filter foo f1:bar | rm f1`, `*`, `f1`)
	f(`* | limit 10 | filter foo f1:bar | rm f1`, `*`, ``)
	f(`* | filter foo f1:bar | rm f2`, `*`, `f2`)
	f(`* | limit 10 | filter foo f1:bar | rm f2`, `*`, `f2`)
	f(`* | fields x | filter foo f1:bar | rm f2`, `x`, ``)
	f(`* | fields x,f1 | filter foo f1:bar | rm f2`, `f1,x`, ``)
	f(`* | rm x,f1 | filter foo f1:bar`, `*`, `f1,x`)

	f(`* | field_names as foo`, `*`, `_time`)
	f(`* | field_names foo | fields bar`, `*`, `_time`)
	f(`* | field_names foo | fields foo`, `*`, `_time`)
	f(`* | field_names foo | rm foo`, `*`, `_time`)
	f(`* | field_names foo | rm bar`, `*`, `_time`)
	f(`* | field_names foo | rm _time`, `*`, `_time`)
	f(`* | fields x,y | field_names as bar | fields baz`, `x,y`, ``)
	f(`* | rm x,y | field_names as bar | fields baz`, `*`, `x,y`)

	f(`* | blocks_count as foo`, ``, ``)
	f(`* | blocks_count foo | fields bar`, ``, ``)
	f(`* | blocks_count foo | fields foo`, ``, ``)
	f(`* | blocks_count foo | rm foo`, ``, ``)
	f(`* | blocks_count foo | rm bar`, ``, ``)
	f(`* | fields x,y | blocks_count as bar | fields baz`, ``, ``)
	f(`* | rm x,y | blocks_count as bar | fields baz`, ``, ``)

	f(`* | format "foo" as s1`, `*`, `s1`)
	f(`* | format "foo<f1>" as s1`, `*`, `s1`)
	f(`* | format "foo<s1>" as s1`, `*`, ``)

	f(`* | format if (x1:y) "foo" as s1`, `*`, `s1`)
	f(`* | format if (x1:y) "foo<f1>" as s1`, `*`, `s1`)
	f(`* | format if (s1:y) "foo<f1>" as s1`, `*`, ``)
	f(`* | format if (x1:y) "foo<s1>" as s1`, `*`, ``)

	f(`* | format "foo" as s1 | fields f1`, `f1`, ``)
	f(`* | format "foo" as s1 | fields s1`, ``, ``)
	f(`* | format "foo<f1>" as s1 | fields f2`, `f2`, ``)
	f(`* | format "foo<f1>" as s1 | fields f1`, `f1`, ``)
	f(`* | format "foo<f1>" as s1 | fields s1`, `f1`, ``)
	f(`* | format "foo<s1>" as s1 | fields f1`, `f1`, ``)
	f(`* | format "foo<s1>" as s1 | fields s1`, `s1`, ``)

	f(`* | format if (f1:x) "foo" as s1 | fields s1`, `f1`, ``)
	f(`* | format if (f1:x) "foo" as s1 | fields s2`, `s2`, ``)

	f(`* | format "foo" as s1 | rm f1`, `*`, `f1,s1`)
	f(`* | format "foo" as s1 | rm s1`, `*`, `s1`)
	f(`* | format "foo<f1>" as s1 | rm f2`, `*`, `f2,s1`)
	f(`* | format "foo<f1>" as s1 | rm f1`, `*`, `s1`)
	f(`* | format "foo<f1>" as s1 | rm s1`, `*`, `s1`)
	f(`* | format "foo<s1>" as s1 | rm f1`, `*`, `f1`)
	f(`* | format "foo<s1>" as s1 | rm s1`, `*`, `s1`)

	f(`* | format if (f1:x) "foo" as s1 | rm s1`, `*`, `s1`)
	f(`* | format if (f1:x) "foo" as s1 | rm f1`, `*`, `s1`)
	f(`* | format if (f1:x) "foo" as s1 | rm f2`, `*`, `f2,s1`)

	f(`* | extract "<f1>x<f2>" from s1`, `*`, `f1,f2`)
	f(`* | extract if (f3:foo) "<f1>x<f2>" from s1`, `*`, `f1,f2`)
	f(`* | extract if (f1:foo) "<f1>x<f2>" from s1`, `*`, `f2`)
	f(`* | extract "<f1>x<f2>" from s1 | fields foo`, `foo`, ``)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | fields foo`, `foo`, ``)
	f(`* | extract "<f1>x<f2>" from s1| fields foo,s1`, `foo,s1`, ``)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | fields foo,s1`, `foo,s1`, ``)
	f(`* | extract "<f1>x<f2>" from s1 | fields foo,f1`, `foo,s1`, ``)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | fields foo,f1`, `foo,s1,x`, ``)
	f(`* | extract "<f1>x<f2>" from s1 | fields foo,f1,f2`, `foo,s1`, ``)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | fields foo,f1,f2`, `foo,s1,x`, ``)
	f(`* | extract "<f1>x<f2>" from s1 | rm foo`, `*`, `f1,f2,foo`)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | rm foo`, `*`, `f1,f2,foo`)
	f(`* | extract "<f1>x<f2>" from s1 | rm foo,s1`, `*`, `f1,f2,foo`)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | rm foo,s1`, `*`, `f1,f2,foo`)
	f(`* | extract "<f1>x<f2>" from s1 | rm foo,f1`, `*`, `f1,f2,foo`)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | rm foo,f1`, `*`, `f1,f2,foo`)
	f(`* | extract "<f1>x<f2>" from s1 | rm foo,f1,f2`, `*`, `f1,f2,foo,s1`)
	f(`* | extract if (x:bar) "<f1>x<f2>" from s1 | rm foo,f1,f2`, `*`, `f1,f2,foo,s1`)

	f(`* | extract "x<s1>y" from s1 `, `*`, ``)
	f(`* | extract if (x:foo) "x<s1>y" from s1`, `*`, ``)
	f(`* | extract if (s1:foo) "x<s1>y" from s1`, `*`, ``)
	f(`* | extract if (s1:foo) "x<f1>y" from s1`, `*`, `f1`)

	f(`* | extract "x<s1>y" from s1 | fields s2`, `s2`, ``)
	f(`* | extract "x<s1>y" from s1 | fields s1`, `s1`, ``)
	f(`* | extract if (x:foo) "x<s1>y" from s1 | fields s1`, `s1,x`, ``)
	f(`* | extract if (x:foo) "x<s1>y" from s1 | fields s2`, `s2`, ``)
	f(`* | extract if (s1:foo) "x<s1>y" from s1 | fields s1`, `s1`, ``)
	f(`* | extract if (s1:foo) "x<s1>y" from s1 | fields s2`, `s2`, ``)
	f(`* | extract if (s1:foo) "x<f1>y" from s1 | fields s1`, `s1`, ``)
	f(`* | extract if (s1:foo) "x<f1>y" from s1 | fields s2`, `s2`, ``)

	f(`* | extract "x<s1>y" from s1 | rm s2`, `*`, `s2`)
	f(`* | extract "x<s1>y" from s1 | rm s1`, `*`, `s1`)
	f(`* | extract if (x:foo) "x<s1>y" from s1 | rm s1`, `*`, `s1`)
	f(`* | extract if (x:foo) "x<s1>y" from s1 | rm s2`, `*`, `s2`)
	f(`* | extract if (s1:foo) "x<s1>y" from s1 | rm s1`, `*`, `s1`)
	f(`* | extract if (s1:foo) "x<s1>y" from s1 | rm s2`, `*`, `s2`)
	f(`* | extract if (s1:foo) "x<f1>y" from s1 | rm s1`, `*`, `f1`)
	f(`* | extract if (s1:foo) "x<f1>y" from s1 | rm s2`, `*`, `f1,s2`)

	f(`* | unpack_json`, `*`, ``)
	f(`* | unpack_json from s1`, `*`, ``)
	f(`* | unpack_json from s1 | fields f1`, `f1,s1`, ``)
	f(`* | unpack_json from s1 | fields s1,f1`, `f1,s1`, ``)
	f(`* | unpack_json from s1 | rm f1`, `*`, `f1`)
	f(`* | unpack_json from s1 | rm f1,s1`, `*`, `f1`)

	f(`* | unpack_logfmt`, `*`, ``)
	f(`* | unpack_logfmt from s1`, `*`, ``)
	f(`* | unpack_logfmt from s1 | fields f1`, `f1,s1`, ``)
	f(`* | unpack_logfmt from s1 | fields s1,f1`, `f1,s1`, ``)
	f(`* | unpack_logfmt from s1 | rm f1`, `*`, `f1`)
	f(`* | unpack_logfmt from s1 | rm f1,s1`, `*`, `f1`)

	f(`* | rm f1, f2`, `*`, `f1,f2`)
	f(`* | rm f1, f2 | mv f2 f3`, `*`, `f1,f2,f3`)
	f(`* | rm f1, f2 | cp f2 f3`, `*`, `f1,f2,f3`)
	f(`* | rm f1, f2 | mv f2 f3 | sort by(f4)`, `*`, `f1,f2,f3`)
	f(`* | rm f1, f2 | mv f2 f3 | sort by(f1)`, `*`, `f1,f2,f3`)
	f(`* | rm f1, f2 | fields f3`, `f3`, ``)
	f(`* | rm f1, f2 | fields f1,f3`, `f3`, ``)
	f(`* | rm f1, f2 | stats count(f3) r1`, `f3`, ``)
	f(`* | rm f1, f2 | stats count(f1) r1`, ``, ``)
	f(`* | rm f1, f2 | stats count(f1,f3) r1`, `f3`, ``)
	f(`* | rm f1, f2 | stats by(f1) count(f2) r1`, ``, ``)
	f(`* | rm f1, f2 | stats by(f3) count(f2) r1`, `f3`, ``)
	f(`* | rm f1, f2 | stats by(f3) count(f4) r1`, `f3,f4`, ``)

	// Verify that fields are correctly tracked before count(*)
	f(`* | copy a b, c d | count() r1`, ``, ``)
	f(`* | delete a, b | count() r1`, ``, ``)
	f(`* | extract "<f1>bar" from x | count() r1`, ``, ``)
	f(`* | extract if (q:w p:a) "<f1>bar" from x | count() r1`, `p,q`, ``)
	f(`* | extract_regexp "(?P<f1>.*)bar" from x | count() r1`, ``, ``)
	f(`* | extract_regexp if (q:w p:a) "(?P<f1>.*)bar" from x | count() r1`, `p,q`, ``)
	f(`* | field_names | count() r1`, `*`, `_time`)
	f(`* | limit 10 | field_names as abc | count() r1`, `*`, ``)
	f(`* | blocks_count | count() r1`, ``, ``)
	f(`* | limit 10 | blocks_count as abc | count() r1`, ``, ``)
	f(`* | fields a, b | count() r1`, ``, ``)
	f(`* | field_values a | count() r1`, `a`, ``)
	f(`* | limit 10 | filter a:b c:d | count() r1`, `a,c`, ``)
	f(`* | limit 10 | count() r1`, ``, ``)
	f(`* | format "<a><b>" as c | count() r1`, ``, ``)
	f(`* | format if (q:w p:a) "<a><b>" as c | count() r1`, `p,q`, ``)
	f(`* | math (a + b) as c, d * 2 as x | count() r1`, ``, ``)
	f(`* | offset 10 | count() r1`, ``, ``)
	f(`* | pack_json | count() r1`, ``, ``)
	f(`* | pack_json fields(a,b) | count() r1`, ``, ``)
	f(`* | rename a b, c d | count() r1`, ``, ``)
	f(`* | replace ("a", "b") at x | count() r1`, ``, ``)
	f(`* | replace if (q:w p:a) ("a", "b") at x | count() r1`, `p,q`, ``)
	f(`* | replace_regexp ("a", "b") at x | count() r1`, ``, ``)
	f(`* | replace_regexp if (q:w p:a) ("a", "b") at x | count() r1`, `p,q`, ``)
	f(`* | sort by (a,b) | count() r1`, ``, ``)
	f(`* | stats count_uniq(a, b) as c | count() r1`, ``, ``)
	f(`* | stats count_uniq(a, b) if (q:w p:a) as c | count() r1`, ``, ``)
	f(`* | stats by (a1,a2) count_uniq(a, b) as c | count() r1`, `a1,a2`, ``)
	f(`* | stats by (a1,a2) count_uniq(a, b) if (q:w p:a) as c | count() r1`, `a1,a2`, ``)
	f(`* | uniq by (a, b) | count() r1`, `a,b`, ``)
	f(`* | unpack_json from x | count() r1`, ``, ``)
	f(`* | unpack_json from x fields (a,b) | count() r1`, ``, ``)
	f(`* | unpack_json if (q:w p:a) from x | count() r1`, `p,q`, ``)
	f(`* | unpack_json if (q:w p:a) from x fields(a,b) | count() r1`, `p,q`, ``)
	f(`* | unpack_logfmt from x | count() r1`, ``, ``)
	f(`* | unpack_logfmt from x fields (a,b) | count() r1`, ``, ``)
	f(`* | unpack_logfmt if (q:w p:a) from x | count() r1`, `p,q`, ``)
	f(`* | unpack_logfmt if (q:w p:a) from x fields(a,b) | count() r1`, `p,q`, ``)
	f(`* | unroll (a, b) | count() r1`, `a,b`, ``)
	f(`* | unroll if (q:w p:a) (a, b) | count() r1`, `a,b,p,q`, ``)
}

func TestQueryClone(t *testing.T) {
	f := func(qStr string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		timestamp := q.GetTimestamp()
		qCopy := q.Clone(timestamp)
		qCopyStr := qCopy.String()
		if qStr != qCopyStr {
			t.Fatalf("unexpected cloned query\ngot\n%s\nwant\n%s", qCopyStr, qStr)
		}
	}

	f("*")
	f("error")
	f("_time:5m error | fields foo, bar")
	f("ip:in(foo | fields user_ip) bar | stats by (x:1h, y) count(*) if (user_id:in(q:w | fields abc)) as ccc")
}

func TestQueryGetFilterTimeRange(t *testing.T) {
	f := func(qStr string, startExpected, endExpected int64) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		start, end := q.GetFilterTimeRange()
		if start != startExpected || end != endExpected {
			t.Fatalf("unexpected filter time range; got [%d, %d]; want [%d, %d]", start, end, startExpected, endExpected)
		}
	}

	f("*", -9223372036854775808, 9223372036854775807)
	f("_time:2024-05-31T10:20:30.456789123Z", 1717150830456789123, 1717150830456789123)
	f("_time:2024-05-31Z", 1717113600000000000, 1717199999999999999)
	f("_time:2024-05-31Z _time:day_range[08:00, 16:00]", 1717113600000000000, 1717199999999999999)
}

func TestQueryCanReturnLastNResults(t *testing.T) {
	f := func(qStr string, resultExpected bool) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		result := q.CanReturnLastNResults()
		if result != resultExpected {
			t.Fatalf("unexpected result for CanRetrurnLastNResults(%q); got %v; want %v", qStr, result, resultExpected)
		}
	}

	f("*", true)
	f("error", true)
	f("error | fields foo | filter foo:bar", true)
	f("error | extract '<foo>bar<baz>'", true)
	f("* | rm x", true)
	f("* | stats count() rows", false)
	f("* | sort by (x)", false)
	f("* | len(x)", true)
	f("* | limit 10", false)
	f("* | offset 10", false)
	f("* | uniq (x)", false)
	f("* | blocks_count", false)
	f("* | field_names", false)
	f("* | field_values x", false)
	f("* | top 5 by (x)", false)

}

func TestQueryCanLiveTail(t *testing.T) {
	f := func(qStr string, resultExpected bool) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		result := q.CanLiveTail()
		if result != resultExpected {
			t.Fatalf("unexpected result for CanLiveTail(%q); got %v; want %v", qStr, result, resultExpected)
		}
	}

	f("foo", true)
	f("* | copy a b", true)
	f("* | rm a, b", true)
	f("* | drop_empty_fields", true)
	f("* | extract 'foo<bar>baz'", true)
	f("* | extract_regexp 'foo(?P<bar>baz)'", true)
	f("* | blocks_count a", false)
	f("* | field_names a", false)
	f("* | fields a, b", true)
	f("* | field_values a", false)
	f("* | filter foo", true)
	f("* | format 'a<b>c'", true)
	f("* | len(x)", true)
	f("* | limit 10", false)
	f("* | math a/b as c", true)
	f("* | offset 10", false)
	f("* | pack_json", true)
	f("* | pack_logfmt", true)
	f("* | rename a b", true)
	f("* | replace ('foo', 'bar')", true)
	f("* | replace_regexp ('foo', 'bar')", true)
	f("* | sort by (a)", false)
	f("* | stats count() rows", false)
	f("* | stream_context after 10", false)
	f("* | top 10 by (x)", false)
	f("* | uniq by (a)", false)
	f("* | unpack_json", true)
	f("* | unpack_logfmt", true)
	f("* | unpack_syslog", true)
	f("* | unroll by (a)", true)
}

func TestQueryDropAllPipes(t *testing.T) {
	f := func(qStr, resultExpected string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		q.Optimize()
		q.DropAllPipes()
		result := q.String()
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f(`*`, `*`)
	f(`foo | stats count()`, `foo`)
	f(`foo or bar and baz | top 5 by (x)`, `foo or bar baz`)
	f(`foo | filter bar:baz | stats by (x) min(y)`, `foo bar:baz`)
}

func TestQueryGetStatsByFieldsAddGroupingByTime_Success(t *testing.T) {
	f := func(qStr string, step int64, fieldsExpected []string, qExpected string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		fields, err := q.GetStatsByFieldsAddGroupingByTime(step)
		if err != nil {
			t.Fatalf("unexpected error in GetStatsByFieldsAddGroupingByTime(): %s", err)
		}
		if !reflect.DeepEqual(fields, fieldsExpected) {
			t.Fatalf("unexpected byFields;\ngot\n%q\nwant\n%q", fields, fieldsExpected)
		}

		// Verify the resulting query
		qResult := q.String()
		if qResult != qExpected {
			t.Fatalf("unexpected query\ngot\n%s\nwant\n%s", qResult, qExpected)
		}
	}

	f(`* | count()`, nsecsPerHour, []string{"_time"}, `* | stats by (_time:3600000000000) count(*) as "count(*)"`)
	f(`* | by (level) count() x`, nsecsPerDay, []string{"level", "_time"}, `* | stats by (level, _time:86400000000000) count(*) as x`)
	f(`* | by (_time:1m) count() x`, nsecsPerDay, []string{"_time"}, `* | stats by (_time:86400000000000) count(*) as x`)
	f(`* | by (_time:1m offset 30s,level) count() x, count_uniq(z) y`, nsecsPerDay, []string{"_time", "level"}, `* | stats by (_time:86400000000000, level) count(*) as x, count_uniq(z) as y`)
}

func TestQueryGetStatsByFieldsAddGroupingByTime_Failure(t *testing.T) {
	f := func(qStr string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		fields, err := q.GetStatsByFieldsAddGroupingByTime(nsecsPerHour)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if fields != nil {
			t.Fatalf("unexpected non-nil fields: %q", fields)
		}
	}

	f(`*`)
	f(`_time:5m | count() | drop _time`)
	f(`* | by (x) count() | keep x`)
}

func TestQueryGetStatsByFields_Success(t *testing.T) {
	f := func(qStr string, fieldsExpected []string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		fields, err := q.GetStatsByFields()
		if err != nil {
			t.Fatalf("unexpected error in GetStatsByFields(): %s", err)
		}
		if !reflect.DeepEqual(fields, fieldsExpected) {
			t.Fatalf("unexpected byFields;\ngot\n%q\nwant\n%q", fields, fieldsExpected)
		}
	}

	f(`* | stats count()`, []string{})
	f(`* | count()`, []string{})
	f(`* | by (foo) count(), count_uniq(bar)`, []string{"foo"})
	f(`* | stats by (a, b, cd) min(foo), max(bar)`, []string{"a", "b", "cd"})

	// multiple pipes before stats is ok
	f(`foo | extract "ip=<ip>," | stats by (host) count_uniq(ip)`, []string{"host"})

	// sort, offset and limit pipes are allowed after stats
	f(`foo | stats by (x, y) count() rows | sort by (rows) desc | offset 5 | limit 10`, []string{"x", "y"})

	// filter pipe is allowed after stats
	f(`foo | stats by (x, y) count() rows | filter rows:>100`, []string{"x", "y"})

	// math pipe is allowed after stats
	f(`foo | stats by (x) count() total, count() if (error) errors | math errors / total`, []string{"x"})

	// keep containing all the by(...) fields
	f(`foo | stats by (x) count() total | keep x, y`, []string{"x"})

	// drop which doesn't contain by(...) fields
	f(`foo | stats by (x) count() total | drop y`, []string{"x"})

	// copy which doesn't contain by(...) fields
	f(`foo | stats by (x) count() total | copy total abc`, []string{"x"})

	// mv by(...) fields
	f(`foo | stats by (x) count() total | mv x y`, []string{"y"})
}

func TestQueryGetStatsByFields_Failure(t *testing.T) {
	f := func(qStr string) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("cannot parse [%s]: %s", qStr, err)
		}
		fields, err := q.GetStatsByFields()
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if fields != nil {
			t.Fatalf("expectig nil fields; got %q", fields)
		}
	}

	f(`*`)
	f(`foo bar`)
	f(`foo | by (a, b) count() | copy a b`)
	f(`foo | by (a, b) count() | delete a`)
	f(`foo | count() | drop_empty_fields`)
	f(`foo | count() | extract "foo<bar>baz"`)
	f(`foo | count() | extract_regexp "(?P<ip>([0-9]+[.]){3}[0-9]+)"`)
	f(`foo | count() | blocks_count`)
	f(`foo | count() | field_names`)
	f(`foo | count() | field_values abc`)
	f(`foo | by (x) count() | fields a, b`)
	f(`foo | count() | format "foo<bar>baz"`)
	f(`foo | count() | pack_json`)
	f(`foo | count() | pack_logfmt`)
	f(`foo | rename x y`)
	f(`foo | count() | replace ("foo", "bar")`)
	f(`foo | count() | replace_regexp ("foo.+bar", "baz")`)
	f(`foo | count() | stream_context after 10`)
	f(`foo | count() | top 5 by (x)`)
	f(`foo | count() | uniq by (x)`)
	f(`foo | count() | unpack_json`)
	f(`foo | count() | unpack_logfmt`)
	f(`foo | count() | unpack_syslog`)
	f(`foo | count() | unroll by (x)`)

	f(`* | by (x) count() as rows | math rows * 10, rows / 10 | drop x`)
}
