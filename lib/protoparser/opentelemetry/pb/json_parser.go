package pb

import (
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/valyala/fastjson"
)

// UnmarshalJSONExportMetricsServiceRequest parses OTLP MetricsServiceRequest from given buffer
func UnmarshalJSONExportMetricsServiceRequest(buf []byte, m *ExportMetricsServiceRequest) error {
	v, err := fastjson.ParseBytes(buf)
	if err != nil {
		return err
	}
	return parseExportMetricsRequest(m, v)
}

func parseExportMetricsRequest(dst *ExportMetricsServiceRequest, v *fastjson.Value) error {
	obj, err := v.Object()
	if err != nil {
		return err
	}
	var visitErr error
	obj.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "resourceMetrics", "resource_metrics":
			rmss, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			for _, rm := range rmss {
				prm, err := parseResourceMetrics(rm)
				if err != nil {
					visitErr = err
				}
				dst.ResourceMetrics = append(dst.ResourceMetrics, prm)
			}
		}
	})
	return visitErr
}

func parseResourceMetrics(v *fastjson.Value) (*ResourceMetrics, error) {
	rmObj, err := v.Object()
	if err != nil {
		return nil, fmt.Errorf("cannot parse resourceMetrics as object: %s: %w", v.String(), err)
	}
	var visitErr error
	var rm ResourceMetrics
	rmObj.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "resource":
			rm.Resource, err = parseResource(v)
			if err != nil {
				visitErr = err
				return
			}
		case "scopeMetrics", "scope_metrics":
			smArr, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			for _, smItem := range smArr {
				sm, err := parseScopeMetrics(smItem)
				if err != nil {
					visitErr = err
					return
				}
				rm.ScopeMetrics = append(rm.ScopeMetrics, sm)
			}

		case "schemaUrl", "schema_url":
			rm.SchemaUrl = string(v.GetStringBytes())
		}
	})
	return &rm, visitErr
}

func parseScopeMetrics(v *fastjson.Value) (*ScopeMetrics, error) {
	smObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var sm ScopeMetrics
	var visitErr error
	smObject.Visit(func(key []byte, v *fastjson.Value) {
		switch string(key) {
		case "metrics":
			ma, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			for _, mv := range ma {
				m, err := parseMetric(mv)
				if err != nil {
					visitErr = err
					return
				}
				sm.Metrics = append(sm.Metrics, m)
			}
		case "scope":
			// TODO add parser, current we do not using scope values
		case "schemaUrl", "schema_url":
			// TODO add parser, current we do not using schemaUrls
		}
	})
	return &sm, visitErr
}

func parseMetric(v *fastjson.Value) (*Metric, error) {
	mObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var m Metric
	var visitErr error
	mObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "name":
			m.Name = string(v.GetStringBytes())
		case "description":
			m.Description = string(v.GetStringBytes())
		case "unit":
			m.Unit = string(v.GetStringBytes())
		case "sum":
			m.Data, visitErr = parseSumMetric(v)
		case "gauge":
			m.Data, visitErr = parseGaugeMetric(v)
		case "histogram":
			m.Data, visitErr = parseHistogramMetric(v)
		case "exponential_histogram", "exponentialHistogram":
			// TODO add exponentionalHistogram currently its not supported
		case "summary":
			m.Data, visitErr = parseSummaryMetric(v)
		}
	})

	return &m, visitErr
}

func parseSummaryMetric(v *fastjson.Value) (*Metric_Summary, error) {
	smObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	sm := &Metric_Summary{Summary: &Summary{}}
	var visitErr error
	smObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "data_points", "dataPoints":
			dpsArr, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			dps := make([]*SummaryDataPoint, 0, len(dpsArr))
			for _, dpsItem := range dpsArr {
				dp, err := parseSummaryDatapoint(dpsItem)
				if err != nil {
					visitErr = err
					return
				}
				dps = append(dps, dp)
			}

			sm.Summary.DataPoints = dps
		}
	})
	return sm, visitErr
}

