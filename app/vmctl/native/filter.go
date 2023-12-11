package native

import "fmt"

// Filter represents request filter
type Filter struct {
	Match       string
	TimeStart   string
	TimeEnd     string
	Chunk       string
	TimeReverse bool
}

func (f Filter) String() string {
	s := fmt.Sprintf("\n\tfilter: match[]=%s", f.Match)
	if f.TimeStart != "" {
		s += fmt.Sprintf("\n\tstart: %s", f.TimeStart)
	}
	if f.TimeEnd != "" {
		s += fmt.Sprintf("\n\tend: %s", f.TimeEnd)
	}
	return s
}
