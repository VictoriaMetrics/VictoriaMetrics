package dockerswarm

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
	}, "%7B%22name%22%3A%5B%22foo%22%2C%22bar%22%5D%2C%22xxx%22%3A%5B%22aa%22%5D%7D")
	f([]Filter{
		{
			Name:   "desired-state",
			Values: []string{"running", "shutdown"},
		},
	}, "%7B%22desired-state%22%3A%5B%22running%22%2C%22shutdown%22%5D%7D")
}
