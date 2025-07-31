package protoparserutil

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// GetExtraLabels extracts name:value labels from `extra_label=name=value` query args from req.
//
// It also extracts Pushgateways-compatible extra labels from req.URL.Path
// according to https://github.com/prometheus/pushgateway#url .
func GetExtraLabels(req *http.Request) ([]prompb.Label, error) {
	labels, err := getPushgatewayLabels(req.URL.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pushgateway-style labels from %q: %w", req.URL.Path, err)
	}
	q := req.URL.Query()
	for _, label := range q["extra_label"] {
		tmp := strings.SplitN(label, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("`extra_label` query arg must have the format `name=value`; got %q", label)
		}
		labels = append(labels, prompb.Label{
			Name:  tmp[0],
			Value: tmp[1],
		})
	}
	return labels, nil
}

func getPushgatewayLabels(path string) ([]prompb.Label, error) {
	n := strings.Index(path, "/metrics/job")
	if n < 0 {
		return nil, nil
	}
	s := path[n+len("/metrics/"):]
	if !strings.HasPrefix(s, "job/") && !strings.HasPrefix(s, "job@base64/") {
		return nil, nil
	}
	labelsCount := (strings.Count(s, "/") + 1) / 2
	labels := make([]prompb.Label, 0, labelsCount)
	for len(s) > 0 {
		n := strings.IndexByte(s, '/')
		if n < 0 {
			return nil, fmt.Errorf("missing value for label %q", s)
		}
		name := s[:n]
		s = s[n+1:]
		isBase64 := strings.HasSuffix(name, "@base64")
		if isBase64 {
			name = name[:len(name)-len("@base64")]
		}
		var value string
		n = strings.IndexByte(s, '/')
		if n < 0 {
			value = s
			s = ""
		} else {
			value = s[:n]
			s = s[n+1:]
		}
		if isBase64 {
			data, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
			if err != nil {
				return nil, fmt.Errorf("cannot base64-decode value=%q for label=%q: %w", value, name, err)
			}
			value = string(data)
		}
		if len(value) == 0 {
			// Skip labels with empty values
			continue
		}
		labels = append(labels, prompb.Label{
			Name:  name,
			Value: value,
		})
	}
	return labels, nil
}
