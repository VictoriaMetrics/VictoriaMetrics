package logstorage

import (
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
		v = &SyslogParser{
			unescaper: strings.NewReplacer(`\]`, `]`),
		}
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

	// buf contains temporary data used in Fields.
	buf []byte

	// sdParser is used for structured data parsing in rfc5424.
	// See https://datatracker.ietf.org/doc/html/rfc5424#section-6.3
	sdParser logfmtParser

	// currentYear is used as the current year for rfc3164 messages.
	currentYear int

	// timezone is used as the current timezone for rfc3164 messages.
	timezone *time.Location

	// unescaper is a replacer, which unescapes \] that is allowed in rfc5424, but breaks strings unquoting
	unescaper *strings.Replacer
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
	p.sdParser.reset()
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

	facilityKeyword := syslogFacilityToLevel(facility)
	p.addField("facility_keyword", facilityKeyword)

	level := syslogSeverityToLevel(severity)
	p.addField("level", level)

	bufLen := len(p.buf)
	p.buf = marshalUint64String(p.buf, facility)
	p.addField("facility", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	bufLen = len(p.buf)
	p.buf = marshalUint64String(p.buf, severity)
	p.addField("severity", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	p.parseNoHeader(s)
}

func syslogSeverityToLevel(severity uint64) string {
	// See https://en.wikipedia.org/wiki/Syslog#Severity_level
	// and https://grafana.com/docs/grafana/latest/explore/logs-integration/#log-level
	switch severity {
	case 0:
		return "emerg"
	case 1:
		return "alert"
	case 2:
		return "critical"
	case 3:
		return "error"
	case 4:
		return "warning"
	case 5:
		return "notice"
	case 6:
		return "info"
	case 7:
		return "debug"
	default:
		return "unknown"
	}
}

func syslogFacilityToLevel(facitlity uint64) string {
	// See https://en.wikipedia.org/wiki/Syslog#Facility
	switch facitlity {
	case 0:
		return "kern"
	case 1:
		return "user"
	case 2:
		return "mail"
	case 3:
		return "daemon"
	case 4:
		return "auth"
	case 5:
		return "syslog"
	case 6:
		return "lpr"
	case 7:
		return "news"
	case 8:
		return "uucp"
	case 9:
		return "cron"
	case 10:
		return "authpriv"
	case 11:
		return "ftp"
	case 12:
		return "ntp"
	case 13:
		return "security"
	case 14:
		return "console"
	case 15:
		return "solaris-cron"
	case 16:
		return "local0"
	case 17:
		return "local1"
	case 18:
		return "local2"
	case 19:
		return "local3"
	case 20:
		return "local4"
	case 21:
		return "local5"
	case 22:
		return "local6"
	case 23:
		return "local7"
	default:
		return "unknown"
	}
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

	if n := strings.IndexByte(sdID, '='); n >= 0 {
		// Special case when sdID contains `key=value`
		p.addField(sdID[:n], sdID[n+1:])
		sdID = ""
	}

	// Parse structured data
	i := 0
	for i < len(s) && (s[i] != ']' || (i > 0 && s[i-1] == '\\')) {
		// skip whitespace
		if s[i] == ' ' {
			i++
			continue
		}

		// Parse name
		n := strings.IndexByte(s[i:], '=')
		if n < 0 {
			return s, false
		}
		i += n + 1

		// Parse value
		if s[i] == '"' {
			valid := false
			i++
			for i < len(s) {
				if s[i] == '"' && s[i-1] != '\\' {
					valid = true
					break
				}
				i++
			}
			if !valid {
				return s, false
			}
			i++
		} else {
			n := strings.IndexAny(s[i:], " ]")
			if n < 0 {
				return s, false
			}
			i += n
		}
	}
	if i == len(s) {
		return s, false
	}

	sdValue := p.unescaper.Replace(strings.TrimSpace(s[:i]))
	p.sdParser.parse(sdValue)
	if len(p.sdParser.fields) == 0 {
		// Special case when structured data doesn't contain any fields
		if sdID != "" {
			p.addField(sdID, "")
		}
	} else {
		for _, f := range p.sdParser.fields {
			if sdID == "" {
				p.addField(f.Name, f.Value)
				continue
			}

			bufLen := len(p.buf)
			p.buf = append(p.buf, sdID...)
			p.buf = append(p.buf, '.')
			p.buf = append(p.buf, f.Name...)

			fieldName := bytesutil.ToUnsafeString(p.buf[bufLen:])
			p.addField(fieldName, f.Value)
		}
	}

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
