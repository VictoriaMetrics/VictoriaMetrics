package pb

import (
	"testing"
)

func strptr(v string) *string {
	return &v
}

func TestFormatString(t *testing.T) {
	f := func(attr *AnyValue, expected string) {
		t.Helper()
		got := attr.FormatString(true)
		if got != expected {
			t.Fatalf("unexpected attribute string representation, got: %s, want: %s", got, expected)
		}
	}

	f(&AnyValue{
		StringValue: strptr("test1"),
	}, `test1`)
	f(&AnyValue{
		KeyValueList: &KeyValueList{
			Values: []*KeyValue{
				{
					Key: "test1",
					Value: &AnyValue{
						StringValue: strptr("1"),
					},
				},
				{
					Key: "test2",
					Value: &AnyValue{
						StringValue: strptr("2"),
					},
				},
			},
		},
	}, `{"test1":"1","test2":"2"}`)
	f(&AnyValue{
		ArrayValue: &ArrayValue{
			Values: []*AnyValue{
				{
					StringValue: strptr("1"),
				},
				{
					ArrayValue: &ArrayValue{
						Values: []*AnyValue{
							{
								StringValue: strptr("1"),
							},
						},
					},
				},
			},
		},
	}, `["1",["1"]]`)
}
