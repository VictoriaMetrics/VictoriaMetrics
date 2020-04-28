package driver

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.mongodb.org/mongo-driver/internal/testutil/assert"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

func TestBatches(t *testing.T) {
	t.Run("Valid", func(t *testing.T) {
		testCases := []struct {
			name    string
			batches *Batches
			want    bool
		}{
			{"nil", nil, false},
			{"missing identifier", &Batches{}, false},
			{"no documents", &Batches{Identifier: "documents"}, false},
			{"valid", &Batches{Identifier: "documents", Documents: make([]bsoncore.Document, 5)}, true},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				want := tc.want
				got := tc.batches.Valid()
				if got != want {
					t.Errorf("Did not get expected result from Valid. got %t; want %t", got, want)
				}
			})
		}
	})
	t.Run("ClearBatch", func(t *testing.T) {
		batches := &Batches{Identifier: "documents", Current: make([]bsoncore.Document, 2, 10)}
		if len(batches.Current) != 2 {
			t.Fatalf("Length of current batch should be 2, but is %d", len(batches.Current))
		}
		batches.ClearBatch()
		if len(batches.Current) != 0 {
			t.Fatalf("Length of current batch should be 0, but is %d", len(batches.Current))
		}
	})
	t.Run("AdvanceBatch", func(t *testing.T) {
		documents := make([]bsoncore.Document, 0)
		for i := 0; i < 5; i++ {
			doc := make(bsoncore.Document, 100)
			documents = append(documents, doc)
		}

		testCases := []struct {
			name            string
			batches         *Batches
			maxCount        int
			targetBatchSize int
			maxDocSize      int
			err             error
			want            *Batches
		}{
			{
				"current batch non-zero",
				&Batches{Current: make([]bsoncore.Document, 2, 10)},
				0, 0, 0, nil,
				&Batches{Current: make([]bsoncore.Document, 2, 10)},
			},
			{
				// all of the documents in the batch fit in targetBatchSize so the batch is created successfully
				"documents fit in targetBatchSize",
				&Batches{Documents: documents},
				10, 600, 1000, nil,
				&Batches{Documents: documents[:0], Current: documents[0:]},
			},
			{
				// the first doc is bigger than targetBatchSize but smaller than maxDocSize so it is taken alone
				"first document larger than targetBatchSize, smaller than maxDocSize",
				&Batches{Documents: documents},
				10, 5, 100, nil,
				&Batches{Documents: documents[1:], Current: documents[:1]},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				err := tc.batches.AdvanceBatch(tc.maxCount, tc.targetBatchSize, tc.maxDocSize)
				if !cmp.Equal(err, tc.err, cmp.Comparer(compareErrors)) {
					t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
				}
				if !cmp.Equal(tc.batches, tc.want) {
					t.Errorf("Batches is not in correct state after AdvanceBatch. got %v; want %v", tc.batches, tc.want)
				}
			})
		}

		t.Run("middle document larger than targetBatchSize, smaller than maxDocSize", func(t *testing.T) {
			// a batch is made but one document is too big, so everything before it is taken.
			// on the second call to AdvanceBatch, only the large document is taken

			middleLargeDoc := make([]bsoncore.Document, 0)
			for i := 0; i < 5; i++ {
				doc := make(bsoncore.Document, 100)
				middleLargeDoc = append(middleLargeDoc, doc)
			}
			largeDoc := make(bsoncore.Document, 900)
			middleLargeDoc[2] = largeDoc
			batches := &Batches{Documents: middleLargeDoc}
			maxCount := 10
			targetSize := 600
			maxDocSize := 1000

			// first batch should take first 2 docs (size 100 each)
			err := batches.AdvanceBatch(maxCount, targetSize, maxDocSize)
			assert.Nil(t, err, "AdvanceBatch error: %v", err)
			want := &Batches{Current: middleLargeDoc[:2], Documents: middleLargeDoc[2:]}
			assert.Equal(t, want, batches, "expected batches %v, got %v", want, batches)

			// second batch should take single large doc (size 900)
			batches.ClearBatch()
			err = batches.AdvanceBatch(maxCount, targetSize, maxDocSize)
			assert.Nil(t, err, "AdvanceBatch error: %v", err)
			want = &Batches{Current: middleLargeDoc[2:3], Documents: middleLargeDoc[3:]}
			assert.Equal(t, want, batches, "expected batches %v, got %v", want, batches)

			// last batch should take last 2 docs (size 100 each)
			batches.ClearBatch()
			err = batches.AdvanceBatch(maxCount, targetSize, maxDocSize)
			assert.Nil(t, err, "AdvanceBatch error: %v", err)
			want = &Batches{Current: middleLargeDoc[3:], Documents: middleLargeDoc[:0]}
			assert.Equal(t, want, batches, "expected batches %v, got %v", want, batches)
		})
	})
}
