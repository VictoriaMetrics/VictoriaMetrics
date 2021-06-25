package dns

import (
	"context"
	"flag"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.dnsSDCheckInterval", 30*time.Second, "Interval for checking for changes in dns. "+
	"This works only if dns_sd_configs is configured in '-promscrape.config' file. "+
	"See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config for details")

// SDConfig represents service discovery config for DNS.
//
// See https://prometheus.io/docs/prometheus/latest/configuration/configuration/#dns_sd_config
type SDConfig struct {
	Names []string `yaml:"names"`
	Type  string   `yaml:"type,omitempty"`
	Port  *int     `yaml:"port,omitempty"`
	// RefreshInterval time.Duration `yaml:"refresh_interval"`
	// refresh_interval is obtained from `-promscrape.dnsSDCheckInterval` command-line option.
}

// GetLabels returns DNS labels according to sdc.
func (sdc *SDConfig) GetLabels(baseDir string) ([]map[string]string, error) {
	if len(sdc.Names) == 0 {
		return nil, fmt.Errorf("`names` cannot be empty in `dns_sd_config`")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	typ := sdc.Type
	if typ == "" {
		typ = "SRV"
	}
	typ = strings.ToUpper(typ)
	switch typ {
	case "SRV":
		ms := getSRVAddrLabels(ctx, sdc)
		return ms, nil
	case "A", "AAAA":
		return getAAddrLabels(ctx, sdc, typ)
	default:
		return nil, fmt.Errorf("unexpected `type` in `dns_sd_config`: %q; supported values: SRV, A, AAAA", typ)
	}
}

// MustStop stops further usage for sdc.
func (sdc *SDConfig) MustStop() {
	// nothing to do
}

func getSRVAddrLabels(ctx context.Context, sdc *SDConfig) []map[string]string {
	type result struct {
		name string
		as   []*net.SRV
		err  error
	}
	ch := make(chan result, len(sdc.Names))
	for _, name := range sdc.Names {
		go func(name string) {
			_, as, err := resolver.LookupSRV(ctx, "", "", name)
			ch <- result{
				name: name,
				as:   as,
				err:  err,
			}
		}(name)
	}
	var ms []map[string]string
	for range sdc.Names {
		r := <-ch
		if r.err != nil {
			logger.Errorf("error in SRV lookup for %q; skipping it; error: %s", r.name, r.err)
			continue
		}
		for _, a := range r.as {
			target := a.Target
			for strings.HasSuffix(target, ".") {
				target = target[:len(target)-1]
			}
			ms = appendAddrLabels(ms, r.name, target, int(a.Port))
		}
	}
	return ms
}

func getAAddrLabels(ctx context.Context, sdc *SDConfig, lookupType string) ([]map[string]string, error) {
	if sdc.Port == nil {
		return nil, fmt.Errorf("missing `port` in `dns_sd_config`")
	}
	port := *sdc.Port
	type result struct {
		name string
		ips  []net.IPAddr
		err  error
	}
	ch := make(chan result, len(sdc.Names))
	for _, name := range sdc.Names {
		go func(name string) {
			ips, err := resolver.LookupIPAddr(ctx, name)
			ch <- result{
				name: name,
				ips:  ips,
				err:  err,
			}
		}(name)
	}
	var ms []map[string]string
	for range sdc.Names {
		r := <-ch
		if r.err != nil {
			logger.Errorf("error in %s lookup for %q: %s", lookupType, r.name, r.err)
			continue
		}
		for _, ip := range r.ips {
			isIPv4 := ip.IP.To4() != nil
			if lookupType == "AAAA" && isIPv4 || lookupType == "A" && !isIPv4 {
				continue
			}
			ms = appendAddrLabels(ms, r.name, ip.IP.String(), port)
		}
	}
	return ms, nil
}

func appendAddrLabels(ms []map[string]string, name, target string, port int) []map[string]string {
	addr := discoveryutils.JoinHostPort(target, port)
	m := map[string]string{
		"__address__":                  addr,
		"__meta_dns_name":              name,
		"__meta_dns_srv_record_target": target,
		"__meta_dns_srv_record_port":   strconv.Itoa(port),
	}
	return append(ms, m)
}

var resolver = &net.Resolver{
	PreferGo:     true,
	StrictErrors: true,
}
