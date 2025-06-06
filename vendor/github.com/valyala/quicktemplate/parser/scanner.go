package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// token ids
const (
	text = iota
	tagName
	tagContents
)

var tokenStrMap = map[int]string{
	text:        "text",
	tagName:     "tagName",
	tagContents: "tagContents",
}

func tokenIDToStr(id int) string {
	str := tokenStrMap[id]
	if str == "" {
		panic(fmt.Sprintf("unknown tokenID=%d", id))
	}
	return str
}

type token struct {
	ID    int
	Value []byte

	line int
	pos  int
}

func (t *token) init(id, line, pos int) {
	t.ID = id
	t.Value = t.Value[:0]

	t.line = line
	t.pos = pos
}

func (t *token) String() string {
	return fmt.Sprintf("Token %q, value %q", tokenIDToStr(t.ID), t.Value)
}

type scanner struct {
	r   *bufio.Reader
	t   token
	c   byte
	err error

	filePath string

	line    int
	lineStr []byte

	nextTokenID int

	capture       bool
	capturedValue []byte

	collapseSpaceDepth int
	stripSpaceDepth    int
	stripToNewLine     bool
	rewind             bool
}

var tailOfLine = regexp.MustCompile(`^[[:blank:]]*(?:\r*\n)?`)
var prevBlank = regexp.MustCompile(`[[:blank:]]+$`)

func newScanner(r io.Reader, filePath string) *scanner {
	// Substitute backslashes with forward slashes in filePath
	// for the sake of consistency on different platforms (windows, linux).
	// See https://github.com/valyala/quicktemplate/issues/62.
	filePath = strings.Replace(filePath, "\\", "/", -1)

	return &scanner{
		r:        bufio.NewReader(r),
		filePath: filePath,
	}
}

func (s *scanner) Rewind() {
	if s.rewind {
		panic("BUG: duplicate Rewind call")
	}
	s.rewind = true
}

func (s *scanner) Next() bool {
	if s.rewind {
		s.rewind = false
		return true
	}

	for {
		if !s.scanToken() {
			return false
		}
		switch s.t.ID {
		case text:
			if s.stripToNewLine {
				s.t.Value = tailOfLine.ReplaceAll(s.t.Value, nil)
				s.stripToNewLine = false
			}
			if len(s.t.Value) == 0 {
				// skip empty text
				continue
			}
		case tagName:
			switch string(s.t.Value) {
			case "comment":
				if !s.skipComment() {
					return false
				}
				continue
			case "plain":
				if !s.readPlain() {
					return false
				}
				if len(s.t.Value) == 0 {
					// skip empty text
					continue
				}
			case "collapsespace":
				if !s.readTagContents() {
					return false
				}
				s.collapseSpaceDepth++
				continue
			case "stripspace":
				if !s.readTagContents() {
					return false
				}
				s.stripSpaceDepth++
				continue
			case "endcollapsespace":
				if s.collapseSpaceDepth == 0 {
					s.err = fmt.Errorf("endcollapsespace tag found without the corresponding collapsespace tag")
					return false
				}
				if !s.readTagContents() {
					return false
				}
				s.collapseSpaceDepth--
				continue
			case "endstripspace":
				if s.stripSpaceDepth == 0 {
					s.err = fmt.Errorf("endstripspace tag found without the corresponding stripspace tag")
					return false
				}
				if !s.readTagContents() {
					return false
				}
				s.stripSpaceDepth--
				continue
			case "space":
				if !s.readTagContents() {
					return false
				}
				s.t.init(text, s.t.line, s.t.pos)
				s.t.Value = append(s.t.Value[:0], ' ')
				return true
			case "newline":
				if !s.readTagContents() {
					return false
				}
				s.t.init(text, s.t.line, s.t.pos)
				s.t.Value = append(s.t.Value[:0], '\n')
				return true
			}
		}
		return true
	}
}

func (s *scanner) readPlain() bool {
	if !s.readTagContents() {
		return false
	}
	startLine := s.line
	startPos := s.pos()
	s.startCapture()
	ok := s.skipUntilTag("endplain")
	v := s.stopCapture()
	s.t.init(text, startLine, startPos)
	if ok {
		n := bytes.LastIndex(v, strTagOpen)
		v = v[:n]
		s.t.Value = append(s.t.Value[:0], v...)
	}
	return ok
}

var strTagOpen = []byte("{%")

func (s *scanner) skipComment() bool {
	if !s.readTagContents() {
		return false
	}
	return s.skipUntilTag("endcomment")
}

func (s *scanner) skipUntilTag(tagName string) bool {
	ok := false
	for {
		if !s.nextByte() {
			break
		}
		if s.c != '{' {
			continue
		}
		if !s.nextByte() {
			break
		}
		if s.c != '%' {
			s.unreadByte('~')
			continue
		}
		ok = s.readTagName()
		s.nextTokenID = text
		if !ok {
			s.err = nil
			continue
		}
		if string(s.t.Value) == tagName {
			ok = s.readTagContents()
			break
		}
	}
	if !ok {
		s.err = fmt.Errorf("cannot find %q tag: %s", tagName, s.err)
	}
	return ok
}

func (s *scanner) scanToken() bool {
	switch s.nextTokenID {
	case text:
		return s.readText()
	case tagName:
		return s.readTagName()
	case tagContents:
		return s.readTagContents()
	default:
		panic(fmt.Sprintf("BUG: unknown nextTokenID %d", s.nextTokenID))
	}
}

