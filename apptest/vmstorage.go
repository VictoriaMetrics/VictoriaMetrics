package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

// StartVmstorage starts the latest version of vmstorage.
//
// The path to the binary can be provided via VMSTORAGE_PATH environment
// variable. If the variable is not set, ../../bin/vmstorage-race will be used.
func StartVmstorage(instance string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
	binary := os.Getenv("VMSTORAGE_PATH")
	if binary == "" {
		binary = "../../bin/vmstorage-race"
	}
	return startVmstorage(instance, binary, flags, cli, output)
}

// startVmstorage starts an instance of vmstorage with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func startVmstorage(instance, binary string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath": fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":  "127.0.0.1:0",
			"-vminsertAddr":    "127.0.0.1:0",
			"-vmselectAddr":    "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			vminsertAddrRE,
			vmselectAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return newVmstorage(app, cli, vmstorageRuntimeValues{
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],
		vminsertAddr:    stderrExtracts[2],
		vmselectAddr:    stderrExtracts[3],
	}), nil
}

type vmstorageRuntimeValues struct {
	storageDataPath string
	httpListenAddr  string
	vminsertAddr    string
	vmselectAddr    string
}

func newVmstorage(app *app, cli *Client, rt vmstorageRuntimeValues) *Vmstorage {
	return &Vmstorage{
		app:           app,
		metricsClient: newMetricsClient(cli, rt.httpListenAddr),
		vmstorageClient: &vmstorageClient{
			cli:            cli,
			httpListenAddr: rt.httpListenAddr,
		},
		storageDataPath: rt.storageDataPath,
		httpListenAddr:  rt.httpListenAddr,
		vminsertAddr:    rt.vminsertAddr,
		vmselectAddr:    rt.vmselectAddr,
	}
}

// Vmstorage holds the state of a vmstorage app and provides vmstorage-specific
// functions.
type Vmstorage struct {
	*app
	*metricsClient
	*vmstorageClient

	storageDataPath string
	httpListenAddr  string
	vminsertAddr    string
	vmselectAddr    string
}

// VminsertAddr returns the address at which the vmstorage process is listening
// for vminsert connections.
func (app *Vmstorage) VminsertAddr() string {
	return app.vminsertAddr
}

// VmselectAddr returns the address at which the vmstorage process is listening
// for vmselect connections.
func (app *Vmstorage) VmselectAddr() string {
	return app.vmselectAddr
}

// String returns the string representation of the vmstorage app state.
func (app *Vmstorage) String() string {
	return fmt.Sprintf("{app: %s storageDataPath: %q httpListenAddr: %q vminsertAddr: %q vmselectAddr: %q}", []any{
		app.app, app.storageDataPath, app.httpListenAddr, app.vminsertAddr, app.vmselectAddr}...)
}
