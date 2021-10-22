package influx

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestSeries_Unmarshal(t *testing.T) {
	tag := func(name, value string) LabelPair {
		return LabelPair{
			Name:  name,
			Value: value,
		}
	}
	series := func(measurement string, lp ...LabelPair) Series {
		return Series{
			Measurement: measurement,
			LabelPairs:  lp,
		}
	}
	testCases := []struct {
		got  string
		want Series
	}{
		{
			got:  "cpu",
			want: series("cpu"),
		},
		{
			got:  "cpu,host=localhost",
			want: series("cpu", tag("host", "localhost")),
		},
		{
			got:  "cpu,host=localhost,instance=instance",
			want: series("cpu", tag("host", "localhost"), tag("instance", "instance")),
		},
		{
			got:  `fo\,bar\=baz,x\=\b=\\a\,\=\q\ `,
			want: series("fo,bar=baz", tag(`x=\b`, `\a,=\q `)),
		},
		{
			got:  "cpu,host=192.168.0.1,instance=fe80::fdc8:5e36:c2c6:baac%utun1",
			want: series("cpu", tag("host", "192.168.0.1"), tag("instance", "fe80::fdc8:5e36:c2c6:baac%utun1")),
		},
		{
			got: `cpu,db=db1,host=localhost,server=host\=localhost\ user\=user\ `,
			want: series("cpu", tag("db", "db1"),
				tag("host", "localhost"), tag("server", "host=localhost user=user ")),
		},
	}
	for _, tc := range testCases {
		s := Series{}
		if err := s.unmarshal(tc.got); err != nil {
			t.Fatalf("%q: unmarshal err: %s", tc.got, err)
		}
		if !reflect.DeepEqual(s, tc.want) {
			t.Fatalf("%q: expected\n%#v\nto be equal\n%#v", tc.got, s, tc.want)
		}
	}
}

func TestToFloat64(t *testing.T) {
	f := func(in interface{}, want float64) {
		t.Helper()
		got, err := toFloat64(in)
		if err != nil {
			t.Fatalf("unexpected err: %s", err)
		}
		if got != want {
			t.Errorf("got %v; want %v", got, want)
		}
	}
	f("123.4", 123.4)
	f(float64(123.4), 123.4)
	f(float32(12), 12)
	f(123, 123)
	f(true, 1)
	f(false, 0)
	f(json.Number("123456.789"), 123456.789)

	_, err := toFloat64("text")
	if err == nil {
		t.Fatalf("expected to get err; got nil instead")
	}
}
