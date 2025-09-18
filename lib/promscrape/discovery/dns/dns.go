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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

// SDCheckInterval defines interval for targets refresh.
var SDCheckInterval = flag.Duration("promscrape.dnsSDCheckInterval", 30*time.Second, "Interval for checking for changes in dns. "+
	"This works only if dns_sd_configs is configured in '-promscrape.config' file. "+
	"See https://docs.victoriametrics.com/victoriametrics/sd_configs/#dns_sd_configs for details")

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
func (sdc *SDConfig) GetLabels(_ string) ([]*promutil.Labels, error) {
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
	case "MX":
		ms := getMXAddrLabels(ctx, sdc)
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

func getMXAddrLabels(ctx context.Context, sdc *SDConfig) []*promutil.Labels {
	port := 25
	if sdc.Port != nil {
		port = *sdc.Port
	}
	type result struct {
		name string
		mx   []*net.MX
		err  error
	}
	ch := make(chan result, len(sdc.Names))
	for _, name := range sdc.Names {
		go func(name string) {
			mx, err := netutil.Resolver.LookupMX(ctx, name)
			ch <- result{
				name: name,
				mx:   mx,
				err:  err,
			}
		}(name)
	}
	var ms []*promutil.Labels
	for range sdc.Names {
		r := <-ch
		if r.err != nil {
			logger.Errorf("dns_sd_config: skipping MX lookup for %q because of error: %s", r.name, r.err)
			continue
		}
		for _, mx := range r.mx {
			target := mx.Host
			for strings.HasSuffix(target, ".") {
				target = target[:len(target)-1]
			}
			ms = appendMXLabels(ms, r.name, target, port)
		}
	}
	return ms
}

func getSRVAddrLabels(ctx context.Context, sdc *SDConfig) []*promutil.Labels {
	type result struct {
		name string
		as   []*net.SRV
		err  error
	}
	ch := make(chan result, len(sdc.Names))
	for _, name := range sdc.Names {
		go func(name string) {
			_, as, err := netutil.Resolver.LookupSRV(ctx, "", "", name)
			ch <- result{
				name: name,
				as:   as,
				err:  err,
			}
		}(name)
	}
	var ms []*promutil.Labels
	for range sdc.Names {
		r := <-ch
		if r.err != nil {
			logger.Errorf("dns_sd_config: skipping SRV lookup for %q because of error: %s", r.name, r.err)
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

func getAAddrLabels(ctx context.Context, sdc *SDConfig, lookupType string) ([]*promutil.Labels, error) {
	if sdc.Port == nil {
		return nil, fmt.Errorf("missing `port` in `dns_sd_config` for `type: %s`", lookupType)
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
			ips, err := netutil.Resolver.LookupIPAddr(ctx, name)
			ch <- result{
				name: name,
				ips:  ips,
				err:  err,
			}
		}(name)
	}
	var ms []*promutil.Labels
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

func appendMXLabels(ms []*promutil.Labels, name, target string, port int) []*promutil.Labels {
	addr := discoveryutil.JoinHostPort(target, port)
	m := promutil.NewLabels(3)
	m.Add("__address__", addr)
	m.Add("__meta_dns_name", name)
	m.Add("__meta_dns_mx_record_target", target)
	return append(ms, m)
}

func appendAddrLabels(ms []*promutil.Labels, name, target string, port int) []*promutil.Labels {
	addr := discoveryutil.JoinHostPort(target, port)
	m := promutil.NewLabels(4)
	m.Add("__address__", addr)
	m.Add("__meta_dns_name", name)
	m.Add("__meta_dns_srv_record_target", target)
	m.Add("__meta_dns_srv_record_port", strconv.Itoa(port))
	return append(ms, m)
}
