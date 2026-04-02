package netstorage

import (
	"bytes"
	"testing"
)

func TestInsertCtxResizeStorageNodeBuf(t *testing.T) {
	maxBufSizePerStorageNode = 30 * 1024 * 1024
	t.Run("test resize and restore", func(t *testing.T) {
		insertCtx := &InsertCtx{}
		insertCtx.bufRowss = make([]bufRows, 1)
		insertCtx.bufRowss[0] = bufRows{
			buf:  make([]byte, 0),
			rows: 0,
		}

		insertCtx.bufRowss[0].buf = append(insertCtx.bufRowss[0].buf, []byte{'a', 'b'}...)

		before := insertCtx.bufRowss[0].buf
		insertCtx.ResizeStorageNodeBuf(0, 125)
		after := insertCtx.bufRowss[0].buf

		if cap(after) < len(before)+125 {
			t.Fatalf("ResizeStorageNodeBuf doesn't resize the buf")
		}

		if !bytes.Equal(after, before) {
			t.Fatalf("InsertCtx.ResizeStorageNodeBuf doesn't restored to previous status")
		}
	})

	t.Run("test exceed limit", func(t *testing.T) {
		insertCtx := &InsertCtx{}
		insertCtx.bufRowss = make([]bufRows, 1)
		insertCtx.bufRowss[0] = bufRows{
			buf:  make([]byte, 0),
			rows: 0,
		}

		insertCtx.bufRowss[0].buf = append(insertCtx.bufRowss[0].buf, []byte{'a', 'b'}...)

		before := insertCtx.bufRowss[0].buf
		insertCtx.ResizeStorageNodeBuf(0, 31*1024*1024)
		after := insertCtx.bufRowss[0].buf

		if cap(after) > maxBufSizePerStorageNode {
			t.Fatalf("InsertCtx.ResizeStorageNodeBuf exceed limit")
		}

		if !bytes.Equal(after, before) {
			t.Fatalf("InsertCtx.ResizeStorageNodeBuf doesn't restored to previous status")
		}
	})
}
