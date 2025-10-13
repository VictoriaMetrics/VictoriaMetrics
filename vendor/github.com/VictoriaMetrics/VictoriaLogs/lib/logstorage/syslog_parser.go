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

func (p *SyslogParser) AddMessageField(s string) {
	if !strings.HasPrefix(s, "CEF:") {
		p.AddField("message", s)
		return
	}

	s = strings.TrimPrefix(s, "CEF:")
	fields := p.Fields
	if p.parseCEFMessage(s) {
		return
	}
	p.Fields = fields
	p.AddField("message", s)
}

// AddField adds name=value log field to p.Fields.
func (p *SyslogParser) AddField(name, value string) {
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

	p.AddField("priority", priorityStr)
	priority, ok := tryParseUint64(priorityStr)
	if !ok {
		// Cannot parse priority
		return
	}
	facility := priority / 8
	severity := priority % 8

	facilityKeyword := syslogFacilityToLevel(facility)
	p.AddField("facility_keyword", facilityKeyword)

	level := syslogSeverityToLevel(severity)
	p.AddField("level", level)

	bufLen := len(p.buf)
	p.buf = marshalUint64String(p.buf, facility)
	p.AddField("facility", bytesutil.ToUnsafeString(p.buf[bufLen:]))

	bufLen = len(p.buf)
	p.buf = marshalUint64String(p.buf, severity)
	p.AddField("severity", bytesutil.ToUnsafeString(p.buf[bufLen:]))

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

	p.AddField("format", "rfc5424")

	if len(s) == 0 {
		return
	}

	// Parse timestamp
	n := strings.IndexByte(s, ' ')
	if n < 0 {
		p.AddField("timestamp", s)
		return
	}
	p.AddField("timestamp", s[:n])
	s = s[n+1:]

	// Parse hostname
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.AddField("hostname", s)
		return
	}
	p.AddField("hostname", s[:n])
	s = s[n+1:]

	// Parse app-name
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.AddField("app_name", s)
		return
	}
	p.AddField("app_name", s[:n])
	s = s[n+1:]

	// Parse procid
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.AddField("proc_id", s)
		return
	}
	p.AddField("proc_id", s[:n])
	s = s[n+1:]

	// Parse msgID
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		p.AddField("msg_id", s)
		return
	}
	p.AddField("msg_id", s[:n])
	s = s[n+1:]

	// Parse structured data
	tail, ok := p.parseRFC5424SD(s)
	if !ok {
		return
	}
	s = tail

	// Parse message
	p.AddMessageField(s)
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
		p.AddField(sdID[:n], sdID[n+1:])
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
			p.AddField(sdID, "")
		}
	} else {
		for _, f := range p.sdParser.fields {
			if sdID == "" {
				p.AddField(f.Name, f.Value)
				continue
			}

			bufLen := len(p.buf)
			p.buf = append(p.buf, sdID...)
			p.buf = append(p.buf, '.')
			p.buf = append(p.buf, f.Name...)

			fieldName := bytesutil.ToUnsafeString(p.buf[bufLen:])
			p.AddField(fieldName, f.Value)
		}
	}

	s = s[i+1:]
	return s, true
}

func (p *SyslogParser) parseRFC3164(s string) {
	// See https://datatracker.ietf.org/doc/html/rfc3164

	p.AddField("format", "rfc3164")

	// Parse timestamp: prefer classic RFC3164
	n := len(time.Stamp)
	if len(s) < n {
		p.AddMessageField(s)
		return
	}

	if s[len("2006-01-02")] != 'T' {
		// Parse RFC3164 timestamp.
		if !p.tryParseTimestampRFC3164(s[:n]) {
			p.AddMessageField(s)
			return
		}
	} else {
		// Parse RFC3339 timestamp.
		// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/303
		n = strings.IndexByte(s, ' ')
		if n < 0 {
			p.AddMessageField(s)
			return
		}
		if !p.tryParseTimestampRFC3339Nano(s[:n]) {
			p.AddMessageField(s)
			return
		}
	}
	s = s[n:]

	if len(s) == 0 || s[0] != ' ' {
		// Missing space after the time field
		if len(s) > 0 {
			p.AddMessageField(s)
		}
		return
	}
	s = s[1:]

	// Parse hostname
	n = strings.IndexByte(s, ' ')
	if n < 0 {
		// If there is no space, the remainder could be either hostname or tag.
		// Detect common tag patterns (contains ':' or '['). If detected, skip hostname assignment
		// and let the tag parsing below handle it.
		candidate := s
		if strings.ContainsAny(candidate, ":[") {
			// no hostname; continue without consuming s
		} else {
			p.AddField("hostname", s)
			return
		}
	} else {
		candidate := s[:n]
		if strings.ContainsAny(candidate, ":[") {
			// The token after timestamp looks like a tag (e.g. "app[pid]:").
			// Treat as missing hostname and do not consume it; proceed to tag parsing with s unchanged.
		} else {
			p.AddField("hostname", candidate)
			s = s[n+1:]
		}
	}

	// Parse tag (aka app_name)
	n = strings.IndexAny(s, "[: ")
	if n < 0 {
		p.AddField("app_name", s)
		return
	}
	appName := s[:n]
	p.AddField("app_name", appName)
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
		p.AddField("proc_id", s[:n])
		s = s[n+1:]
	}

	// Skip optional ': ' in front of message
	s = strings.TrimPrefix(s, ":")
	s = strings.TrimPrefix(s, " ")

	if len(s) > 0 {
		if appName == "CEF" {
			fields := p.Fields
			if p.parseCEFMessage(s) {
				return
			}
			p.Fields = fields
		}
		p.AddMessageField(s)
	}
}