func parseSummaryDatapoint(v *fastjson.Value) (*SummaryDataPoint, error) {
	sdObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var visitErr error
	var sd SummaryDataPoint
	sdObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "timeUnixNano", "time_unix_nano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.TimeUnixNano = uint64(val)
		case "start_time_unix_nano", "startTimeUnixNano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.StartTimeUnixNano = uint64(val)
		case "attributes":
			attrs, err := parseAttributes(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.Attributes = attrs
		case "count":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.Count = uint64(val)
		case "sum":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.Sum = val
		case "quantile_values", "quantileValues":
			qvArr, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			qvs := make([]*SummaryDataPoint_ValueAtQuantile, 0, len(qvArr))
			for _, qvItem := range qvArr {
				quantile := qvItem.Get("quantile")
				if quantile == nil {
					visitErr = fmt.Errorf("madradory key `quantile` is missing for quantile_values: %s", qvItem.String())
					return
				}
				value := qvItem.Get("value")
				if quantile == nil {
					visitErr = fmt.Errorf("madradory key `value` is missing for quantile_values: %s", qvItem.String())
					return
				}
				qvs = append(qvs, &SummaryDataPoint_ValueAtQuantile{Value: value.GetFloat64(), Quantile: quantile.GetFloat64()})
			}
			sd.QuantileValues = qvs
		case "flags":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			sd.Flags = uint32(val)
		}
	})
	return &sd, visitErr
}

func parseHistogramMetric(v *fastjson.Value) (*Metric_Histogram, error) {
	hmObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	mh := &Metric_Histogram{Histogram: &Histogram{}}
	var visitErr error
	hmObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "data_points", "dataPoints":
			dps, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			hdps := make([]*HistogramDataPoint, 0, len(dps))
			for _, dpItem := range dps {
				dp, err := parseHistogramDataPoint(dpItem)
				if err != nil {
					visitErr = err
					return
				}
				hdps = append(hdps, dp)
			}
			mh.Histogram.DataPoints = hdps
		case "aggregation_temporality", "aggregationTemporality":
			val, err := readEnumValue(v, AggregationTemporality_value)
			if err != nil {
				visitErr = err
				return
			}
			mh.Histogram.AggregationTemporality = AggregationTemporality(val)
		}
	})
	return mh, visitErr
}

func parseHistogramDataPoint(v *fastjson.Value) (*HistogramDataPoint, error) {
	hdpObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var visitErr error
	var hdp HistogramDataPoint
	hdpObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "timeUnixNano", "time_unix_nano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.TimeUnixNano = uint64(val)
		case "start_time_unix_nano", "startTimeUnixNano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.StartTimeUnixNano = uint64(val)
		case "attributes":
			attrs, err := parseAttributes(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Attributes = attrs

		case "count":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Count = uint64(val)
		case "sum":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Sum = &val
		case "bucket_counts", "bucketCounts":
			bucketsArr, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			bucketsCount := make([]uint64, 0, len(bucketsArr))
			for _, bucketsItem := range bucketsArr {
				val, err := parseInt64(bucketsItem)
				if err != nil {
					visitErr = err
					return
				}
				bucketsCount = append(bucketsCount, uint64(val))
			}
			hdp.BucketCounts = bucketsCount
		case "explicit_bounds", "explicitBounds":
			boundsArr, err := v.Array()
			if err != nil {
				visitErr = err
				return
			}
			bounds := make([]float64, 0, len(boundsArr))
			for _, boundsItem := range boundsArr {
				val, err := parseFloatValue(boundsItem)
				if err != nil {
					visitErr = err
					return
				}
				bounds = append(bounds, val)
			}
			hdp.ExplicitBounds = bounds

		case "exemplars":
			// TODO add exemplars parse, we do not use it
		case "flags":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Flags = uint32(val)
		case "max":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Max = &val
		case "min":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			hdp.Min = &val
		}
	})
	return &hdp, visitErr
}

func parseGaugeMetric(v *fastjson.Value) (*Metric_Gauge, error) {
	gaugeObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var visitErr error
	gaugeMetric := &Metric_Gauge{Gauge: &Gauge{}}
	gaugeObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "data_points", "dataPoints":
			dataPoints, err := parseNumberDatapoints(v)
			if err != nil {
				visitErr = err
				return
			}
			gaugeMetric.Gauge.DataPoints = dataPoints
		}
	})
	return gaugeMetric, visitErr
}

