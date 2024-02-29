package firehose

import (
	"testing"
)

func TestProcessRequestBody(t *testing.T) {
	data := []byte(`{"records": [{"data": "SGVsbG8sIA=="}, {"data": "d29ybGQh"}]}`)
	result, err := ProcessRequestBody(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resultExpected := "Hello, world!"
	if string(result) != resultExpected {
		t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
	}
}
