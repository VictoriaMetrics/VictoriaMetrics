package apptest

import (
	"fmt"
	"io"
	"os"
)

// StartVmstorage_v1_132_0 starts vmstorage-v1.132.0 (the last version that uses
// legacy index).
//
// This binary is used in apptests that check the compatibility of v1.132.0 with
// later versions.
//
// The path to the binary must be provided via VMSTORAGE_V1_132_0_PATH
// environment variable.
func StartVmstorage_v1_132_0(instance string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
	binary := os.Getenv("VMSTORAGE_V1_132_0_PATH")
	if binary == "" {
		return nil, fmt.Errorf("vmstorage-v1.132.0 binary path must be provided via VMSTORAGE_V1_132_0_PATH environment variable")
	}
	return startVmstorage(instance, binary, flags, cli, output)
}

// StartVmstorage_v1_157_0 starts vmstorage-v1.157.0 (the last version that writes
// and reads both global and per-day indexes when -disablePerDayIndex is false).
//
// This binary is used in apptests that check the compatibility of v1.157.0 with
// later versions.
//
// The path to the binary must be provided via VMSTORAGE_V1_157_0_PATH
// environment variable.
func StartVmstorage_v1_157_0(instance string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
	binary := os.Getenv("VMSTORAGE_V1_157_0_PATH")
	if binary == "" {
		return nil, fmt.Errorf("vmstorage-v1.157.0 binary path must be provided via VMSTORAGE_V1_157_0_PATH environment variable")
	}
	return startVmstorage(instance, binary, flags, cli, output)
}
