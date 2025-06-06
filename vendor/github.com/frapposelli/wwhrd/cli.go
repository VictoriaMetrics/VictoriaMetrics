package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/jessevdk/go-flags"
	log "github.com/sirupsen/logrus"
)

type cliOpts struct {
	List        `command:"list" alias:"ls" description:"List licenses"`
	Check       `command:"check" alias:"chk" description:"Check licenses against config file"`
	Graph       `command:"graph" alias:"dot" description:"Generate dot graph dependency tree"`
	VersionFlag func() error `long:"version" short:"v" description:"Show CLI version"`

	Quiet func() error `short:"q" long:"quiet" description:"quiet mode, do not log accepted packages"`
	Debug func() error `short:"d" long:"debug" description:"verbose mode, log everything"`
}

type List struct {
	NoColor           bool    `long:"no-color" description:"disable colored output"`
	CoverageThreshold float64 `short:"c" long:"coverage" description:"coverage threshold is the minimum percentage of the file that must contain license text" default:"75"`
	CheckTestFiles    bool    `short:"t" long:"check-test-files" description:"check imported dependencies for test files"`
}

type Check struct {
	File              string  `short:"f" long:"file" description:"input file, use - for stdin" default:".wwhrd.yml"`
	NoColor           bool    `long:"no-color" description:"disable colored output"`
	CoverageThreshold float64 `short:"c" long:"coverage" description:"coverage threshold is the minimum percentage of the file that must contain license text" default:"75"`
	CheckTestFiles    bool    `short:"t" long:"check-test-files" description:"check imported dependencies for test files"`
}

type Graph struct {
	File           string `short:"o" long:"output" description:"output file, use - for stdout" default:"wwhrd-graph.dot"`
	CheckTestFiles bool   `short:"t" long:"check-test-files" description:"check imported dependencies for test files"`
}

const VersionHelp flags.ErrorType = 1961

var (
	version = "dev"
	commit  = "1961213"
	date    = "1961-02-13T20:06:35Z"
)

func setQuiet() error {
	log.SetLevel(log.ErrorLevel)
	return nil
}

func setDebug() error {
	log.SetLevel(log.DebugLevel)
	return nil
}

func newCli() *flags.Parser {
	opts := cliOpts{
		VersionFlag: func() error {
			return &flags.Error{
				Type:    VersionHelp,
				Message: fmt.Sprintf("version %s\ncommit %s\ndate %s\n", version, commit, date),
			}
		},
		Quiet: setQuiet,
		Debug: setDebug,
	}
	parser := flags.NewParser(&opts, flags.HelpFlag|flags.PassDoubleDash)
	parser.LongDescription = "What would Henry Rollins do?"

	return parser
}

func (g *Graph) Execute(args []string) error {
	root, err := rootDir()
	if err != nil {
		return err
	}

	log.Infof("Generating DOT graph")

	dotGraph, err := GraphImports(root, g.CheckTestFiles)
	if err != nil {
		log.Fatal(err)
	}

	if g.File == "-" {
		mf := bufio.NewWriter(os.Stdout)
		defer mf.Flush()

		_, err = mf.Write([]byte(dotGraph))
		if err != nil {
			return err
		}

		return nil
	}

	log.Debug("Creating file... ", g.File)
	mf, err := os.Create(g.File)
	if err != nil {
		return err
	}
	defer mf.Close()

	_, err = mf.Write([]byte(dotGraph))
	if err != nil {
		return err
	}

	log.Infof("Graph saved in %q", g.File)

	return nil
}

func (l *List) Execute(args []string) error {

	if l.NoColor {
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
	} else {
		log.SetFormatter(&log.TextFormatter{ForceColors: true})
	}

	root, err := rootDir()
	if err != nil {
		return err
	}

	pkgs, err := WalkImports(root, l.CheckTestFiles)
	if err != nil {
		return err
	}
	lics := GetLicenses(root, pkgs, l.CoverageThreshold)

	for k, v := range lics {
		log.WithFields(log.Fields{
			"package": k,
			"license": v,
		}).Info("Found License")
	}

	return nil
}

func (c *Check) Execute(args []string) error {

	if c.NoColor {
		log.SetFormatter(&log.TextFormatter{DisableColors: true})
	} else {
		log.SetFormatter(&log.TextFormatter{ForceColors: true})
	}

	var config []byte

	if c.File == "-" {
		mf := bufio.NewReader(os.Stdin)
		var err error
		config, err = ioutil.ReadAll(mf)
		if err != nil {
			return err
		}
	} else {
		if _, err := os.Stat(c.File); os.IsNotExist(err) {
			return fmt.Errorf("can't read config file: %s", err)
		}

		f, err := os.Open(c.File)
		if err != nil {
			return err
		}

		config, err = ioutil.ReadAll(f)
		if err != nil {
			return err
		}

		if err = f.Close(); err != nil {
			return err
		}

	}

	t, err := ReadConfig(config)
	if err != nil {
		err = fmt.Errorf("can't read config file: %s", err)
		return err
	}

	log.Debugf("Loaded config: %+v", t)

	root, err := rootDir()
	if err != nil {
		return err
	}

	pkgs, err := WalkImports(root, c.CheckTestFiles)
	if err != nil {
		return err
	}
	lics := GetLicenses(root, pkgs, c.CoverageThreshold)

	// Make a map out of the blacklist
	blacklist := make(map[string]bool)
	for _, v := range t.Denylist {
		blacklist[v] = true
	}

	// Make a map out of the whitelist
	whitelist := make(map[string]bool)
	for _, v := range t.Allowlist {
		whitelist[v] = true
	}

	// Make a map out of the exceptions list
	exceptions := make(map[string]bool)
	exceptionsWildcard := make(map[string]bool)
	for _, v := range t.Exceptions {
		if strings.HasSuffix(v, "/...") {
			exceptionsWildcard[strings.TrimSuffix(v, "/...")] = true
		} else {
			exceptions[v] = true
		}
	}

PackageList:
	for pkg, lic := range lics {
		contextLogger := log.WithFields(log.Fields{
			"package": pkg,
			"license": lic,
		})

		// License is whitelisted and not specified in blacklist
		if whitelist[lic] && !blacklist[lic] {
			contextLogger.Info("Found Approved license")
			continue PackageList
		}

		// if we have exceptions wildcards, let's run through them
		if len(exceptionsWildcard) > 0 {
			for wc := range exceptionsWildcard {
				if strings.HasPrefix(pkg, wc) {
					// we have a match
					contextLogger.Warn("Found exceptioned package")
					continue PackageList
				}
			}
		}

		// match single-package exceptions
		if _, exists := exceptions[pkg]; exists {
			contextLogger.Warn("Found exceptioned package")
			continue PackageList
		}

		// no matches, it's a non-approved license
		contextLogger.Error("Found Non-Approved license")
		err = fmt.Errorf("Non-Approved license found")

	}

	return err
}

func rootDir() (string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", err
	}

	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		root, err = os.Readlink(root)
		if err != nil {
			return "", err
		}
	}
	return root, nil
}
