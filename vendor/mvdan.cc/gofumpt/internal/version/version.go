// Copyright (c) 2020, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package version

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"golang.org/x/mod/module"
)

// Note that this is not a main package, so a "var version" will not work with
// our go-cross script which uses -ldflags=main.version=xxx.

const ourModulePath = "mvdan.cc/gofumpt"

const fallbackVersion = "(devel)" // to match the default from runtime/debug

func findModule(info *debug.BuildInfo, modulePath string) *debug.Module {
	if info.Main.Path == modulePath {
		return &info.Main
	}
	for _, dep := range info.Deps {
		if dep.Path == modulePath {
			return dep
		}
	}
	return nil
}

func gofumptVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return fallbackVersion // no build info available
	}
	// Note that gofumpt may be used as a library via the format package,
	// so we cannot assume it is the main module in the build.
	mod := findModule(info, ourModulePath)
	if mod == nil {
		return fallbackVersion // not found?
	}
	if mod.Replace != nil {
		mod = mod.Replace
	}

	// If we found a meaningful version, we are done.
	// If gofumpt is not the main module, stop as well,
	// as VCS info is only for the main module.
	if mod.Version != "(devel)" || mod != &info.Main {
		return mod.Version
	}

	// Fall back to trying to use VCS information.
	// Until https://github.com/golang/go/issues/50603 is implemented,
	// manually construct something like a pseudo-version.
	// TODO: remove when this code is dead, hopefully in Go 1.20.

	// For the tests, as we don't want the VCS information to change over time.
	if v := os.Getenv("GARBLE_TEST_BUILDSETTINGS"); v != "" {
		var extra []debug.BuildSetting
		if err := json.Unmarshal([]byte(v), &extra); err != nil {
			panic(err)
		}
		info.Settings = append(info.Settings, extra...)
	}

	var vcsTime time.Time
	var vcsRevision string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.time":
			// If the format is invalid, we'll print a zero timestamp.
			vcsTime, _ = time.Parse(time.RFC3339Nano, setting.Value)
		case "vcs.revision":
			vcsRevision = setting.Value
			if len(vcsRevision) > 12 {
				vcsRevision = vcsRevision[:12]
			}
		}
	}
	if vcsRevision != "" {
		return module.PseudoVersion("", "", vcsTime, vcsRevision)
	}
	return fallbackVersion
}

func goVersion() string {
	// For the tests, as we don't want the Go version to change over time.
	if testVersion := os.Getenv("GO_VERSION_TEST"); testVersion != "" {
		return testVersion
	}
	return runtime.Version()
}

func String(injected string) string {
	if injected != "" {
		return fmt.Sprintf("%s (%s)", injected, goVersion())
	}
	return fmt.Sprintf("%s (%s)", gofumptVersion(), goVersion())
}
