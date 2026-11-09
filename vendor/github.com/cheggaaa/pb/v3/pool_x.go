//go:build linux || darwin || freebsd || netbsd || openbsd || solaris || dragonfly || plan9 || aix
// +build linux darwin freebsd netbsd openbsd solaris dragonfly plan9 aix

package pb

import (
	"fmt"
	"os"
	"strings"

	"github.com/cheggaaa/pb/v3/termutil"
)

func (p *Pool) print(first bool) bool {
	p.m.Lock()
	defer p.m.Unlock()
	rows, cols, err := termutil.TerminalSize()
	if err != nil {
		cols = defaultBarWidth
	}
	out, isFinished := p.render(first, rows, cols)
	if p.Output != nil {
		fmt.Fprint(p.Output, out)
	} else {
		fmt.Fprint(os.Stderr, out)
	}
	return isFinished
}

// render builds a frame for a terminal of the given size
func (p *Pool) render(first bool, rows, cols int) (out string, isFinished bool) {
	if !first && p.lastBarsCount > 0 {
		out = fmt.Sprintf("\033[%dA", p.lastBarsCount)
	}
	bars, isFinished := p.visibleBars(rows)
	for _, bar := range bars {
		bar.SetWidth(cols)
		result := bar.String()
		if r := cols - CellCount(result); r > 0 {
			result += strings.Repeat(" ", r)
		}
		out += fmt.Sprintf("\r%s\n", result)
	}
	if !first && len(bars) < p.lastBarsCount {
		// erase the lines of the bars that left the frame
		out += "\r\033[J"
	}
	p.lastBarsCount = len(bars)
	return
}
