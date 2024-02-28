package firehose

import (
	"testing"
)

func TestProcessRequestBody(t *testing.T) {
	data := []byte(`{"records": [{"data": "SGVsbG8sIA=="}, {"data": "d29ybGQh"}]}`)
	err := ProcessRequestBody(&data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "Hello, world!" {
		t.Fatalf("unexpected string: %q", string(data))
	}
}
