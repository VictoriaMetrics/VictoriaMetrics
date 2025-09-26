package zabbixconnector

import (
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fastjson"
)

var (
	addGroups          = flag.String("zabbixconnector.addGroups", "", "Enable adding Zabbix host groups to labels and set value for these labels.")
	addEmptyTags       = flag.String("zabbixconnector.addEmptyTags", "", "Enable adding Zabbix tags without values to labels and set value for these labels.")
	mergeDuplicateTags = flag.String("zabbixconnector.mergeDuplicateTags", "", "Enable merging of duplicate Zabbix tag values and set a separator for the values of these labels.")
)

// Rows represents Zabbix Connector lines.
type Rows struct {
	Rows []Row
}

// Reset resets rs.
func (rs *Rows) Reset() {
	for i := range rs.Rows {
		rs.Rows[i].reset()
	}
	rs.Rows = rs.Rows[:0]
}

// Unmarshal unmarshals Zabbix Connector lines from s.
func (rs *Rows) Unmarshal(s string) {
	rs.Rows = unmarshalRows(rs.Rows[:0], s)
}

// Row is a single Zabbix Connector line
type Row struct {
	Tags      []Tag
	Value     float64
	Timestamp int64
}

func (r *Row) reset() {
	tags := r.Tags
	for i := range tags {
		tags[i].reset()
	}
	r.Tags = tags[:0]

	r.Value = 0
	r.Timestamp = 0
}

func (r *Row) addTag() *Tag {
	dst := r.Tags
	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Tag{})
	}
	tag := &dst[len(dst)-1]
	r.Tags = dst
	return tag
}

func (r *Row) unmarshal(o *fastjson.Value) error {
	r.reset()

	host := o.GetObject("host")
	if host == nil {
		return fmt.Errorf("missing `host` object")
	}

	v := host.Get("host").GetStringBytes()
	if len(v) == 0 {
		return fmt.Errorf("missing `host` element in `host` object")
	}
	tag := r.addTag()
	tag.Key = append(tag.Key[:0], []byte("host")...)
	tag.Value = append(tag.Value[:0], v...)

	v = host.Get("name").GetStringBytes()
	if len(v) == 0 {
		return fmt.Errorf("missing `name` element in `host` object")
	}
	tag = r.addTag()
	tag.Key = append(tag.Key[:0], []byte("hostname")...)
	tag.Value = append(tag.Value[:0], v...)

	v = o.GetStringBytes("name")
	if len(v) == 0 {
		return fmt.Errorf("missing `name` element")
	}
	tag = r.addTag()
	tag.Key = append(tag.Key[:0], []byte("__name__")...)
	tag.Value = append(tag.Value[:0], v...)

	n, err := getFloat64(o, "value")
	if err != nil {
		return fmt.Errorf("missing `value` element, %s", err)
	}
	r.Value = n

	cl, err := getInt64(o, "clock")
	if err != nil {
		return fmt.Errorf("missing `clock` element, %s", err)
	}
	ns, err := getInt64(o, "ns")
	if err != nil {
		return fmt.Errorf("missing `ns` element, %s", err)
	}
	// clock - Number of seconds since Epoch to the moment when value was collected (integer part).
	// ns - Number of nanoseconds to be added to clock to get a precise value collection time.
	//
	// See https://www.zabbix.com/documentation/current/en/manual/appendix/protocols/real_time_export#item-values
	r.Timestamp = cl*1e3 + ns/1e6

	addGroups := []byte(*addGroups)
	if len(addGroups) != 0 {
		groups, err := getArray(o, "groups")
		if err != nil {
			return fmt.Errorf("missing `groups` element, %s", err)
		}
		for _, g := range groups {
			k := g.GetStringBytes()
			if len(k) == 0 {
				continue
			}

			tag = r.addTag()
			tag.Key = append(tag.Key[:0], []byte("group_")...)
			tag.Key = append(tag.Key, k...)
			tag.Value = append(tag.Value[:0], addGroups...)
		}
	}

	addEmptyTags := []byte(*addEmptyTags)
	mergeDuplicateTags := []byte(*mergeDuplicateTags)

	itemTags, err := getArray(o, "item_tags")
	if err != nil {
		return fmt.Errorf("missing `item_tags` element, %s", err)
	}

	if len(mergeDuplicateTags) == 0 { // Do not merge tags
		for _, t := range itemTags {
			k := t.GetStringBytes("tag")
			if len(k) == 0 {
				continue
			}

			v := t.GetStringBytes("value")
			if len(v) == 0 && len(addEmptyTags) == 0 {
				continue
			}
			tag = r.addTag()
			tag.Key = append(tag.Key[:0], []byte("tag_")...)
			tag.Key = append(tag.Key, k...)
			if len(v) == 0 {
				tag.Value = append(tag.Value[:0], addEmptyTags...)
			} else {
				tag.Value = append(tag.Value[:0], v...)
			}
		}
	} else { // Merge Tags
		mapTags := make(map[string][]byte)
		for _, t := range itemTags {
			k := t.GetStringBytes("tag")
			if len(k) == 0 {
				continue
			}

			v := t.GetStringBytes("value")
			if len(v) == 0 && len(addEmptyTags) == 0 {
				continue
			}
			sk := bytesutil.ToUnsafeString(k)
			if _, ok := mapTags[sk]; !ok {
				if len(v) == 0 {
					mapTags[sk] = addEmptyTags
				} else {
					mapTags[sk] = v
				}
			} else {
				mapTags[sk] = append(mapTags[sk], mergeDuplicateTags...)
				if len(v) == 0 {
					mapTags[sk] = append(mapTags[sk], addEmptyTags...)
				} else {
					mapTags[sk] = append(mapTags[sk], v...)
				}
			}
		}

		// Sorting merged tags
		ks := make([]string, len(mapTags))
		i := 0
		for k := range mapTags {
			ks[i] = k
			i++
		}
		sort.Strings(ks)

		for _, k := range ks {
			tag = r.addTag()
			tag.Key = append(tag.Key[:0], []byte("tag_")...)
			tag.Key = append(tag.Key, []byte(k)...)
			tag.Value = append(tag.Value[:0], mapTags[k]...)
		}
	}

	return nil
}

