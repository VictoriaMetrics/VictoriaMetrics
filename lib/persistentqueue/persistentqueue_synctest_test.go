//go:build synctest

package persistentqueue

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFlushReaderMetainfoFlushesPendingWriterData(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := "queue-flush-reader-metainfo"
		fs.MustRemoveDir(path)
		q := mustOpen(path, "foobar", 0)
		defer func() {
			q.MustClose()
			fs.MustRemoveDir(path)
		}()

		block := []byte("foobar")
		data := encoding.MarshalUint64(nil, uint64(len(block)))
		data = append(data, block...)
		// it will call `flushBufAndMetainfoIfNeeded` internally to flush the data and metadata.
		err := q.writeBlock(data)
		if err != nil {
			t.Fatalf("unexpected error when writing data to queue: %s", err)
		}
		// the second call will update the writeOffset in memory without flushing the data and metadata,
		// because the last flush was performed less than 1 second ago.
		err = q.writeBlock(data)
		if err != nil {
			t.Fatalf("unexpected error when writing data to queue: %s", err)
		}
		time.Sleep(2 * time.Second)

		// it will call `flushBufAndMetainfoIfNeeded` internally to flush the data and metadata.
		if _, err = q.readBlock(nil); err != nil {
			t.Fatalf("unexpected error when flushing reader metainfo: %s", err)
		}

		if fileSize := fs.MustFileSize(q.writerPath); fileSize != q.writerOffset {
			t.Fatalf("unexpected writer file size after flushing reader metainfo; got %d bytes; want %d bytes", fileSize, q.writerOffset)
		}
		var mi metainfo
		if err := mi.ReadFromFile(q.metainfoPath()); err != nil {
			t.Fatalf("cannot read metainfo: %s", err)
		}
		if mi.ReaderOffset != q.readerOffset {
			t.Fatalf("unexpected ReaderOffset in metainfo; got %d; want %d", mi.ReaderOffset, q.readerOffset)
		}
		if mi.WriterOffset != q.writerOffset {
			t.Fatalf("unexpected WriterOffset in metainfo; got %d; want %d", mi.WriterOffset, q.writerOffset)
		}

	})
}
