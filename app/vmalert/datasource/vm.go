package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

type response struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Label map[string]string `json:"metric"`
			TV    [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (r response) metrics() []Metric {
	var ms []Metric
	var m Metric
	var f float64
	var err error
	for i, res := range r.Data.Result {
		f, err = strconv.ParseFloat(res.TV[1].(string), 64)
		if err != nil {
			logger.Errorf("Unable to parse float64 from %s: %s", res.TV[1], err)
			continue
		}
		for k, v := range r.Data.Result[i].Label {
			m.Label = append(m.Label, Label{Name: k, Value: v})
		}
		m.Timestamp = int64(res.TV[0].(float64))
		m.Value = f
		ms = append(ms, m)
	}
	return ms
}

const queryPath = "/api/v1/query?query="

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct {
	c                            *http.Client
	queryURL                     string
	basicAuthUser, basicAuthPass string
}

// NewVMStorage is a constructor for VMStorage
func NewVMStorage(baseURL, basicAuthUser, basicAuthPass string, c *http.Client) *VMStorage {
	return &VMStorage{
		c:             c,
		basicAuthUser: basicAuthUser,
		basicAuthPass: basicAuthPass,
		queryURL:      strings.TrimSuffix(baseURL, "/") + queryPath,
	}
}

// Query reads metrics from datasource by given query
func (s *VMStorage) Query(ctx context.Context, query string) ([]Metric, error) {
	const (
		statusSuccess, statusError, rtVector = "success", "error", "vector"
	)
	req, err := http.NewRequest("POST", s.queryURL+url.QueryEscape(query), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.basicAuthPass != "" {
		req.SetBasicAuth(s.basicAuthUser, s.basicAuthPass)
	}
	resp, err := s.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s:%s", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error parsing metrics for %s:%s", req.URL, err)
	}
	r := &response{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing metrics for %s:%s", req.URL, err)
	}
	if r.Status == statusError {
		return nil, fmt.Errorf("response error, query: %s, errorType: %s, error: %s", req.URL, r.ErrorType, r.Error)
	}
	if r.Status != statusSuccess {
		return nil, fmt.Errorf("unkown status:%s, Expected success or error ", r.Status)
	}
	if r.Data.ResultType != rtVector {
		return nil, fmt.Errorf("unkown restul type:%s. Expected vector", r.Data.ResultType)
	}
	return r.metrics(), nil
}
