package readline

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ergochat/readline/internal/runes"
)

type runeBuffer struct {
	buf []rune
	idx int
	w   *terminal

	cpos cursorPosition
	ppos int // prompt start position (0 == column 1)

	lastKill []rune

	sync.Mutex
}

func (r *runeBuffer) pushKill(text []rune) {
	r.lastKill = append([]rune{}, text...)
}

func newRuneBuffer(w *terminal) *runeBuffer {
	rb := &runeBuffer{
		w: w,
	}
	return rb
}

func (r *runeBuffer) CurrentWidth(x int) int {
	r.Lock()
	defer r.Unlock()
	return runes.WidthAll(r.buf[:x])
}

func (r *runeBuffer) PromptLen() int {
	r.Lock()
	defer r.Unlock()
	return r.promptLen()
}

func (r *runeBuffer) promptLen() int {
	return runes.WidthAll(runes.ColorFilter([]rune(r.prompt())))
}

func (r *runeBuffer) RuneSlice(i int) []rune {
	r.Lock()
	defer r.Unlock()

	if i > 0 {
		rs := make([]rune, i)
		copy(rs, r.buf[r.idx:r.idx+i])
		return rs
	}
	rs := make([]rune, -i)
	copy(rs, r.buf[r.idx+i:r.idx])
	return rs
}

func (r *runeBuffer) Runes() []rune {
	r.Lock()
	newr := make([]rune, len(r.buf))
	copy(newr, r.buf)
	r.Unlock()
	return newr
}

func (r *runeBuffer) Pos() int {
	r.Lock()
	defer r.Unlock()
	return r.idx
}

func (r *runeBuffer) Len() int {
	r.Lock()
	defer r.Unlock()
	return len(r.buf)
}

func (r *runeBuffer) MoveToLineStart() {
	r.Refresh(func() {
		r.idx = 0
	})
}

func (r *runeBuffer) MoveBackward() {
	r.Refresh(func() {
		if r.idx == 0 {
			return
		}
		r.idx--
	})
}

func (r *runeBuffer) WriteString(s string) {
	r.WriteRunes([]rune(s))
}

func (r *runeBuffer) WriteRune(s rune) {
	r.WriteRunes([]rune{s})
}

func (r *runeBuffer) getConfig() *Config {
	return r.w.GetConfig()
}

func (r *runeBuffer) isInteractive() bool {
	return r.getConfig().isInteractive
}

func (r *runeBuffer) prompt() string {
	return r.getConfig().Prompt
}

func (r *runeBuffer) WriteRunes(s []rune) {
	r.Lock()
	defer r.Unlock()

	if r.idx == len(r.buf) {
		// cursor is already at end of buf data so just call
		// append instead of refesh to save redrawing.
		r.buf = append(r.buf, s...)
		r.idx += len(s)
		if r.isInteractive() {
			r.append(s)
		}
	} else {
		// writing into the data somewhere so do a refresh
		r.refresh(func() {
			tail := append(s, r.buf[r.idx:]...)
			r.buf = append(r.buf[:r.idx], tail...)
			r.idx += len(s)
		})
	}
}

func (r *runeBuffer) MoveForward() {
	r.Refresh(func() {
		if r.idx == len(r.buf) {
			return
		}
		r.idx++
	})
}

func (r *runeBuffer) IsCursorInEnd() bool {
	r.Lock()
	defer r.Unlock()
	return r.idx == len(r.buf)
}

func (r *runeBuffer) Replace(ch rune) {
	r.Refresh(func() {
		r.buf[r.idx] = ch
	})
}

func (r *runeBuffer) Erase() {
	r.Refresh(func() {
		r.idx = 0
		r.pushKill(r.buf[:])
		r.buf = r.buf[:0]
	})
}

func (r *runeBuffer) Delete() (success bool) {
	r.Refresh(func() {
		if r.idx == len(r.buf) {
			return
		}
		r.pushKill(r.buf[r.idx : r.idx+1])
		r.buf = append(r.buf[:r.idx], r.buf[r.idx+1:]...)
		success = true
	})
	return
}

func (r *runeBuffer) DeleteWord() {
	if r.idx == len(r.buf) {
		return
	}
	init := r.idx
	for init < len(r.buf) && runes.IsWordBreak(r.buf[init]) {
		init++
	}
	for i := init + 1; i < len(r.buf); i++ {
		if !runes.IsWordBreak(r.buf[i]) && runes.IsWordBreak(r.buf[i-1]) {
			r.pushKill(r.buf[r.idx : i-1])
			r.Refresh(func() {
				r.buf = append(r.buf[:r.idx], r.buf[i-1:]...)
			})
			return
		}
	}
	r.Kill()
}

