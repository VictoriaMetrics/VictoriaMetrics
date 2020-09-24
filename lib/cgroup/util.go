package cgroup

import (
	"bytes"
	"io/ioutil"
	"os/exec"
	"strconv"
)

func readInt64(path, altCommand string) (int64, error) {
	data, err := ioutil.ReadFile(path)
	if err == nil {
		data = bytes.TrimSpace(data)
		return strconv.ParseInt(string(data), 10, 64)
	}
	return readInt64FromCommand(altCommand)
}

func readInt64FromCommand(command string) (int64, error) {
	cmd := exec.Command("/bin/sh", "-c", command)
	data, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	data = bytes.TrimSpace(data)
	return strconv.ParseInt(string(data), 10, 64)
}
