// +build linux darwin freebsd netbsd openbsd solaris dragonfly plan9 aix

package pb

import (
	"fmt"
	"os"

	"github.com/cheggaaa/pb/v3/termutil"
)

func (p *Pool) print(first bool) bool {
	p.m.Lock()
	defer p.m.Unlock()
	var out string
	if !first {
		out = fmt.Sprintf("\033[%dA", p.lastBarsCount)
	}
	isFinished := true
	bars := p.bars
	rows, cols, err := termutil.TerminalSize()
	if err != nil {
		cols = defaultBarWidth
	}
	if rows > 0 && len(bars) > rows {
		// we need to hide bars that overflow terminal height
		bars = bars[len(bars)-rows:]
	}
	for _, bar := range bars {
		if !bar.IsFinished() {
			isFinished = false
		}
		bar.SetWidth(cols)
		out += fmt.Sprintf("\r%s\n", bar.String())
	}
	if p.Output != nil {
		fmt.Fprint(p.Output, out)
	} else {
		fmt.Fprint(os.Stderr, out)
	}
	p.lastBarsCount = len(bars)
	return isFinished
}
