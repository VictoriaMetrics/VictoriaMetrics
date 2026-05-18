package logstorage

import (
	"sync"

	"github.com/valyala/fastjson"
)

// JSONScanner scans all JSON messages from a string in a streaming manner.
//
// Call Init() for initializing the scanner and then call NextLogMessage() for scanning
// JSON messages one by one into the Fields.
//
// See https://docs.victoriametrics.com/victorialogs/keyconcepts/#data-model
//
// Use GetJSONScanner() for obtaining the scanner.
type JSONScanner struct {
	commonJSON

	// s is used for JSON parsing
	s fastjson.Scanner

	// err contains parsing error
	err error
}

func (s *JSONScanner) reset() {
	s.commonJSON.reset()
	s.err = nil
}

// GetJSONScanner returns JSONScanner ready to parse JSON lines.
//
// Return the parser to the pool when it is no longer needed by calling PutJSONScanner().
func GetJSONScanner() *JSONScanner {
	v := scannerPool.Get()
	if v == nil {
		return &JSONScanner{}
	}
	return v.(*JSONScanner)
}

// PutJSONScanner returns the parser to the pool.
//
// The parser cannot be used after returning to the pool.
func PutJSONScanner(s *JSONScanner) {
	s.reset()
	scannerPool.Put(s)
}

var scannerPool sync.Pool

// Init initializes s for scanning JSON messages from msg
//
// Call NextLogMessage() for scanning the next JSON message into Fields.
func (s *JSONScanner) Init(msg []byte, preserveKeys []string, fieldPrefix string) {
	s.reset()
	s.s.InitBytes(msg)
	s.init(preserveKeys, fieldPrefix, maxFieldNameSize)
}

// NextLogMessage scans the next log message into Fields.
//
// true is returned on success, false is returned on error or on the end of logs messages.
// Call Error() after NextLogMessage() returns false in order to verify the last error.
func (s *JSONScanner) NextLogMessage() bool {
	s.resetKeepSettings()

	if !s.s.Next() {
		s.err = s.s.Error()
		return false
	}
	v := s.s.Value()
	o, err := v.Object()
	if err != nil {
		s.err = err
		return false
	}
	s.appendLogFields(o)
	return true
}

// Error returns the last error from NextLogMessage() call.
func (s *JSONScanner) Error() error {
	return s.err
}