func (r *runeBuffer) MoveToPrevWord() (success bool) {
	r.Refresh(func() {
		if r.idx == 0 {
			return
		}

		for i := r.idx - 1; i > 0; i-- {
			if !runes.IsWordBreak(r.buf[i]) && runes.IsWordBreak(r.buf[i-1]) {
				r.idx = i
				success = true
				return
			}
		}
		r.idx = 0
		success = true
	})
	return
}

func (r *runeBuffer) KillFront() {
	r.Refresh(func() {
		if r.idx == 0 {
			return
		}

		length := len(r.buf) - r.idx
		r.pushKill(r.buf[:r.idx])
		copy(r.buf[:length], r.buf[r.idx:])
		r.idx = 0
		r.buf = r.buf[:length]
	})
}

func (r *runeBuffer) Kill() {
	r.Refresh(func() {
		r.pushKill(r.buf[r.idx:])
		r.buf = r.buf[:r.idx]
	})
}

func (r *runeBuffer) Transpose() {
	r.Refresh(func() {
		if r.idx == 0 {
			// match the GNU Readline behavior, Ctrl-T at the start of the line
			// is a no-op:
			return
		}

		// OK, we have at least one character behind us:
		if r.idx < len(r.buf) {
			// swap the character in front of us with the one behind us
			r.buf[r.idx], r.buf[r.idx-1] = r.buf[r.idx-1], r.buf[r.idx]
			// advance the cursor
			r.idx++
		} else if r.idx == len(r.buf) && len(r.buf) >= 2 {
			// swap the two characters behind us
			r.buf[r.idx-2], r.buf[r.idx-1] = r.buf[r.idx-1], r.buf[r.idx-2]
			// leave the cursor in place since there's nowhere to go
		}
	})
}

func (r *runeBuffer) MoveToNextWord() {
	r.Refresh(func() {
		for i := r.idx + 1; i < len(r.buf); i++ {
			if !runes.IsWordBreak(r.buf[i]) && runes.IsWordBreak(r.buf[i-1]) {
				r.idx = i
				return
			}
		}

		r.idx = len(r.buf)
	})
}

func (r *runeBuffer) MoveToEndWord() {
	r.Refresh(func() {
		// already at the end, so do nothing
		if r.idx == len(r.buf) {
			return
		}
		// if we are at the end of a word already, go to next
		if !runes.IsWordBreak(r.buf[r.idx]) && runes.IsWordBreak(r.buf[r.idx+1]) {
			r.idx++
		}

		// keep going until at the end of a word
		for i := r.idx + 1; i < len(r.buf); i++ {
			if runes.IsWordBreak(r.buf[i]) && !runes.IsWordBreak(r.buf[i-1]) {
				r.idx = i - 1
				return
			}
		}
		r.idx = len(r.buf)
	})
}

func (r *runeBuffer) BackEscapeWord() {
	r.Refresh(func() {
		if r.idx == 0 {
			return
		}
		for i := r.idx - 1; i >= 0; i-- {
			if i == 0 || (runes.IsWordBreak(r.buf[i-1])) && !runes.IsWordBreak(r.buf[i]) {
				r.pushKill(r.buf[i:r.idx])
				r.buf = append(r.buf[:i], r.buf[r.idx:]...)
				r.idx = i
				return
			}
		}

		r.buf = r.buf[:0]
		r.idx = 0
	})
}

func (r *runeBuffer) Yank() {
	if len(r.lastKill) == 0 {
		return
	}
	r.Refresh(func() {
		buf := make([]rune, 0, len(r.buf)+len(r.lastKill))
		buf = append(buf, r.buf[:r.idx]...)
		buf = append(buf, r.lastKill...)
		buf = append(buf, r.buf[r.idx:]...)
		r.buf = buf
		r.idx += len(r.lastKill)
	})
}

func (r *runeBuffer) Backspace() {
	r.Refresh(func() {
		if r.idx == 0 {
			return
		}

		r.idx--
		r.buf = append(r.buf[:r.idx], r.buf[r.idx+1:]...)
	})
}

func (r *runeBuffer) MoveToLineEnd() {
	r.Lock()
	defer r.Unlock()
	if r.idx == len(r.buf) {
		return
	}
	r.refresh(func() {
		r.idx = len(r.buf)
	})
}

// LineCount returns number of lines the buffer takes as it appears in the terminal.
func (r *runeBuffer) LineCount() int {
	sp := r.getSplitByLine(r.buf, 1)
	return len(sp)
}

