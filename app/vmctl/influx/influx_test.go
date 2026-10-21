package influx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestFetchQuery(t *testing.T) {
	f := func(s *Series, timeFilter, resultExpected string) {
		t.Helper()

		result := s.fetchQuery(timeFilter)
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "bar",
			},
		},
	}, "", `select "value" from "cpu" where "foo"::tag='bar'`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "bar",
			},
			{
				Name:  "baz",
				Value: "qux",
			},
		},
	}, "", `select "value" from "cpu" where "foo"::tag='bar' and "baz"::tag='qux'`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "foo",
				Value: "b'ar",
			},
		},
	}, "time >= now()", `select "value" from "cpu" where "foo"::tag='b\'ar' and time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
		LabelPairs: []LabelPair{
			{
				Name:  "name",
				Value: `dev-mapper-centos\x2dswap.swap`,
			},
			{
				Name:  "state",
				Value: "dev-mapp'er-c'en'tos",
			},
		},
	}, "time >= now()", `select "value" from "cpu" where "name"::tag='dev-mapper-centos\\x2dswap.swap' and "state"::tag='dev-mapp\'er-c\'en\'tos' and time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
	}, "time >= now()", `select "value" from "cpu" where time >= now()`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value",
	}, "", `select "value" from "cpu"`)

	f(&Series{
		Measurement: "cpu",
		Field:       "value1",
		EmptyTags:   []string{"e1", "e2", "e3"},
	}, "", `select "value1" from "cpu" where "e1"::tag='' and "e2"::tag='' and "e3"::tag=''`)
}

func TestTimeFilter(t *testing.T) {
	f := func(start, end, resultExpected string) {
		t.Helper()

		result := timeFilter(start, end)
		if result != resultExpected {
			t.Fatalf("unexpected result\ngot\n%v\nwant\n%s", result, resultExpected)
		}
	}

	// no start and end filters
	f("", "", "")

	// missing end filter
	f("2020-01-01T20:07:00Z", "", "time >= '2020-01-01T20:07:00Z'")

	// missing start filter
	f("", "2020-01-01T21:07:00Z", "time <= '2020-01-01T21:07:00Z'")

	// both start and end filters
	f("2020-01-01T20:07:00Z", "2020-01-01T21:07:00Z", "time >= '2020-01-01T20:07:00Z' and time <= '2020-01-01T21:07:00Z'")
}

