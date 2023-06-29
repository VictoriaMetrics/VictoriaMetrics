package loki

import (
	"bytes"
	"strconv"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestProcessProtobufRequest(t *testing.T) {
	body := getProtobufBody(5, 5, 5)

	reader := bytes.NewReader(body)
	_, err := processProtobufRequest(reader, func(timestamp int64, fields []logstorage.Field) {})
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func getProtobufBody(streams, rows, labels int) []byte {
	var pr PushRequest

	for i := 0; i < streams; i++ {
		var st Stream

		st.Labels = `{`
		for j := 0; j < labels; j++ {
			st.Labels += `label_` + strconv.Itoa(j) + `="value_` + strconv.Itoa(j) + `"`
			if j < labels-1 {
				st.Labels += `,`
			}
		}
		st.Labels += `}`

		for j := 0; j < rows; j++ {
			st.Entries = append(st.Entries, Entry{Timestamp: time.Now(), Line: "value_" + strconv.Itoa(j)})
		}

		pr.Streams = append(pr.Streams, st)
	}

	body, _ := pr.Marshal()
	encodedBody := snappy.Encode(nil, body)

	return encodedBody
}