func parseSumMetric(v *fastjson.Value) (*Metric_Sum, error) {
	sumObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	sm := Metric_Sum{Sum: &Sum{}}
	var visitErr error
	sumObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "aggregation_temporality", "aggregationTemporality":
			val, err := readEnumValue(v, AggregationTemporality_value)
			if err != nil {
				visitErr = err
				return
			}
			sm.Sum.AggregationTemporality = AggregationTemporality(val)
		case "is_monotonic", "isMonotonic":
			sm.Sum.IsMonotonic = v.GetBool()
		case "data_points", "dataPoints":
			dataPoints, err := parseNumberDatapoints(v)
			if err != nil {
				visitErr = err
				return
			}
			sm.Sum.DataPoints = dataPoints
		}
	})
	return &sm, visitErr
}

func parseNumberDatapoints(v *fastjson.Value) ([]*NumberDataPoint, error) {
	dpss, err := v.Array()
	if err != nil {
		return nil, err
	}
	dataPoints := make([]*NumberDataPoint, 0, len(dpss))
	for _, dpItem := range dpss {
		dp, err := parseNumberDataPoint(dpItem)
		if err != nil {
			return nil, err
		}
		dataPoints = append(dataPoints, dp)
	}
	return dataPoints, nil
}

func parseNumberDataPoint(v *fastjson.Value) (*NumberDataPoint, error) {
	ndpObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	var point NumberDataPoint
	var visitErr error
	ndpObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "timeUnixNano", "time_unix_nano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			point.TimeUnixNano = uint64(val)
		case "start_time_unix_nano", "startTimeUnixNano":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			point.StartTimeUnixNano = uint64(val)
		case "as_int", "asInt":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			point.Value = &NumberDataPoint_AsInt{
				AsInt: val,
			}
		case "as_double", "asDouble":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			point.Value = &NumberDataPoint_AsDouble{
				AsDouble: val,
			}
		case "attributes":
			attrs, err := parseAttributes(v)
			if err != nil {
				visitErr = err
				return
			}
			point.Attributes = attrs
		case "exemplars":
			// TODO add exemplar parser, currently we do not use it
		case "flags":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			point.Flags = uint32(val)
		}
	})

	return &point, visitErr
}

func readEnumValue(v *fastjson.Value, valueMap map[string]int32) (int32, error) {
	switch v.Type() {
	case fastjson.TypeNumber:
		return int32(v.GetInt64()), nil
	case fastjson.TypeString:
		val, ok := valueMap[string(v.GetStringBytes())]
		if !ok {
			return 0, fmt.Errorf("unsupported enum value: %s", string(v.GetStringBytes()))
		}
		return val, nil
	default:
		return 0, fmt.Errorf("unsupported type: %s for enum, want string or number", v.Type())
	}
}

func parseResource(v *fastjson.Value) (*Resource, error) {
	resourceObject, err := v.Object()
	if err != nil {
		return nil, fmt.Errorf("cannot parse resource as object: %s, err: %w", v.String(), err)
	}
	var r Resource
	var visitErr error
	resourceObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "droppedAttributesCount", "dropped_attributes_count":
			r.DroppedAttributesCount = uint32(v.GetUint64())
		case "attributes":
			attrs, err := parseAttributes(v)
			if err != nil {
				visitErr = err
				return
			}
			r.Attributes = attrs
		}

	})
	return &r, visitErr
}

func parseAttributes(v *fastjson.Value) ([]*KeyValue, error) {
	attrsArr, err := v.Array()
	if err != nil {
		return nil, fmt.Errorf("cannot parse attributes as array: %s, err: %w", v.String(), err)
	}
	attrs := make([]*KeyValue, 0, len(attrsArr))
	for _, att := range attrsArr {
		kv, err := parseKeyValue(att)
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, kv)
	}
	return attrs, nil
}

