package readline

import (
	"bytes"
	"container/list"
	"fmt"
	"sync"
)

type searchState uint

const (
	searchStateFound searchState = iota
	searchStateFailing
)

type searchDirection uint

const (
	searchDirectionForward searchDirection = iota
	searchDirectionBackward
)

type opSearch struct {
	mutex     sync.Mutex
	inMode    bool
	state     searchState
	dir       searchDirection
	source    *list.Element
	w         *terminal
	buf       *runeBuffer
	data      []rune
	history   *opHistory
	markStart int
	markEnd   int
}

func newOpSearch(w *terminal, buf *runeBuffer, history *opHistory) *opSearch {
	return &opSearch{
		w:       w,
		buf:     buf,
		history: history,
	}
}

func (o *opSearch) IsSearchMode() bool {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.inMode
}

func (o *opSearch) SearchBackspace() {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if len(o.data) > 0 {
		o.data = o.data[:len(o.data)-1]
		o.search(true)
	}
}

func (o *opSearch) findHistoryBy(isNewSearch bool) (int, *list.Element) {
	if o.dir == searchDirectionBackward {
		return o.history.FindBck(isNewSearch, o.data, o.buf.idx)
	}
	return o.history.FindFwd(isNewSearch, o.data, o.buf.idx)
}

func (o *opSearch) search(isChange bool) bool {
	if len(o.data) == 0 {
		o.state = searchStateFound
		o.searchRefresh(-1)
		return true
	}
	idx, elem := o.findHistoryBy(isChange)
	if elem == nil {
		o.searchRefresh(-2)
		return false
	}
	o.history.current = elem

	item := o.history.showItem(o.history.current.Value)
	start, end := 0, 0
	if o.dir == searchDirectionBackward {
		start, end = idx, idx+len(o.data)
	} else {
		start, end = idx, idx+len(o.data)
		idx += len(o.data)
	}
	o.buf.SetWithIdx(idx, item)
	o.markStart, o.markEnd = start, end
	o.searchRefresh(idx)
	return true
}

func (o *opSearch) SearchChar(r rune) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	o.data = append(o.data, r)
	o.search(true)
}

func (o *opSearch) SearchMode(dir searchDirection) bool {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	tWidth, _ := o.w.GetWidthHeight()
	if tWidth == 0 {
		return false
	}
	alreadyInMode := o.inMode
	o.inMode = true
	o.dir = dir
	o.source = o.history.current
	if alreadyInMode {
		o.search(false)
	} else {
		o.searchRefresh(-1)
	}
	return true
}

func (o *opSearch) ExitSearchMode(revert bool) {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if revert {
		o.history.current = o.source
		var redrawValue []rune
		if o.history.current != nil {
			redrawValue = o.history.showItem(o.history.current.Value)
		}
		o.buf.Set(redrawValue)
	}
	o.markStart, o.markEnd = 0, 0
	o.state = searchStateFound
	o.inMode = false
	o.source = nil
	o.data = nil
}

func (o *opSearch) searchRefresh(x int) {
	tWidth, _ := o.w.GetWidthHeight()
	if x == -2 {
		o.state = searchStateFailing
	} else if x >= 0 {
		o.state = searchStateFound
	}
	if x < 0 {
		x = o.buf.idx
	}
	x = o.buf.CurrentWidth(x)
	x += o.buf.PromptLen()
	x = x % tWidth

	if o.markStart > 0 {
		o.buf.SetStyle(o.markStart, o.markEnd, "4")
	}

	lineCnt := o.buf.CursorLineCount()
	buf := bytes.NewBuffer(nil)
	buf.Write(bytes.Repeat([]byte("\n"), lineCnt))
	buf.WriteString("\033[J")
	if o.state == searchStateFailing {
		buf.WriteString("failing ")
	}
	if o.dir == searchDirectionBackward {
		buf.WriteString("bck")
	} else if o.dir == searchDirectionForward {
		buf.WriteString("fwd")
	}
	buf.WriteString("-i-search: ")
	buf.WriteString(string(o.data))         // keyword
	buf.WriteString("\033[4m \033[0m")      // _
	fmt.Fprintf(buf, "\r\033[%dA", lineCnt) // move prev
	if x > 0 {
		fmt.Fprintf(buf, "\033[%dC", x) // move forward
	}
	o.w.Write(buf.Bytes())
}

func (o *opSearch) RefreshIfNeeded() {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.inMode {
		o.searchRefresh(-1)
	}
}