func TestGetSeriesCommand(t *testing.T) {
	f := func(filterSeries, filterTime, expCommand string) {
		t.Helper()

		c := &Client{
			filterTime:   filterTime,
			filterSeries: filterSeries,
		}
		gotCommand := c.getSeriesCommand()
		if gotCommand != expCommand {
			t.Fatalf("unexpected command\ngot\n%s\nwant\n%s", gotCommand, expCommand)
		}
	}

	f("", "", "show series")
	f("from cpu", "", "show series from cpu")
	f("from cpu where arch='x86'", "", "show series from cpu where arch='x86'")
	f("", "time >= '2020-01-01T20:07:00Z'", "show series where time >= '2020-01-01T20:07:00Z'")
	f("from cpu", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where time >= '2020-01-01T20:07:00Z'")
	f("from cpu where arch='x86'", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where arch='x86' AND time >= '2020-01-01T20:07:00Z'")
	f("from cpu where arch='x86' AND hostname='host_2753'", "time >= '2020-01-01T20:07:00Z'", "show series from cpu where arch='x86' AND hostname='host_2753' AND time >= '2020-01-01T20:07:00Z'")
}

func TestResolveAuth(t *testing.T) {
	f := func(username, password, token, userExpected, passExpected string) {
		t.Helper()

		user, pass := resolveAuth(username, password, token)
		if user != userExpected || pass != passExpected {
			t.Fatalf("unexpected credentials for (username=%q, password=%q, token=%q)\ngot\n(%q, %q)\nwant\n(%q, %q)",
				username, password, token, user, pass, userExpected, passExpected)
		}
	}

	// InfluxDB 1.x: no token, credentials are passed through unchanged.
	f("", "", "", "", "")
	f("user", "pass", "", "user", "pass")

	// InfluxDB 2.x: the API token is sent as the password. The username is
	// required by the v1 compatibility API but may be any value, so a
	// placeholder is used when the caller did not provide one.
	f("", "", "token", defaultV1CompatUser, "token")

	// An explicitly provided username is preserved.
	f("myuser", "", "token", "myuser", "token")

	// The token takes precedence over a password.
	f("user", "pass", "token", "user", "token")
}

func TestConfigValidateSuccess(t *testing.T) {
	f := func(cfg Config) {
		t.Helper()

		if err := cfg.validate(); err != nil {
			t.Fatalf("unexpected error for config %+v: %s", cfg, err)
		}
	}

	// InfluxDB 1.x with and without credentials.
	f(Config{Version: 1, Database: "mydb"})
	f(Config{Version: 1, Database: "mydb", Username: "user", Password: "pass"})

	// InfluxDB 2.x requires a token.
	f(Config{Version: 2, Database: "mydb", Token: "my-token"})
}

func TestConfigValidateFailure(t *testing.T) {
	f := func(cfg Config, errStrExpected string) {
		t.Helper()

		err := cfg.validate()
		if err == nil {
			t.Fatalf("expecting non-nil error for config %+v", cfg)
		}
		if !strings.Contains(err.Error(), errStrExpected) {
			t.Fatalf("unexpected error for config %+v\ngot\n%s\nwant it to contain\n%s", cfg, err, errStrExpected)
		}
	}

	// unsupported versions
	f(Config{Version: 0, Database: "mydb"}, "unsupported InfluxDB version")
	f(Config{Version: 3, Database: "mydb"}, "unsupported InfluxDB version")

	// InfluxDB 2.x without a token cannot authenticate
	f(Config{Version: 2, Database: "mydb"}, "influx-token")

	// a token is meaningless without opting into v2
	f(Config{Version: 1, Database: "mydb", Token: "my-token"}, "influx-version=2")

	// the database is mandatory for both versions, since it is the `db`
	// parameter of the query API
	f(Config{Version: 1}, "influx-database")
	f(Config{Version: 2, Token: "my-token"}, "influx-database")
}

func TestResolveRetention(t *testing.T) {
	f := func(version int, retention, resultExpected string) {
		t.Helper()

		result := resolveRetention(version, retention)
		if result != resultExpected {
			t.Fatalf("unexpected retention policy for (version=%d, retention=%q)\ngot\n%q\nwant\n%q",
				version, retention, result, resultExpected)
		}
	}

	// InfluxDB 1.x keeps its historical default.
	f(VersionV1, "", "autogen")
	f(VersionV1, "all_data", "all_data")

	// For InfluxDB 2.x the retention policy is part of a DBRP mapping whose
	// name is arbitrary, so no default may be assumed: an empty value lets
	// InfluxDB pick the default mapping of the database.
	f(VersionV2, "", "")
	f(VersionV2, "myrp", "myrp")
}

// TestNewClientValidatesConfig ensures an invalid configuration is rejected
// before any connection to InfluxDB is attempted.
func TestNewClientValidatesConfig(t *testing.T) {
	f := func(cfg Config, errStrExpected string) {
		t.Helper()

		cfg.Addr = "http://127.0.0.1:1"

		c, err := NewClient(cfg)
		if err == nil {
			t.Fatalf("expecting non-nil error for config %+v", cfg)
		}
		if c != nil {
			t.Fatalf("expecting nil client for config %+v", cfg)
		}
		if !strings.Contains(err.Error(), errStrExpected) {
			t.Fatalf("unexpected error for config %+v\ngot\n%s\nwant it to contain\n%s", cfg, err, errStrExpected)
		}
	}

	f(Config{Version: 0, Database: "mydb"}, "unsupported InfluxDB version")
	f(Config{Version: 2, Database: "mydb"}, "influx-token")
	f(Config{Version: 1, Database: "mydb", Token: "my-token"}, "influx-version=2")
}

// TestNewClientTokenAuth ensures the API token is used as the password when
// migrating from InfluxDB 2.x.
func TestNewClientTokenAuth(t *testing.T) {
	f := func(cfg Config, userExpected, passExpected string) {
		t.Helper()

		user, pass := resolveAuth(cfg.Username, cfg.Password, cfg.Token)
		if user != userExpected || pass != passExpected {
			t.Fatalf("unexpected credentials for config %+v\ngot\n(%q, %q)\nwant\n(%q, %q)",
				cfg, user, pass, userExpected, passExpected)
		}
	}

	f(Config{Version: 2, Database: "mydb", Token: "my-token"}, defaultV1CompatUser, "my-token")
	f(Config{Version: 1, Database: "mydb", Username: "user", Password: "pass"}, "user", "pass")
}

func TestQueryRequestAuthAndRetention(t *testing.T) {
	f := func(cfg Config, userExpected, passExpected, rpExpected string) {
		t.Helper()

		var lastQuery queryRequest
		s := newTestServer(t, &lastQuery)
		defer s.Close()

		cfg.Addr = s.URL
		c, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error creating client: %s", err)
		}

		if _, err := c.fieldsByMeasurement(); err != nil {
			t.Fatalf("unexpected error querying field keys: %s", err)
		}

		got := lastQuery.get()
		if !got.seen {
			t.Fatalf("the client did not issue a /query request")
		}
		if !got.hasAuth {
			t.Fatalf("expecting basic auth to be set on the request")
		}
		if got.user != userExpected || got.password != passExpected {
			t.Fatalf("unexpected credentials on the wire\ngot\n(%q, %q)\nwant\n(%q, %q)",
				got.user, got.password, userExpected, passExpected)
		}
		if got.rp != rpExpected {
			t.Fatalf("unexpected rp parameter\ngot\n%q\nwant\n%q", got.rp, rpExpected)
		}
		if got.db != cfg.Database {
			t.Fatalf("unexpected db parameter\ngot\n%q\nwant\n%q", got.db, cfg.Database)
		}
	}

	// InfluxDB 1.x: credentials are passed through and `autogen` is the
	// default retention policy.
	f(Config{Version: VersionV1, Database: "mydb", Username: "user", Password: "pass"},
		"user", "pass", "autogen")

	// An explicit retention policy is honoured.
	f(Config{Version: VersionV1, Database: "mydb", Username: "user", Password: "pass", Retention: "all_data"},
		"user", "pass", "all_data")

	// InfluxDB 2.x: the API token is sent as the password with a placeholder
	// username, and no retention policy is assumed so that the default DBRP
	// mapping of the database applies.
	f(Config{Version: VersionV2, Database: "mydb", Token: "my-token"},
		defaultV1CompatUser, "my-token", "")

	// An explicit retention policy names the DBRP mapping to query.
	f(Config{Version: VersionV2, Database: "mydb", Token: "my-token", Retention: "myrp"},
		defaultV1CompatUser, "my-token", "myrp")
}

// queryRequest holds what a /query request carried on the wire. Only the values
// under test are kept, so nothing of the *http.Request outlives the handler.
type queryRequest struct {
	mu sync.Mutex

	seen     bool
	user     string
	password string
	hasAuth  bool
	rp       string
	db       string
}

func (q *queryRequest) set(r *http.Request) {
	user, password, hasAuth := r.BasicAuth()
	params := r.URL.Query()

	q.mu.Lock()
	defer q.mu.Unlock()
	q.seen = true
	q.user, q.password, q.hasAuth = user, password, hasAuth
	q.rp = params.Get("rp")
	q.db = params.Get("db")
}

func (q *queryRequest) get() queryRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	return queryRequest{
		seen:     q.seen,
		user:     q.user,
		password: q.password,
		hasAuth:  q.hasAuth,
		rp:       q.rp,
		db:       q.db,
	}
}

func newTestServer(t *testing.T, lastQuery *queryRequest) *httptest.Server {
	t.Helper()

	const fieldKeysResponse = `{"results":[{"statement_id":0,"series":[{"name":"cpu","columns":["fieldKey","fieldType"],"values":[["value","float"]]}]}]}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Influxdb-Version", "test")
		switch r.URL.Path {
		case "/ping":
			w.WriteHeader(http.StatusNoContent)
		case "/query":
			lastQuery.set(r)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fieldKeysResponse))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}
