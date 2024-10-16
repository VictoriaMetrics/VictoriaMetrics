package readline

import (
	"github.com/ergochat/readline/internal/ringbuf"
)

type undoEntry struct {
	pos int
	buf []rune
}

// nil receiver is a valid no-op object
type opUndo struct {
	op    *operation
	stack ringbuf.Buffer[undoEntry]
}

func newOpUndo(op *operation) *opUndo {
	o := &opUndo{op: op}
	o.stack.Initialize(32, 64)
	o.init()
	return o
}

func (o *opUndo) add() {
	if o == nil {
		return
	}

	top, success := o.stack.Pop()
	buf, pos, changed := o.op.buf.CopyForUndo(top.buf) // if !success, top.buf is nil
	newEntry := undoEntry{pos: pos, buf: buf}
	if !success {
		o.stack.Add(newEntry)
	} else if !changed {
		o.stack.Add(newEntry) // update cursor position
	} else {
		o.stack.Add(top)
		o.stack.Add(newEntry)
	}
}

func (o *opUndo) undo() {
	if o == nil {
		return
	}

	top, success := o.stack.Pop()
	if !success {
		return
	}
	o.op.buf.Restore(top.buf, top.pos)
	o.op.buf.Refresh(nil)
}

func (o *opUndo) init() {
	if o == nil {
		return
	}

	buf, pos, _ := o.op.buf.CopyForUndo(nil)
	initialEntry := undoEntry{
		pos: pos,
		buf: buf,
	}
	o.stack.Clear()
	o.stack.Add(initialEntry)
}
