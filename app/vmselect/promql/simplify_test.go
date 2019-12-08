package promql

import (
	"testing"
)

func TestSimplify(t *testing.T) {
	another := func(s string, sExpected string) {
		t.Helper()

		e, err := parsePromQL(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		res := e.AppendString(nil)
		if string(res) != sExpected {
			t.Fatalf("unexpected string constructed;\ngot\n%q\nwant\n%q", res, sExpected)
		}
	}

	// Simplification cases
	another(`nan == nan`, `NaN`)
	another(`nan ==bool nan`, `1`)
	another(`nan !=bool nan`, `0`)
	another(`nan !=bool 2`, `1`)
	another(`2 !=bool nan`, `1`)
	another(`nan >bool nan`, `0`)
	another(`nan <bool nan`, `0`)
	another(`1 ==bool nan`, `0`)
	another(`NaN !=bool 1`, `1`)
	another(`inf >=bool 2`, `1`)
	another(`-1 >bool -inf`, `1`)
	another(`-1 <bool -inf`, `0`)
	another(`nan + 2 *3 * inf`, `NaN`)
	another(`INF - Inf`, `NaN`)
	another(`Inf + inf`, `+Inf`)
	another(`1/0`, `+Inf`)
	another(`0/0`, `NaN`)
	another(`1 or 2`, `1`)
	another(`1 and 2`, `1`)
	another(`1 unless 2`, `NaN`)
	another(`1 default 2`, `1`)
	another(`1 default NaN`, `1`)
	another(`NaN default 2`, `2`)
	another(`1 > 2`, `NaN`)
	another(`1 > bool 2`, `0`)
	another(`3 >= 2`, `3`)
	another(`3 <= bool 2`, `0`)
	another(`1 + -2 - 3`, `-4`)
	another(`1 / 0 + 2`, `+Inf`)
	another(`2 + -1 / 0`, `-Inf`)
	another(`-1 ^ 0.5`, `NaN`)
	another(`512.5 - (1 + 3) * (2 ^ 2) ^ 3`, `256.5`)
	another(`1 == bool 1 != bool 24 < bool 4 > bool -1`, `1`)
	another(`1 == bOOl 1 != BOOL 24 < Bool 4 > booL -1`, `1`)
	another(`1+2 if 2>3`, `NaN`)
	another(`1+4 if 2<3`, `5`)
	another(`2+6 default 3 if 2>3`, `8`)
	another(`2+6 if 2>3 default NaN`, `NaN`)
	another(`42 if 3>2 if 2+2<5`, `42`)
	another(`42 if 3>2 if 2+2>=5`, `NaN`)
	another(`1+2 ifnot 2>3`, `3`)
	another(`1+4 ifnot 2<3`, `NaN`)
	another(`2+6 default 3 ifnot 2>3`, `8`)
	another(`2+6 ifnot 2>3 default NaN`, `8`)
	another(`42 if 3>2 ifnot 2+2<5`, `NaN`)
	another(`42 if 3>2 ifnot 2+2>=5`, `42`)
	another(`5 - 1 + 3 * 2 ^ 2 ^ 3 - 2  OR Metric {Bar= "Baz", aaa!="bb",cc=~"dd" ,zz !~"ff" } `,
		`770 or Metric{Bar="Baz", aaa!="bb", cc=~"dd", zz!~"ff"}`)
	another(`((foo(bar,baz)), (1+(2)+(3,4)+()))`, `(foo(bar, baz), (3 + (3, 4)) + ())`)
	another(` FOO (bar) + f  (  m  (  ),ff(1 + (  2.5)) ,M[5m ]  , "ff"  )`, `FOO(bar) + f(m(), ff(3.5), M[5m], "ff")`)
	another(`sum without (a, b) (xx,2+2)`, `sum(xx, 4) without (a, b)`)
	another(`Sum WIthout (a, B) (XX,2+2)`, `sum(XX, 4) without (a, B)`)
	another(`with (foo = bar{x="x"}) 1+1`, `2`)
	another(`with (x(a, b) = a + b) x(foo, x(1, 2))`, `foo + 3`)
	another(`WITH (
		x2(x) = x^2,
		f(x, y) = x2(x) + x*y + x2(y)
	)
	f(a, 3)
	`, `((a ^ 2) + (a * 3)) + 9`)
	another(`WITH (
		x2(x) = x^2,
		f(x, y) = x2(x) + x*y + x2(y)
	)
	f(2, 3)
	`, `19`)
	another(`with(y=123,z=5) union(with(y=3,f(x)=x*y) f(2) + f(3), with(x=5,y=2) x*y*z)`, `union(15, 50)`)
}
