package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ergochat/readline"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

var (
	datasourceURL = flag.String("datasource.url", "http://localhost:9428/select/logsql/query", "URL for querying VictoriaLogs; "+
		"see https://docs.victoriametrics.com/victorialogs/querying/#querying-logs . See also -tail.url")
	tailURL = flag.String("tail.url", "", "URL for live tailing queries to VictoriaLogs; see https://docs.victoriametrics.com/victorialogs/querying/#live-tailing ."+
		"The url is automatically detected from -datasource.url by replacing /query with /tail at the end if -tail.url is empty")
	historyFile = flag.String("historyFile", "vlogscli-history", "Path to file with command history")
	header      = flagutil.NewArrayString("header", "Optional header to pass in request -datasource.url in the form 'HeaderName: value'")
	accountID   = flag.Int("accountID", 0, "Account ID to query; see https://docs.victoriametrics.com/victorialogs/#multitenancy")
	projectID   = flag.Int("projectID", 0, "Project ID to query; see https://docs.victoriametrics.com/victorialogs/#multitenancy")
)

const (
	firstLinePrompt = ";> "
	nextLinePrompt  = ""
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.InitNoLogFlags()

	hes, err := parseHeaders(*header)
	if err != nil {
		fatalf("cannot parse -header command-line flag: %s", err)
	}
	headers = hes

	incompleteLine := ""
	cfg := &readline.Config{
		Prompt:                 firstLinePrompt,
		DisableAutoSaveHistory: true,
		Listener: func(line []rune, pos int, _ rune) ([]rune, int, bool) {
			incompleteLine = string(line)
			return line, pos, false
		},
	}
	rl, err := readline.NewFromConfig(cfg)
	if err != nil {
		fatalf("cannot initialize readline: %s", err)
	}

	fmt.Fprintf(rl, "sending queries to %s\n", *datasourceURL)

	runReadlineLoop(rl, &incompleteLine)

	if err := rl.Close(); err != nil {
		fatalf("cannot close readline: %s", err)
	}

}

func runReadlineLoop(rl *readline.Instance, incompleteLine *string) {
	historyLines, err := loadFromHistory(*historyFile)
	if err != nil {
		fatalf("cannot load query history: %s", err)
	}
	for _, line := range historyLines {
		if err := rl.SaveToHistory(line); err != nil {
			fatalf("cannot initialize query history: %s", err)
		}
	}

	outputMode := outputModeJSONMultiline
	s := ""
	for {
		line, err := rl.ReadLine()
		if err != nil {
			switch err {
			case io.EOF:
				if s != "" {
					// This is non-interactive query execution.
					executeQuery(context.Background(), rl, s, outputMode)
				}
				return
			case readline.ErrInterrupt:
				if s == "" && *incompleteLine == "" {
					fmt.Fprintf(rl, "interrupted\n")
					os.Exit(128 + int(syscall.SIGINT))
				}
				// Default value for Ctrl+C - clear the prompt and store the incompletely entered line into history
				s += *incompleteLine
				historyLines = pushToHistory(rl, historyLines, s)
				s = ""
				rl.SetPrompt(firstLinePrompt)
				continue
			default:
				fatalf("unexpected error in readline: %s", err)
			}
		}

		s += line
		if s == "" {
			// Skip empty lines
			continue
		}

		if isQuitCommand(s) {
			fmt.Fprintf(rl, "bye!\n")
			_ = pushToHistory(rl, historyLines, s)
			return
		}
		if isHelpCommand(s) {
			printCommandsHelp(rl)
			historyLines = pushToHistory(rl, historyLines, s)
			s = ""
			continue
		}
		if s == `\s` {
			fmt.Fprintf(rl, "singleline json output mode\n")
			outputMode = outputModeJSONSingleline
			historyLines = pushToHistory(rl, historyLines, s)
			s = ""
			continue
		}
		if s == `\m` {
			fmt.Fprintf(rl, "multiline json output mode\n")
			outputMode = outputModeJSONMultiline
			historyLines = pushToHistory(rl, historyLines, s)
			s = ""
			continue
		}
		if s == `\c` {
			fmt.Fprintf(rl, "compact output mode\n")
			outputMode = outputModeCompact
			historyLines = pushToHistory(rl, historyLines, s)
			s = ""
			continue
		}
		if s == `\logfmt` {
			fmt.Fprintf(rl, "logfmt output mode\n")
			outputMode = outputModeLogfmt
			historyLines = pushToHistory(rl, historyLines, s)
			s = ""
			continue
		}
		if line != "" && !strings.HasSuffix(line, ";") {
			// Assume the query is incomplete and allow the user finishing the query on the next line
			s += "\n"
			rl.SetPrompt(nextLinePrompt)
			continue
		}

		// Execute the query
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		executeQuery(ctx, rl, s, outputMode)
		cancel()

		historyLines = pushToHistory(rl, historyLines, s)
		s = ""
		rl.SetPrompt(firstLinePrompt)
	}
}

func pushToHistory(rl *readline.Instance, historyLines []string, s string) []string {
	s = strings.TrimSpace(s)
	if len(historyLines) == 0 || historyLines[len(historyLines)-1] != s {
		historyLines = append(historyLines, s)
		if len(historyLines) > 500 {
			historyLines = historyLines[len(historyLines)-500:]
		}
		if err := saveToHistory(*historyFile, historyLines); err != nil {
			fatalf("cannot save query history: %s", err)
		}
	}
	if err := rl.SaveToHistory(s); err != nil {
		fatalf("cannot update query history: %s", err)
	}
	return historyLines
}

