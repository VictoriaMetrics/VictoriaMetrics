package apptest

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// Regular expressions for runtime information to extract from the app logs.
var (
	storageDataPathRE           = regexp.MustCompile(`successfully opened storage "(.*)"`)
	httpListenAddrRE            = regexp.MustCompile(`started server at http://(.*:\d{1,5})/`)
	graphiteListenAddrRE        = regexp.MustCompile(`started TCP Graphite server at "(.*:\d{1,5})"`)
	openTSDBListenAddrRE        = regexp.MustCompile(`started TCP OpenTSDB collector at "(.*:\d{1,5})"`)
	vminsertAddrRE              = regexp.MustCompile(`accepting vminsert conns at (.*:\d{1,5})$`)
	vminsertClusterNativeAddrRE = regexp.MustCompile(`started TCP clusternative server at "(.*:\d{1,5})"`)
	vmselectAddrRE              = regexp.MustCompile(`accepting vmselect conns at (.*:\d{1,5})$`)

	logsStorageDataPathRE = regexp.MustCompile(`opening storage at -storageDataPath=(.*)`)
)

// app represents an instance of some VictoriaMetrics server (such as vmstorage,
// vminsert, or vmselect).
type app struct {
	instance string
	binary   string
	flags    []string
	process  *os.Process
	wait     bool
}

// appOptions holds the optional configuration of an app, such as default flags
// to set and things to extract from the app's log.
type appOptions struct {
	defaultFlags map[string]string
	extractREs   []*regexp.Regexp
	wait         bool
}

