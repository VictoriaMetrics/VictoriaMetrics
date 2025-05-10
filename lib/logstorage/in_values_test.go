package logstorage

import (
	"reflect"
	"slices"
	"testing"
)

func TestGetCommonTokensAndTokenSets(t *testing.T) {
	f := func(values []string, commonTokensExpected []string, tokenSetsExpected [][]string) {
		t.Helper()

		commonTokens, tokenSets := getCommonTokensAndTokenSets(values)
		slices.Sort(commonTokens)

		if !reflect.DeepEqual(commonTokens, commonTokensExpected) {
			t.Fatalf("unexpected commonTokens for values=%q\ngot\n%q\nwant\n%q", values, commonTokens, commonTokensExpected)
		}

		for i, tokens := range tokenSets {
			slices.Sort(tokens)
			tokensExpected := tokenSetsExpected[i]
			if !reflect.DeepEqual(tokens, tokensExpected) {
				t.Fatalf("unexpected tokens for value=%q\ngot\n%q\nwant\n%q", values[i], tokens, tokensExpected)
			}
		}
	}

	f(nil, nil, nil)
	f([]string{"foo"}, []string{"foo"}, [][]string{{}})
	f([]string{"foo", "foo"}, []string{"foo"}, [][]string{{}, {}})
	f([]string{"foo", "bar", "bar", "foo"}, nil, [][]string{{"foo"}, {"bar"}, {"bar"}, {"foo"}})
	f([]string{"foo", "foo bar", "bar foo"}, []string{"foo"}, [][]string{{}, {"bar"}, {"bar"}})
	f([]string{"a foo bar", "bar abc foo", "foo abc a bar"}, []string{"bar", "foo"}, [][]string{{"a"}, {"abc"}, {"a", "abc"}})
	f([]string{"a xfoo bar", "xbar abc foo", "foo abc a bar"}, nil, [][]string{{"a", "bar", "xfoo"}, {"abc", "foo", "xbar"}, {"a", "abc", "bar", "foo"}})
}
