package storage

import (
	"reflect"
	"testing"
)

func TestGetRegexpFromCache(t *testing.T) {
	f := func(s string, orValuesExpected, expectedMatches, expectedMismatches []string) {
		t.Helper()

		for i := 0; i < 3; i++ {
			rcv, err := getRegexpFromCache([]byte(s))
			if err != nil {
				t.Fatalf("unexpected error for s=%q: %s", s, err)
			}
			if !reflect.DeepEqual(rcv.orValues, orValuesExpected) {
				t.Fatalf("unexpected orValues for s=%q; got %q; want %q", s, rcv.orValues, orValuesExpected)
			}
			for _, expectedMatch := range expectedMatches {
				if !rcv.reMatch([]byte(expectedMatch)) {
					t.Fatalf("s=%q must match %q", s, expectedMatch)
				}
			}
			for _, expectedMismatch := range expectedMismatches {
				if rcv.reMatch([]byte(expectedMismatch)) {
					t.Fatalf("s=%q must mismatch %q", s, expectedMismatch)
				}
			}
		}
	}

	f("", []string{""}, []string{""}, []string{"foo", "x"})
	f("foo", []string{"foo"}, []string{"foo"}, []string{"", "bar"})
	f("foo.*", nil, []string{"foo", "foobar"}, []string{"xfoo", "xfoobar", "", "a"})
	f(".*foo", nil, []string{"foo", "xfoo"}, []string{"foox", "xfoobar", "", "a"})
	f(".*foo.*", nil, []string{"foo", "xfoo", "foox", "xfoobar"}, []string{"", "bar", "foxx"})
	f("((.*)foo(.*))", nil, []string{"foo", "xfoo", "foox", "xfoobar"}, []string{"", "bar", "foxx"})
	f(".+foo", nil, []string{"afoo", "bbfoo"}, []string{"foo", "foobar", "afoox", ""})
	f("a|b", []string{"a", "b"}, []string{"a", "b"}, []string{"xa", "bx", "xab", ""})
	f("foo.+", nil, []string{"foox", "foobar"}, []string{"foo", "afoox", "afoo", ""})
	f(".*foo.*bar", nil, []string{"foobar", "xfoobar", "xfooxbar", "fooxbar"}, []string{"", "foobarx", "afoobarx", "aaa"})
	f("foo.*bar", nil, []string{"foobar", "fooxbar"}, []string{"xfoobar", "", "foobarx", "aaa"})
	f("foo.*bar.*", nil, []string{"foobar", "fooxbar", "foobarx", "fooxbarx"}, []string{"", "afoobarx", "aaa", "afoobar"})

	f(".*", nil, []string{"", "a", "foo", "foobar"}, nil)
	f("foo|.*", nil, []string{"", "a", "foo", "foobar"}, nil)
	f(".+", nil, []string{"a", "foo"}, []string{""})
	f("(.+)*(foo)?", nil, []string{"a", "foo", ""}, nil)
}

