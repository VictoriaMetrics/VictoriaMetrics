package bsoncore

import (
	"bytes"
	"io"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDocumentSequence(t *testing.T) {

	genArrayStyle := func(num int) []byte {
		idx, seq := AppendDocumentStart(nil)
		for i := 0; i < num; i++ {
			seq = AppendDocumentElement(
				seq, strconv.Itoa(i),
				BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159)),
			)
		}
		seq, _ = AppendDocumentEnd(seq, idx)
		return seq
	}
	genSequenceStyle := func(num int) []byte {
		var seq []byte
		for i := 0; i < num; i++ {
			seq = append(seq, BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159))...)
		}
		return seq
	}

	idx, arrayStyle := AppendDocumentStart(nil)
	idx2, arrayStyle := AppendDocumentElementStart(arrayStyle, "0")
	arrayStyle = AppendDoubleElement(arrayStyle, "pi", 3.14159)
	arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx2)
	idx2, arrayStyle = AppendDocumentElementStart(arrayStyle, "1")
	arrayStyle = AppendStringElement(arrayStyle, "hello", "world")
	arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx2)
	arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx)

	t.Run("Documents", func(t *testing.T) {
		testCases := []struct {
			name      string
			style     DocumentSequenceStyle
			data      []byte
			documents []Document
			err       error
		}{
			{
				"SequenceStle/corrupted document",
				SequenceStyle,
				[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
				nil,
				ErrCorruptedDocument,
			},
			{
				"SequenceStyle/success",
				SequenceStyle,
				BuildDocument(
					BuildDocument(
						nil,
						AppendStringElement(AppendDoubleElement(nil, "pi", 3.14159), "hello", "world"),
					),
					AppendDoubleElement(AppendStringElement(nil, "hello", "world"), "pi", 3.14159),
				),
				[]Document{
					BuildDocument(nil, AppendStringElement(AppendDoubleElement(nil, "pi", 3.14159), "hello", "world")),
					BuildDocument(nil, AppendDoubleElement(AppendStringElement(nil, "hello", "world"), "pi", 3.14159)),
				},
				nil,
			},
			{
				"ArrayStyle/insufficient bytes",
				ArrayStyle,
				[]byte{0x01, 0x02, 0x03, 0x04, 0x05},
				nil,
				ErrCorruptedDocument,
			},
			{
				"ArrayStyle/non-document",
				ArrayStyle,
				BuildDocument(nil, AppendDoubleElement(nil, "0", 12345.67890)),
				nil,
				ErrNonDocument,
			},
			{
				"ArrayStyle/success",
				ArrayStyle,
				arrayStyle,
				[]Document{
					BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159)),
					BuildDocument(nil, AppendStringElement(nil, "hello", "world")),
				},
				nil,
			},
			{"Invalid DocumentSequenceStyle", 0, nil, nil, ErrInvalidDocumentSequenceStyle},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ds := &DocumentSequence{
					Style: tc.style,
					Data:  tc.data,
				}
				documents, err := ds.Documents()
				if !cmp.Equal(documents, tc.documents) {
					t.Errorf("Documents do not match. got %v; want %v", documents, tc.documents)
				}
				if err != tc.err {
					t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
				}
			})
		}
	})
	t.Run("Next", func(t *testing.T) {
		seqDoc := BuildDocument(
			BuildDocument(
				nil,
				AppendDoubleElement(nil, "pi", 3.14159),
			),
			AppendStringElement(nil, "hello", "world"),
		)

		idx, arrayStyle := AppendDocumentStart(nil)
		idx2, arrayStyle := AppendDocumentElementStart(arrayStyle, "0")
		arrayStyle = AppendDoubleElement(arrayStyle, "pi", 3.14159)
		arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx2)
		idx2, arrayStyle = AppendDocumentElementStart(arrayStyle, "1")
		arrayStyle = AppendStringElement(arrayStyle, "hello", "world")
		arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx2)
		arrayStyle, _ = AppendDocumentEnd(arrayStyle, idx)

		testCases := []struct {
			name     string
			style    DocumentSequenceStyle
			data     []byte
			pos      int
			document Document
			err      error
		}{
			{"io.EOF", 0, make([]byte, 10), 10, nil, io.EOF},
			{
				"SequenceStyle/corrupted document",
				SequenceStyle,
				[]byte{0x01, 0x02, 0x03, 0x04},
				0,
				nil,
				ErrCorruptedDocument,
			},
			{
				"SequenceStyle/success/first",
				SequenceStyle,
				seqDoc,
				0,
				BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159)),
				nil,
			},
			{
				"SequenceStyle/success/second",
				SequenceStyle,
				seqDoc,
				17,
				BuildDocument(nil, AppendStringElement(nil, "hello", "world")),
				nil,
			},
			{
				"ArrayStyle/corrupted document/too short",
				ArrayStyle,
				[]byte{0x01, 0x02, 0x03},
				0,
				nil,
				ErrCorruptedDocument,
			},
			{
				"ArrayStyle/corrupted document/invalid element",
				ArrayStyle,
				[]byte{0x00, 0x00, 0x00, 0x00, 0x01, '0', 0x00, 0x01, 0x02},
				0,
				nil,
				ErrCorruptedDocument,
			},
			{
				"ArrayStyle/non-document",
				ArrayStyle,
				BuildDocument(nil, AppendDoubleElement(nil, "0", 12345.67890)),
				0,
				nil,
				ErrNonDocument,
			},
			{
				"ArrayStyle/success/first",
				ArrayStyle,
				arrayStyle,
				0,
				BuildDocument(nil, AppendDoubleElement(nil, "pi", 3.14159)),
				nil,
			},
			{
				"ArrayStyle/success/second",
				ArrayStyle,
				arrayStyle,
				24,
				BuildDocument(nil, AppendStringElement(nil, "hello", "world")),
				nil,
			},
			{"Invalid DocumentSequenceStyle", 0, make([]byte, 4), 0, nil, ErrInvalidDocumentSequenceStyle},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ds := &DocumentSequence{
					Style: tc.style,
					Data:  tc.data,
					Pos:   tc.pos,
				}
				document, err := ds.Next()
				if !bytes.Equal(document, tc.document) {
					t.Errorf("Documents do not match. got %v; want %v", document, tc.document)
				}
				if err != tc.err {
					t.Errorf("Errors do not match. got %v; want %v", err, tc.err)
				}
			})
		}
	})

	t.Run("Full Iteration", func(t *testing.T) {
		testCases := []struct {
			name  string
			style DocumentSequenceStyle
			data  []byte
			count int
		}{
			{"SequenceStyle/success/nil", SequenceStyle, nil, 0},
			{"SequenceStyle/success/0", SequenceStyle, []byte{}, 0},
			{"SequenceStyle/success/1", SequenceStyle, genSequenceStyle(1), 1},
			{"SequenceStyle/success/2", SequenceStyle, genSequenceStyle(2), 2},
			{"SequenceStyle/success/10", SequenceStyle, genSequenceStyle(10), 10},
			{"SequenceStyle/success/100", SequenceStyle, genSequenceStyle(100), 100},
			{"ArrayStyle/success/nil", ArrayStyle, nil, 0},
			{"ArrayStyle/success/0", ArrayStyle, []byte{0x05, 0x00, 0x00, 0x00, 0x00}, 0},
			{"ArrayStyle/success/1", ArrayStyle, genArrayStyle(1), 1},
			{"ArrayStyle/success/2", ArrayStyle, genArrayStyle(2), 2},
			{"ArrayStyle/success/10", ArrayStyle, genArrayStyle(10), 10},
			{"ArrayStyle/success/100", ArrayStyle, genArrayStyle(100), 100},
		}

		for _, tc := range testCases {
			t.Run("Documents/"+tc.name, func(t *testing.T) {
				ds := &DocumentSequence{
					Style: tc.style,
					Data:  tc.data,
				}
				docs, err := ds.Documents()
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				count := len(docs)
				if count != tc.count {
					t.Errorf("Coun't fully iterate documents, wrong count. got %v; want %v", count, tc.count)
				}
			})
			t.Run("Next/"+tc.name, func(t *testing.T) {
				ds := &DocumentSequence{
					Style: tc.style,
					Data:  tc.data,
				}
				var docs []Document
				for {
					doc, err := ds.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatalf("Unexpected error: %v", err)
					}
					docs = append(docs, doc)
				}
				count := len(docs)
				if count != tc.count {
					t.Errorf("Coun't fully iterate documents, wrong count. got %v; want %v", count, tc.count)
				}
			})
		}
	})
	t.Run("DocumentCount", func(t *testing.T) {
		testCases := []struct {
			name  string
			style DocumentSequenceStyle
			data  []byte
			count int
		}{
			{
				"SequenceStyle/corrupt document/first",
				SequenceStyle,
				[]byte{0x01, 0x02, 0x03},
				0,
			},
			{
				"SequenceStyle/corrupt document/second",
				SequenceStyle,
				[]byte{0x05, 0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03},
				0,
			},
			{"SequenceStyle/success/nil", SequenceStyle, nil, 0},
			{"SequenceStyle/success/0", SequenceStyle, []byte{}, 0},
			{"SequenceStyle/success/1", SequenceStyle, genSequenceStyle(1), 1},
			{"SequenceStyle/success/2", SequenceStyle, genSequenceStyle(2), 2},
			{"SequenceStyle/success/10", SequenceStyle, genSequenceStyle(10), 10},
			{"SequenceStyle/success/100", SequenceStyle, genSequenceStyle(100), 100},
			{
				"ArrayStyle/corrupt document/length",
				ArrayStyle,
				[]byte{0x01, 0x02, 0x03},
				0,
			},
			{
				"ArrayStyle/corrupt element/first",
				ArrayStyle,
				BuildDocument(nil, []byte{0x01, 0x00, 0x03, 0x04, 0x05}),
				0,
			},
			{
				"ArrayStyle/corrupt element/second",
				ArrayStyle,
				BuildDocument(nil, []byte{0x0A, 0x00, 0x01, 0x00, 0x03, 0x04, 0x05}),
				0,
			},
			{"ArrayStyle/success/nil", ArrayStyle, nil, 0},
			{"ArrayStyle/success/0", ArrayStyle, []byte{0x05, 0x00, 0x00, 0x00, 0x00}, 0},
			{"ArrayStyle/success/1", ArrayStyle, genArrayStyle(1), 1},
			{"ArrayStyle/success/2", ArrayStyle, genArrayStyle(2), 2},
			{"ArrayStyle/success/10", ArrayStyle, genArrayStyle(10), 10},
			{"ArrayStyle/success/100", ArrayStyle, genArrayStyle(100), 100},
			{"Invalid DocumentSequenceStyle", 0, nil, 0},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ds := &DocumentSequence{
					Style: tc.style,
					Data:  tc.data,
				}
				count := ds.DocumentCount()
				if count != tc.count {
					t.Errorf("Document counts don't match. got %v; want %v", count, tc.count)
				}
			})
		}
	})
	t.Run("Empty", func(t *testing.T) {
		testCases := []struct {
			name    string
			ds      *DocumentSequence
			isEmpty bool
		}{
			{"ArrayStyle/is empty/nil", nil, true},
			{"ArrayStyle/is empty/0", &DocumentSequence{Style: ArrayStyle, Data: []byte{0x05, 0x00, 0x00, 0x00, 0x00}}, true},
			{"ArrayStyle/is not empty/non-0", &DocumentSequence{Style: ArrayStyle, Data: genArrayStyle(10)}, false},
			{"SequenceStyle/is empty/nil", nil, true},
			{"SequenceStyle/is empty/0", &DocumentSequence{Style: SequenceStyle, Data: []byte{}}, true},
			{"SequenceStyle/is not empty/non-0", &DocumentSequence{Style: SequenceStyle, Data: genSequenceStyle(10)}, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				isEmpty := tc.ds.Empty()
				if isEmpty != tc.isEmpty {
					t.Errorf("Unexpected Empty result. got %v; want %v", isEmpty, tc.isEmpty)
				}
			})
		}
	})
	t.Run("ResetIterator", func(t *testing.T) {
		ds := &DocumentSequence{Pos: 1234567890}
		want := 0
		ds.ResetIterator()
		if ds.Pos != want {
			t.Errorf("Unexpected position after ResetIterator. got %d; want %d", ds.Pos, want)
		}
	})
	t.Run("no panic on nil", func(t *testing.T) {
		capturePanic := func() {
			if err := recover(); err != nil {
				t.Errorf("Unexpected panic. got %v; want <nil>", err)
			}
		}
		t.Run("DocumentCount", func(t *testing.T) {
			defer capturePanic()
			var ds *DocumentSequence
			_ = ds.DocumentCount()
		})
		t.Run("Empty", func(t *testing.T) {
			defer capturePanic()
			var ds *DocumentSequence
			_ = ds.Empty()
		})
		t.Run("ResetIterator", func(t *testing.T) {
			defer capturePanic()
			var ds *DocumentSequence
			ds.ResetIterator()
		})
		t.Run("Documents", func(t *testing.T) {
			defer capturePanic()
			var ds *DocumentSequence
			_, _ = ds.Documents()
		})
		t.Run("Next", func(t *testing.T) {
			defer capturePanic()
			var ds *DocumentSequence
			_, _ = ds.Next()
		})
	})
}
