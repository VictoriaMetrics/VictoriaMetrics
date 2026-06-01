package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"
)

// storageNodes returns the storage node addresses passed to vminsert via
// -storageNode command line flag.
func storageNodes(flags []string) []string {
	for _, flag := range flags {
		if storageNodes, found := strings.CutPrefix(flag, "-storageNode="); found {
			return strings.Split(storageNodes, ",")
		}
	}
	return nil
}

// StartVminsert starts the latest version of vminsert.
//
// The path to the binary can be provided via VMINSERT_PATH environment
// variable. If the variable is not set, ../../bin/vminsert-race will be
// used.
func StartVminsert(instance string, flags []string, cli *Client, output io.Writer) (*Vminsert, error) {
	extractREs := []*regexp.Regexp{
		httpListenAddrRE,
		vminsertClusterNativeAddrRE,
		graphiteListenAddrRE,
		openTSDBListenAddrRE,
	}
	// Add storageNode REs to block until vminsert establishes connections with
	// all storage nodes. The extracted values are unused.
	for _, sn := range storageNodes(flags) {
		logRecord := fmt.Sprintf("successfully dialed -storageNode=\"%s\"", sn)
		extractREs = append(extractREs, regexp.MustCompile(logRecord))
	}

	binary := os.Getenv("VMINSERT_PATH")
	if binary == "" {
		binary = "../../bin/vminsert-race"
	}
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-httpListenAddr":                              "127.0.0.1:0",
			"-clusternativeListenAddr":                     "127.0.0.1:0",
			"-graphiteListenAddr":                          "127.0.0.1:0",
			"-opentsdbListenAddr":                          "127.0.0.1:0",
			"-clusternative.vminsertConnsShutdownDuration": "1ms",
		},
		extractREs: extractREs,
		output:     output,
	})
	if err != nil {
		return nil, err
	}

	return newVminsert(app, cli, vminsertRuntimeValues{
		httpListenAddr:          stderrExtracts[0],
		clusternativeListenAddr: stderrExtracts[1],
		graphiteListenAddr:      stderrExtracts[2],
		openTSDBListenAddr:      stderrExtracts[3],
	}), nil
}

type vminsertRuntimeValues struct {
	httpListenAddr          string
	clusternativeListenAddr string
	graphiteListenAddr      string
	openTSDBListenAddr      string
}

func newVminsert(app *app, cli *Client, rt vminsertRuntimeValues) *Vminsert {
	metricsClient := newMetricsClient(cli, rt.httpListenAddr)
	vminsertClient := &vminsertClient{
		vminsertCli: cli,
		url: func(op, path string, opts QueryOpts) string {
			return getClusterPath(rt.httpListenAddr, op, path, opts)
		},
		openTSDBURL: func(op, path string, opts QueryOpts) string {
			return getClusterPath(rt.openTSDBListenAddr, op, path, opts)
		},
		graphiteListenAddr: rt.graphiteListenAddr,
		sendBlocking: func(t *testing.T, numRecordsToSend int, send func()) {
			t.Helper()
			sendBlocking(t, metricsClient, numRecordsToSend, send)
		},
	}

	return &Vminsert{
		app:                     app,
		metricsClient:           metricsClient,
		vminsertClient:          vminsertClient,
		httpListenAddr:          rt.httpListenAddr,
		clusternativeListenAddr: rt.clusternativeListenAddr,
	}
}

// Vminsert holds the state of a vminsert app and provides vminsert-specific
// functions.
type Vminsert struct {
	*app
	*metricsClient
	*vminsertClient

	httpListenAddr          string
	clusternativeListenAddr string
}

// ClusternativeListenAddr returns the address at which the vminsert process is
// listening for connections from other vminsert apps.
func (app *Vminsert) ClusternativeListenAddr() string {
	return app.clusternativeListenAddr
}

// HTTPAddr returns the address at which the vminsert process is
// listening for incoming HTTP requests.
func (app *Vminsert) HTTPAddr() string {
	return app.httpListenAddr
}

// String returns the string representation of the vminsert app state.
func (app *Vminsert) String() string {
	return fmt.Sprintf("{app: %s httpListenAddr: %q}", app.app, app.httpListenAddr)
}

// sendBlocking sends the data to vmstorage by executing `send` function and
// waits until the data is actually sent.
//
// vminsert does not send the data immediately. It first puts the data into a
// buffer. Then a background goroutine takes the data from the buffer sends it
// to the vmstorage. This happens every 200ms.
//
// Waiting is implemented a retrieving the value of `vm_rpc_rows_sent_total`
// metric and checking whether it is equal or greater than the wanted value.
// If it is, then the data has been sent to vmstorage.
func sendBlocking(t *testing.T, c *metricsClient, numRecordsToSend int, send func()) {
	t.Helper()

	wantRowsSentCount := c.rpcRowsSentTotal(t) + numRecordsToSend

	send()

	const (
		retries = 20
		period  = 100 * time.Millisecond
	)
	for range retries {
		d := c.rpcRowsSentTotal(t)
		if d >= wantRowsSentCount {
			return
		}
		time.Sleep(period)
	}
	t.Fatalf("timed out while waiting for inserted rows to be sent to vmstorage")
}
