package docker

import (
	"testing"
)

func TestGetFiltersQueryArg(t *testing.T) {
	f := func(filters []Filter, queryArgExpected string) {
		t.Helper()
		queryArg := getFiltersQueryArg(filters)
		if queryArg != queryArgExpected {
			t.Fatalf("unexpected query arg; got %s; want %s", queryArg, queryArgExpected)
		}
	}
	f(nil, "")
	f([]Filter{
		{
			Name:   "name",
			Values: []string{"foo", "bar"},
		},
		{
			Name:   "xxx",
			Values: []string{"aa"},
		},
	}, "%7B%22name%22%3A%7B%22bar%22%3Atrue%2C%22foo%22%3Atrue%7D%2C%22xxx%22%3A%7B%22aa%22%3Atrue%7D%7D")
}