func (r *runeBuffer) MoveTo(ch rune, prevChar, reverse bool) (success bool) {
	r.Refresh(func() {
		if reverse {
			for i := r.idx - 1; i >= 0; i-- {
				if r.buf[i] == ch {
					r.idx = i
					if prevChar {
						r.idx++
					}
					success = true
					return
				}
			}
			return
		}
		for i := r.idx + 1; i < len(r.buf); i++ {
			if r.buf[i] == ch {
				r.idx = i
				if prevChar {
					r.idx--
				}
				success = true
				return
			}
		}
	})
	return
}

func (r *runeBuffer) isInLineEdge() bool {
	sp := r.getSplitByLine(r.buf, 1)
	return len(sp[len(sp)-1]) == 0 // last line is 0 len
}

func (r *runeBuffer) getSplitByLine(rs []rune, nextWidth int) [][]rune {
	tWidth, _ := r.w.GetWidthHeight()
	cfg := r.getConfig()
	if cfg.EnableMask {
		w := runes.Width(cfg.MaskRune)
		masked := []rune(strings.Repeat(string(cfg.MaskRune), len(rs)))
		return runes.SplitByLine(runes.ColorFilter([]rune(r.prompt())), masked, r.ppos, tWidth, w)
	} else {
		return runes.SplitByLine(runes.ColorFilter([]rune(r.prompt())), rs, r.ppos, tWidth, nextWidth)
	}
}

func (r *runeBuffer) IdxLine(width int) int {
	r.Lock()
	defer r.Unlock()
	return r.idxLine(width)
}

func (r *runeBuffer) idxLine(width int) int {
	if width == 0 {
		return 0
	}
	nextWidth := 1
	if r.idx < len(r.buf) {
		nextWidth = runes.Width(r.buf[r.idx])
	}
	sp := r.getSplitByLine(r.buf[:r.idx], nextWidth)
	return len(sp) - 1
}

func (r *runeBuffer) CursorLineCount() int {
	tWidth, _ := r.w.GetWidthHeight()
	return r.LineCount() - r.IdxLine(tWidth)
}

func (r *runeBuffer) Refresh(f func()) {
	r.Lock()
	defer r.Unlock()
	r.refresh(f)
}

func (r *runeBuffer) refresh(f func()) {
	if !r.isInteractive() {
		if f != nil {
			f()
		}
		return
	}

	r.clean()
	if f != nil {
		f()
	}
	r.print()
}

func (r *runeBuffer) SetOffset(position cursorPosition) {
	r.Lock()
	defer r.Unlock()
	r.setOffset(position)
}

func (r *runeBuffer) setOffset(cpos cursorPosition) {
	r.cpos = cpos
	tWidth, _ := r.w.GetWidthHeight()
	if cpos.col > 0 && cpos.col < tWidth {
		r.ppos = cpos.col - 1 // c should be 1..tWidth
	} else {
		r.ppos = 0
	}
}

// append s to the end of the current output. append is called in
// place of print() when clean() was avoided. As output is appended on
// the end, the cursor also needs no extra adjustment.
// NOTE: assumes len(s) >= 1 which should always be true for append.
func (r *runeBuffer) append(s []rune) {
	buf := bytes.NewBuffer(nil)
	slen := len(s)
	cfg := r.getConfig()
	if cfg.EnableMask {
		if slen > 1 && cfg.MaskRune != 0 {
			// write a mask character for all runes except the last rune
			buf.WriteString(strings.Repeat(string(cfg.MaskRune), slen-1))
		}
		// for the last rune, write \n or mask it otherwise.
		if s[slen-1] == '\n' {
			buf.WriteRune('\n')
		} else if cfg.MaskRune != 0 {
			buf.WriteRune(cfg.MaskRune)
		}
	} else {
		for _, e := range cfg.Painter(s, slen) {
			if e == '\t' {
				buf.WriteString(strings.Repeat(" ", runes.TabWidth))
			} else {
				buf.WriteRune(e)
			}
		}
	}
	if r.isInLineEdge() {
		buf.WriteString(" \b")
	}
	r.w.Write(buf.Bytes())
}

// Print writes out the prompt and buffer contents at the current cursor position
func (r *runeBuffer) Print() {
	r.Lock()
	defer r.Unlock()
	if !r.isInteractive() {
		return
	}
	r.print()
}

func (r *runeBuffer) print() {
	r.w.Write(r.output())
}

