package cgroup

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"strconv"
)

func readInt64(path, altCommand string) (int64, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		// Read data according to https://unix.stackexchange.com/questions/242718/how-to-find-out-how-much-memory-lxc-container-is-allowed-to-consume
		// This should properly determine the data location inside lxc container.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/84
		cmd := exec.Command("/bin/sh", "-c", altCommand)
		data, err = cmd.Output()
		if err != nil {
			return 0, err
		}
	}
	data = bytes.TrimSpace(data)
	return strconv.ParseInt(string(data), 10, 64)
}
