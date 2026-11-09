//go:build windows
// +build windows

package pb

import (
	"fmt"
	"log"
	"strings"

	"github.com/cheggaaa/pb/v3/termutil"
)

func (p *Pool) print(first bool) bool {
	p.m.Lock()
	defer p.m.Unlock()
	var out string
	if !first {
		moveCursorUp(p.lastBarsCount)
	}
	cols, err := termutil.TerminalWidth()
	if err != nil || cols <= 0 {
		cols = defaultBarWidth
	}
	bars, isFinished := p.visibleBars(0)
	for _, bar := range bars {
		result := bar.String()
		if r := cols - CellCount(result); r > 0 {
			result += strings.Repeat(" ", r)
		}
		out += fmt.Sprintf("\r%s\n", result)
	}
	// blank the lines of the bars that left the frame
	var blanks int
	if !first {
		blanks = p.lastBarsCount - len(bars)
	}
	for i := 0; i < blanks; i++ {
		out += fmt.Sprintf("\r%s\n", strings.Repeat(" ", cols))
	}
	if p.Output != nil {
		fmt.Fprint(p.Output, out)
	} else {
		fmt.Print(out)
	}
	moveCursorUp(blanks)
	p.lastBarsCount = len(bars)
	return isFinished
}

// moveCursorUp moves the cursor to the start of the line n rows above
func moveCursorUp(n int) {
	if n <= 0 {
		return
	}
	coords, err := termutil.GetCursorPos()
	if err != nil {
		log.Panic(err)
	}
	coords.Y -= int16(n)
	if coords.Y < 0 {
		coords.Y = 0
	}
	coords.X = 0

	err = termutil.SetCursorPos(coords)
	if err != nil {
		log.Panic(err)
	}
}