func getFloat64(o *fastjson.Value, k string) (float64, error) {
	v := o.Get(k)
	if v == nil {
		return 0, fmt.Errorf("value is not exist")
	}
	switch v.Type() {
	case fastjson.TypeNumber:
		return v.Float64()
	default:
		return 0, fmt.Errorf("value doesn't contain float64; it contains %s", v.Type())
	}
}

func getInt64(o *fastjson.Value, k string) (int64, error) {
	v := o.Get(k)
	if v == nil {
		return 0, fmt.Errorf("value is not exist")
	}
	switch v.Type() {
	case fastjson.TypeNumber:
		return v.Int64()
	default:
		return 0, fmt.Errorf("value doesn't contain int64; it contains %s", v.Type())
	}
}

func getArray(o *fastjson.Value, k string) ([]*fastjson.Value, error) {
	v := o.Get(k)
	if v == nil {
		return nil, fmt.Errorf("value is not exist")
	}
	switch v.Type() {
	case fastjson.TypeArray:
		return v.Array()
	default:
		return nil, fmt.Errorf("value doesn't contain array; it contains %s", v.Type())
	}
}

// Tag represents metric tag
type Tag struct {
	Key   []byte
	Value []byte
}

func (t *Tag) reset() {
	t.Key = t.Key[:0]
	t.Value = t.Value[:0]
}

func unmarshalRows(dst []Row, s string) []Row {
	for len(s) > 0 {
		n := strings.IndexByte(s, '\n')
		if n < 0 {
			// The last line.
			return unmarshalRow(dst, s)
		}
		dst = unmarshalRow(dst, s[:n])
		s = s[n+1:]
	}
	return dst
}

var jsonParserPool fastjson.ParserPool

func unmarshalRow(dst []Row, s string) []Row {
	p := jsonParserPool.Get()
	defer jsonParserPool.Put(p)

	if len(s) > 0 && s[len(s)-1] == '\r' {
		s = s[:len(s)-1]
	}
	if len(s) == 0 {
		return dst
	}

	v, err := p.Parse(s)
	if err != nil {
		logger.Errorf("skipping json line %q because of error: %s", s, err)
		invalidLines.Inc()
		return dst
	}

	// Skip non numeric metrics
	zt, err := getInt64(v, "type")
	if err != nil {
		logger.Errorf("skipping json line %q because of error: missing `type` element, %s", s, err)
		invalidLines.Inc()
		return dst
	}
	if zt != 0 && zt != 3 {
		invalidLines.Inc()
		return dst
	}

	if cap(dst) > len(dst) {
		dst = dst[:len(dst)+1]
	} else {
		dst = append(dst, Row{})
	}
	r := &dst[len(dst)-1]
	if err := r.unmarshal(v); err != nil {
		dst = dst[:len(dst)-1]
		logger.Errorf("skipping json line %q because of error: %s", s, err)
		invalidLines.Inc()
	}
	return dst
}

var invalidLines = metrics.NewCounter(`vm_rows_invalid_total{type="zabbixconnector"}`)
