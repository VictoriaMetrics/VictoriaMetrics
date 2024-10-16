package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

type outputMode int

const (
	outputModeJSONMultiline  = outputMode(0)
	outputModeJSONSingleline = outputMode(1)
	outputModeLogfmt         = outputMode(2)
	outputModeCompact        = outputMode(3)
)

func getOutputFormatter(outputMode outputMode) func(w io.Writer, fields []logstorage.Field) error {
	switch outputMode {
	case outputModeJSONMultiline:
		return func(w io.Writer, fields []logstorage.Field) error {
			return writeJSONObject(w, fields, true)
		}
	case outputModeJSONSingleline:
		return func(w io.Writer, fields []logstorage.Field) error {
			return writeJSONObject(w, fields, false)
		}
	case outputModeLogfmt:
		return writeLogfmtObject
	case outputModeCompact:
		return writeCompactObject
	default:
		panic(fmt.Errorf("BUG: unexpected outputMode=%d", outputMode))
	}
}

type jsonPrettifier struct {
	r         io.ReadCloser
	formatter func(w io.Writer, fields []logstorage.Field) error

	d *json.Decoder

	pr *io.PipeReader
	pw *io.PipeWriter
	bw *bufio.Writer

	wg sync.WaitGroup
}

func newJSONPrettifier(r io.ReadCloser, outputMode outputMode) *jsonPrettifier {
	d := json.NewDecoder(r)
	pr, pw := io.Pipe()
	bw := bufio.NewWriter(pw)

	formatter := getOutputFormatter(outputMode)

	jp := &jsonPrettifier{
		r:         r,
		formatter: formatter,

		d: d,

		pr: pr,
		pw: pw,
		bw: bw,
	}

	jp.wg.Add(1)
	go func() {
		defer jp.wg.Done()
		err := jp.prettifyJSONLines()
		jp.closePipesWithError(err)
	}()

	return jp
}

func (jp *jsonPrettifier) closePipesWithError(err error) {
	_ = jp.pr.CloseWithError(err)
	_ = jp.pw.CloseWithError(err)
}

func (jp *jsonPrettifier) prettifyJSONLines() error {
	for jp.d.More() {
		fields, err := readNextJSONObject(jp.d)
		if err != nil {
			return err
		}
		sort.Slice(fields, func(i, j int) bool {
			return fields[i].Name < fields[j].Name
		})
		if err := jp.formatter(jp.bw, fields); err != nil {
			return err
		}

		// Flush bw after every output line in order to show results as soon as they appear.
		if err := jp.bw.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func (jp *jsonPrettifier) Close() error {
	jp.closePipesWithError(io.ErrUnexpectedEOF)
	err := jp.r.Close()
	jp.wg.Wait()
	return err
}

func (jp *jsonPrettifier) Read(p []byte) (int, error) {
	return jp.pr.Read(p)
}

func readNextJSONObject(d *json.Decoder) ([]logstorage.Field, error) {
	t, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("cannot read '{': %w", err)
	}
	delim, ok := t.(json.Delim)
	if !ok || delim.String() != "{" {
		return nil, fmt.Errorf("unexpected token read; got %q; want '{'", delim)
	}

	var fields []logstorage.Field
	for {
		// Read object key
		t, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot read JSON object key or closing brace: %w", err)
		}
		delim, ok := t.(json.Delim)
		if ok {
			if delim.String() == "}" {
				return fields, nil
			}
			return nil, fmt.Errorf("unexpected delimiter read; got %q; want '}'", delim)
		}
		key, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token read for object key: %v; want string or '}'", t)
		}

		// read object value
		t, err = d.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot read JSON object value: %w", err)
		}
		value, ok := t.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected token read for oject value: %v; want string", t)
		}

		fields = append(fields, logstorage.Field{
			Name:  key,
			Value: value,
		})
	}
}

func writeLogfmtObject(w io.Writer, fields []logstorage.Field) error {
	data := logstorage.MarshalFieldsToLogfmt(nil, fields)
	_, err := fmt.Fprintf(w, "%s\n", data)
	return err
}

func writeCompactObject(w io.Writer, fields []logstorage.Field) error {
	if len(fields) == 1 {
		// Just write field value as is without name
		_, err := fmt.Fprintf(w, "%s\n", fields[0].Value)
		return err
	}
	if len(fields) == 2 && fields[0].Name == "_time" || fields[1].Name == "_time" {
		// Write _time\tfieldValue as is
		if fields[0].Name == "_time" {
			_, err := fmt.Fprintf(w, "%s\t%s\n", fields[0].Value, fields[1].Value)
			return err
		}
		_, err := fmt.Fprintf(w, "%s\t%s\n", fields[1].Value, fields[0].Value)
		return err
	}

	// Fall back to logfmt
	return writeLogfmtObject(w, fields)
}

func writeJSONObject(w io.Writer, fields []logstorage.Field, isMultiline bool) error {
	if len(fields) == 0 {
		fmt.Fprintf(w, "{}\n")
		return nil
	}

	fmt.Fprintf(w, "{")
	writeNewlineIfNeeded(w, isMultiline)
	if err := writeJSONObjectKeyValue(w, fields[0], isMultiline); err != nil {
		return err
	}
	for _, f := range fields[1:] {
		fmt.Fprintf(w, ",")
		writeNewlineIfNeeded(w, isMultiline)
		if err := writeJSONObjectKeyValue(w, f, isMultiline); err != nil {
			return err
		}
	}
	writeNewlineIfNeeded(w, isMultiline)
	fmt.Fprintf(w, "}\n")
	return nil
}

func writeNewlineIfNeeded(w io.Writer, isMultiline bool) {
	if isMultiline {
		fmt.Fprintf(w, "\n")
	}
}

func writeJSONObjectKeyValue(w io.Writer, f logstorage.Field, isMultiline bool) error {
	key := getJSONString(f.Name)
	value := getJSONString(f.Value)
	if isMultiline {
		_, err := fmt.Fprintf(w, "  %s: %s", key, value)
		return err
	}
	_, err := fmt.Fprintf(w, "%s:%s", key, value)
	return err
}

func getJSONString(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error when marshaling string to JSON: %w", err))
	}
	return string(data)
}
