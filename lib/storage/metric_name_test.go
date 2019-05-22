package storage

import (
	"fmt"
	"reflect"
	"testing"
)

func TestMetricNameSortTags(t *testing.T) {
	testMetricNameSortTags(t, []string{}, []string{})
	testMetricNameSortTags(t, []string{"foo"}, []string{"foo"})
	testMetricNameSortTags(t, []string{"job"}, []string{"job"})
	testMetricNameSortTags(t, []string{"server"}, []string{"server"})
	testMetricNameSortTags(t, []string{"host", "foo", "bar", "service"}, []string{"service", "host", "bar", "foo"})
	testMetricNameSortTags(t, []string{"model", "foo", "job", "host", "server", "instance"},
		[]string{"job", "model", "instance", "host", "server", "foo"})
}

func testMetricNameSortTags(t *testing.T, tags, expectedTags []string) {
	t.Helper()

	var mn MetricName
	for _, t := range tags {
		mn.AddTag(t, "")
	}
	mn.sortTags()

	resultTags := []string{}
	for i := range mn.Tags {
		resultTags = append(resultTags, string(mn.Tags[i].Key))
	}
	if !reflect.DeepEqual(resultTags, expectedTags) {
		t.Fatalf("unexpected resultTags\ngot\n%q\nwant\n%q", resultTags, expectedTags)
	}
}

func TestMetricNameMarshalUnmarshal(t *testing.T) {
	for i := 0; i < 10; i++ {
		for tagsCount := 0; tagsCount < 10; tagsCount++ {
			var mn MetricName
			for j := 0; j < tagsCount; j++ {
				key := fmt.Sprintf("key_%d_%d_\x00\x01\x02", i, j)
				value := fmt.Sprintf("\x02\x00\x01value_%d_%d", i, j)
				mn.AddTag(key, value)
			}
			mn.sortTags()
			data := mn.Marshal(nil)
			var mn1 MetricName
			if err := mn1.Unmarshal(data); err != nil {
				t.Fatalf("cannot unmarshal mn %s: %s", &mn, err)
			}
			if !reflect.DeepEqual(&mn, &mn1) {
				t.Fatalf("unexpected mn unmarshaled;\ngot\n%+v\nwant\n%+v", &mn1, &mn)
			}

			// Try unmarshaling MetricName without tag value.
			brokenData := marshalTagValue(data, []byte("foobar"))
			if err := mn1.Unmarshal(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName without tag value")
			}

			// Try unmarshaling MetricName with invalid tag key.
			brokenData[len(brokenData)-1] = 123
			if err := mn1.Unmarshal(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName with invalid tag key")
			}

			// Try unmarshaling MetricName with invalid tag value.
			brokenData = marshalTagValue(data, []byte("foobar"))
			brokenData = marshalTagValue(brokenData, []byte("aaa"))
			brokenData[len(brokenData)-1] = 123
			if err := mn1.Unmarshal(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName with invalid tag value")
			}
		}
	}
}

func TestMetricNameMarshalUnmarshalRaw(t *testing.T) {
	for i := 0; i < 10; i++ {
		for tagsCount := 0; tagsCount < 10; tagsCount++ {
			var mn MetricName
			for j := 0; j < tagsCount; j++ {
				key := fmt.Sprintf("key_%d_%d_\x00\x01\x02", i, j)
				value := fmt.Sprintf("\x02\x00\x01value_%d_%d", i, j)
				mn.AddTag(key, value)
			}
			data := mn.marshalRaw(nil)
			var mn1 MetricName
			if err := mn1.unmarshalRaw(data); err != nil {
				t.Fatalf("cannot unmarshal mn %s: %s", &mn, err)
			}
			if !reflect.DeepEqual(&mn, &mn1) {
				t.Fatalf("unexpected mn unmarshaled;\ngot\n%+v\nwant\n%+v", &mn1, &mn)
			}

			// Try unmarshaling MetricName without tag value.
			brokenData := marshalTagValue(data, []byte("foobar"))
			if err := mn1.unmarshalRaw(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName without tag value")
			}

			// Try unmarshaling MetricName with invalid tag key.
			brokenData[len(brokenData)-1] = 123
			if err := mn1.unmarshalRaw(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName with invalid tag key")
			}

			// Try unmarshaling MetricName with invalid tag value.
			brokenData = marshalTagValue(data, []byte("foobar"))
			brokenData = marshalTagValue(brokenData, []byte("aaa"))
			brokenData[len(brokenData)-1] = 123
			if err := mn1.unmarshalRaw(brokenData); err == nil {
				t.Fatalf("expecting non-zero error when unmarshaling MetricName with invalid tag value")
			}
		}
	}
}