func loadFromHistory(filePath string) ([]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	linesQuoted := strings.Split(string(data), "\n")
	lines := make([]string, 0, len(linesQuoted))
	i := 0
	for _, lineQuoted := range linesQuoted {
		i++
		if lineQuoted == "" {
			continue
		}
		line, err := strconv.Unquote(lineQuoted)
		if err != nil {
			return nil, fmt.Errorf("cannot parse line #%d at %s: %w; line: [%s]", i, filePath, err, line)
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func saveToHistory(filePath string, lines []string) error {
	linesQuoted := make([]string, len(lines))
	for i, line := range lines {
		lineQuoted := strconv.Quote(line)
		linesQuoted[i] = lineQuoted
	}
	data := strings.Join(linesQuoted, "\n")
	return os.WriteFile(filePath, []byte(data), 0600)
}

func isQuitCommand(s string) bool {
	switch s {
	case `\q`, "q", "quit", "exit":
		return true
	default:
		return false
	}
}

func isHelpCommand(s string) bool {
	switch s {
	case `\h`, "h", "help", "?":
		return true
	default:
		return false
	}
}

func printCommandsHelp(w io.Writer) {
	fmt.Fprintf(w, "%s", `List of available commands:
\q - quit
\h - show this help
\s - singleline json output mode
\m - multiline json output mode
\c - compact output
\logfmt - logfmt output mode
\tail <query> - live tail <query> results
`)
}

func executeQuery(ctx context.Context, output io.Writer, qStr string, outputMode outputMode) {
	if strings.HasPrefix(qStr, `\tail `) {
		tailQuery(ctx, output, qStr, outputMode)
		return
	}

	respBody := getQueryResponse(ctx, output, qStr, outputMode, *datasourceURL)
	if respBody == nil {
		return
	}
	defer func() {
		_ = respBody.Close()
	}()

	if err := readWithLess(respBody); err != nil {
		fmt.Fprintf(output, "error when reading query response: %s\n", err)
		return
	}
}

func tailQuery(ctx context.Context, output io.Writer, qStr string, outputMode outputMode) {
	qStr = strings.TrimPrefix(qStr, `\tail `)
	qURL, err := getTailURL()
	if err != nil {
		fmt.Fprintf(output, "%s\n", err)
		return
	}

	respBody := getQueryResponse(ctx, output, qStr, outputMode, qURL)
	if respBody == nil {
		return
	}
	defer func() {
		_ = respBody.Close()
	}()

	if _, err := io.Copy(output, respBody); err != nil {
		if !errors.Is(err, context.Canceled) && !isErrPipe(err) {
			fmt.Fprintf(output, "error when live tailing query response: %s\n", err)
		}
		fmt.Fprintf(output, "\n")
		return
	}
}

func getTailURL() (string, error) {
	if *tailURL != "" {
		return *tailURL, nil
	}

	u, err := url.Parse(*datasourceURL)
	if err != nil {
		return "", fmt.Errorf("cannot parse -datasource.url=%q: %w", *datasourceURL, err)
	}
	if !strings.HasSuffix(u.Path, "/query") {
		return "", fmt.Errorf("cannot find /query suffix in -datasource.url=%q", *datasourceURL)
	}
	u.Path = u.Path[:len(u.Path)-len("/query")] + "/tail"
	return u.String(), nil
}

func getQueryResponse(ctx context.Context, output io.Writer, qStr string, outputMode outputMode, qURL string) io.ReadCloser {
	// Parse the query and convert it to canonical view.
	qStr = strings.TrimSuffix(qStr, ";")
	q, err := logstorage.ParseQuery(qStr)
	if err != nil {
		fmt.Fprintf(output, "cannot parse query: %s\n", err)
		return nil
	}
	qStr = q.String()
	fmt.Fprintf(output, "executing [%s]...", qStr)

	// Prepare HTTP request for qURL
	args := make(url.Values)
	args.Set("query", qStr)
	data := strings.NewReader(args.Encode())

	req, err := http.NewRequestWithContext(ctx, "POST", qURL, data)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot prepare request to server: %w", err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, h := range headers {
		req.Header.Set(h.Name, h.Value)
	}
	req.Header.Set("AccountID", strconv.Itoa(*accountID))
	req.Header.Set("ProjectID", strconv.Itoa(*projectID))

	// Execute HTTP request at qURL
	startTime := time.Now()
	resp, err := httpClient.Do(req)
	queryDuration := time.Since(startTime)
	fmt.Fprintf(output, "; duration: %.3fs\n", queryDuration.Seconds())
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintf(output, "\n")
		} else {
			fmt.Fprintf(output, "cannot execute query: %s\n", err)
		}
		return nil
	}

	// Verify response code
	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte(fmt.Sprintf("cannot read response body: %s", err))
		}
		fmt.Fprintf(output, "unexpected status code: %d; response body:\n%s\n", resp.StatusCode, body)
		return nil
	}

	// Prettify the response body
	jp := newJSONPrettifier(resp.Body, outputMode)

	return jp
}

var httpClient = &http.Client{}

var headers []headerEntry

type headerEntry struct {
	Name  string
	Value string
}

func parseHeaders(a []string) ([]headerEntry, error) {
	hes := make([]headerEntry, len(a))
	for i, s := range a {
		a := strings.SplitN(s, ":", 2)
		if len(a) != 2 {
			return nil, fmt.Errorf("cannot parse header=%q; it must contain at least one ':'; for example, 'Cookie: foo'", s)
		}
		hes[i] = headerEntry{
			Name:  strings.TrimSpace(a[0]),
			Value: strings.TrimSpace(a[1]),
		}
	}
	return hes, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

func usage() {
	const s = `
vlogscli is a command-line tool for querying VictoriaLogs.

See the docs at https://docs.victoriametrics.com/victorialogs/querying/vlogscli/
`
	flagutil.Usage(s)
}
