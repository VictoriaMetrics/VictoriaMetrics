package discoveryutil

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
)

func newTestServer(handler func(w http.ResponseWriter, r *http.Request)) (*httptest.Server, error) {
	testServer := httptest.NewUnstartedServer(http.HandlerFunc(handler))
	testServer.Start()
	return testServer, nil
}

func TestNewClientFromConfig(t *testing.T) {
	f := func(h func(w http.ResponseWriter, r *http.Request), httpCfg *promauth.HTTPClientConfig, expectedMessage string) {
		t.Helper()

		s, err := newTestServer(h)
		if err != nil {
			t.Fatalf("cannot create test server: %s", err)
		}
		defer s.Close()

		client, err := NewClient("http://0.0.0.0:1234", nil, &proxy.URL{}, nil, httpCfg)
		if err != nil {
			t.Fatalf("can't create a client from this config: %+v", httpCfg)
		}

		response, err := client.client.client.Get(s.URL)
		if err != nil {
			t.Fatalf("can't connect to the test server using this config: %+v: %v", httpCfg, err)
		}

		message, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatalf("Can't read the server response body using this config: %+v", httpCfg)
		}

		trimMessage := strings.TrimSpace(string(message))
		if expectedMessage != trimMessage {
			t.Fatalf("The expected message (%s) differs from the obtained message (%s) using this config: %+v", expectedMessage, trimMessage, httpCfg)
		}
	}

	// verify enabled redirects
	allowed := true
	handlerRedirect := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirected":
			fmt.Fprint(w, "I'm here to serve you!!!")
		default:
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(http.StatusFound)
			fmt.Fprint(w, "It should follow the redirect.")
		}
	}
	f(handlerRedirect, &promauth.HTTPClientConfig{
		FollowRedirects: &allowed,
	}, "I'm here to serve you!!!")

	// Verify disabled redirects
	notAllowed := false
	handlerNoRedirect := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirected":
			fmt.Fprint(w, "The redirection was followed.")
		default:
			w.Header().Set("Location", "/redirected")
			w.WriteHeader(http.StatusFound)
			fmt.Fprint(w, "I'm before redirect")
		}
	}
	f(handlerNoRedirect, &promauth.HTTPClientConfig{
		FollowRedirects: &notAllowed,
	}, "I'm before redirect")
}
