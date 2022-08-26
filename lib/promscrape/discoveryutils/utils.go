package discoveryutils

import (
	"encoding/json"
	"net"
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// SanitizeLabelName replaces anything that doesn't match
// client_label.LabelNameRE with an underscore.
//
// This has been copied from Prometheus sources at util/strutil/strconv.go
func SanitizeLabelName(name string) string {
	m := sanitizedLabelNames.Load().(*sync.Map)
	v, ok := m.Load(name)
	if ok {
		// Fast path - the sanitized label name is found in the cache.
		sp := v.(*string)
		return *sp
	}
	// Slow path - sanitize name and store it in the cache.
	sanitizedName := invalidLabelCharRE.ReplaceAllString(name, "_")
	// Make a copy of name in order to limit memory usage to the name length,
	// since the name may point to bigger string.
	s := string(append([]byte{}, name...))
	sp := &sanitizedName
	m.Store(s, sp)
	n := atomic.AddUint64(&sanitizedLabelNamesLen, 1)
	if n > 100e3 {
		atomic.StoreUint64(&sanitizedLabelNamesLen, 0)
		sanitizedLabelNames.Store(&sync.Map{})
	}
	return sanitizedName
}

var (
	sanitizedLabelNames    atomic.Value
	sanitizedLabelNamesLen uint64

	invalidLabelCharRE = regexp.MustCompile(`[^a-zA-Z0-9_]`)
)

func init() {
	sanitizedLabelNames.Store(&sync.Map{})
}

// JoinHostPort returns host:port.
//
// Host may be dns name, ipv4 or ipv6 address.
func JoinHostPort(host string, port int) string {
	portStr := strconv.Itoa(port)
	return net.JoinHostPort(host, portStr)
}

// SortedLabels represents sorted labels.
type SortedLabels []prompbmarshal.Label

// GetByName returns the label with the given name from sls.
func (sls *SortedLabels) GetByName(name string) string {
	for _, lb := range *sls {
		if lb.Name == name {
			return lb.Value
		}
	}
	return ""
}

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