// startApp starts an instance of an app using the app binary file path and
// flags. When the opts are set, it also sets the default flag values and
// extracts runtime information from the app's log.
//
// If the app has started successfully and all the requested items has been
// extracted from logs, the function returns the instance of the app and the
// extracted items. The extracted items are returned in the same order as the
// corresponding extract regular expression have been provided in the opts.
//
// The function returns an error if the application has failed to start or the
// function has timed out extracting items from the log (normally because no log
// records match the regular expression).
func startApp(instance string, binary string, flags []string, opts *appOptions) (*app, []string, error) {
	flags = setDefaultFlags(flags, opts.defaultFlags)

	cmd := exec.Command(binary, flags...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	app := &app{
		instance: instance,
		binary:   binary,
		flags:    flags,
		process:  cmd.Process,
		wait:     opts.wait,
	}

	go app.processOutput("stdout", stdout, app.writeToStderr)

	lineProcessors := make([]lineProcessor, len(opts.extractREs))
	reExtractors := make([]*reExtractor, len(opts.extractREs))
	timeout := time.NewTimer(5 * time.Second).C
	for i, re := range opts.extractREs {
		reExtractors[i] = newREExtractor(re, timeout)
		lineProcessors[i] = reExtractors[i].extractRE
	}
	go app.processOutput("stderr", stderr, append(lineProcessors, app.writeToStderr)...)

	extracts, err := extractREs(reExtractors, timeout)
	if err != nil {
		app.Stop()
		return nil, nil, err
	}

	if app.wait {
		err = cmd.Wait()
	}

	return app, extracts, err
}

// setDefaultFlags adds flags with default values to `flags` if it does not
// initially contain them.
func setDefaultFlags(flags []string, defaultFlags map[string]string) []string {
	for _, flag := range flags {
		for name := range defaultFlags {
			if strings.HasPrefix(flag, name) {
				delete(defaultFlags, name)
				continue
			}
		}
	}
	for name, value := range defaultFlags {
		flags = append(flags, name+"="+value)
	}
	return flags
}

// Stop sends the app process a SIGINT signal and waits until it terminates
// gracefully.
func (app *app) Stop() {
	if app.wait {
		return
	}
	if err := app.process.Signal(os.Interrupt); err != nil {
		log.Fatalf("Could not send SIGINT signal to %s process: %v", app.instance, err)
	}
	if _, err := app.process.Wait(); err != nil {
		log.Fatalf("Could not wait for %s process completion: %v", app.instance, err)
	}
}

// Name returns the application instance name.
func (app *app) Name() string {
	return app.instance
}

// String returns the string representation of the app state.
func (app *app) String() string {
	return fmt.Sprintf("{instance: %q binary: %q flags: %q}", app.instance, app.binary, app.flags)
}

// lineProcessor is a function that is applied to the each line of the app
// output (stdout or stderr). The function returns true to indicate the caller
// that it has completed its work and should not be called again.
type lineProcessor func(line string) (done bool)

// processOutput invokes a set of processors on each line of app output (stdout
// or stderr). Once a line processor is done (returns true) it is never invoked
// again.
//
// A simple use case for this is to pipe the output of the child process to the
// output of the parent process. A more sophisticated one is to retrieve some
// runtime information from the child process logs, such as the server's
// host:port.
func (app *app) processOutput(outputName string, output io.Reader, lps ...lineProcessor) {
	activeLPs := map[int]lineProcessor{}
	for i, lp := range lps {
		activeLPs[i] = lp
	}

	scanner := bufio.NewScanner(output)
	for scanner.Scan() {
		line := scanner.Text()
		for i, process := range activeLPs {
			if process(line) {
				delete(activeLPs, i)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("could not scan %s %s: %v", app.instance, outputName, err)
	}
}

// writeToStderr is a line processor that writes the line to the stderr.
// The function always returns false to indicate its caller that each line must
// be written to the stderr.
func (app *app) writeToStderr(line string) bool {
	fmt.Fprintf(os.Stderr, "%s %s\n", app.instance, line)
	return false
}

// extractREs waits until all reExtractors return the result and then returns
// the combined result with items ordered the same way as reExtractors.
//
// The function returns an error if timeout occurs sooner then all reExtractors
// finish its work.
func extractREs(reExtractors []*reExtractor, timeout <-chan time.Time) ([]string, error) {
	n := len(reExtractors)
	notFoundREs := make(map[int]string)
	extracts := make([]string, n)
	cases := make([]reflect.SelectCase, n+1)
	for i, x := range reExtractors {
		cases[i] = x.selectCase
		notFoundREs[i] = x.re.String()
	}
	cases[n] = reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(timeout),
	}

	for notFound := n; notFound > 0; {
		i, value, _ := reflect.Select(cases)
		if i == n {
			// n-th select case means timeout.

			values := func(m map[int]string) []string {
				s := []string{}
				for _, v := range m {
					s = append(s, v)
				}
				return s
			}
			return nil, fmt.Errorf("could not extract some or all regexps from stderr: %q", values(notFoundREs))
		}
		extracts[i] = value.String()
		delete(notFoundREs, i)
		notFound--
	}
	return extracts, nil
}

// reExtractor extracts some information based on a regular expression from the
// app output within a timeout.
type reExtractor struct {
	re         *regexp.Regexp
	result     chan string
	timeout    <-chan time.Time
	selectCase reflect.SelectCase
}

// newREExtractor create a new reExtractor based on a regexp and a timeout.
func newREExtractor(re *regexp.Regexp, timeout <-chan time.Time) *reExtractor {
	result := make(chan string)
	return &reExtractor{
		re:      re,
		result:  result,
		timeout: timeout,
		selectCase: reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(result),
		},
	}
}

// extractRE is a line processor that extracts some information from a line
// based on a regular expression. The function returns true to indicate that
// it should not be called again, either when the match is found or due to
// the timeout. The found match is written to the x.result channel and it is
// important that this channel is monitored by a separate goroutine, otherwise
// the function will block.
func (x *reExtractor) extractRE(line string) bool {
	submatch := x.re.FindSubmatch([]byte(line))
	if len(submatch) > 0 {
		// Some regexps are used to just find a match without submatches.
		result := ""
		if len(submatch) > 1 {
			// But if submatches have been found, return the first one.
			result = string(submatch[1])
		}
		select {
		case x.result <- result:
		case <-x.timeout:
		}
		return true
	}
	return false
}
