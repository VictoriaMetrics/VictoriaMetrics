package options

import (
	"bytes"
	"testing"

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestCollation(t *testing.T) {
	t.Run("TestCollationToDocument", func(t *testing.T) {
		c := &Collation{
			Locale:          "locale",
			CaseLevel:       true,
			CaseFirst:       "first",
			Strength:        1,
			NumericOrdering: true,
			Alternate:       "alternate",
			MaxVariable:     "maxVariable",
			Normalization:   true,
			Backwards:       true,
		}

		doc := c.ToDocument()
		expected := bsoncore.BuildDocumentFromElements(nil,
			bsoncore.AppendStringElement(nil, "locale", "locale"),
			bsoncore.AppendBooleanElement(nil, "caseLevel", (true)),
			bsoncore.AppendStringElement(nil, "caseFirst", ("first")),
			bsoncore.AppendInt32Element(nil, "strength", (1)),
			bsoncore.AppendBooleanElement(nil, "numericOrdering", (true)),
			bsoncore.AppendStringElement(nil, "alternate", ("alternate")),
			bsoncore.AppendStringElement(nil, "maxVariable", ("maxVariable")),
			bsoncore.AppendBooleanElement(nil, "normalization", (true)),
			bsoncore.AppendBooleanElement(nil, "backwards", (true)),
		)

		if !bytes.Equal(doc, expected) {
			t.Fatalf("collation did not match expected. got %v; wanted %v", doc, expected)
		}
	})
}
