package pb

import (
	"bytes"
	"fmt"
	"math"
	"sync"
	"time"
)

const (
	adElPlaceholder    = "%_ad_el_%"
	adElPlaceholderLen = len(adElPlaceholder)
)

var (
	defaultBarEls = [5]string{"[", "-", ">", "_", "]"}
)

// Element is an interface for bar elements
type Element interface {
	ProgressElement(state *State, args ...string) string
}

// ElementFunc type implements Element interface and created for simplify elements
type ElementFunc func(state *State, args ...string) string

// ProgressElement just call self func
func (e ElementFunc) ProgressElement(state *State, args ...string) string {
	return e(state, args...)
}

var elementsM sync.Mutex

var elements = map[string]Element{
	"percent":  ElementPercent,
	"counters": ElementCounters,
	"bar":      adaptiveWrap(ElementBar),
	"speed":    ElementSpeed,
	"rtime":    ElementRemainingTime,
	"etime":    ElementElapsedTime,
	"string":   ElementString,
	"cycle":    ElementCycle,
}

// RegisterElement give you a chance to use custom elements
func RegisterElement(name string, el Element, adaptive bool) {
	if adaptive {
		el = adaptiveWrap(el)
	}
	elementsM.Lock()
	elements[name] = el
	elementsM.Unlock()
}

type argsHelper []string

func (args argsHelper) getOr(n int, value string) string {
	if len(args) > n {
		return args[n]
	}
	return value
}

func (args argsHelper) getNotEmptyOr(n int, value string) (v string) {
	if v = args.getOr(n, value); v == "" {
		return value
	}
	return
}

func adaptiveWrap(el Element) Element {
	return ElementFunc(func(state *State, args ...string) string {
		state.recalc = append(state.recalc, ElementFunc(func(s *State, _ ...string) (result string) {
			s.adaptive = true
			result = el.ProgressElement(s, args...)
			s.adaptive = false
			return
		}))
		return adElPlaceholder
	})
}

// ElementPercent shows current percent of progress.
// Optionally can take one or two string arguments.
// First string will be used as value for format float64, default is "%.02f%%".
// Second string will be used when percent can't be calculated, default is "?%"
// In template use as follows: {{percent .}} or {{percent . "%.03f%%"}} or {{percent . "%.03f%%" "?"}}
var ElementPercent ElementFunc = func(state *State, args ...string) string {
	argsh := argsHelper(args)
	if state.Total() > 0 {
		return fmt.Sprintf(
			argsh.getNotEmptyOr(0, "%.02f%%"),
			float64(state.Value())/(float64(state.Total())/float64(100)),
		)
	}
	return argsh.getOr(1, "?%")
}

// ElementCounters shows current and total values.
// Optionally can take one or two string arguments.
// First string will be used as format value when Total is present (>0). Default is "%s / %s"
// Second string will be used when total <= 0. Default is "%[1]s"
// In template use as follows: {{counters .}} or {{counters . "%s/%s"}} or {{counters . "%s/%s" "%s/?"}}
var ElementCounters ElementFunc = func(state *State, args ...string) string {
	var f string
	if state.Total() > 0 {
		f = argsHelper(args).getNotEmptyOr(0, "%s / %s")
	} else {
		f = argsHelper(args).getNotEmptyOr(1, "%[1]s")
	}
	return fmt.Sprintf(f, state.Format(state.Value()), state.Format(state.Total()))
}

type elementKey int

const (
	barObj elementKey = iota
	speedObj
	cycleObj
)

type bar struct {
	eb  [5][]byte // elements in bytes
	cc  [5]int    // cell counts
	buf *bytes.Buffer
}

func (p *bar) write(state *State, eln, width int) int {
	repeat := width / p.cc[eln]
	for i := 0; i < repeat; i++ {
		p.buf.Write(p.eb[eln])
	}
	StripStringToBuffer(string(p.eb[eln]), width%p.cc[eln], p.buf)
	return width
}

func getProgressObj(state *State, args ...string) (p *bar) {
	var ok bool
	if p, ok = state.Get(barObj).(*bar); !ok {
		p = &bar{
			buf: bytes.NewBuffer(nil),
		}
		state.Set(barObj, p)
	}
	argsH := argsHelper(args)
	for i := range p.eb {
		arg := argsH.getNotEmptyOr(i, defaultBarEls[i])
		if string(p.eb[i]) != arg {
			p.cc[i] = CellCount(arg)
			p.eb[i] = []byte(arg)
			if p.cc[i] == 0 {
				p.cc[i] = 1
				p.eb[i] = []byte(" ")
			}
		}
	}
	return
}