func (s *scanner) readText() bool {
	s.t.init(text, s.line, s.pos())
	ok := false
	for {
		if !s.nextByte() {
			ok = (len(s.t.Value) > 0)
			break
		}
		if s.c != '{' {
			s.appendByte()
			continue
		}
		if !s.nextByte() {
			s.appendByte()
			ok = true
			break
		}
		if s.c == '%' {
			s.nextTokenID = tagName
			ok = true
			if !s.nextByte() {
				s.appendByte()
				break
			}
			if s.c != '-' {
				s.unreadByte(s.c)
				break
			}
			s.t.Value = prevBlank.ReplaceAll(s.t.Value, nil)
			break
		}
		s.unreadByte('{')
		s.appendByte()
	}
	if s.stripSpaceDepth > 0 {
		s.t.Value = stripSpace(s.t.Value)
	} else if s.collapseSpaceDepth > 0 {
		s.t.Value = collapseSpace(s.t.Value)
	}
	return ok
}

func (s *scanner) readTagName() bool {
	s.skipSpace()
	s.t.init(tagName, s.line, s.pos())
	for {
		if s.isSpace() || s.c == '%' {
			if s.c == '%' {
				s.unreadByte('~')
			}
			s.nextTokenID = tagContents
			return true
		}
		if (s.c >= 'a' && s.c <= 'z') || (s.c >= 'A' && s.c <= 'Z') || (s.c >= '0' && s.c <= '9') || s.c == '=' || s.c == '.' {
			s.appendByte()
			if !s.nextByte() {
				return false
			}
			continue
		}
		if s.c == '-' {
			s.unreadByte(s.c)
			s.nextTokenID = tagContents
			return true
		}
		s.err = fmt.Errorf("unexpected character: '%c'", s.c)
		s.unreadByte('~')
		return false
	}
}

func (s *scanner) readTagContents() bool {
	s.skipSpace()
	s.t.init(tagContents, s.line, s.pos())
	for {
		if s.c != '%' {
			s.appendByte()
			if !s.nextByte() {
				return false
			}
			continue
		}
		if !s.nextByte() {
			s.appendByte()
			return false
		}
		if s.c == '}' {
			if bytes.HasSuffix(s.t.Value, []byte("-")) {
				s.t.Value = s.t.Value[:len(s.t.Value)-1]
				s.stripToNewLine = true
			}
			s.nextTokenID = text
			s.t.Value = stripTrailingSpace(s.t.Value)
			return true
		}
		s.unreadByte('%')
		s.appendByte()
		if !s.nextByte() {
			return false
		}
	}
}

func (s *scanner) skipSpace() {
	for s.nextByte() && s.isSpace() {
	}
}

func (s *scanner) isSpace() bool {
	return isSpace(s.c)
}

func (s *scanner) nextByte() bool {
	if s.err != nil {
		return false
	}
	c, err := s.r.ReadByte()
	if err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		s.err = err
		return false
	}
	if c == '\n' {
		s.line++
		s.lineStr = s.lineStr[:0]
	} else {
		s.lineStr = append(s.lineStr, c)
	}
	s.c = c
	if s.capture {
		s.capturedValue = append(s.capturedValue, c)
	}
	return true
}

func (s *scanner) startCapture() {
	s.capture = true
	s.capturedValue = s.capturedValue[:0]
}

func (s *scanner) stopCapture() []byte {
	s.capture = false
	v := s.capturedValue
	s.capturedValue = s.capturedValue[:0]
	return v
}

func (s *scanner) Token() *token {
	return &s.t
}

func (s *scanner) LastError() error {
	if s.err == nil {
		return nil
	}
	if s.err == io.ErrUnexpectedEOF && s.t.ID == text {
		if s.collapseSpaceDepth > 0 {
			return fmt.Errorf("missing endcollapsespace tag at %s", s.Context())
		}
		if s.stripSpaceDepth > 0 {
			return fmt.Errorf("missing endstripspace tag at %s", s.Context())
		}
		return nil
	}

	return fmt.Errorf("error when reading %s at %s: %s",
		tokenIDToStr(s.t.ID), s.Context(), s.err)
}

func (s *scanner) appendByte() {
	s.t.Value = append(s.t.Value, s.c)
}

func (s *scanner) unreadByte(c byte) {
	if err := s.r.UnreadByte(); err != nil {
		panic(fmt.Sprintf("BUG: bufio.Reader.UnreadByte returned non-nil error: %s", err))
	}
	if s.capture {
		s.capturedValue = s.capturedValue[:len(s.capturedValue)-1]
	}
	if s.c == '\n' {
		s.line--
		s.lineStr = s.lineStr[:0] // TODO: use correct line
	} else {
		s.lineStr = s.lineStr[:len(s.lineStr)-1]
	}
	s.c = c
}

func (s *scanner) pos() int {
	return len(s.lineStr)
}

func (s *scanner) Context() string {
	t := s.Token()
	return fmt.Sprintf("file %q, line %d, pos %d, token %s, last line %s",
		s.filePath, t.line+1, t.pos, snippet(t.Value), snippet(s.lineStr))
}

func (s *scanner) WriteLineComment(w io.Writer) {
	fmt.Fprintf(w, "//line %s:%d\n", s.filePath, s.t.line+1)
}

func snippet(s []byte) string {
	if len(s) <= 40 {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%q ... %q", s[:20], s[len(s)-20:])
}
