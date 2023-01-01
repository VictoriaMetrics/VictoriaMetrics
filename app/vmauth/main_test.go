package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	importPath     = "/api/v1/import"
	queryRangePath = "/api/v1/query_range"
	writePath      = "/api/v1/write"
)

type executor func(host string, numberOfRequests, expectedBlockedRequests uint32) error

type client struct {
	executor
	numberOfConcurrentRequests uint32
	expectedBlockedRequests    uint32
}

func Test_requestHandler(t *testing.T) {

	f := func(name, s string, servers []*httptest.Server, client client) {
		t.Helper()
		serverURLs := make([]*url.URL, 0, len(servers))
		for _, server := range servers {
			u, _ := url.Parse(server.URL)
			serverURLs = append(serverURLs, u)
		}

		m, err := parseAuthConfig([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		// In this test we should rewrite url_prefix from config
		// to httptest.Server URL. It is needed because httptest.Server
		// always initialized on different ports
		for _, info := range m {
			if info.URLPrefix != nil {
				for i := range info.URLPrefix.urls {
					info.URLPrefix.urls[i].Host = serverURLs[i].Host
				}
			} else {
				var idx int
				for _, urlMap := range info.URLMaps {
					for i := range urlMap.URLPrefix.urls {
						urlMap.URLPrefix.urls[i].Host = serverURLs[idx].Host
						idx++
					}
				}
			}
		}

		authConfig.Store(m)

		vmauth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestHandler(w, r)
		}))
		defer vmauth.Close()

		if err := client.executor(vmauth.URL, client.numberOfConcurrentRequests, client.expectedBlockedRequests); err != nil {
			t.Fatalf("got error: %s", err)
		}

		for _, server := range servers {
			server.Close()
		}
	}

	f("header with auth token not set we should got 401", `
users:
  - bearer_token: bbb
    url_prefix: http://foo.bar`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r != nil {
					t.Fatalf("got unexpected request")
				}
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}
				rawURL, err := url.JoinPath(host, importPath)
				if err != nil {
					return fmt.Errorf("cannot joing url path")
				}
				request, _ := http.NewRequest(http.MethodPost, rawURL, nil)

				resp, err := client.Do(request)
				if err != nil {
					return fmt.Errorf("unexpected error on client side: %s", err)
				}

				if resp.StatusCode != http.StatusUnauthorized {
					return fmt.Errorf("expected 401 status code got: %d", resp.StatusCode)
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("initialize correct header with correct Bearer token", `
users:
  - bearer_token: "123"
    url_prefix: "http://localhost:8428"`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}
				importURL, err := url.JoinPath(host, importPath)
				if err != nil {
					t.Fatalf("cannot joing url path")
				}
				request, _ := http.NewRequest(http.MethodPost, importURL, nil)

				request.Header.Set("Authorization", "Bearer 123")

				resp, err := client.Do(request)
				if err != nil {
					return fmt.Errorf("unexpected error on client side: %s", err)
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("expected 200 status code got: %d", resp.StatusCode)
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("client send incorect basic auth and check request on the server side", `
users:
  - username: "local-single-node"
    password: "123"
    url_prefix: "http://localhost:8428?extra_label=team=dev"`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r != nil {
					t.Fatalf("got unexpected request")
				}
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}
				importURL, err := url.JoinPath(host, importPath)
				if err != nil {
					t.Fatalf("cannot joing url path")
				}
				request, _ := http.NewRequest(http.MethodPost, importURL, nil)

				request.SetBasicAuth("local-single-node", "abc")

				resp, err := client.Do(request)
				if err != nil {
					return fmt.Errorf("unexpected error on client side: %s", err)
				}

				if resp.StatusCode != http.StatusBadRequest {
					t.Fatalf("expected 400 status code got: %d", resp.StatusCode)
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("correct basic auth and check request on the server side", `
users:
  - username: "local-single-node"
    password: "123"
    url_prefix: "http://localhost:8428?extra_label=team=dev"`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				values := r.URL.Query()
				got := values.Get("extra_label")
				if got != "team=dev" {
					t.Fatalf("expected \"team=dev\" got: %q", got)
				}
				w.WriteHeader(http.StatusOK)
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}
				importURL, err := url.JoinPath(host, importPath)
				if err != nil {
					t.Fatalf("cannot joing url path")
				}
				request, _ := http.NewRequest(http.MethodPost, importURL, nil)

				request.SetBasicAuth("local-single-node", "123")

				resp, err := client.Do(request)
				if err != nil {
					return fmt.Errorf("unexpected error on client side: %s", err)
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("expected 200 status code got: %d", resp.StatusCode)
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("defined bearer token with headers", `
users:
  - bearer_token: "YYY"
    url_prefix: "http://localhost:8428"
    headers:
    - "X-Scope-OrgID: foobar"`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got := r.Header.Get("X-Scope-OrgID")
				if got != "foobar" {
					t.Fatalf("expected \"team=dev\" got: %q", got)
				}
				w.WriteHeader(http.StatusOK)
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}
				importURL, err := url.JoinPath(host, importPath)
				if err != nil {
					t.Fatalf("cannot joing url path")
				}
				request, _ := http.NewRequest(http.MethodPost, importURL, nil)

				request.Header.Set("Authorization", "Bearer YYY")

				resp, err := client.Do(request)
				if err != nil {
					return fmt.Errorf("unexpected error on client side: %s", err)
				}

				if resp.StatusCode != http.StatusOK {
					return fmt.Errorf("expected 200 status code got: %d", resp.StatusCode)
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("requests to three different servers servers", `
users:
  - username: "foobar"
    url_maps:
    - src_paths:
      - "/api/v1/import"
      - "/api/v1/query_range"
      url_prefix:
      - "http://vmselect1:8481"
      - "http://vmselect2:8481"
    - src_paths: ["/api/v1/write"]
      url_prefix: "http://vminsert:8480"
      headers:
      - "X-Scope-OrgID: abc"
`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == queryRangePath {
					w.WriteHeader(http.StatusOK)
				} else {
					t.Fatalf("tries to handle incorrect request")
				}
			})),
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == importPath {
					w.WriteHeader(http.StatusOK)
				} else {
					t.Fatalf("tries to handle incorrect request")
				}
			})),
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == writePath {
					got := r.Header.Get("X-Scope-OrgID")
					if got != "abc" {
						t.Fatalf("expected \"abc\" got: %q", got)
					}
					w.WriteHeader(http.StatusOK)
				} else {
					t.Fatalf("tries to handle incorrect request")
				}
			})),
		},
		client{
			executor: func(host string, _, _ uint32) error {
				client := http.Client{}

				paths := []string{
					fmt.Sprintf("%s%s", host, importPath),
					fmt.Sprintf("%s%s", host, queryRangePath),
					fmt.Sprintf("%s%s", host, writePath),
				}

				for _, path := range paths {
					request, _ := http.NewRequest(http.MethodPost, path, nil)

					request.SetBasicAuth("foobar", "")

					resp, err := client.Do(request)
					if err != nil {
						return fmt.Errorf("unexpected error on client side: %s", err)
					}

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("expected 200 status code got: %d", resp.StatusCode)
					}
				}
				return nil
			},
			numberOfConcurrentRequests: 0,
			expectedBlockedRequests:    0,
		},
	)

	f("set max concurrent requests limit for single client", `
users:
  - bearer_token: "YYY"
    url_prefix: "http://localhost:8428"
    headers:
      - "X-Scope-OrgID: foobar"
    max_concurrent_requests: 3`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(time.Millisecond * 10)
				w.WriteHeader(http.StatusOK)
			})),
		},
		client{
			executor: func(host string, numberOfRequests, expectedBlockedRequests uint32) error {
				client := http.Client{}
				errs, _ := errgroup.WithContext(context.Background())

				var counter uint32
				for i := 0; i < int(numberOfRequests); i++ {
					errs.Go(func() error {
						importURL, err := url.JoinPath(host, importPath)
						if err != nil {
							return fmt.Errorf("cannot joing url path")
						}
						request, _ := http.NewRequest(http.MethodPost, importURL, nil)

						request.Header.Set("Authorization", "Bearer YYY")
						resp, err := client.Do(request)
						if err != nil {
							return fmt.Errorf("unexpected error on client side: %s", err)
						}
						if resp.StatusCode == http.StatusTooManyRequests {
							atomic.AddUint32(&counter, 1)
						}
						return nil
					})
				}
				if err := errs.Wait(); err != nil {
					return err
				}

				got := atomic.LoadUint32(&counter)
				if got != expectedBlockedRequests {
					return fmt.Errorf("expected blocked requests %d; got: %d", expectedBlockedRequests, got)
				}
				return nil
			},
			numberOfConcurrentRequests: 10,
			expectedBlockedRequests:    7,
		},
	)

	f("set max concurrent requests limit for different clients ", `
users:
  - bearer_token: "YYY"
    url_prefix: "http://localhost:8428"
    max_concurrent_requests: 3

  - username: "local-single-node"
    password: "123123"
    url_prefix: "http://localhost:8428"
    max_concurrent_requests: 2`,
		[]*httptest.Server{
			httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(time.Millisecond * 10)
				w.WriteHeader(http.StatusOK)
			})),
		},
		client{
			executor: func(host string, numberOfRequests, expectedBlockedRequests uint32) error {
				clients := []*http.Client{&http.Client{}, &http.Client{}}
				errs, _ := errgroup.WithContext(context.Background())

				var counter uint32
				for i, c := range clients {
					client := c
					idx := i
					for j := 0; j < int(numberOfRequests); j++ {
						errs.Go(func() error {
							importURL, err := url.JoinPath(host, importPath)
							if err != nil {
								return fmt.Errorf("cannot joing url path")
							}
							request, _ := http.NewRequest(http.MethodPost, importURL, nil)
							if idx == 0 {
								request.Header.Set("Authorization", "Bearer YYY")
							} else {
								request.SetBasicAuth("local-single-node", "123123")
							}

							resp, err := client.Do(request)
							if err != nil {
								return fmt.Errorf("unexpected error on client side: %s", err)
							}
							if resp.StatusCode == http.StatusTooManyRequests {
								atomic.AddUint32(&counter, 1)
							}
							return nil
						})
					}
				}

				if err := errs.Wait(); err != nil {
					return err
				}

				got := atomic.LoadUint32(&counter)
				if got != expectedBlockedRequests {
					return fmt.Errorf("expected blocked requests %d; got: %d", expectedBlockedRequests, got)
				}
				return nil
			},
			numberOfConcurrentRequests: 10,
			expectedBlockedRequests:    15,
		},
	)
}
