package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func execCommand(command string) *exec.Cmd {
	cmd := strings.Fields(command)
	return exec.Command(cmd[0], cmd[1:]...)
}

func getArchOsMap() map[string][]string {
	return map[string][]string{
		"darwin":  {"amd64", "arm64"},
		"freebsd": {"amd64"},
		"linux":   {"386", "amd64", "arm", "arm64"},
		"openbsd": {"amd64"},
		"windows": {"amd64"},
	}
}

func getFileInfo(filePath string) (fs.FileInfo, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fileInfo, err
	}

	return fileInfo, nil
}

func getParentDirectory(path string, directoriesUp int) string {
	for i := 0; i < directoriesUp; i++ {
		path = filepath.Dir(path)
	}
	return path
}

func getTag() (string, error) {
	stdOut, err := execCommand("git describe --long --all").Output()

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	tag := strings.ReplaceAll(strings.ReplaceAll(string(stdOut), "/", "-"), "\n", "")

	// Check if there is a difference with the HEAD.
	gitDiff := execCommand("git diff-index --quiet HEAD --")
	err = gitDiff.Run()
	if _, ok := err.(*exec.ExitError); ok {
		gitDiffStdOut, _ := execCommand("git diff-index -u HEAD").Output()
		hashGenerator := sha1.New()
		io.WriteString(hashGenerator, string(gitDiffStdOut))
		sha1Hex := fmt.Sprintf("%x", hashGenerator.Sum(nil)[0:4])
		tag = strings.Join([]string{tag, "-dirty-", sha1Hex}, "")
	}

	return tag, nil
}

func testComponentAssets(t *testing.T, componentNames []string) {
	tag, err := getTag()
	if err != nil {
		t.Fatalf("Unable to get a tag: %v", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Unable to get CWD: %v", err)
	}

	binPath := filepath.Join(getParentDirectory(cwd, 2), "bin")

	for _, componentName := range componentNames {
		for osName, archNames := range getArchOsMap() {
			for _, archName := range archNames {
				fileExtension := ".tar.gz"
				if osName == "windows" {
					fileExtension = ".zip"
				}
				fileNamePrefix := strings.Join([]string{componentName, osName, archName, tag}, "-")

				// Check archive file.
				archiveFileName := strings.Join([]string{fileNamePrefix, fileExtension}, "")
				archiveFilePath := filepath.Join(binPath, archiveFileName)
				archiveFileInfo, err := getFileInfo(archiveFilePath)
				if err != nil {
					t.Errorf("Could not get archive file information: %s", archiveFilePath)
				} else if archiveFileInfo.Size() == 0 {
					t.Errorf("Archive file is empty: %s", archiveFilePath)
				}

				// Check checksums file.
				checksumsFileName := strings.Join([]string{fileNamePrefix, "_checksums.txt"}, "")
				checksumsFilePath := filepath.Join(binPath, checksumsFileName)
				checksumsFileInfo, err := getFileInfo(checksumsFilePath)
				if err != nil {
					t.Errorf("Could not get checksums file information: %s", checksumsFilePath)
				} else if checksumsFileInfo.Size() == 0 {
					t.Errorf("Checksums file is empty: %s", checksumsFilePath)
				}
			}
		}
	}
}

func TestVictoriaMetrics(t *testing.T) {
	testComponentAssets(t, []string{"victoria-metrics", "vmutils"})
}
