package ovhcloud

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
)

func getAuthHeaders(cfg *apiConfig, headers http.Header, endpoint, path string) (http.Header, error) {
	headers = setGeneralHeaders(cfg, headers)

	timeDelta, err := getTimeDelta(cfg)
	if err != nil {
		logger.Errorf("get time delta for auth headers failed: %w", err)
		return nil, err
	}

	timestamp := time.Now().Add(-timeDelta).Unix()
	headers.Set("X-Ovh-Timestamp", strconv.FormatInt(timestamp, 10))
	headers.Add("X-Ovh-Consumer", cfg.consumerKey)

	h := sha1.New()
	h.Write([]byte(fmt.Sprintf("%s+%s+%s+%s+%s+%d",
		cfg.applicationSecret,
		cfg.consumerKey,
		"GET",
		endpoint+path,
		"", // no body contained in any service discovery request, so it's set to empty by default
		timestamp,
	)))
	headers.Set("X-Ovh-Signature", fmt.Sprintf("$1$%x", h.Sum(nil)))
	return headers, nil
}

func setGeneralHeaders(cfg *apiConfig, headers http.Header) http.Header {
	headers.Set("X-Ovh-Application", cfg.applicationKey)
	headers.Set("Accept", "application/json")
	headers.Set("User-Agent", "github.com/VictoriaMetrics/VictoriaMetrics")
	return headers
}

func getServerTime(cfg *apiConfig) (*time.Time, error) {
	resp, err := cfg.client.GetAPIResponseWithReqParams("/auth/time", func(req *http.Request) {
		req.Header = setGeneralHeaders(cfg, req.Header)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get server time from /auth/time: %w", err)
	}
	ts, err := strconv.ParseInt(string(resp), 10, 0)
	if err != nil {
		return nil, fmt.Errorf("parse ovh response to timestamp failed: %w", err)
	}
	serverTime := time.Unix(ts, 0)
	return &serverTime, nil
}

// getTimeDelta calculates the time difference between the host and the remote API.
// It also saves the time difference for future reference.
func getTimeDelta(cfg *apiConfig) (time.Duration, error) {
	d, ok := cfg.timeDelta.Load().(time.Duration)
	if ok {
		return d, nil
	}

	ovhTime, err := getServerTime(cfg)
	if err != nil {
		return 0, err
	}

	d = time.Since(*ovhTime)
	cfg.timeDelta.Store(d)

	return d, nil

}

func parseIPList(ipList []string) ([]netip.Addr, error) {
	var ipAddresses []netip.Addr
	for _, ip := range ipList {
		ipAddr, err := netip.ParseAddr(ip)
		if err != nil {
			ipPrefix, err := netip.ParsePrefix(ip)
			if err != nil {
				return nil, fmt.Errorf("could not parse IP addresses: %s", ip)
			}
			if ipPrefix.IsValid() {
				netmask := ipPrefix.Bits()
				if netmask != 32 {
					continue
				}
				ipAddr = ipPrefix.Addr()
			}
		}
		if ipAddr.IsValid() && !ipAddr.IsUnspecified() {
			ipAddresses = append(ipAddresses, ipAddr)
		}
	}

	if len(ipAddresses) == 0 {
		return nil, errors.New("could not parse IP addresses from list")
	}
	return ipAddresses, nil
}
