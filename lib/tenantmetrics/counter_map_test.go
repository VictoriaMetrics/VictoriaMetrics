package tenantmetrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
)

func TestCreateMetricNameError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expecting non-nil panic")
		}
	}()
	_ = createMetricName("", TenantID{})
}

func TestCreateMetricNameSuccess(t *testing.T) {
	f := func(s string, at *auth.Token, metricExpected string) {
		t.Helper()
		metric := createMetricName(s, TenantID{
			AccountID: at.AccountID,
			ProjectID: at.ProjectID,
		})
		if metric != metricExpected {
			t.Fatalf("unexpected result for createMetricName(%q, %v); got %q; want %q", s, at, metric, metricExpected)
		}
	}
	f(`a`, &auth.Token{AccountID: 1, ProjectID: 2}, `a{accountID="1",projectID="2"}`)
	f(`foo{bar="baz"}`, &auth.Token{AccountID: 33, ProjectID: 41}, `foo{bar="baz",accountID="33",projectID="41"}`)
	f(`foo{bar="baz",a="aa"}`, &auth.Token{AccountID: 33, ProjectID: 41}, `foo{bar="baz",a="aa",accountID="33",projectID="41"}`)
}

func TestCounterMap(t *testing.T) {
	cm := NewCounterMap("foobar")
	cm.Get(&auth.Token{AccountID: 1, ProjectID: 2}).Inc()
	cm.Get(&auth.Token{AccountID: 4, ProjectID: 0}).Add(12)

	if n := cm.Get(&auth.Token{AccountID: 1, ProjectID: 2}).Get(); n != 1 {
		t.Fatalf("unexpected counter value; got %d; want %d", n, 1)
	}
	if n := cm.Get(&auth.Token{AccountID: 4, ProjectID: 0}).Get(); n != 12 {
		t.Fatalf("unexpected counter value; got %d; want %d", n, 12)
	}
	if n := cm.Get(&auth.Token{}).Get(); n != 0 {
		t.Fatalf("unexpected counter value; got %d; want %d", n, 0)
	}
}

func TestCounterMapConcurrent(t *testing.T) {
	cm := NewCounterMap(`aaa{bb="cc"}`)
	f := func() error {
		for i := 0; i < 10; i++ {
			cm.Get(&auth.Token{AccountID: 1, ProjectID: 2}).Inc()
			if n := cm.Get(&auth.Token{AccountID: 3, ProjectID: 4}).Get(); n != 0 {
				return fmt.Errorf("unexpected counter value; got %d; want %d", n, 0)
			}
			cm.Get(&auth.Token{AccountID: 1, ProjectID: 3}).Add(5)
		}
		return nil
	}

	const concurrency = 5
	ch := make(chan error, concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			ch <- f()
		}()
	}

	for i := 0; i < concurrency; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout")
		}
	}

	if n := cm.Get(&auth.Token{AccountID: 1, ProjectID: 2}).Get(); n != concurrency*10 {
		t.Fatalf("unexpected counter value; got %d; want %d", n, concurrency*10)
	}
	if n := cm.Get(&auth.Token{AccountID: 1, ProjectID: 3}).Get(); n != concurrency*10*5 {
		t.Fatalf("unexpected counter value; got %d; want %d", n, concurrency*10*5)
	}
}