func parseKeyValue(v *fastjson.Value) (*KeyValue, error) {
	key := v.Get("key")
	if key == nil {
		return nil, fmt.Errorf("missing madratory `key` for KeyValue object: %s", v.String())
	}
	value := v.Get("value")
	if value == nil {
		return nil, fmt.Errorf("missing mandratory `value` for KeyValue object: %s", v.String())
	}

	av, err := parseAnyValue(value)
	if err != nil {
		return nil, err
	}

	return &KeyValue{Key: string(key.GetStringBytes()), Value: av}, nil
}

func parseAnyValue(v *fastjson.Value) (*AnyValue, error) {
	var av AnyValue
	var visitErr error
	valueObject, err := v.Object()
	if err != nil {
		return nil, err
	}
	valueObject.Visit(func(key []byte, v *fastjson.Value) {
		if visitErr != nil {
			return
		}
		switch string(key) {
		case "stringValue", "string_value":
			av.Value = &AnyValue_StringValue{StringValue: string(v.GetStringBytes())}
		case "boolValue", "bool_value":
			av.Value = &AnyValue_BoolValue{
				BoolValue: v.GetBool(),
			}
		case "intValue", "int_value":
			val, err := parseInt64(v)
			if err != nil {
				visitErr = err
				return
			}
			av.Value = &AnyValue_IntValue{IntValue: val}
		case "doubleValue", "double_value":
			val, err := parseFloatValue(v)
			if err != nil {
				visitErr = err
				return
			}
			av.Value = &AnyValue_DoubleValue{DoubleValue: val}
		case "bytesValue", "bytes_value":
			val, err := base64.StdEncoding.DecodeString(string(v.GetStringBytes()))
			if err != nil {
				visitErr = fmt.Errorf("bytesValue: %s has incorrect format - it must be base64 encoded: %w", v.String(), err)
				return
			}
			av.Value = &AnyValue_BytesValue{BytesValue: val}
		case "arrayValue", "array_value":
			values := v.Get("values")
			if values == nil {
				visitErr = fmt.Errorf("cannot find `values` key for arrayValue: %s", v.String())
				return
			}
			vals, err := values.Array()
			if err != nil {
				visitErr = fmt.Errorf("arrayValue: %s has incorrect format - it must be array type: %w", v.String(), err)
				return
			}
			var anyValues ArrayValue
			for _, val := range vals {
				val, err := parseAnyValue(val)
				if err != nil {
					visitErr = err
					return
				}
				anyValues.Values = append(anyValues.Values, val)
			}
			av.Value = &AnyValue_ArrayValue{ArrayValue: &anyValues}

		case "kvlistValue", "kvlist_value":
			kvs := v.Get("values")
			if kvs == nil {
				visitErr = fmt.Errorf("missing madratory key `values` for kvlistValue value: %s", v.String())
				return
			}
			kvsArray, err := kvs.Array()
			if err != nil {
				visitErr = err
				return
			}
			var kvList KeyValueList
			for _, kvItem := range kvsArray {
				kv, err := parseKeyValue(kvItem)
				if err != nil {
					visitErr = err
					return
				}
				kvList.Values = append(kvList.Values, kv)
			}
			av.Value = &AnyValue_KvlistValue{KvlistValue: &kvList}
		}
	})
	return &av, nil
}

func parseFloatValue(v *fastjson.Value) (float64, error) {
	switch v.Type() {
	case fastjson.TypeString:
		val, err := strconv.ParseFloat(string(v.GetStringBytes()), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse string: %s as float64: %w", v.GetStringBytes(), err)
		}
		return val, nil
	case fastjson.TypeNumber:
		return v.GetFloat64(), nil
	default:
		return 0, fmt.Errorf("incorrect type for float64 want Number or String, got: %s ", v.Type())
	}
}

func parseInt64(v *fastjson.Value) (int64, error) {
	switch v.Type() {
	case fastjson.TypeString:
		val, err := strconv.ParseInt(string(v.GetStringBytes()), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse string: %s as int64: %w", v.GetStringBytes(), err)
		}
		return val, nil
	case fastjson.TypeNumber:
		return v.GetInt64(), nil
	default:
		return 0, fmt.Errorf("incorrect type for int64 want Number or String, got: %s ", v.Type())
	}
}
