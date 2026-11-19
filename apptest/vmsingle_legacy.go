package apptest

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"time"
)

// StartVmsingle_v1_132_0 starts vmsingle-v1.132.0 (the last version that uses
// legacy index).
//
// This binary is used in apptests that check the compatility of v1.132.0 with
// later versions.
//
// The path to the binary must be provided via VMSINGLE_V1_132_0_PATH
// environment variable.
func StartVmsingle_v1_132_0(instance string, flags []string, cli *Client, output io.Writer) (*Vmsingle, error) {
	binary := os.Getenv("VMSINGLE_V1_132_0_PATH")
	app, stderrExtracts, err := startApp(instance, binary, flags, &appOptions{
		defaultFlags: map[string]string{
			"-storageDataPath":    fmt.Sprintf("%s/%s-%d", os.TempDir(), instance, time.Now().UnixNano()),
			"-httpListenAddr":     "127.0.0.1:0",
			"-graphiteListenAddr": "127.0.0.1:0",
			"-opentsdbListenAddr": "127.0.0.1:0",
		},
		extractREs: []*regexp.Regexp{
			storageDataPathRE,
			httpListenAddrRE,
			graphiteListenAddrRE,
			openTSDBListenAddrRE,
		},
		output: output,
	})
	if err != nil {
		return nil, err
	}

	return newVmsingle(app, cli, vmsingleRuntimeValues{
		storageDataPath:    stderrExtracts[0],
		httpListenAddr:     stderrExtracts[1],
		graphiteListenAddr: stderrExtracts[2],
		openTSDBListenAddr: stderrExtracts[3],
	}), nil
}

// StartVmsingle_v1_156_0 starts vmsingle-v1.156.0 (the last version that writes
// and reads both global and per-day indexes when -disablePerDayIndex is false).
//
// This binary is used in apptests that check the compatibility of v1.156.0 with
// later versions.
//
// The path to the binary must be provided via VMSINGLE_V1_156_0_PATH
// environment variable.
func StartVmsingle_v1_156_0(instance string, flags []string, cli *Client, output io.Writer) (*Vmsingle, error) {
	binary := os.Getenv("VMSINGLE_V1_156_0_PATH")
	if binary == "" {
		return nil, fmt.Errorf("vmsingle-v1.156.0 binary path must be provided via VMSINGLE_V1_156_0_PATH environment variable")
	}
	return startVmsingle(instance, binary, flags, cli, output)
}
