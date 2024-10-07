package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
)

type jsonPrettifier struct {
	rOriginal io.ReadCloser

	d *json.Decoder

	pr *io.PipeReader
	pw *io.PipeWriter

	wg sync.WaitGroup
}

func newJSONPrettifier(r io.ReadCloser) *jsonPrettifier {
	d := json.NewDecoder(r)
	pr, pw := io.Pipe()

	jp := &jsonPrettifier{
		rOriginal: r,
		d:         d,
		pr:        pr,
		pw:        pw,
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
		kvs, err := readNextJSONObject(jp.d)
		if err != nil {
			return err
		}
		if err := writeJSONObject(jp.pw, kvs); err != nil {
			return err
		}
	}
	return nil
}

func (jp *jsonPrettifier) Close() error {
	jp.closePipesWithError(io.ErrUnexpectedEOF)
	err := jp.rOriginal.Close()
	jp.wg.Wait()
	return err
}

func (jp *jsonPrettifier) Read(p []byte) (int, error) {
	return jp.pr.Read(p)
}

func readNextJSONObject(d *json.Decoder) ([]kv, error) {
	t, err := d.Token()
	if err != nil {
		return nil, fmt.Errorf("cannot read '{': %w", err)
	}
	delim, ok := t.(json.Delim)
	if !ok || delim.String() != "{" {
		return nil, fmt.Errorf("unexpected token read; got %q; want '{'", delim)
	}

	var kvs []kv
	for {
		// Read object key
		t, err := d.Token()
		if err != nil {
			return nil, fmt.Errorf("cannot read JSON object key or closing brace: %w", err)
		}
		delim, ok := t.(json.Delim)
		if ok {
			if delim.String() == "}" {
				return kvs, nil
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

		kvs = append(kvs, kv{
			key:   key,
			value: value,
		})
	}
}

func writeJSONObject(w io.Writer, kvs []kv) error {
	if len(kvs) == 0 {
		fmt.Fprintf(w, "{}\n")
		return nil
	}

	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].key < kvs[j].key
	})

	fmt.Fprintf(w, "{\n")
	if err := writeJSONObjectKeyValue(w, kvs[0]); err != nil {
		return err
	}
	for _, kv := range kvs[1:] {
		fmt.Fprintf(w, ",\n")
		if err := writeJSONObjectKeyValue(w, kv); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "\n}\n")
	return nil
}

func writeJSONObjectKeyValue(w io.Writer, kv kv) error {
	key := getJSONString(kv.key)
	value := getJSONString(kv.value)
	_, err := fmt.Fprintf(w, "  %s: %s", key, value)
	return err
}

func getJSONString(s string) string {
	data, err := json.Marshal(s)
	if err != nil {
		panic(fmt.Errorf("unexpected error when marshaling string to JSON: %w", err))
	}
	return string(data)
}

type kv struct {
	key   string
	value string
}
