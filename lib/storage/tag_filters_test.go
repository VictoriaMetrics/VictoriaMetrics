package storage

import (
	"reflect"
	"testing"
)

func TestExtractRegexpPrefix(t *testing.T) {
	f := func(s string, expectedPrefix, expectedSuffix string) {
		t.Helper()
		prefix, suffix := extractRegexpPrefix([]byte(s))
		if string(prefix) != expectedPrefix {
			t.Fatalf("unexpected prefix for %q; got %q; want %q", s, prefix, expectedPrefix)
		}
		if string(suffix) != expectedSuffix {
			t.Fatalf("unexpected suffix for %q; got %q; want %q", s, suffix, expectedSuffix)
		}
	}
	f("", "", "")
	f("foobar", "foobar", "")
}

func TestGetRegexpFromCache(t *testing.T) {
	f := func(s string, orValuesExpected, expectedMatches, expectedMismatches []string, suffixExpected string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			rcv, err := getRegexpFromCache([]byte(s))
			if err != nil {
				t.Fatalf("unexpected error for s=%q: %s", s, err)
			}
			if !reflect.DeepEqual(rcv.orValues, orValuesExpected) {
				t.Fatalf("unexpected orValues for s=%q; got %q; want %q", s, rcv.orValues, orValuesExpected)
			}
			if rcv.literalSuffix != suffixExpected {
				t.Fatalf("unexpected literal suffix for s=%q; got %q; want %q", s, rcv.literalSuffix, suffixExpected)
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

	f("", []string{""}, []string{""}, []string{"foo", "x"}, "")
	f("foo", []string{"foo"}, []string{"foo"}, []string{"", "bar"}, "")
	f("(?s)(foo)?", nil, []string{"foo", ""}, []string{"s", "bar"}, "")
	f("foo.*", nil, []string{"foo", "foobar"}, []string{"xfoo", "xfoobar", "", "a"}, "")
	f("foo(a|b)?", nil, []string{"fooa", "foob", "foo"}, []string{"xfoo", "xfoobar", "", "fooc", "fooba"}, "")
	f(".*foo", nil, []string{"foo", "xfoo"}, []string{"foox", "xfoobar", "", "a"}, "foo")
	f("(a|b)?foo", nil, []string{"foo", "afoo", "bfoo"}, []string{"foox", "xfoobar", "", "a"}, "foo")
	f(".*foo.*", nil, []string{"foo", "xfoo", "foox", "xfoobar"}, []string{"", "bar", "foxx"}, "")
	f(".*foo.+", nil, []string{"foo1", "xfoodff", "foox", "xfoobar"}, []string{"", "bar", "foo", "fox"}, "")
	f(".+foo.+", nil, []string{"xfoo1", "xfoodff", "xfoox", "xfoobar"}, []string{"", "bar", "foo", "foox", "xfoo"}, "")
	f(".+foo.*", nil, []string{"xfoo", "xfoox", "xfoobar"}, []string{"", "bar", "foo", "fox"}, "")
	f(".+foo(a|b)?", nil, []string{"xfoo", "xfooa", "xafoob"}, []string{"", "bar", "foo", "foob"}, "")
	f(".*foo(a|b)?", nil, []string{"foo", "foob", "xafoo", "xfooa"}, []string{"", "bar", "fooba"}, "")
	f("(a|b)?foo(a|b)?", nil, []string{"foo", "foob", "afoo", "afooa"}, []string{"", "bar", "fooba", "xfoo"}, "")
	f("((.*)foo(.*))", nil, []string{"foo", "xfoo", "foox", "xfoobar"}, []string{"", "bar", "foxx"}, "")
	f(".+foo", nil, []string{"afoo", "bbfoo"}, []string{"foo", "foobar", "afoox", ""}, "foo")
	f("a|b", []string{"a", "b"}, []string{"a", "b"}, []string{"xa", "bx", "xab", ""}, "")
	f("(a|b)", []string{"a", "b"}, []string{"a", "b"}, []string{"xa", "bx", "xab", ""}, "")
	f("(a|b)foo(c|d)", []string{"afooc", "afood", "bfooc", "bfood"}, []string{"afooc", "bfood"}, []string{"foo", "", "afoo", "fooc", "xfood"}, "")
	f("foo.+", nil, []string{"foox", "foobar"}, []string{"foo", "afoox", "afoo", ""}, "")
	f(".*foo.*bar", nil, []string{"foobar", "xfoobar", "xfooxbar", "fooxbar"}, []string{"", "foobarx", "afoobarx", "aaa"}, "bar")
	f("foo.*bar", nil, []string{"foobar", "fooxbar"}, []string{"xfoobar", "", "foobarx", "aaa"}, "bar")
	f("foo.*bar.*", nil, []string{"foobar", "fooxbar", "foobarx", "fooxbarx"}, []string{"", "afoobarx", "aaa", "afoobar"}, "")
	f("foo.*bar.*baz", nil, []string{"foobarbaz", "fooxbarxbaz", "foobarxbaz", "fooxbarbaz"}, []string{"", "afoobarx", "aaa", "afoobar", "foobarzaz"}, "baz")
	f(".+foo.+(b|c).+", nil, []string{"xfooxbar", "xfooxca"}, []string{"", "foo", "foob", "xfooc", "xfoodc"}, "")

	f("(?i)foo", nil, []string{"foo", "Foo", "FOO"}, []string{"xfoo", "foobar", "xFOObar"}, "")
	f("(?i).+foo", nil, []string{"xfoo", "aaFoo", "bArFOO"}, []string{"foosdf", "xFOObar"}, "")
	f("(?i)(foo|bar)", nil, []string{"foo", "Foo", "BAR", "bAR"}, []string{"foobar", "xfoo", "xFOObAR"}, "")
	f("(?i)foo.*bar", nil, []string{"foobar", "FooBAR", "FOOxxbaR"}, []string{"xfoobar", "foobarx", "xFOObarx"}, "")

	f(".*", nil, []string{"", "a", "foo", "foobar"}, nil, "")
	f("foo|.*", nil, []string{"", "a", "foo", "foobar"}, nil, "")
	f(".+", nil, []string{"a", "foo"}, []string{""}, "")
	f("(.+)*(foo)?", nil, []string{"a", "foo", ""}, nil, "")

	// Graphite-like regexps
	f(`foo\.[^.]*\.bar\.ba(xx|zz)[^.]*\.a`, nil, []string{"foo.ss.bar.baxx.a", "foo.s.bar.bazzasd.a"}, []string{"", "foo", "foo.ss.xar.baxx.a"}, ".a")
	f(`foo\.[^.]*?\.bar\.baz\.aaa`, nil, []string{"foo.aa.bar.baz.aaa"}, []string{"", "foo"}, ".bar.baz.aaa")
}

func TestTagFilterMatchSuffix(t *testing.T) {
	commonPrefix := []byte("prefix")
	key := []byte("key")
	var tf tagFilter

	tvNoTrailingTagSeparator := func(s string) string {
		return string(marshalTagValueNoTrailingTagSeparator(nil, []byte(s)))
	}
	init := func(value string, isNegative, isRegexp bool, expectedPrefix string) {
		t.Helper()
		if err := tf.Init(commonPrefix, key, []byte(value), isNegative, isRegexp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		prefix := string(commonPrefix) + string(marshalTagValue(nil, []byte(key))) + expectedPrefix
		if prefix != string(tf.prefix) {
			t.Fatalf("unexpected tf.prefix; got %q; want %q", tf.prefix, prefix)
		}
	}
	match := func(suffix string) {
		t.Helper()
		suffixEscaped := marshalTagValue(nil, []byte(suffix))
		ok, err := tf.matchSuffix(suffixEscaped)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if ok == tf.isNegative {
			t.Fatalf("%q must match suffix %q", tf.String(), suffix)
		}
	}
	mismatch := func(suffix string) {
		t.Helper()
		suffixEscaped := marshalTagValue(nil, []byte(suffix))
		ok, err := tf.matchSuffix(suffixEscaped)
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
		expectedPrefix := tvNoTrailingTagSeparator(value)
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
		expectedPrefix := tvNoTrailingTagSeparator(value)
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
		expectedPrefix := tvNoTrailingTagSeparator(value)
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
		expectedPrefix := tvNoTrailingTagSeparator(value)
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
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix
		match("")
		match("x")
		match("http")
		match("foobar")
	})
	t.Run("negative-regexp-prefix-any-suffix", func(t *testing.T) {
		value := "http.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match any suffix
		mismatch("")
		mismatch("x")
		mismatch("xhttp")
		mismatch("http")
		mismatch("httpsdf")
		mismatch("foobar")
	})
	t.Run("regexp-prefix-contains-suffix", func(t *testing.T) {
		value := "http.*foo.*"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix with `foo`
		mismatch("")
		mismatch("x")
		mismatch("http")
		match("foo")
		match("foobar")
		match("xfoobar")
		match("xfoo")
	})
	t.Run("negative-regexp-prefix-contains-suffix", func(t *testing.T) {
		value := "http.*foo.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match any suffix without `foo`
		match("")
		match("x")
		match("http")
		mismatch("foo")
		mismatch("foobar")
		mismatch("xfoobar")
		mismatch("xfoo")
		mismatch("httpfoo")
		mismatch("httpfoobar")
		mismatch("httpxfoobar")
		mismatch("httpxfoo")
	})
	t.Run("negative-regexp-noprefix-contains-suffix", func(t *testing.T) {
		value := ".*foo.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match anything not matching `.*foo.*`
		match("")
		match("x")
		match("http")
		mismatch("foo")
		mismatch("foobar")
		mismatch("xfoobar")
		mismatch("xfoo")
	})
	t.Run("regexp-prefix-special-suffix", func(t *testing.T) {
		value := "http.*bar"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match suffix ending on bar
		mismatch("")
		mismatch("x")
		match("bar")
		mismatch("barx")
		match("foobar")
		mismatch("foobarx")
	})
	t.Run("negative-regexp-prefix-special-suffix", func(t *testing.T) {
		value := "http.*bar"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match suffix ending on bar
		match("")
		mismatch("bar")
		mismatch("xhttpbar")
		mismatch("httpbar")
		match("httpbarx")
		mismatch("httpxybar")
		match("httpxybarx")
		mismatch("ahttpxybar")
	})
	t.Run("negative-regexp-noprefix-special-suffix", func(t *testing.T) {
		value := ".*bar"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match all except the regexp from value
		match("")
		mismatch("bar")
		mismatch("xhttpbar")
		match("barx")
		match("pbarx")
	})
	t.Run("regexp-or-suffixes", func(t *testing.T) {
		value := "http(foo|bar)"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)
		if !reflect.DeepEqual(tf.orSuffixes, []string{"bar", "foo"}) {
			t.Fatalf("unexpected orSuffixes; got %q; want %q", tf.orSuffixes, []string{"bar", "foo"})
		}

		// Must match foo or bar suffix
		mismatch("")
		mismatch("x")
		match("bar")
		mismatch("barx")
		match("foo")
		mismatch("foobar")
	})
	t.Run("negative-regexp-or-suffixes", func(t *testing.T) {
		value := "http(foo|bar)"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("http")
		init(value, isNegative, isRegexp, expectedPrefix)
		if !reflect.DeepEqual(tf.orSuffixes, []string{"bar", "foo"}) {
			t.Fatalf("unexpected or suffixes; got %q; want %q", tf.orSuffixes, []string{"bar", "foo"})
		}

		// Mustn't match foo or bar suffix
		match("")
		match("x")
		mismatch("foo")
		match("fooa")
		match("xfooa")
		mismatch("bar")
		match("xhttpbar")
	})
	t.Run("regexp-iflag-no-suffix", func(t *testing.T) {
		value := "(?i)http"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match case-insenstive http
		match("http")
		match("HTTP")
		match("hTTp")

		mismatch("")
		mismatch("foobar")
		mismatch("xhttp")
		mismatch("xhttp://")
		mismatch("hTTp://foobar.com")
	})
	t.Run("negative-regexp-iflag-no-suffix", func(t *testing.T) {
		value := "(?i)http"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match case-insensitive http
		mismatch("http")
		mismatch("HTTP")
		mismatch("hTTp")

		match("")
		match("foobar")
		match("xhttp")
		match("xhttp://")
		match("hTTp://foobar.com")
	})
	t.Run("regexp-iflag-any-suffix", func(t *testing.T) {
		value := "(?i)http.*"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Must match case-insenstive http
		match("http")
		match("HTTP")
		match("hTTp://foobar.com")

		mismatch("")
		mismatch("foobar")
		mismatch("xhttp")
		mismatch("xhttp://")
	})
	t.Run("negative-regexp-iflag-any-suffix", func(t *testing.T) {
		value := "(?i)http.*"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)

		// Mustn't match case-insensitive http
		mismatch("http")
		mismatch("HTTP")
		mismatch("hTTp://foobar.com")

		match("")
		match("foobar")
		match("xhttp")
		match("xhttp://")
	})
	t.Run("non-empty-string-regexp-negative-match", func(t *testing.T) {
		value := ".+"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)
		if len(tf.orSuffixes) != 0 {
			t.Fatalf("unexpected non-zero number of or suffixes: %d; %q", len(tf.orSuffixes), tf.orSuffixes)
		}

		match("")
		mismatch("x")
		mismatch("foo")
	})
	t.Run("non-empty-string-regexp-match", func(t *testing.T) {
		value := ".+"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)
		if len(tf.orSuffixes) != 0 {
			t.Fatalf("unexpected non-zero number of or suffixes: %d; %q", len(tf.orSuffixes), tf.orSuffixes)
		}

		mismatch("")
		match("x")
		match("foo")
	})
	t.Run("match-all-regexp-negative-match", func(t *testing.T) {
		value := ".*"
		isNegative := true
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)
		if len(tf.orSuffixes) != 0 {
			t.Fatalf("unexpected non-zero number of or suffixes: %d; %q", len(tf.orSuffixes), tf.orSuffixes)
		}

		mismatch("")
		mismatch("x")
		mismatch("foo")
	})
	t.Run("match-all-regexp-match", func(t *testing.T) {
		value := ".*"
		isNegative := false
		isRegexp := true
		expectedPrefix := tvNoTrailingTagSeparator("")
		init(value, isNegative, isRegexp, expectedPrefix)
		if len(tf.orSuffixes) != 0 {
			t.Fatalf("unexpected non-zero number of or suffixes: %d; %q", len(tf.orSuffixes), tf.orSuffixes)
		}

		match("")
		match("x")
		match("foo")
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
	f("|foo", []string{"", "foo"})
	f("|foo|", []string{"", "", "foo"})
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
	f("(?i)foo", nil)
	f("(?i)(foo|bar)", nil)
}

func TestGetRegexpPrefix(t *testing.T) {
	f := func(t *testing.T, s, expectedPrefix, expectedSuffix string) {
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

	f(t, "", "", "")
	f(t, "^", "", "")
	f(t, "$", "", "")
	f(t, "^()$", "", "")
	f(t, "^(?:)$", "", "")
	f(t, "foobar", "foobar", "")
	f(t, "foo$|^foobar", "foo", "(?:(?:)|bar)")
	f(t, "^(foo$|^foobar)$", "foo", "(?:(?:)|bar)")
	f(t, "foobar|foobaz", "fooba", "[rz]")
	f(t, "(fo|(zar|bazz)|x)", "", "fo|zar|bazz|x")
	f(t, "(тестЧЧ|тест)", "тест", "(?:ЧЧ|(?:))")
	f(t, "foo(bar|baz|bana)", "fooba", "(?:[rz]|na)")
	f(t, "^foobar|foobaz", "fooba", "[rz]")
	f(t, "^foobar|^foobaz$", "fooba", "[rz]")
	f(t, "foobar|foobaz", "fooba", "[rz]")
	f(t, "(?:^foobar|^foobaz)aa.*", "fooba", "[rz]aa(?-s:.)*")
	f(t, "foo[bar]+", "foo", "[a-br]+")
	f(t, "foo[a-z]+", "foo", "[a-z]+")
	f(t, "foo[bar]*", "foo", "[a-br]*")
	f(t, "foo[a-z]*", "foo", "[a-z]*")
	f(t, "foo[x]+", "foo", "x+")
	f(t, "foo[^x]+", "foo", "[^x]+")
	f(t, "foo[x]*", "foo", "x*")
	f(t, "foo[^x]*", "foo", "[^x]*")
	f(t, "foo[x]*bar", "foo", "x*bar")
	f(t, "fo\\Bo[x]*bar?", "fo", "\\Box*bar?")
	f(t, "foo.+bar", "foo", "(?-s:.)+bar")
	f(t, "a(b|c.*).+", "a", "(?:b|c(?-s:.)*)(?-s:.)+")
	f(t, "ab|ac", "a", "[b-c]")
	f(t, "(?i)xyz", "", "(?i:XYZ)")
	f(t, "(?i)foo|bar", "", "(?i:FOO)|(?i:BAR)")
	f(t, "(?i)up.+x", "", "(?i:UP)(?-s:.)+(?i:X)")
	f(t, "(?smi)xy.*z$", "", "(?i:XY)(?s:.)*(?i:Z)(?m:$)")

	// test invalid regexps
	f(t, "a(", "a(", "")
	f(t, "a[", "a[", "")
	f(t, "a[]", "a[]", "")
	f(t, "a{", "a{", "")
	f(t, "a{}", "a{}", "")
	f(t, "invalid(regexp", "invalid(regexp", "")

	// The transformed regexp mustn't match aba
	f(t, "a?(^ba|c)", "", "a?(?:\\Aba|c)")

	// The transformed regexp mustn't match barx
	f(t, "(foo|bar$)x*", "", "(?:foo|bar(?-m:$))x*")
}

func TestTagFiltersString(t *testing.T) {
	tfs := NewTagFilters()
	mustAdd := func(key, value string, isNegative, isRegexp bool) {
		t.Helper()
		if err := tfs.Add([]byte(key), []byte(value), isNegative, isRegexp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	mustAdd("", "metric_name", false, false)
	mustAdd("tag_re", "re.value", false, true)
	mustAdd("tag_nre", "nre.value", true, true)
	mustAdd("tag_n", "n_value", true, false)
	s := tfs.String()
	sExpected := `{__name__="metric_name", tag_re=~"re.value", tag_nre!~"nre.value", tag_n!="n_value"}`
	if s != sExpected {
		t.Fatalf("unexpected TagFilters.String(); got %q; want %q", s, sExpected)
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
		if idx >= len(tfs.tfs) {
			t.Fatalf("missing tag filter #%d; len(tfs)=%d, tfs=%s", idx, len(tfs.tfs), tfs)
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
	expectTagFilter(0, ".+", true, true)
	mustAdd([]byte("foo"), []byte("foo||bar"), false, true)
	expectTagFilter(1, "foo||bar", false, true)

	// Verify that other filters are added normally.
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
