// vmauth-oss

package ipfilters

import (
	"fmt"
	"net/http"
)

// Init initializes ip filters checking
func Init(filters *IPLists) error {
	if filters == nil {
		return nil
	}

	return fmt.Errorf("the ip_filters section is not available in the opensource version of vmauth")
}

// CheckRequest checks if request is allowed
// returns nil if request is allowed
// returns error with description if request is not allowed
func CheckRequest(req *http.Request) error {
	return nil
}
