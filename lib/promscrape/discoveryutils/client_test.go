package discoveryutils

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
	allowed := true
	notAllowed := false
	newClientValidConfig := []struct {
		httpCfg         promauth.HTTPClientConfig
		handler         func(w http.ResponseWriter, r *http.Request)
		expectedMessage string
	}{
		{
			httpCfg: promauth.HTTPClientConfig{
				FollowRedirects: &allowed,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/redirected":
					fmt.Fprint(w, "I'm here to serve you!!!")
				default:
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
					fmt.Fprint(w, "It should follow the redirect.")
				}
			},
			expectedMessage: "I'm here to serve you!!!",
		},
		{
			httpCfg: promauth.HTTPClientConfig{
				FollowRedirects: &notAllowed,
			},
			handler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/redirected":
					fmt.Fprint(w, "The redirection was followed.")
				default:
					w.Header().Set("Location", "/redirected")
					w.WriteHeader(http.StatusFound)
					fmt.Fprint(w, "I'm before redirect")
				}
			},
			expectedMessage: "I'm before redirect",
		},
	}

	for _, validConfig := range newClientValidConfig {
		testServer, err := newTestServer(validConfig.handler)
		if err != nil {
			t.Fatal(err.Error())
		}
		defer testServer.Close()

		client, err := NewClient("http://0.0.0.0:1234", nil, &proxy.URL{}, nil, &validConfig.httpCfg)
		if err != nil {
			t.Errorf("Can't create a client from this config: %+v", validConfig.httpCfg)
			continue
		}

		response, err := client.client.client.Get(testServer.URL)
		if err != nil {
			t.Errorf("Can't connect to the test server using this config: %+v: %v", validConfig.httpCfg, err)
			continue
		}

		message, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Errorf("Can't read the server response body using this config: %+v", validConfig.httpCfg)
			continue
		}

		trimMessage := strings.TrimSpace(string(message))
		if validConfig.expectedMessage != trimMessage {
			t.Errorf("The expected message (%s) differs from the obtained message (%s) using this config: %+v",
				validConfig.expectedMessage, trimMessage, validConfig.httpCfg)
		}
	}
}
