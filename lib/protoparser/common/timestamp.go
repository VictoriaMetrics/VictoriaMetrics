package common

import (
	"fmt"
	"net/http"
	"strconv"
)

// GetTimestamp extracts unix timestamp from `timestamp` query arg.
//
// It returns 0 if there is no `timestamp` query arg.
func GetTimestamp(req *http.Request) (int64, error) {
	ts := req.FormValue("timestamp")
	if len(ts) == 0 {
		return 0, nil
	}
	timestamp, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse `timestamp=%s` query arg: %w", ts, err)
	}
	// Convert seconds to milliseconds.
	return int64(timestamp * 1e3), nil
}
