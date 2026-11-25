package cgroup

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// readCgroupSubPath returns cgroup sub-path from cgroupPath for the line containing cgroupGrepLine.
// for example 0::/user.slice/user-1000.slice/session-5.scope for cgroup v2
// or 8:memory:/system.slice/vmagent.service for cgroup v1.
// It returns "/" if sub-path cannot be read.
// See https://www.freedesktop.org/software/systemd/man/latest/systemd.slice.html
// and https://docs.oracle.com/en/operating-systems/oracle-linux/9/systemd/SystemdMngCgroupsV2.html#SystemdScopes
func readCgroupSubPath(cgroupPath, cgroupGrepLine string) string {
	data, err := os.ReadFile(cgroupPath)
	if err != nil {
		return "/"
	}
	subPath, err := grepFirstMatch(string(data), cgroupGrepLine, 2, ":")
	if err != nil {
		return "/"
	}
	return subPath
}

// cgroupDirs returns all cgroup directories starting from sysfsPrefix/subPath up to sysfsPrefix.
func cgroupDirs(sysfsPrefix, subPath string) []string {
	var dirs []string
	for {
		dirs = append(dirs, path.Join(sysfsPrefix, subPath))
		if subPath == "/" || subPath == "." || subPath == "" {
			return dirs
		}
		subPath = path.Dir(subPath)
	}
}

// grepFirstMatch searches match line at data and returns item from it by index with given delimiter.
func grepFirstMatch(data string, match string, index int, delimiter string) (string, error) {
	lines := strings.SplitSeq(string(data), "\n")
	for s := range lines {
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
