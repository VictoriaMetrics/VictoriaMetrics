package promscrape

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/stretchr/testify/assert"
)

func TestHTTPToHTTPSRedirect(t *testing.T) {
	c := newClient(context.TODO(), &ScrapeWork{
		ScrapeURL:      "http://172.21.55.131:8080/_status/vars",
		ScrapeInterval: 5 * time.Second,
		ScrapeTimeout:  10 * time.Second,
		StreamParse:    false,
		AuthConfig: &promauth.Config{
			TLSInsecureSkipVerify: true,
		},
		DenyRedirects: false,
	})

	b := []byte{}
	result, err := c.ReadData(b)
	assert.NoError(t, err)

	fmt.Printf("ERROR: %v\n", err)
	fmt.Printf("RESULT: %s\n", string(result))
}
