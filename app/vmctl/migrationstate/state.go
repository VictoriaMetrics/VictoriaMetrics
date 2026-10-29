// Package migrationstate tracks finished units of migration work and persists it on disk.
// State can be reloaded from disk to track how much work was done and from where migration can continue.
package migrationstate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// Unit is a unit of work: data for MetricName of Tenant on TR.
type Unit struct {
	Tenant     string
	MetricName string
	TR         storage.TimeRange
}

// State tracks finished units of work in an append-only file.
//
// The first line of the file is the migration params in JSON.
// Every next line is a finished unit: `<tenant> <minTimestamp> <maxTimestamp> <metricName>`.
// The state file is verbose, but human-readable. Append-only structure makes it easy to maintain.
type State struct {
	path string
	done map[string]struct{}

	mu sync.Mutex
	f  *os.File
}

// Load loads the state from the path and opens it for appending finished units.
//
// params must be JSON-serializable. They identify the migration and are stored in the first line of the file.
// If the file doesn't exist, Load creates it. If the file has different params, Load returns an error.
// The path must identify the migration, so different migrations don't share the state.
//
// The returned state must be closed with Close when no longer needed.
func Load(path string, params any) (*State, error) {
	header, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal state params: %w", err)
	}
	st := &State{
		path: path,
		done: make(map[string]struct{}),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot read state file: %w", err)
		}
		// state doesn't exist, so we create a new one
		if err := st.open(); err != nil {
			return nil, err
		}
		if _, err := st.f.Write(append(header, '\n')); err != nil {
			st.Close()
			return nil, fmt.Errorf("cannot write header to state file %q: %w", path, err)
		}
		return st, nil
	}
	// State is append-only file. If \n is missing it means vmctl was killed ungracefully.
	// Restore state from the last written \n char and discard the rest.
	n := bytes.LastIndexByte(data, '\n') + 1
	firstLine, lines, _ := bytes.Cut(data[:n], []byte{'\n'})
	if !bytes.Equal(firstLine, header) {
		return nil, fmt.Errorf("unexpected header in state file %q; got %q; want %q; remove the file to start the migration over", path, firstLine, header)
	}
	// Remove the incomplete last line, so the next appended line isn't glued to it.
	if n < len(data) {
		log.Printf("WARNING: incomplete last line %q in %q; removing it", data[n:], st.path)
		if err := os.Truncate(st.path, int64(n)); err != nil {
			return nil, fmt.Errorf("cannot remove incomplete last line from state file %q: %w", st.path, err)
		}
	}
	// lines ends with a newline, so every line is complete.
	for i := 1; len(lines) > 0; i++ {
		var line []byte
		line, lines, _ = bytes.Cut(lines, []byte{'\n'})
		s := string(line)
		// Skip lines, which don't parse.
		if !isValidStateLine(s) {
			log.Printf("WARNING: invalid state line %d: %q in %q; skipping it", i, s, st.path)
			continue
		}
		st.done[s] = struct{}{}
	}
	return st, st.open()
}

// Path returns the path to the state file.
func (st *State) Path() string {
	return st.path
}

// DoneCount returns the number of finished units loaded from the state file.
func (st *State) DoneCount() int {
	return len(st.done)
}

// open opens the state file for appending finished units.
func (st *State) open() error {
	f, err := os.OpenFile(st.path, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("cannot open state file: %w", err)
	}
	st.f = f
	return nil
}

// Close closes the state file.
func (st *State) Close() {
	if st.f != nil {
		_ = st.f.Close()
		st.f = nil
	}
}

// IsDone returns true if u is marked as finished.
func (st *State) IsDone(u *Unit) bool {
	_, ok := st.done[u.StateLine()]
	return ok
}

// MarkDone appends u to the state file.
// It doesn't update in-memory state, so IsDone will not get the change.
// We don't update st.done because it is not needed - IsDone is called before MarkDone is called.
func (st *State) MarkDone(u *Unit) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, err := st.f.WriteString(u.StateLine() + "\n"); err != nil {
		return fmt.Errorf("cannot write to state file %q: %w", st.path, err)
	}
	return nil
}

// StateLine returns u representation in the state file.
func (u *Unit) StateLine() string {
	tenant := u.Tenant
	if tenant == "" {
		tenant = "-"
	}
	return fmt.Sprintf("%s %d %d %s", tenant, u.TR.MinTimestamp, u.TR.MaxTimestamp, u.MetricName)
}

// expected valid line:
// <tenant> <start> <end> <metric>
// 0:0 1788220800000 1792673981569 go_info
func isValidStateLine(line string) bool {
	a := strings.SplitN(line, " ", 4)
	if len(a) != 4 || a[0] == "" || a[3] == "" {
		return false
	}
	if _, err := strconv.ParseInt(a[1], 10, 64); err != nil {
		return false
	}
	_, err := strconv.ParseInt(a[2], 10, 64)
	return err == nil
}
