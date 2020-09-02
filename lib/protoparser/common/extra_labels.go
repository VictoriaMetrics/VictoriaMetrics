package common

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// GetExtraLabels extracts name:value labels from `extra_label=name=value` query args from req.
func GetExtraLabels(req *http.Request) ([]prompbmarshal.Label, error) {
	q := req.URL.Query()
	var result []prompbmarshal.Label
	for _, label := range q["extra_label"] {
		tmp := strings.SplitN(label, "=", 2)
		if len(tmp) != 2 {
			return nil, fmt.Errorf("`extra_label` query arg must have the format `name=value`; got %q", label)
		}
		result = append(result, prompbmarshal.Label{
			Name:  tmp[0],
			Value: tmp[1],
		})
	}
	return result, nil
}
