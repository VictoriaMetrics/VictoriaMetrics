package firehose

import (
	"fmt"
	"html"
	"net/http"
	"time"
)

// WriteSuccessResponse writes success response for AWS Firehose request.
//
// See https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
func WriteSuccessResponse(w http.ResponseWriter, r *http.Request) {
	requestID := r.Header.Get("X-Amz-Firehose-Request-Id")
	if requestID == "" {
		// This isn't an AWS firehose request - just return an empty response in this case.
		w.WriteHeader(http.StatusOK)
		return
	}

	requestID = html.EscapeString(requestID)
	body := fmt.Sprintf(`{"requestId":%q,"timestamp":%d}`, requestID, time.Now().UnixMilli())

	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	w.Write([]byte(body))
}
