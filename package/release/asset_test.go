package main

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
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

const tarGzExt = ".tar.gz"
const unixExecSuffix = "-prod"
const windowsExecSuffix = "-prod.exe"
const zipExt = ".zip"

func assertArchiveFile(t *testing.T, extension string, path string, expectedPrefixes []string) {
	// Check if file exists
	archiveFileInfo, err := getFileInfo(path)
	if err != nil {
		t.Fatalf("Could not get archive file information: %s", path)
	} else if archiveFileInfo.Size() == 0 {
		t.Fatalf("Archive file is empty: %s", path)
	}

	var archiveFiles []string
	var binaryFileSuffix string
	if extension == tarGzExt { // Unix-like stuff
		binaryFileSuffix = unixExecSuffix

		// Get file handler
		tarGzFile, err := os.Open(path)
		if err != nil {
			t.Errorf("Failed to open file: %s", path)
		}
		defer tarGzFile.Close()

		// Get gzip handler
		gzipFile, err := gzip.NewReader(tarGzFile)
		if err != nil {
			t.Errorf("Failed to create gzip reader: %s", err)
		}
		defer gzipFile.Close()

		// Get tar handler
		tarFile := tar.NewReader(gzipFile)
		for {
			header, err := tarFile.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Failed to read tar header: %s", err)
			}
			if header.Size == 0 {
				t.Fatalf("Archive file is empty: %s (%s)", header.Name, path)
			}
			archiveFiles = append(archiveFiles, header.Name)
		}

	} else if extension == zipExt { // Windows stuff
		binaryFileSuffix = "-windows-amd64" + windowsExecSuffix

		// Get file handler
		zipFile, err := os.Open(path)
		if err != nil {
			t.Errorf("Failed to open file: %s", path)
		}
		defer zipFile.Close()

		fileInfo, err := zipFile.Stat()
		if err != nil {
			t.Fatalf("Failed to get file info: %s", err)
		}

		// Get zip handler
		zipReader, err := zip.NewReader(zipFile, fileInfo.Size())
		if err != nil {
			t.Fatalf("Failed to create zip reader: %s", err)
		}

		for _, file := range zipReader.File {
			if file.CompressedSize64 == 0 {
				t.Fatalf("Archive file is empty: %s (%s)", file.Name, path)
			}
			archiveFiles = append(archiveFiles, file.Name)
		}

	} else { // Unexpected stuff.
		t.Fatalf("Unknown archive type: %s", extension)
	}

	var expectedFiles []string
	for _, expectedFilePrefix := range expectedPrefixes {
		expectedFiles = append(expectedFiles, strings.Join([]string{expectedFilePrefix, binaryFileSuffix}, ""))
	}
	if !compareSlices(archiveFiles, expectedFiles) {
		t.Fatalf("Archive contents `%s` doesn't match the expected one: `%s`", archiveFiles, expectedFiles)
	}
}

func assertChecksumsFile(t *testing.T, extension string, path string, expectedPrefixes []string) {
	// Check if file exists
	checksumsFileInfo, err := getFileInfo(path)
	if err != nil {
		t.Errorf("Could not get checksums file information: %s", path)
	} else if checksumsFileInfo.Size() == 0 {
		t.Errorf("Checksums file is empty: %s", path)
	}

	checksumsFile, err := os.Open(path)
	if err != nil {
		t.Fatalf("Failed to open file: %s", err)
	}
	defer checksumsFile.Close()

	checksumsFiles := []string{}
	scanner := bufio.NewScanner(checksumsFile)
	for scanner.Scan() {
		checksumsFiles = append(checksumsFiles, strings.Fields(scanner.Text())[1])
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("Failed to  read file: %s", err)
	}

	var binaryFileSuffix string
	if extension == tarGzExt { // Unix-like stuff
		binaryFileSuffix = unixExecSuffix
	} else if extension == zipExt { // Windows stuff
		binaryFileSuffix = "-windows-amd64" + windowsExecSuffix
	} else { // Unexpected stuff.
		t.Fatalf("Unknown archive type: %s", extension)
	}

	archiveFileName := strings.ReplaceAll(filepath.Base(path), "_checksums.txt", extension)
	expectedFiles := []string{archiveFileName}
	for _, expectedFilePrefix := range expectedPrefixes {
		expectedFiles = append(expectedFiles, strings.Join([]string{expectedFilePrefix, binaryFileSuffix}, ""))
	}
	if !compareSlices(checksumsFiles, expectedFiles) {
		t.Fatalf("Archive contents `%s` doesn't match the expected one: `%s`", checksumsFiles, expectedFiles)
	}
}

func compareSlices(slice1, slice2 []string) bool {
	if len(slice1) != len(slice2) {
		return false
	}
	for i := range slice1 {
		if slice1[i] != slice2[i] {
			return false
		}
	}
	return true
}

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

func getComponentFileMap() map[string][]string {
	return map[string][]string{
		"victoria-metrics": {"victoria-metrics"},
		"vmutils":          {"vmagent", "vmalert", "vmalert-tool", "vmauth", "vmbackup", "vmrestore", "vmctl"},
	}
}

func getGitTag() (string, error) {
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
		_, err := io.WriteString(hashGenerator, string(gitDiffStdOut))
		if err != nil {
			return "", err
		}
		sha1Hex := fmt.Sprintf("%x", hashGenerator.Sum(nil)[0:4])
		tag = strings.Join([]string{tag, "-dirty-", sha1Hex}, "")
	}
	return tag, nil
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

func testReleaseAssets(t *testing.T, componentNames []string) {
	gitTag, err := getGitTag()
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
			var archiveFileExtension string
			if osName == "windows" {
				archiveFileExtension = zipExt
			} else {
				archiveFileExtension = tarGzExt
			}

			for _, archName := range archNames {
				fileNamePrefix := strings.Join([]string{componentName, osName, archName, gitTag}, "-")

				// Check archive file.
				archiveFileName := strings.Join([]string{fileNamePrefix, archiveFileExtension}, "")
				archiveFilePath := filepath.Join(binPath, archiveFileName)
				assertArchiveFile(t, archiveFileExtension, archiveFilePath, getComponentFileMap()[componentName])

				// Check checksums file.
				checksumsFileName := strings.Join([]string{fileNamePrefix, "_checksums.txt"}, "")
				checksumsFilePath := filepath.Join(binPath, checksumsFileName)
				assertChecksumsFile(t, archiveFileExtension, checksumsFilePath, getComponentFileMap()[componentName])
			}
		}
	}
}

func TestVictoriaMetrics(t *testing.T) {
	testReleaseAssets(t, []string{"victoria-metrics", "vmutils"})
}
