package apptest

import (
	"io"
	"os"
)

// StartVmstorage_v1_132_0 starts vmstorage-v1.132.0 (the last version that uses
// legacy index).
//
// The path to the binary must be provided via VMSTORAGE_V1_132_0_PATH
// environment variable.
func StartVmstorage_v1_132_0(instance string, flags []string, cli *Client, output io.Writer) (*Vmstorage, error) {
	binary := os.Getenv("VMSTORAGE_V1_132_0_PATH")
	return startVmstorage(instance, binary, flags, cli, output)
}