func TestTagFilterMatchSuffix(t *testing.T) {
	commonPrefix := []byte("prefix")
	key := []byte("key")
	var tf tagFilter

	tv := func(s string) string {
		return string(marshalTagValue(nil, []byte(s)))
	}
	init := func(value string, isNegative, isRegexp bool, expectedPrefix string) {
		t.Helper()
		if err := tf.Init(commonPrefix, key, []byte(value), isNegative, isRegexp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		prefix := string(commonPrefix) + tv(string(key)) + expectedPrefix
		if prefix != string(tf.prefix) {
			t.Fatalf("unexpected tf.prefix; got %q; want %q", tf.prefix, prefix)
		}
	}
	match := func(suffix string) {
		t.Helper()
		ok, err := tf.matchSuffix([]byte(suffix))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if ok == tf.isNegative {
			t.Fatalf("%q must match suffix %q", tf.String(), suffix)
		}
	}
	mismatch := func(suffix string) {
		t.Helper()
		ok, err := tf.matchSuffix([]byte(suffix))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if ok != tf.isNegative {
			t.Fatalf("%q mustn't match suffix %q", tf.String(), suffix)
		}
	}

	t.Run("plain-value", func(t *testing.T) {
		value := "xx"
		isNegative := false
		isRegexp := false
		expectedPrefix := tv(value)
		init(value, isNegative, isRegexp, expectedPrefix)

		// Plain value must match empty suffix only
		match("")
		mismatch("foo")
		mismatch("xx")
	})
	t.Run("negative-plain-value", func(t *testing.T) {
		value := "xx"
		isNegative := true
		isRegexp := false
		expectedPrefix := tv(value)
		init(value, isNegative, isRegexp, expectedPrefix)

		// Negaitve plain value must match all except empty suffix
		mismatch("")
		match("foo")
		match("foxx")
		match("xx")
		match("xxx")
		match("xxfoo")
	})
	t.Run("regexp-convert-to-plain-value", func(t *testing.T) {
		value := "http"
		isNegative := false
		isRegexp := true
		expectedPrefix := tv("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match only empty suffix
		match("")
		mismatch("x")
		mismatch("http")
		mismatch("foobar")
	})
	t.Run("negative-regexp-convert-to-plain-value", func(t *testing.T) {
		value := "http"
		isNegative := true
		isRegexp := true
		expectedPrefix := tv("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match all except empty suffix
		mismatch("")
		match("x")
		match("xhttp")
		match("http")
		match("httpx")
		match("foobar")
	})
	t.Run("regexp-prefix-any-suffix", func(t *testing.T) {
		value := "http.*"
		isNegative := false
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix
		match(tv(""))
		match(tv("x"))
		match(tv("http"))
		match(tv("foobar"))
	})
	t.Run("negative-regexp-prefix-any-suffix", func(t *testing.T) {
		value := "http.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match any suffix
		mismatch(tv(""))
		mismatch(tv("x"))
		mismatch(tv("xhttp"))
		mismatch(tv("http"))
		mismatch(tv("httpsdf"))
		mismatch(tv("foobar"))
	})
	t.Run("regexp-prefix-contains-suffix", func(t *testing.T) {
		value := "http.*foo.*"
		isNegative := false
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix with `foo`
		mismatch(tv(""))
		mismatch(tv("x"))
		mismatch(tv("http"))
		match(tv("foo"))
		match(tv("foobar"))
		match(tv("xfoobar"))
		match(tv("xfoo"))
	})
	t.Run("negative-regexp-prefix-contains-suffix", func(t *testing.T) {
		value := "http.*foo.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix without `foo`
		match(tv(""))
		match(tv("x"))
		match(tv("http"))
		mismatch(tv("foo"))
		mismatch(tv("foobar"))
		mismatch(tv("xfoobar"))
		mismatch(tv("xfoo"))
		mismatch(tv("httpfoo"))
		mismatch(tv("httpfoobar"))
		mismatch(tv("httpxfoobar"))
		mismatch(tv("httpxfoo"))
	})
	t.Run("negative-regexp-noprefix-contains-suffix", func(t *testing.T) {
		value := ".*foo.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := ""
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match anything not matching `.*foo.*`
		match(tv(""))
		match(tv("x"))
		match(tv("http"))
		mismatch(tv("foo"))
		mismatch(tv("foobar"))
		mismatch(tv("xfoobar"))
		mismatch(tv("xfoo"))
	})
	t.Run("regexp-prefix-special-suffix", func(t *testing.T) {
		value := "http.*bar"
		isNegative := false
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match suffix ending on bar
		mismatch(tv(""))
		mismatch(tv("x"))
		match(tv("bar"))
		mismatch(tv("barx"))
		match(tv("foobar"))
		mismatch(tv("foobarx"))
	})
	t.Run("negative-regexp-prefix-special-suffix", func(t *testing.T) {
		value := "http.*bar"
		isNegative := true
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match suffix ending on bar
		match(tv(""))
		mismatch(tv("bar"))
		mismatch(tv("xhttpbar"))
		mismatch(tv("httpbar"))
		match(tv("httpbarx"))
		mismatch(tv("httpxybar"))
		match(tv("httpxybarx"))
		mismatch(tv("ahttpxybar"))
	})
	t.Run("negative-regexp-noprefix-special-suffix", func(t *testing.T) {
		value := ".*bar"
		isNegative := true
		isRegexp := true
		expectedPrefix := ""
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match all except the regexp from value
		match(tv(""))
		mismatch(tv("bar"))
		mismatch(tv("xhttpbar"))
		match(tv("barx"))
		match(tv("pbarx"))
	})
	t.Run("regexp-or-suffixes", func(t *testing.T) {
		value := "http(foo|bar)"
		isNegative := false
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)
		if !reflect.DeepEqual(tf.orSuffixes, []string{"bar", "foo"}) {
			t.Fatalf("unexpected orSuffixes; got %q; want %q", tf.orSuffixes, []string{"bar", "foo"})
		}

		// Must match foo or bar suffix
		mismatch(tv(""))
		mismatch(tv("x"))
		match(tv("bar"))
		mismatch(tv("barx"))
		match(tv("foo"))
		mismatch(tv("foobar"))
	})
	t.Run("negative-regexp-or-suffixes", func(t *testing.T) {
		value := "http(foo|bar)"
		isNegative := true
		isRegexp := true
		expectedPrefix := "http"
		init(value, isNegative, isRegexp, expectedPrefix)
		if !reflect.DeepEqual(tf.orSuffixes, []string{"bar", "foo"}) {
			t.Fatalf("unexpected or suffixes; got %q; want %q", tf.orSuffixes, []string{"bar", "foo"})
		}

		// Mustn't match foo or bar suffix
		match(tv(""))
		match(tv("x"))
		mismatch(tv("foo"))
		match(tv("fooa"))
		match(tv("xfooa"))
		mismatch(tv("bar"))
		match(tv("xhttpbar"))
	})
}

func TestGetOrValues(t *testing.T) {
	f := func(s string, valuesExpected []string) {
		t.Helper()

		values := getOrValues(s)
		if !reflect.DeepEqual(values, valuesExpected) {
			t.Fatalf("unexpected values for s=%q; got %q; want %q", s, values, valuesExpected)
		}
	}

	f("", []string{""})
	f("foo.+", nil)
	f("foo.*", nil)
	f(".*", nil)
	f("foo|.*", nil)
	f("foobar", []string{"foobar"})
	f("z|x|c", []string{"c", "x", "z"})
	f("foo|bar", []string{"bar", "foo"})
	f("(foo|bar)", []string{"bar", "foo"})
	f("(foo|bar)baz", []string{"barbaz", "foobaz"})
	f("[a-z]", nil)
	f("[a-d]", []string{"a", "b", "c", "d"})
	f("x[a-d]we", []string{"xawe", "xbwe", "xcwe", "xdwe"})
	f("foo(bar|baz)", []string{"foobar", "foobaz"})
	f("foo(ba[rz]|(xx|o))", []string{"foobar", "foobaz", "fooo", "fooxx"})
	f("foo(?:bar|baz)x(qwe|rt)", []string{"foobarxqwe", "foobarxrt", "foobazxqwe", "foobazxrt"})
	f("foo(bar||baz)", []string{"foo", "foobar", "foobaz"})
	f("(a|b|c)(d|e|f)(g|h|k)", nil)
}

func TestGetRegexpPrefix(t *testing.T) {
	testGetRegexpPrefix(t, "", "", "")
	testGetRegexpPrefix(t, "^", "", "")
	testGetRegexpPrefix(t, "$", "", "")
	testGetRegexpPrefix(t, "^()$", "", "")
	testGetRegexpPrefix(t, "^(?:)$", "", "")
	testGetRegexpPrefix(t, "foobar", "foobar", "")
	testGetRegexpPrefix(t, "foo$|^foobar", "foo", "(?:(?:)|bar)")
	testGetRegexpPrefix(t, "^(foo$|^foobar)$", "foo", "(?:(?:)|bar)")
	testGetRegexpPrefix(t, "foobar|foobaz", "fooba", "[rz]")
	testGetRegexpPrefix(t, "(fo|(zar|bazz)|x)", "", "fo|zar|bazz|x")
	testGetRegexpPrefix(t, "(тестЧЧ|тест)", "тест", "(?:ЧЧ|(?:))")
	testGetRegexpPrefix(t, "foo(bar|baz|bana)", "fooba", "(?:[rz]|na)")
	testGetRegexpPrefix(t, "^foobar|foobaz", "fooba", "[rz]")
	testGetRegexpPrefix(t, "^foobar|^foobaz$", "fooba", "[rz]")
	testGetRegexpPrefix(t, "foobar|foobaz", "fooba", "[rz]")
	testGetRegexpPrefix(t, "(?:^foobar|^foobaz)aa.*", "fooba", "[rz]aa(?-s:.)*")
	testGetRegexpPrefix(t, "foo[bar]+", "foo", "[a-br]+")
	testGetRegexpPrefix(t, "foo[a-z]+", "foo", "[a-z]+")
	testGetRegexpPrefix(t, "foo[bar]*", "foo", "[a-br]*")
	testGetRegexpPrefix(t, "foo[a-z]*", "foo", "[a-z]*")
	testGetRegexpPrefix(t, "foo[x]+", "foo", "x+")
	testGetRegexpPrefix(t, "foo[^x]+", "foo", "[^x]+")
	testGetRegexpPrefix(t, "foo[x]*", "foo", "x*")
	testGetRegexpPrefix(t, "foo[^x]*", "foo", "[^x]*")
	testGetRegexpPrefix(t, "foo[x]*bar", "foo", "x*bar")
	testGetRegexpPrefix(t, "fo\\Bo[x]*bar?", "fo", "\\Box*bar?")

	// test invalid regexps
	testGetRegexpPrefix(t, "a(", "a(", "")
	testGetRegexpPrefix(t, "a[", "a[", "")
	testGetRegexpPrefix(t, "a[]", "a[]", "")
	testGetRegexpPrefix(t, "a{", "a{", "")
	testGetRegexpPrefix(t, "a{}", "a{}", "")
	testGetRegexpPrefix(t, "invalid(regexp", "invalid(regexp", "")

	// The transformed regexp mustn't match aba
	testGetRegexpPrefix(t, "a?(^ba|c)", "", "a?(?:\\Aba|c)")

	// The transformed regexp mustn't match barx
	testGetRegexpPrefix(t, "(foo|bar$)x*", "", "(?:foo|bar(?-m:$))x*")
}

func testGetRegexpPrefix(t *testing.T, s, expectedPrefix, expectedSuffix string) {
	t.Helper()

	prefix, suffix := getRegexpPrefix([]byte(s))
	if string(prefix) != expectedPrefix {
		t.Fatalf("unexpected prefix for s=%q; got %q; want %q", s, prefix, expectedPrefix)
	}
	if string(suffix) != expectedSuffix {
		t.Fatalf("unexpected suffix for s=%q; got %q; want %q", s, suffix, expectedSuffix)
	}

	// Get the prefix from cache.
	prefix, suffix = getRegexpPrefix([]byte(s))
	if string(prefix) != expectedPrefix {
		t.Fatalf("unexpected prefix for s=%q; got %q; want %q", s, prefix, expectedPrefix)
	}
	if string(suffix) != expectedSuffix {
		t.Fatalf("unexpected suffix for s=%q; got %q; want %q", s, suffix, expectedSuffix)
	}
}

func TestTagFiltersAddEmpty(t *testing.T) {
	tfs := NewTagFilters()

	mustAdd := func(key, value []byte, isNegative, isRegexp bool) {
		t.Helper()
		if err := tfs.Add(key, value, isNegative, isRegexp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	expectTagFilter := func(idx int, value string, isNegative, isRegexp bool) {
		t.Helper()
		if len(tfs.tfs) != idx+1 {
			t.Fatalf("expecting non-empty tag filter")
		}
		tf := tfs.tfs[idx]
		if string(tf.value) != value {
			t.Fatalf("unexpected tag filter value; got %q; want %q", tf.value, value)
		}
		if tf.isNegative != isNegative {
			t.Fatalf("unexpected isNegative; got %v; want %v", tf.isNegative, isNegative)
		}
		if tf.isRegexp != isRegexp {
			t.Fatalf("unexpected isRegexp; got %v; want %v", tf.isRegexp, isRegexp)
		}
	}

	// Empty filters
	mustAdd(nil, nil, false, false)
	expectTagFilter(0, ".+", true, true)
	mustAdd([]byte("foo"), nil, false, false)
	expectTagFilter(1, ".+", true, true)
	mustAdd([]byte("foo"), nil, true, false)
	expectTagFilter(2, ".+", false, true)

	// Empty regexp filters
	tfs.Reset()
	mustAdd([]byte("foo"), []byte(".*"), false, true)
	if len(tfs.tfs) != 0 {
		t.Fatalf("unexpectedly added empty regexp filter %s", &tfs.tfs[0])
	}
	mustAdd([]byte("foo"), []byte(".*"), true, true)
	expectTagFilter(0, ".*", true, true)
	mustAdd([]byte("foo"), []byte("foo||bar"), false, true)
	expectTagFilter(1, "foo||bar", false, true)
	mustAdd(nil, []byte("foo||bar"), true, true)
	expectTagFilter(2, "foo||bar", true, true)

	// Verify that otner filters are added normally.
	tfs.Reset()
	mustAdd(nil, []byte("foobar"), false, false)
	if len(tfs.tfs) != 1 {
		t.Fatalf("missing added filter")
	}
	mustAdd([]byte("bar"), []byte("foobar"), true, false)
	if len(tfs.tfs) != 2 {
		t.Fatalf("missing added filter")
	}
	mustAdd(nil, []byte("foo.+bar"), true, true)
	if len(tfs.tfs) != 3 {
		t.Fatalf("missing added filter")
	}
	mustAdd([]byte("bar"), []byte("foo.+bar"), false, true)
	if len(tfs.tfs) != 4 {
		t.Fatalf("missing added filter")
	}
	mustAdd([]byte("bar"), []byte("foo.*"), false, true)
	if len(tfs.tfs) != 5 {
		t.Fatalf("missing added filter")
	}
}
