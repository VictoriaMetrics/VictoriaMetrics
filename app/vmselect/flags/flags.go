package flags

import "flag"

var (
	maxTagKeysPerSearch = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned from /api/v1/labels")
)

func GetMaxTagKeysPerSearch() int {
	return *maxTagKeysPerSearch
}
