package readline

import (
	"container/list"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"

	"github.com/ergochat/readline/internal/term"
)

const (
	CharLineStart = 1
	CharBackward  = 2
	CharInterrupt = 3
	CharEOT       = 4
	CharLineEnd   = 5
	CharForward   = 6
	CharBell      = 7
	CharCtrlH     = 8
	CharTab       = 9
	CharCtrlJ     = 10
	CharKill      = 11
	CharCtrlL     = 12
	CharEnter     = 13
	CharNext      = 14
	CharPrev      = 16
	CharBckSearch = 18
	CharFwdSearch = 19
	CharTranspose = 20
	CharCtrlU     = 21
	CharCtrlW     = 23
	CharCtrlY     = 25
	CharCtrlZ     = 26
	CharEsc       = 27
	CharCtrl_     = 31
	CharO         = 79
	CharEscapeEx  = 91
	CharBackspace = 127
)

const (
	MetaBackward rune = -iota - 1
	MetaForward
	MetaDelete
	MetaBackspace
	MetaTranspose
	MetaShiftTab
	MetaDeleteKey
)

type rawModeHandler struct {
	sync.Mutex
	state *term.State
}

func (r *rawModeHandler) Enter() (err error) {
	r.Lock()
	defer r.Unlock()
	r.state, err = term.MakeRaw(int(syscall.Stdin))
	return err
}

func (r *rawModeHandler) Exit() error {
	r.Lock()
	defer r.Unlock()
	if r.state == nil {
		return nil
	}
	err := term.Restore(int(syscall.Stdin), r.state)
	if err == nil {
		r.state = nil
	}
	return err
}

func clearScreen(w io.Writer) error {
	_, err := w.Write([]byte("\x1b[H\x1b[J"))
	return err
}

// -----------------------------------------------------------------------------

// print a linked list to Debug()
func debugList(l *list.List) {
	idx := 0
	for e := l.Front(); e != nil; e = e.Next() {
		debugPrint("%d %+v", idx, e.Value)
		idx++
	}
}

// append log info to another file
func debugPrint(fmtStr string, o ...interface{}) {
	f, _ := os.OpenFile("debug.tmp", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	fmt.Fprintf(f, fmtStr, o...)
	fmt.Fprintln(f)
	f.Close()
}
