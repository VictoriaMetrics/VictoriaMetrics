package discoveryutils

import (
	"encoding/json"
	"net"
	"regexp"
	"sort"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// SanitizeLabelName replaces anything that doesn't match
// client_label.LabelNameRE with an underscore.
//
// This has been copied from Prometheus sources at util/strutil/strconv.go
func SanitizeLabelName(name string) string {
	return invalidLabelCharRE.ReplaceAllString(name, "_")
}

var (
	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

// JoinHostPort returns host:port.
//
// Host may be dns name, ipv4 or ipv6 address.
func JoinHostPort(host string, port int) string {
	portStr := strconv.Itoa(port)
	return net.JoinHostPort(host, portStr)
}

// SortedLabels represents sorted labels.
type SortedLabels []prompbmarshal.Label

// UnmarshalJSON unmarshals JSON from data.
func (sls *SortedLabels) UnmarshalJSON(data []byte) error {
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*sls = GetSortedLabels(m)
	return nil
}

// GetSortedLabels returns SortedLabels built from m.
func GetSortedLabels(m map[string]string) SortedLabels {
	a := make([]prompbmarshal.Label, 0, len(m))
	for k, v := range m {
		a = append(a, prompbmarshal.Label{
			Name:  k,
			Value: v,
		})
	}
	sort.Slice(a, func(i, j int) bool {
		return a[i].Name < a[j].Name
	})
	return a
}
