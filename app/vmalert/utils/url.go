package utils

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
)

func ParseURL(s string) (string, string, *auth.Token, error) {
	// find scheme
	var scheme string
	n := strings.Index(s, "://")
	if n < 0 {
		scheme = "http"
	}
	scheme = s[:n]
	s = s[n+3:]
	// find host
	n = strings.IndexByte(s, '/')
	if n < 0 {
		return "", "", nil, fmt.Errorf("cannot find host")
	}
	host := s[:n]
	s = s[n+1:]
	// find suffix
	n = strings.IndexByte(s, '/')
	if n < 0 {
		return "", "", nil, fmt.Errorf("cannot find prefix")
	}
	prefix := s[:n]
	s = s[n+1:]
	baseURL := fmt.Sprintf("%s://%s/%s", scheme, host, prefix)

	// find auth token and suffix
	var accountID uint32 = 0
	var projectID uint32 = 0
	suffix := ""
	if len(s) > 0 {
		n = strings.IndexByte(s, '/')
		if n < 0 {
			return "", "", nil, fmt.Errorf("invalid auth token: %s", s)
		}
		atStr := s[:n]
		s := s[n+1:]
		atList := strings.Split(atStr, ":")
		switch len(atList) {
		case 1, 2:
		default:
			return "", "", nil, fmt.Errorf("invalid auth token: %s", atStr)
		}
		for idx, item := range atList {
			if idx == 0 {
				// parse AccountID
				accountIDUint64, err := strconv.ParseUint(item, 10, 0)
				if err != nil {
					return "", "", nil, fmt.Errorf("invalid accoutID: %s", atStr)
				}
				accountID = uint32(accountIDUint64)
			} else {
				// parse projectID
				projectIDUint64, err := strconv.ParseUint(item, 10, 0)
				if err != nil {
					return "", "", nil, fmt.Errorf("invalid projectID: %s", atStr)
				}
				projectID = uint32(projectIDUint64)
			}
		}
		if len(s) > 0 {
			if s[len(s)-1] == '/' {
				s = s[:len(s)-1]
			}
			suffix = s
		}
	}
	at := auth.Token{
		AccountID: accountID,
		ProjectID: projectID,
	}

	return baseURL, suffix, &at, nil
}
