package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

// Vmstorage holds the state of a vmstorage app and provides vmstorage-specific
// functions.
type Vmstorage struct {
	*app
	*ServesMetrics
	*vmstorageClient

	storageDataPath string
	httpListenAddr  string
	vminsertAddr    string
	vmselectAddr    string
}

// StartVmstorageAt starts an instance of vmstorage with the given flags. It also
// sets the default flags and populates the app instance state with runtime
// values extracted from the application log (such as httpListenAddr)
func StartVmstorageAt(instance, binary string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
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

	return &Vmstorage{
		app: app,
		ServesMetrics: &ServesMetrics{
			metricsURL: fmt.Sprintf("http://%s/metrics", stderrExtracts[1]),
			cli:        cli,
		},
		vmstorageClient: &vmstorageClient{
			vmstorageCli:   cli,
			httpListenAddr: stderrExtracts[1],
		},
		storageDataPath: stderrExtracts[0],
		httpListenAddr:  stderrExtracts[1],
		vminsertAddr:    stderrExtracts[2],
		vmselectAddr:    stderrExtracts[3],
	}, nil
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
