package common

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"net/http"
	"strconv"
)

// GetExtraLabels appends auth-token labels to extra labels specified in the request.
func GetExtraLabels(at *auth.Token, req *http.Request) ([]prompbmarshal.Label, error) {
	extraLabels, err := common.GetExtraLabels(req)
	if err != nil {
		return nil, err
	}
	if at != nil {
		extraLabels = append(extraLabels,
			prompbmarshal.Label{
				Name:  "vm_account_id",
				Value: strconv.Itoa(int(at.AccountID)),
			},
			prompbmarshal.Label{
				Name:  "vm_project_id",
				Value: strconv.Itoa(int(at.ProjectID)),
			},
		)
	}
	return extraLabels, nil
}
