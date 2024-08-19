package promql

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/metricsql"
)

func TestExtractTenantFilters(t *testing.T) {
	f := func(expr, extractedExpr string, expectedFilters []string) {
		t.Helper()

		e, err := metricsql.Parse(expr)
		if err != nil {
			t.Fatalf("unexpected error when parsing expression: %s", err)
		}

		tfs, ne := extractTenantFilters(e, nil)
		neStr := ne.AppendString(nil)
		if string(neStr) != extractedExpr {
			t.Fatalf("unexpected extracted expression; got\n%s\nwant\n%s", neStr, extractedExpr)
		}

		if len(expectedFilters) == 0 && len(tfs) == 0 {
			return
		}
		tfss := make([]string, len(tfs))
		for i, tf := range tfs {
			ctf := make([]string, len(tf))
			for j, f := range tf {
				ctf[j] = string(f.AppendString(nil))
			}
			tfss[i] = "{" + strings.Join(ctf, ",") + "}"
		}
		sort.Stable(sort.StringSlice(tfss))
		sort.Stable(sort.StringSlice(expectedFilters))
		if !reflect.DeepEqual(tfss, expectedFilters) {
			t.Fatalf("unexpected tenant filters; got\n%v\nwant\n%v", tfss, expectedFilters)
		}
	}

	f(`{a="b"}`, `{a="b"}`, nil)
	f(`up{vm_account_id="1"}`, `up`, []string{`{vm_account_id="1"}`})
	f(`up{vm_account_id="1",a="b"}`, `up{a="b"}`, []string{`{vm_account_id="1"}`})
	f(`up{a="b",vm_account_id="1",vm_project_id="2"}`, `up{a="b"}`, []string{`{vm_account_id="1",vm_project_id="2"}`})

	f(`up{a="b",vm_account_id="1",vm_project_id="2" or vm_account_id="3"}`, `up{a="b"}`, []string{`{vm_account_id="1",vm_project_id="2"}`, `{vm_account_id="3"}`})

	f(`rate(foo{a="b"}[5m])`, `rate(foo{a="b"}[5m])`, nil)
	f(`rate(foo{vm_account_id="1",a="b"}[5m])`, `rate(foo{a="b"}[5m])`, []string{`{vm_account_id="1"}`})
	f(`sum(rate(foo{vm_account_id="1",a="b"}[5m]))`, `sum(rate(foo{a="b"}[5m]))`, []string{`{vm_account_id="1"}`})

	f(`sum(rate(foo{vm_account_id="1",a="b"}[5m]))`, `sum(rate(foo{a="b"}[5m]))`, []string{`{vm_account_id="1"}`})
	f(`sum(rate(sum(rate(foo{vm_account_id="1",a="b"}[5m]))))`, `sum(rate(sum(rate(foo{a="b"}[5m]))))`, []string{`{vm_account_id="1"}`})

	f(`sum_over_time(rate(foo{vm_account_id="1",a="b"}[5m])[5m:1m])`, `sum_over_time(rate(foo{a="b"}[5m])[5m:1m])`, []string{`{vm_account_id="1"}`})
	f(`sum_over_time(up{vm_account_id="1"}[5m:1m])`, `sum_over_time(up[5m:1m])`, []string{`{vm_account_id="1"}`})
}
