package storage

import (
	"errors"
	"fmt"
)

const FlagSeriesLimit = "-search.max*"

var ErrSeriesLimit = errors.New(fmt.Sprintf("either narrow down the search or increase %s command-line "+
	"flag values at vmselect; see https://docs.victoriametrics.com/#resource-usage-limits", FlagSeriesLimit))
