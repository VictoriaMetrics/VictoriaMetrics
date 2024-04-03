package firehose

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// ProcessRequestBody converts Cloudwatch Stream protobuf metrics HTTP request body delivered via Firehose into OpenTelemetry protobuf message.
//
// See https://docs.aws.amazon.com/AmazonCloudWatch/latest/monitoring/CloudWatch-Metric-Streams.html
//
// It joins decoded "data" fields from "record" list:
//
//	{
//	  "requestId": "<uuid-string>",
//	  "timestamp": <int64-value>,
//	  "records": [
//	    {
//	      "data": "<base64-encoded-payload>"
//	    }
//	  ]
//	}
func ProcessRequestBody(b []byte) ([]byte, error) {
	var req struct {
		Records []struct {
			Data []byte
		}
	}
	if err := json.Unmarshal(b, &req); err != nil {
		return nil, fmt.Errorf("cannot unmarshal Firehose JSON in request body: %s", err)
	}

	var dst []byte
	for _, r := range req.Records {
		for len(r.Data) > 0 {
			messageLength, varIntLength := binary.Uvarint(r.Data)
			if varIntLength > binary.MaxVarintLen32 {
				return nil, fmt.Errorf("failed to parse OpenTelemetry message: invalid variant")
			}
			totalLength := varIntLength + int(messageLength)
			if totalLength > len(r.Data) {
				return nil, fmt.Errorf("failed to parse OpenTelementry message: insufficient length of buffer")
			}
			dst = append(dst, r.Data[varIntLength:totalLength]...)
			r.Data = r.Data[totalLength:]
		}
	}
	return dst, nil
}

// ResponseWriter writes response for AWS Firehose HTTP Endpoint request
// https://docs.aws.amazon.com/firehose/latest/dev/httpdeliveryrequestresponse.html#responseformat
func ResponseWriter(w http.ResponseWriter, ct time.Time, reqID string, err error) {
	var respBody string
	ts := ct.UnixMilli()
	if err == nil {
		respBody = fmt.Sprintf(`{"requestId": %q,"timestamp": %d}`, reqID, ts)
	} else {
		respBody = fmt.Sprintf(`{"requestId": %q,"timestamp": %d,"errorMessage": %q}`, reqID, ts, err)
	}
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte(respBody))
}
