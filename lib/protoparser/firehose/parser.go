package firehose

import (
	"encoding/base64"
	"fmt"
	"github.com/valyala/fastjson"
)

// ProcessRequestBody converts Cloudwatch Stream protobuf metrics HTTP body delivered via Firehose into protobuf message.
func ProcessRequestBody(b *[]byte) error {
	return unmarshalRequest(b)
}

var jsonParserPool fastjson.ParserPool

// unmarshalRequest extracts and decodes b64 data from Firehose HTTP destination request
//
//	{
//	  "requestId": "<uuid-string>",
//	  "timestamp": <int64-value>,
//	  "records": [
//	    {
//	      "data": "<base64-encoded-payload>"
//	    }
//	  ]
//	}
func unmarshalRequest(b *[]byte) error {
	p := jsonParserPool.Get()
	defer jsonParserPool.Put(p)

	v, err := p.ParseBytes(*b)
	if err != nil {
		return err
	}
	o, err := v.Object()
	if err != nil {
		return fmt.Errorf("cannot find Firehose Request objects: %w", err)
	}
	index := 0
	o.Visit(func(k []byte, v *fastjson.Value) {
		if err != nil {
			return
		}
		switch string(k) {
		case "records":
			recordObjects, errLocal := v.Array()
			if errLocal != nil {
				err = fmt.Errorf("cannot find Records array in Firehose Request object: %w", errLocal)
				return
			}
			for _, fr := range recordObjects {
				recordObject, errLocal := fr.Object()
				if errLocal != nil {
					err = fmt.Errorf("cannot find Record object: %w", errLocal)
					return
				}
				if errLocal := unmarshalRecord(b, &index, recordObject); errLocal != nil {
					err = fmt.Errorf("cannot unmarshal Record object: %w", errLocal)
					return
				}
			}
		}
	})
	*b = (*b)[:index]
	if err != nil {
		return fmt.Errorf("cannot parse Firehose Request object: %w", err)
	}
	return nil
}

func unmarshalRecord(b *[]byte, index *int, o *fastjson.Object) error {
	var err error
	var inc int
	o.Visit(func(k []byte, v *fastjson.Value) {
		if v.Type() != fastjson.TypeString {
			err = fmt.Errorf("invalid record data type, %q", v.Type())
			return
		}
		valueBytes := v.GetStringBytes()
		if len(valueBytes) == 0 {
			return
		}
		inc, err = base64.StdEncoding.Decode((*b)[*index:], valueBytes)
		if err != nil {
			err = fmt.Errorf("failed to decode and append Firehose Record data: %w", err)
			return
		}
		*index = *index + inc
	})
	return err
}
