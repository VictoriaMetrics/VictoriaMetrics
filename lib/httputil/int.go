package httputil

import (
	"fmt"
	"net/http"
	"strconv"
)

// GetInt returns integer value from the given argKey.
func GetInt(r *http.Request, argKey string) (int, error) {
	argValue := r.FormValue(argKey)
	if len(argValue) == 0 {
		return 0, nil
	}
	n, err := strconv.Atoi(argValue)
	if err != nil {
		return 0, fmt.Errorf("cannot parse integer %q=%q: %w", argKey, argValue, err)
	}
	return n, nil
}
