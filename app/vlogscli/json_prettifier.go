package main

import (
	"encoding/json"
	"fmt"
	"io"
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
		var v any
		if err := jp.d.Decode(&v); err != nil {
			return err
		}
		line, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			panic(fmt.Errorf("BUG: cannot marshal %v to JSON: %w", v, err))
		}
		if _, err := fmt.Fprintf(jp.pw, "%s\n", line); err != nil {
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