// parseCEFMessage parses CEF message. See https://www.microfocus.com/documentation/arcsight/arcsight-smartconnectors-8.3/cef-implementation-standard/Content/CEF/Chapter%201%20What%20is%20CEF.htm
func (p *SyslogParser) parseCEFMessage(s string) bool {
	// Parse CEF version
	n := nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.version", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse device_vendor
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.device_vendor", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse device_product
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.device_product", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse device_version
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.device_version", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse device_event_class_id
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.device_event_class_id", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse name
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.name", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse severity
	n = nextUnescapedChar(s, '|')
	if n < 0 {
		return false
	}
	p.AddField("cef.severity", unescapeCEFValue(s[:n]))
	s = s[n+1:]

	// Parse extension
	return p.parseCEFExtension(s)
}

func (p *SyslogParser) parseCEFExtension(s string) bool {
	if s == "" {
		return true
	}
	for {
		// Parse key name
		n := nextUnescapedChar(s, '=')
		if n < 0 {
			return false
		}
		keyName := "cef.extension." + unescapeCEFValue(s[:n])
		s = s[n+1:]

		// Parse key value
		n = nextUnescapedChar(s, '=')
		if n < 0 {
			p.AddField(keyName, s)
			return true
		}

		n = strings.LastIndexByte(s[:n], ' ')
		if n < 0 {
			return false
		}
		p.AddField(keyName, unescapeCEFValue(s[:n]))
		s = s[n+1:]
	}

}

func (p *SyslogParser) tryParseTimestampRFC3164(s string) bool {
	t, err := time.Parse(time.Stamp, s)
	if err != nil {
		return false
	}

	t = t.UTC()
	t = time.Date(p.currentYear, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), p.timezone)
	if uint64(t.Unix())-24*3600 > fasttime.UnixTimestamp() {
		// Adjust time to the previous year
		t = time.Date(t.Year()-1, t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), p.timezone)
	}
	bufLen := len(p.buf)
	p.buf = marshalTimestampRFC3339NanoString(p.buf, t.UnixNano())
	p.AddField("timestamp", bytesutil.ToUnsafeString(p.buf[bufLen:]))
	return true
}

func (p *SyslogParser) tryParseTimestampRFC3339Nano(s string) bool {
	nsecs, ok := TryParseTimestampRFC3339Nano(s)
	if !ok {
		return false
	}

	bufLen := len(p.buf)
	p.buf = marshalTimestampRFC3339NanoString(p.buf, nsecs)
	p.AddField("timestamp", bytesutil.ToUnsafeString(p.buf[bufLen:]))
	return true
}

func nextUnescapedChar(s string, c byte) int {
	offset := 0
	for {
		n := strings.IndexByte(s[offset:], c)
		if n < 0 {
			return -1
		}
		offset += n

		if prevBackslashesCount(s, offset)%2 == 0 {
			return offset
		}
		offset++
	}
}

func unescapeCEFValue(s string) string {
	n := strings.IndexByte(s, '\\')
	if n < 0 {
		return s
	}

	b := make([]byte, 0, len(s))
	for {
		b = append(b, s[:n]...)
		n++
		if n >= len(s) {
			b = append(b, '\\')
			return string(b)
		}
		switch s[n] {
		case 'n':
			b = append(b, '\n')
		case 'r':
			b = append(b, '\r')
		default:
			b = append(b, s[n])
		}
		s = s[n+1:]

		n = strings.IndexByte(s, '\\')
		if n < 0 {
			b = append(b, s...)
			return string(b)
		}
	}
}

func prevBackslashesCount(s string, offset int) int {
	offsetOrig := offset
	for offset > 0 && s[offset-1] == '\\' {
		offset--
	}
	return offsetOrig - offset
}
