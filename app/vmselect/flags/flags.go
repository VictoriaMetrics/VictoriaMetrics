package flags

import "flag"

var (
	maxTagKeysPerSearch   = flag.Int("search.maxTagKeys", 100e3, "The maximum number of tag keys returned from /api/v1/labels and Graphite /tags, /tags/autoComplete/*, /tags/findSeries API")
	maxTagValuesPerSearch = flag.Int("search.maxTagValues", 100e3, "The maximum number of tag values returned from /api/v1/label/<label_name>/values and Graphite /tags/<tag_name> API")
)

// GetMaxTagKeysPerSearch returns search.maxTagKeys flag value
func GetMaxTagKeysPerSearch() int {
	return *maxTagKeysPerSearch
}

// GetMaxTagValuesPerSearch returns search.maxTagValues flag value
func GetMaxTagValuesPerSearch() int {
	return *maxTagValuesPerSearch
}
