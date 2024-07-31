package firehose

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

// WriteSuccessResponse writes success response for AWS Firehose request.
//
// See https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
func WriteSuccessResponse(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Amz-Firehose-Request-Id")
	if requestID == "" {
		// This isn't a AWS firehose request - just return an empty response in this case.
		w.WriteHeader(http.StatusOK)
		return
	}

	body := fmt.Sprintf(`{"requestId":%s,"timestamp":%d}`, stringsutil.JSONString(requestID), time.Now().UnixMilli())

	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.Write([]byte(body))
}
