package puppetdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

const (
	separator = ","
)

type resource struct {
	Certname    string     `json:"certname"`
	Resource    string     `json:"resource"`
	Type        string     `json:"type"`
	Title       string     `json:"title"`
	Exported    bool       `json:"exported"`
	Tags        []string   `json:"tags"`
	File        string     `json:"file"`
	Environment string     `json:"environment"`
	Parameters  parameters `json:"parameters"`
}

type parameters map[string]interface{}

// addToLabels add Parameters map to existing labels.
// See: https://github.com/prometheus/prometheus/blob/685493187ec5f5734777769f595cf8418d49900d/discovery/puppetdb/resources.go#L39
func (p *parameters) addToLabels(keyPrefix string, m *promutil.Labels) {
	if p == nil {
		return
	}

	for k, v := range *p {
		var labelValue string
		switch value := v.(type) {
		case string:
			labelValue = value
		case bool:
			labelValue = strconv.FormatBool(value)
		case int64:
			labelValue = strconv.FormatInt(value, 10)
		case float64:
			labelValue = strconv.FormatFloat(value, 'g', -1, 64)
		case []string:
			labelValue = separator + strings.Join(value, separator) + separator
		case []interface{}:
			if len(value) == 0 {
				continue
			}
			values := make([]string, len(value))
			for i, v := range value {
				switch value := v.(type) {
				case string:
					values[i] = value
				case bool:
					values[i] = strconv.FormatBool(value)
				case int64:
					values[i] = strconv.FormatInt(value, 10)
				case float64:
					values[i] = strconv.FormatFloat(value, 'g', -1, 64)
				case []string:
					values[i] = separator + strings.Join(value, separator) + separator
				}
			}
			labelValue = strings.Join(values, separator)
		case map[string]interface{}:
			subParameter := parameters(value)
			subParameter.addToLabels(keyPrefix+discoveryutil.SanitizeLabelName(k+"_"), m)
		default:
			continue
		}
		if labelValue == "" {
			continue
		}
		name := discoveryutil.SanitizeLabelName(k)
		m.Add(keyPrefix+name, labelValue)
	}
}

func getResourceList(cfg *apiConfig) ([]resource, error) {
	body := struct {
		Query string `json:"query"`
	}{cfg.query}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	modifyRequestFunc := func(request *http.Request) {
		request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		request.Method = http.MethodPost
	}

	// https://www.puppet.com/docs/puppetdb/8/api/query/v4/overview#pdbqueryv4
	resp, err := cfg.client.GetAPIResponseWithReqParams("/pdb/query/v4", modifyRequestFunc)
	if err != nil {
		return nil, err
	}

	var resources []resource
	if err = json.Unmarshal(resp, &resources); err != nil {
		return nil, err
	}

	return resources, nil
}

func getResourceLabels(resources []resource, cfg *apiConfig) []*promutil.Labels {
	ms := make([]*promutil.Labels, 0, len(resources))

	for _, res := range resources {
		m := promutil.NewLabels(18)

		m.Add("__address__", discoveryutil.JoinHostPort(res.Certname, cfg.port))
		m.Add("__meta_puppetdb_certname", res.Certname)
		m.Add("__meta_puppetdb_environment", res.Environment)
		m.Add("__meta_puppetdb_exported", fmt.Sprintf("%t", res.Exported))
		m.Add("__meta_puppetdb_file", res.File)
		m.Add("__meta_puppetdb_query", cfg.query)
		m.Add("__meta_puppetdb_resource", res.Resource)
		m.Add("__meta_puppetdb_title", res.Title)
		m.Add("__meta_puppetdb_type", res.Type)

		if len(res.Tags) > 0 {
			//discoveryutil.AddTagsToLabels(m, resource.Tags, "__meta_puppetdb_tags", separator)
			m.Add("__meta_puppetdb_tags", separator+strings.Join(res.Tags, separator)+separator)
		}

		// Parameters are not included by default. This should only be enabled
		// on select resources as it might expose secrets on the Prometheus UI
		// for certain resources.
		if cfg.includeParameters {
			res.Parameters.addToLabels("__meta_puppetdb_parameter_", m)
		}

		ms = append(ms, m)
	}

	return ms
}
