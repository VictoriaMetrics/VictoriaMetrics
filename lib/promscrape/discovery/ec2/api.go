package ec2

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promscrape/discoveryutils"
)

type apiConfig struct {
	awsConfig          *awsapi.Config
	filtersQueryString string
	port               int

	// A map from AZ name to AZ id.
	azMap     map[string]string
	azMapLock sync.Mutex
}

var configMap = discoveryutils.NewConfigMap()

func getAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	v, err := configMap.Get(sdc, func() (interface{}, error) { return newAPIConfig(sdc) })
	if err != nil {
		return nil, err
	}
	return v.(*apiConfig), nil
}

func newAPIConfig(sdc *SDConfig) (*apiConfig, error) {
	fqs := getFiltersQueryString(sdc.Filters)
	port := 80
	if sdc.Port != nil {
		port = *sdc.Port
	}
	awsCfg, err := awsapi.NewConfig(sdc.Region, sdc.RoleARN, sdc.AccessKey, sdc.SecretKey.String())
	if err != nil {
		return nil, err
	}
	cfg := &apiConfig{
		awsConfig:          awsCfg,
		filtersQueryString: fqs,
		port:               port,
	}
	return cfg, nil
}

func getFiltersQueryString(filters []Filter) string {
	// See how to build filters query string at examples at https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_DescribeInstances.html
	var args []string
	for i, f := range filters {
		args = append(args, fmt.Sprintf("Filter.%d.Name=%s", i+1, url.QueryEscape(f.Name)))
		for j, v := range f.Values {
			args = append(args, fmt.Sprintf("Filter.%d.Value.%d=%s", i+1, j+1, url.QueryEscape(v)))
		}
	}
	return strings.Join(args, "&")
}
