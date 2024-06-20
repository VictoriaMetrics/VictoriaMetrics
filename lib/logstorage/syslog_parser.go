package logstorage

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// GetSyslogParser returns syslog parser from the pool.
//
// currentYear must contain the current year. It is used for properly setting timestamp
// field for rfc3164 format, which doesn't contain year.
//
// the timezone is used for rfc3164 format for setting the desired timezone.
//
// Return back the parser to the pool by calling PutSyslogParser when it is no longer needed.
func GetSyslogParser(currentYear int, timezone *time.Location) *SyslogParser {
	v := syslogParserPool.Get()
	if v == nil {
		v = &SyslogParser{}
	}
	p := v.(*SyslogParser)
	p.currentYear = currentYear
	p.timezone = timezone
	return p
}

// PutSyslogParser returns back syslog parser to the pool.
//
// p cannot be used after returning to the pool.
func PutSyslogParser(p *SyslogParser) {
	p.reset()
	syslogParserPool.Put(p)
}

var syslogParserPool sync.Pool

// SyslogParser is parser for syslog messages.
//
// It understands the following syslog formats:
//
// - https://datatracker.ietf.org/doc/html/rfc5424
// - https://datatracker.ietf.org/doc/html/rfc3164
//
// It extracts the following list of syslog message fields into Fields -
// https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe
type SyslogParser struct {
	// Fields contains parsed fields after Parse call.
	Fields []Field

	buf []byte

	currentYear int
	timezone    *time.Location
}

func (p *SyslogParser) reset() {
	p.currentYear = 0
	p.timezone = nil
	p.resetFields()
}

func (p *SyslogParser) resetFields() {
	clear(p.Fields)
	p.Fields = p.Fields[:0]

	p.buf = p.buf[:0]
}

func (p *SyslogParser) addField(name, value string) {
	p.Fields = append(p.Fields, Field{
		Name:  name,
		Value: value,
	})
}

// Parse parses syslog message from s into p.Fields.
//
// p.Fields is valid until s is modified or p state is changed.
func (p *SyslogParser) Parse(s string) {
	p.resetFields()

	if len(s) == 0 {
		// Cannot parse syslog message
		return
	}

	if s[0] != '<' {
		p.parseNoHeader(s)
		return
	}

	// parse priority
	s = s[1:]
	n := strings.IndexByte(s, '>')
	if n < 0 {
		// Cannot parse priority
		return
	}
	priorityStr := s[:n]
	s = s[n+1:]

	p.addField("priority", priorityStr)
	priority, ok := tryParseUint64(priorityStr)
	if !ok {
		// Cannot parse priority
		return
	}
	facility := priority / 8
	severity := priority % 8

	bufLen := len(p.buf)
	p.buf = marshalUint64String(p.buf, facility)
	p.addField("facility", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	bufLen = len(p.buf)
	p.buf = marshalUint64String(p.buf, severity)
	p.addField("severity", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	p.parseNoHeader(s)
}

func (p *SyslogParser) parseNoHeader(s string) {
	if len(s) == 0 {
		return
	}
	if strings.HasPrefix(s, "1 ") {
		p.parseRFC5424(s[2:])
	} else {
		p.parseRFC3164(s)
	}
}

func (p *SyslogParser) parseRFC5424(s string) {
	// See https://datatracker.ietf.org/doc/html/rfc5424

	p.addField("format", "rfc5424")

	if len(s) == 0 {
		return
	}

	// Parse timestamp
	n := strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("timestamp", s)
		return
	}
	p.addField("timestamp", s[:n])
	s = s[n+1:]

	// Parse hostname
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("hostname", s)
		return
	}
	p.addField("hostname", s[:n])
	s = s[n+1:]

	// Parse app-name
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("app_name", s)
		return
	}
	p.addField("app_name", s[:n])
	s = s[n+1:]

	// Parse procid
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("proc_id", s)
		return
	}
	p.addField("proc_id", s[:n])
	s = s[n+1:]

	// Parse msgID
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("msg_id", s)
		return
	}
	p.addField("msg_id", s[:n])
	s = s[n+1:]

	// Parse structured data
	tail, ok := p.parseRFC5424SD(s)
	if !ok {
		return
	}
	s = tail

	// Parse message
	p.addField("message", s)
}

func (p *SyslogParser) parseRFC5424SD(s string) (string, bool) {
	if strings.HasPrefix(s, "- ") {
		return s[2:], true
	}

	for {
		tail, ok := p.parseRFC5424SDLine(s)
		if !ok {
			return tail, false
		}
		s = tail
		if strings.HasPrefix(s, " ") {
			s = s[1:]
			return s, true
		}
	}
}

func (p *SyslogParser) parseRFC5424SDLine(s string) (string, bool) {
	if len(s) == 0 || s[0] != '[' {
		return s, false
	}
	s = s[1:]

	n := strings.IndexAny(s, " ]")
	if n < 0 {
		return s, false
	}
	sdID := s[:n]
	s = s[n:]

	// Parse structured data
	i := 0
	for i < len(s) && s[i] != ']' {
		// skip whitespace
		if s[i] != ' ' {
			return s, false
		}
		i++

		// Parse name
		n := strings.IndexByte(s[i:], '=')
		if n < 0 {
			return s, false
		}
		i += n + 1

		// Parse value
		qp, err := strconv.QuotedPrefix(s[i:])
		if err != nil {
			return s, false
		}
		i += len(qp)
	}
	if i == len(s) {
		return s, false
	}

	sdValue := strings.TrimSpace(s[:i])
	p.addField(sdID, sdValue)
	s = s[i+1:]
	return s, true
}

func (p *SyslogParser) parseRFC3164(s string) {
	// See https://datatracker.ietf.org/doc/html/rfc3164

	p.addField("format", "rfc3164")

	// Parse timestamp
	n := len(time.Stamp)
	if len(s) < n {
		p.addField("message", s)
		return
	}

	t, err := time.Parse(time.Stamp, s[:n])
	if err != nil {
		// TODO: fall back to parsing ISO8601 timestamp?
		p.addField("message", s)
		return
	}
	s = s[n:]

	t = t.UTC()
	t = time.Date(p.currentYear, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), p.timezone)
	if uint64(t.Unix())-24*3600 > fasttime.UnixTimestamp() {
		// Adjust time to the previous year
		t = time.Date(t.Year()-1, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), p.timezone)
	}

	bufLen := len(p.buf)
	p.buf = marshalTimestampISO8601String(p.buf, t.UnixNano())
	p.addField("timestamp", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	if len(s) == 0 || s[0] != ' ' {
		// Missing space after the time field
		if len(s) > 0 {
			p.addField("message", s)
		}
		return
	}
	s = s[1:]

	// Parse hostname
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.addField("hostname", s)
		return
	}
	p.addField("hostname", s[:n])
	s = s[n+1:]

	// Parse tag (aka app_name)
	n = strings.IndexAny(s, "[: ")
	if n < 0 {
		p.addField("app_name", s)
		return
	}
	p.addField("app_name", s[:n])
	s = s[n:]

	// Parse proc_id
	if len(s) == 0 {
		return
	}
	if s[0] == '[' {
		s = s[1:]
		n = strings.IndexByte(s, ']')
		if n < 0 {
			return
		}
		p.addField("proc_id", s[:n])
		s = s[n+1:]
	}

	// Skip optional ': ' in front of message
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimPrefix(s, " ")

	if len(s) > 0 {
		p.addField("message", s)
	}
}
