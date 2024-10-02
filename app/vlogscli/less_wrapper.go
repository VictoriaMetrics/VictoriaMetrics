package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/mattn/go-isatty"
)

func isTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd()) && isatty.IsTerminal(os.Stderr.Fd())
}

func readWithLess(r io.Reader) error {
	if !isTerminal() {
		// Just write everything to stdout if no terminal is available.
		_, err := io.Copy(os.Stdout, r)
		if err != nil && !isErrPipe(err) {
			return fmt.Errorf("error when forwarding data to stdout: %w", err)
		}
		if err := os.Stdout.Sync(); err != nil {
			return fmt.Errorf("cannot sync data to stdout: %w", err)
		}
		return nil
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		return fmt.Errorf("cannot create pipe: %w", err)
	}
	defer func() {
		_ = pr.Close()
		_ = pw.Close()
	}()

	// Ignore Ctrl+C in the current process, so 'less' could handle it properly
	cancel := ignoreSignals(os.Interrupt)
	defer cancel()

	// Start 'less' process
	path, err := exec.LookPath("less")
	if err != nil {
		return fmt.Errorf("cannot find 'less' command: %w", err)
	}
	p, err := os.StartProcess(path, []string{"less", "-F", "-X"}, &os.ProcAttr{
		Env:   append(os.Environ(), "LESSCHARSET=utf-8"),
		Files: []*os.File{pr, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("cannot start 'less' process: %w", err)
	}

	// Close pr after 'less' finishes in a parallel goroutine
	// in order to unblock forwarding data to stopped 'less' below.
	waitch := make(chan *os.ProcessState)
	go func() {
		// Wait for 'less' process to finish.
		ps, err := p.Wait()
		if err != nil {
			fatalf("unexpected error when waiting for 'less' process: %w", err)
		}
		_ = pr.Close()
		waitch <- ps
	}()

	// Forward data from r to 'less'
	_, err = io.Copy(pw, r)
	_ = pw.Sync()
	_ = pw.Close()

	// Wait until 'less' finished
	ps := <-waitch

	// Verify 'less' status.
	if !ps.Success() {
		return fmt.Errorf("'less' finished with unexpected code %d", ps.ExitCode())
	}

	if err != nil && !isErrPipe(err) {
		return fmt.Errorf("error when forwarding data to 'less': %w", err)
	}

	return nil
}

func isErrPipe(err error) bool {
	return errors.Is(err, syscall.EPIPE) || errors.Is(err, io.ErrClosedPipe)
}

func ignoreSignals(sigs ...os.Signal) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, sigs...)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, ok := <-ch
			if !ok {
				return
			}
		}
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
		wg.Wait()
	}
}
