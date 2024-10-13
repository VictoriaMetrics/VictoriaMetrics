package puppetdb

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

const (
	separator = ","
)

var matchContentTypeRegex = regexp.MustCompile(`^(?i:application\/json(;\s*charset=("utf-8"|utf-8))?)$`)

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

// toLabels convert Parameters map into label-value map.
// See: https://github.com/prometheus/prometheus/blob/685493187ec5f5734777769f595cf8418d49900d/discovery/puppetdb/resources.go#L39
func (p *parameters) toLabels() map[string]string {
	if p == nil {
		return nil
	}
	m := make(map[string]string, len(*p))

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
			prefix := discoveryutils.SanitizeLabelName(k + "_")
			for subk, subv := range subParameter.toLabels() {
				m[prefix+subk] = subv
			}
		default:
			continue
		}
		if labelValue == "" {
			continue
		}
		name := discoveryutils.SanitizeLabelName(k)
		m[name] = labelValue
	}
	return m
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
		request.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Content-Type", "application/json")
		request.Method = http.MethodPost
	}

	var responseContentType string
	inspectResponseFunc := func(resp *http.Response) {
		responseContentType = resp.Header.Get("Content-Type")
	}

	// https://www.puppet.com/docs/puppetdb/8/api/query/v4/overview#pdbqueryv4
	resp, err := cfg.client.GetAPIResponseWithParamsCtx(cfg.client.Context(), "/pdb/query/v4", modifyRequestFunc, inspectResponseFunc)
	if err != nil {
		return nil, err
	}

	if !matchContentTypeRegex.MatchString(responseContentType) {
		return nil, fmt.Errorf("unsupported content type %s", responseContentType)
	}

	var resources []resource

	if err = json.Unmarshal(resp, &resources); err != nil {
		return nil, err
	}

	return resources, nil
}

func getResourceLabels(resources []resource, cfg *apiConfig) []*promutils.Labels {
	ms := make([]*promutils.Labels, 0, len(resources))

	for _, resource := range resources {
		m := promutils.NewLabels(18)

		m.Add("__address__", discoveryutils.JoinHostPort(resource.Certname, cfg.port))
		m.Add("__meta_puppetdb_query", cfg.query)
		m.Add("__meta_puppetdb_certname", resource.Certname)
		m.Add("__meta_puppetdb_resource", resource.Resource)
		m.Add("__meta_puppetdb_type", resource.Type)
		m.Add("__meta_puppetdb_title", resource.Title)
		m.Add("__meta_puppetdb_exported", fmt.Sprintf("%t", resource.Exported))
		m.Add("__meta_puppetdb_file", resource.File)
		m.Add("__meta_puppetdb_environment", resource.Environment)

		if len(resource.Tags) > 0 {
			//discoveryutils.AddTagsToLabels(m, resource.Tags, "__meta_puppetdb_tags", separator)
			m.Add("__meta_puppetdb_tags", separator+strings.Join(resource.Tags, separator)+separator)
		}

		// Parameters are not included by default. This should only be enabled
		// on select resources as it might expose secrets on the Prometheus UI
		// for certain resources.
		if cfg.includeParameters {
			for k, v := range resource.Parameters.toLabels() {
				m.Add("__meta_puppetdb_parameter_"+k, v)
			}
		}

		ms = append(ms, m)
	}

	return ms
}
