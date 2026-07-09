package cgroup

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
)

func getStatGeneric(statName, sysfsPrefix, cgroupPath, cgroupGrepLine string) (int64, error) {
	data, err := getFileContents(statName, sysfsPrefix, cgroupPath, cgroupGrepLine)
	if err != nil {
		return 0, err
	}
	data = strings.TrimSpace(data)
	n, err := strconv.ParseInt(data, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q: %w", cgroupPath, err)
	}
	return n, nil
}

func getFileContents(statName, sysfsPrefix, cgroupPath, cgroupGrepLine string) (string, error) {
	filepath := path.Join(sysfsPrefix, statName)
	data, err := os.ReadFile(filepath)
	if err == nil {
		return string(data), nil
	}
	cgroupData, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "", err
	}
	subPath, err := grepFirstMatch(string(cgroupData), cgroupGrepLine, 2, ":")
	if err != nil {
		return "", fmt.Errorf("cannot find cgroup path for %q in %q: %w", cgroupGrepLine, cgroupPath, err)
	}
	filepath = path.Join(sysfsPrefix, subPath, statName)
	data, err = os.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// readCgroupV2SubPath reads cgroupv2 sub-path
// for example 0::/user.slice/user-1000.slice/session-5.scope
// See https://www.freedesktop.org/software/systemd/man/latest/systemd.slice.html
// and https://docs.oracle.com/en/operating-systems/oracle-linux/9/systemd/SystemdMngCgroupsV2.html#SystemdScopes
func readCgroupV2SubPath(cgroupPath string) (string, error) {
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "", err
	}
	return grepFirstMatch(string(data), "", 2, ":")
}

// grepFirstMatch searches match line at data and returns item from it by index with given delimiter.
func grepFirstMatch(data string, match string, index int, delimiter string) (string, error) {
	lines := strings.Split(string(data), "\n")
	for _, s := range lines {
		if !strings.Contains(s, match) {
			continue
		}
		parts := strings.Split(s, delimiter)
		if index < len(parts) {
			return strings.TrimSpace(parts[index]), nil
		}
	}
	return "", fmt.Errorf("cannot find %q in %q", match, data)
}
