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

func TestMetricNameMarshalDuplicateKeys(t *testing.T) {
	var mn MetricName
	mn.MetricGroup = []byte("xxx")
	mn.AddTag("foo", "bar")
	mn.AddTag("duplicate", "tag")
	mn.AddTag("duplicate", "tag")
	mn.AddTag("tt", "xx")
	mn.AddTag("duplicate", "tag2")

	var mnExpected MetricName
	mnExpected.MetricGroup = []byte("xxx")
	mnExpected.AddTag("duplicate", "tag")
	mnExpected.AddTag("foo", "bar")
	mnExpected.AddTag("tt", "xx")

	mn.sortTags()
	data := mn.Marshal(nil)
	var mn1 MetricName
	if err := mn1.Unmarshal(data); err != nil {
		t.Fatalf("cannot unmarshal mn %s: %s", &mn, err)
	}
	if !reflect.DeepEqual(&mnExpected, &mn1) {
		t.Fatalf("unexpected mn unmarshaled;\ngot\n%+v\nwant\n%+v", &mn1, &mnExpected)
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

func TestMetricNameCopyFrom(t *testing.T) {
	var from MetricName
	from.MetricGroup = []byte("group")
	from.AddTag("key", "value")

	var to MetricName
	to.CopyFrom(&from)

	var expected MetricName
	expected.MetricGroup = []byte("group")
	expected.AddTag("key", "value")

	if !reflect.DeepEqual(expected, to) {
		t.Fatalf("expecting equal metics exp: %s, got %s", &expected, &to)
	}
}

func TestMetricNameRemoveTagsOn(t *testing.T) {
	var emptyMN MetricName
	emptyMN.MetricGroup = []byte("name")
	emptyMN.AddTag("key", "value")
	emptyMN.RemoveTagsOn(nil)
	if len(emptyMN.MetricGroup) != 0 || len(emptyMN.Tags) != 0 {
		t.Fatalf("expecitng empty metric name got %s", &emptyMN)
	}

	var asIsMN MetricName
	asIsMN.MetricGroup = []byte("name")
	asIsMN.AddTag("key", "value")
	asIsMN.RemoveTagsOn([]string{"__name__", "key"})
	var expAsIsMN MetricName
	expAsIsMN.MetricGroup = []byte("name")
	expAsIsMN.AddTag("key", "value")
	if !reflect.DeepEqual(expAsIsMN, asIsMN) {
		t.Fatalf("expecitng %s got %s", &expAsIsMN, &asIsMN)
	}

	var mn MetricName
	mn.MetricGroup = []byte("name")
	mn.AddTag("foo", "bar")
	mn.AddTag("baz", "qux")
	mn.RemoveTagsOn([]string{"baz"})
	var expMN MetricName
	expMN.AddTag("baz", "qux")
	if !reflect.DeepEqual(expMN.Tags, mn.Tags) || len(mn.MetricGroup) != len(expMN.MetricGroup) {
		t.Fatalf("expecitng %s got %s", &expMN, &mn)
	}
}

func TestMetricNameRemoveTag(t *testing.T) {
	var mn MetricName
	mn.MetricGroup = []byte("name")
	mn.AddTag("foo", "bar")
	mn.AddTag("baz", "qux")
	mn.RemoveTag("__name__")
	if len(mn.MetricGroup) != 0 {
		t.Fatalf("expecting empty metric group got %s", &mn)
	}
	mn.RemoveTag("foo")
	var expMN MetricName
	expMN.AddTag("baz", "qux")
	if !reflect.DeepEqual(expMN.Tags, mn.Tags) || len(mn.MetricGroup) != len(expMN.MetricGroup) {
		t.Fatalf("expecitng %s got %s", &expMN, &mn)
	}
}

func TestMetricNameRemoveTagsIgnoring(t *testing.T) {
	var mn MetricName
	mn.MetricGroup = []byte("name")
	mn.AddTag("foo", "bar")
	mn.AddTag("baz", "qux")
	mn.RemoveTagsIgnoring([]string{"__name__", "foo"})
	var expMN MetricName
	expMN.AddTag("baz", "qux")
	if !reflect.DeepEqual(expMN.Tags, mn.Tags) || len(mn.MetricGroup) != len(expMN.MetricGroup) {
		t.Fatalf("expecitng %s got %s", &expMN, &mn)
	}
}