func (r *runeBuffer) output() []byte {
	buf := bytes.NewBuffer(nil)
	buf.WriteString(r.prompt())
	buf.WriteString("\x1b[0K") // VT100 "Clear line from cursor right", see #38
	cfg := r.getConfig()
	if cfg.EnableMask && len(r.buf) > 0 {
		if cfg.MaskRune != 0 {
			buf.WriteString(strings.Repeat(string(cfg.MaskRune), len(r.buf)-1))
		}
		if r.buf[len(r.buf)-1] == '\n' {
			buf.WriteRune('\n')
		} else if cfg.MaskRune != 0 {
			buf.WriteRune(cfg.MaskRune)
		}
	} else {
		for _, e := range cfg.Painter(r.buf, r.idx) {
			if e == '\t' {
				buf.WriteString(strings.Repeat(" ", runes.TabWidth))
			} else {
				buf.WriteRune(e)
			}
		}
	}
	if r.isInLineEdge() {
		buf.WriteString(" \b")
	}
	// cursor position
	if len(r.buf) > r.idx {
		buf.Write(r.getBackspaceSequence())
	}
	return buf.Bytes()
}

func (r *runeBuffer) getBackspaceSequence() []byte {
	bcnt := len(r.buf) - r.idx // backwards count to index
	sp := r.getSplitByLine(r.buf, 1)

	// Calculate how many lines up to the index line
	up := 0
	spi := len(sp) - 1
	for spi >= 0 {
		bcnt -= len(sp[spi])
		if bcnt <= 0 {
			break
		}
		up++
		spi--
	}

	// Calculate what column the index should be set to
	column := 1
	if spi == 0 {
		column += r.ppos
	}
	for _, rune := range sp[spi] {
		if bcnt >= 0 {
			break
		}
		column += runes.Width(rune)
		bcnt++
	}

	buf := bytes.NewBuffer(nil)
	if up > 0 {
		fmt.Fprintf(buf, "\033[%dA", up) // move cursor up to index line
	}
	fmt.Fprintf(buf, "\033[%dG", column) // move cursor to column

	return buf.Bytes()
}

func (r *runeBuffer) CopyForUndo(prev []rune) (cur []rune, idx int, changed bool) {
	if runes.Equal(r.buf, prev) {
		return prev, r.idx, false
	} else {
		return runes.Copy(r.buf), r.idx, true
	}
}

func (r *runeBuffer) Restore(buf []rune, idx int) {
	r.buf = buf
	r.idx = idx
}

func (r *runeBuffer) Reset() []rune {
	ret := runes.Copy(r.buf)
	r.buf = r.buf[:0]
	r.idx = 0
	return ret
}

func (r *runeBuffer) calWidth(m int) int {
	if m > 0 {
		return runes.WidthAll(r.buf[r.idx : r.idx+m])
	}
	return runes.WidthAll(r.buf[r.idx+m : r.idx])
}

func (r *runeBuffer) SetStyle(start, end int, style string) {
	if end < start {
		panic("end < start")
	}

	// goto start
	move := start - r.idx
	if move > 0 {
		r.w.Write([]byte(string(r.buf[r.idx : r.idx+move])))
	} else {
		r.w.Write(bytes.Repeat([]byte("\b"), r.calWidth(move)))
	}
	r.w.Write([]byte("\033[" + style + "m"))
	r.w.Write([]byte(string(r.buf[start:end])))
	r.w.Write([]byte("\033[0m"))
	// TODO: move back
}

func (r *runeBuffer) SetWithIdx(idx int, buf []rune) {
	r.Refresh(func() {
		r.buf = buf
		r.idx = idx
	})
}

func (r *runeBuffer) Set(buf []rune) {
	r.SetWithIdx(len(buf), buf)
}

func (r *runeBuffer) SetNoRefresh(buf []rune) {
	r.buf = buf
	r.idx = len(buf)
}

func (r *runeBuffer) cleanOutput(w io.Writer, idxLine int) {
	buf := bufio.NewWriter(w)

	tWidth, _ := r.w.GetWidthHeight()
	if tWidth == 0 {
		buf.WriteString(strings.Repeat("\r\b", len(r.buf)+r.promptLen()))
		buf.Write([]byte("\033[J"))
	} else {
		if idxLine > 0 {
			fmt.Fprintf(buf, "\033[%dA", idxLine) // move cursor up by idxLine
		}
		fmt.Fprintf(buf, "\033[%dG", r.ppos+1) // move cursor back to initial ppos position
		buf.Write([]byte("\033[J"))            // clear from cursor to end of screen
	}
	buf.Flush()
	return
}

func (r *runeBuffer) Clean() {
	r.Lock()
	r.clean()
	r.Unlock()
}

func (r *runeBuffer) clean() {
	tWidth, _ := r.w.GetWidthHeight()
	r.cleanWithIdxLine(r.idxLine(tWidth))
}

func (r *runeBuffer) cleanWithIdxLine(idxLine int) {
	if !r.isInteractive() {
		return
	}
	r.cleanOutput(r.w, idxLine)
}
