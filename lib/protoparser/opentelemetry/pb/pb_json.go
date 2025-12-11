package pb

import (
	"fmt"
	"math"

	"github.com/VictoriaMetrics/easyproto"
	"github.com/valyala/fastjson"
)

// decodeArrayValueToJSON decodes a protobuf ArrayValue message into a JSON array represented by fastjson.Value.
func decodeArrayValueToJSON(src []byte, a *fastjson.Arena, fb *fmtBuffer) (*fastjson.Value, error) {
	// message ArrayValue {
	//   repeated AnyValue values = 1;
	// }

	dst := a.NewArray()

	var fc easyproto.FieldContext
	i := 0
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read Value data")
			}

			v, err := decodeAnyValueToJSON(data, a, fb)
			if err != nil {
				return nil, fmt.Errorf("cannot decode AnyValue: %w", err)
			}
			dst.SetArrayItem(i, v)
			i++
		}
	}

	return dst, nil
}

func decodeAnyValueToJSON(src []byte, a *fastjson.Arena, fb *fmtBuffer) (*fastjson.Value, error) {
	// message AnyValue {
	//   oneof value {
	//     string string_value = 1;
	//     bool bool_value = 2;
	//     int64 int_value = 3;
	//     double double_value = 4;
	//     ArrayValue array_value = 5;
	//     KeyValueList kvlist_value = 6;
	//     bytes bytes_value = 7;
	//   }
	// }

	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			stringValue, ok := fc.String()
			if !ok {
				return nil, fmt.Errorf("cannot read StringValue")
			}
			return a.NewString(stringValue), nil
		case 2:
			boolValue, ok := fc.Bool()
			if !ok {
				return nil, fmt.Errorf("cannot read BoolValue")
			}
			if boolValue {
				return a.NewTrue(), nil
			} else {
				return a.NewFalse(), nil
			}
		case 3:
			intValue, ok := fc.Int64()
			if !ok {
				return nil, fmt.Errorf("cannot read IntValue")
			}
			if intValue >= math.MinInt && intValue <= math.MaxInt {
				return a.NewNumberInt(int(intValue)), nil
			}
			return a.NewNumberFloat64(float64(intValue)), nil
		case 4:
			doubleValue, ok := fc.Double()
			if !ok {
				return nil, fmt.Errorf("cannot read DoubleValue")
			}
			return a.NewNumberFloat64(doubleValue), nil
		case 5:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read ArrayValue")
			}
			arr, err := decodeArrayValueToJSON(data, a, fb)
			if err != nil {
				return nil, fmt.Errorf("cannot decode ArrayValue: %w", err)
			}
			return arr, nil
		case 6:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read KeyValueList")
			}
			obj, err := decodeKeyValueListToJSON(data, a, fb)
			if err != nil {
				return nil, fmt.Errorf("cannot decode KeyValueList: %w", err)
			}
			return obj, nil
		case 7:
			bytesValue, ok := fc.Bytes()
			if !ok {
				return nil, fmt.Errorf("cannot read BytesValue")
			}
			v := fb.formatBase64(bytesValue)
			return a.NewString(v), nil
		}
	}
	return a.NewNull(), nil
}

func decodeKeyValueListToJSON(src []byte, a *fastjson.Arena, fb *fmtBuffer) (*fastjson.Value, error) {
	// message KeyValueList {
	//   repeated KeyValue values = 1;
	// }

	dst := a.NewObject()

	var fc easyproto.FieldContext
	for len(src) > 0 {
		var err error
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("cannot read the next field: %w", err)
		}
		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read Value data")
			}

			if err := decodeKeyValueToJSON(data, dst, a, fb); err != nil {
				return nil, fmt.Errorf("cannot decode KeyValue: %w", err)
			}
		}
	}
	return dst, nil
}

func decodeKeyValueToJSON(src []byte, dst *fastjson.Value, a *fastjson.Arena, fb *fmtBuffer) error {
	// message KeyValue {
	//   string key = 1;
	//   AnyValue value = 2;
	// }

	// Decode key
	fieldName, ok, err := easyproto.GetString(src, 1)
	if err != nil {
		return fmt.Errorf("cannot find Key in KeyValue: %w", err)
	}
	if !ok {
		// Key is missing, skip it.
		// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/869#issuecomment-3631307996
		return nil
	}

	// Decode value
	valueData, ok, err := easyproto.GetMessageData(src, 2)
	if err != nil {
		return fmt.Errorf("cannot find Value in KeyValue: %w", err)
	}
	if !ok {
		// Value is null, skip it.
		return nil
	}

	v, err := decodeAnyValueToJSON(valueData, a, fb)
	if err != nil {
		return fmt.Errorf("cannot decode AnyValue: %w", err)
	}

	dst.Set(fieldName, v)

	return nil
}

var jsonArenaPool fastjson.ArenaPool
