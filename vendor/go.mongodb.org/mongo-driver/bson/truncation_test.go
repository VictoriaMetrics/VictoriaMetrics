package bson

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson/bsoncodec"
)

type inputArgs struct {
	Name string
	Val  *float64
}

type outputArgs struct {
	Name string
	Val  *int64
}

func TestTruncation(t *testing.T) {
	t.Run("truncation", func(t *testing.T) {
		inputName := "truncation"
		inputVal := 4.7892

		input := inputArgs{Name: inputName, Val: &inputVal}
		ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}

		doc, err := MarshalWithContext(ec, &input)
		assert.Nil(t, err)

		var output outputArgs
		dc := bsoncodec.DecodeContext{
			Registry: DefaultRegistry,
			Truncate: true,
		}

		err = UnmarshalWithContext(dc, doc, &output)
		assert.Nil(t, err)

		assert.Equal(t, inputName, output.Name)
		assert.Equal(t, int64(inputVal), *output.Val)
	})
	t.Run("no truncation", func(t *testing.T) {
		inputName := "no truncation"
		inputVal := 7.382

		input := inputArgs{Name: inputName, Val: &inputVal}
		ec := bsoncodec.EncodeContext{Registry: DefaultRegistry}

		doc, err := MarshalWithContext(ec, &input)
		assert.Nil(t, err)

		var output outputArgs
		dc := bsoncodec.DecodeContext{
			Registry: DefaultRegistry,
			Truncate: false,
		}

		// case throws an error when truncation is disabled
		err = UnmarshalWithContext(dc, doc, &output)
		assert.NotNil(t, err)
	})
}