// ElementBar make progress bar view [-->__]
// Optionally can take up to 5 string arguments. Defaults is "[", "-", ">", "_", "]"
// In template use as follows: {{bar . }} or {{bar . "<" "oOo" "|" "~" ">"}}
// Color args: {{bar . (red "[") (green "-") ...
var ElementBar ElementFunc = func(state *State, args ...string) string {
	// init
	var p = getProgressObj(state, args...)

	total, value := state.Total(), state.Value()
	if total < 0 {
		total = -total
	}
	if value < 0 {
		value = -value
	}

	// check for overflow
	if total != 0 && value > total {
		total = value
	}

	p.buf.Reset()

	var widthLeft = state.AdaptiveElWidth()
	if widthLeft <= 0 || !state.IsAdaptiveWidth() {
		widthLeft = 30
	}

	// write left border
	if p.cc[0] < widthLeft {
		widthLeft -= p.write(state, 0, p.cc[0])
	} else {
		p.write(state, 0, widthLeft)
		return p.buf.String()
	}

	// check right border size
	if p.cc[4] < widthLeft {
		// write later
		widthLeft -= p.cc[4]
	} else {
		p.write(state, 4, widthLeft)
		return p.buf.String()
	}

	var curCount int

	if total > 0 {
		// calculate count of currenct space
		curCount = int(math.Ceil((float64(value) / float64(total)) * float64(widthLeft)))
	}

	// write bar
	if total == value && state.IsFinished() {
		widthLeft -= p.write(state, 1, curCount)
	} else if toWrite := curCount - p.cc[2]; toWrite > 0 {
		widthLeft -= p.write(state, 1, toWrite)
		widthLeft -= p.write(state, 2, p.cc[2])
	} else if curCount > 0 {
		widthLeft -= p.write(state, 2, curCount)
	}
	if widthLeft > 0 {
		widthLeft -= p.write(state, 3, widthLeft)
	}
	// write right border
	p.write(state, 4, p.cc[4])
	// cut result and return string
	return p.buf.String()
}

// ElementRemainingTime calculates remaining time based on speed (EWMA)
// Optionally can take one or two string arguments.
// First string will be used as value for format time duration string, default is "%s".
// Second string will be used when bar finished and value indicates elapsed time, default is "%s"
// Third string will be used when value not available, default is "?"
// In template use as follows: {{rtime .}} or {{rtime . "%s remain"}} or {{rtime . "%s remain" "%s total" "???"}}
var ElementRemainingTime ElementFunc = func(state *State, args ...string) string {
	var rts string
	sp := getSpeedObj(state).value(state)
	if !state.IsFinished() {
		if sp > 0 {
			remain := float64(state.Total() - state.Value())
			remainDur := time.Duration(remain/sp) * time.Second
			rts = remainDur.String()
		} else {
			return argsHelper(args).getOr(2, "?")
		}
	} else {
		rts = state.Time().Truncate(time.Second).Sub(state.StartTime().Truncate(time.Second)).String()
		return fmt.Sprintf(argsHelper(args).getOr(1, "%s"), rts)
	}
	return fmt.Sprintf(argsHelper(args).getOr(0, "%s"), rts)
}

// ElementElapsedTime shows elapsed time
// Optionally cat take one argument - it's format for time string.
// In template use as follows: {{etime .}} or {{etime . "%s elapsed"}}
var ElementElapsedTime ElementFunc = func(state *State, args ...string) string {
	etm := state.Time().Truncate(time.Second).Sub(state.StartTime().Truncate(time.Second))
	return fmt.Sprintf(argsHelper(args).getOr(0, "%s"), etm.String())
}

// ElementString get value from bar by given key and print them
// bar.Set("myKey", "string to print")
// In template use as follows: {{string . "myKey"}}
var ElementString ElementFunc = func(state *State, args ...string) string {
	if len(args) == 0 {
		return ""
	}
	v := state.Get(args[0])
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// ElementCycle return next argument for every call
// In template use as follows: {{cycle . "1" "2" "3"}}
// Or mix width other elements: {{ bar . "" "" (cycle . "↖" "↗" "↘" "↙" )}}
var ElementCycle ElementFunc = func(state *State, args ...string) string {
	if len(args) == 0 {
		return ""
	}
	n, _ := state.Get(cycleObj).(int)
	if n >= len(args) {
		n = 0
	}
	state.Set(cycleObj, n+1)
	return args[n]
}
