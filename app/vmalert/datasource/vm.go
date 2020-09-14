package datasource

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type response struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Labels map[string]string `json:"metric"`
			TV     [2]interface{}    `json:"value"`
		} `json:"result"`
	} `json:"data"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

func (r response) metrics() ([]Metric, error) {
	var ms []Metric
	var m Metric
	var f float64
	var err error
	for i, res := range r.Data.Result {
		f, err = strconv.ParseFloat(res.TV[1].(string), 64)
		if err != nil {
			return nil, fmt.Errorf("metric %v, unable to parse float64 from %s: %w", res, res.TV[1], err)
		}
		m.Labels = nil
		for k, v := range r.Data.Result[i].Labels {
			m.Labels = append(m.Labels, Label{Name: k, Value: v})
		}
		m.Timestamp = int64(res.TV[0].(float64))
		m.Value = f
		ms = append(ms, m)
	}
	return ms, nil
}

const queryPath = "/api/v1/query?query="

// VMStorage represents vmstorage entity with ability to read and write metrics
type VMStorage struct {
	c                *http.Client
	baseURL          string
	suffix           string
	basicAuthUser    string
	basicAuthPass    string
	defaultAuthToken *auth.Token
}

// NewVMStorage is a constructor for VMStorage
func NewVMStorage(baseURL, basicAuthUser, basicAuthPass string, c *http.Client) (*VMStorage, error) {
	baseURL, suffix, at, err := utils.ParseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &VMStorage{
		c:                c,
		baseURL:          baseURL,
		suffix:           suffix,
		basicAuthUser:    basicAuthUser,
		basicAuthPass:    basicAuthPass,
		defaultAuthToken: at,
	}, nil
}

// Query reads metrics from datasource by given query
func (s *VMStorage) Query(ctx context.Context, at *auth.Token, query string) ([]Metric, error) {
	const (
		statusSuccess, statusError, rtVector = "success", "error", "vector"
	)
	if at == nil {
		at = s.defaultAuthToken
	}
	queryURL := fmt.Sprintf("%v/%d:%d/%s%s%s", s.baseURL, at.AccountID, at.ProjectID, s.suffix, queryPath, url.QueryEscape(query))
	fmt.Printf("*** queryURL: %s\n", queryURL)
	req, err := http.NewRequest("POST", queryURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.basicAuthPass != "" {
		req.SetBasicAuth(s.basicAuthUser, s.basicAuthPass)
	}
	resp, err := s.c.Do(req.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("error getting response from %s: %w", req.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("datasource returns unexpected response code %d for %s with err %w. Reponse body %s", resp.StatusCode, req.URL, err, body)
	}
	r := &response{}
	if err := json.NewDecoder(resp.Body).Decode(r); err != nil {
		return nil, fmt.Errorf("error parsing metrics for %s: %w", req.URL, err)
	}
	if r.Status == statusError {
		return nil, fmt.Errorf("response error, query: %s, errorType: %s, error: %s", req.URL, r.ErrorType, r.Error)
	}
	if r.Status != statusSuccess {
		return nil, fmt.Errorf("unknown status: %s, Expected success or error ", r.Status)
	}
	if r.Data.ResultType != rtVector {
		return nil, fmt.Errorf("unknown restul type:%s. Expected vector", r.Data.ResultType)
	}
	return r.metrics()
}
