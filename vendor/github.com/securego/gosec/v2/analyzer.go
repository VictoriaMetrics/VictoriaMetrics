// (c) Copyright 2016 Hewlett Packard Enterprise Development LP
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package gosec holds the central scanning logic used by gosec security scanner
package gosec

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"log"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/packages"

	"github.com/securego/gosec/v2/analyzers"
	"github.com/securego/gosec/v2/issue"
)

// LoadMode controls the amount of details to return when loading the packages
const LoadMode = packages.NeedName |
	packages.NeedFiles |
	packages.NeedCompiledGoFiles |
	packages.NeedImports |
	packages.NeedTypes |
	packages.NeedTypesSizes |
	packages.NeedTypesInfo |
	packages.NeedSyntax |
	packages.NeedModule |
	packages.NeedEmbedFiles |
	packages.NeedEmbedPatterns

const externalSuppressionJustification = "Globally suppressed."

const aliasOfAllRules = "*"

type ignore struct {
	start        int
	end          int
	suppressions map[string][]issue.SuppressionInfo
}

type ignores map[string][]ignore

func newIgnores() ignores {
	return make(map[string][]ignore)
}

func (i ignores) parseLine(line string) (int, int) {
	parts := strings.Split(line, "-")
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		start = 0
	}
	end := start
	if len(parts) > 1 {
		if e, err := strconv.Atoi(parts[1]); err == nil {
			end = e
		}
	}
	return start, end
}

func (i ignores) add(file string, line string, suppressions map[string]issue.SuppressionInfo) {
	is := []ignore{}
	if _, ok := i[file]; ok {
		is = i[file]
	}
	found := false
	start, end := i.parseLine(line)
	for _, ig := range is {
		if ig.start <= start && ig.end >= end {
			found = true
			for r, s := range suppressions {
				ss, ok := ig.suppressions[r]
				if !ok {
					ss = []issue.SuppressionInfo{}
				}
				ss = append(ss, s)
				ig.suppressions[r] = ss
			}
			break
		}
	}
	if !found {
		ig := ignore{
			start:        start,
			end:          end,
			suppressions: map[string][]issue.SuppressionInfo{},
		}
		for r, s := range suppressions {
			ig.suppressions[r] = []issue.SuppressionInfo{s}
		}
		is = append(is, ig)
	}
	i[file] = is
}

func (i ignores) get(file string, line string) map[string][]issue.SuppressionInfo {
	start, end := i.parseLine(line)
	if is, ok := i[file]; ok {
		for _, i := range is {
			if i.start <= start && i.end >= end || start <= i.start && end >= i.end {
				return i.suppressions
			}
		}
	}
	return map[string][]issue.SuppressionInfo{}
}

// The Context is populated with data parsed from the source code as it is scanned.
// It is passed through to all rule functions as they are called. Rules may use
// this data in conjunction with the encountered AST node.
type Context struct {
	FileSet      *token.FileSet
	Comments     ast.CommentMap
	Info         *types.Info
	Pkg          *types.Package
	PkgFiles     []*ast.File
	Root         *ast.File
	Imports      *ImportTracker
	Config       Config
	Ignores      ignores
	PassedValues map[string]interface{}
}

// GetFileAtNodePos returns the file at the node position in the file set available in the context.
func (ctx *Context) GetFileAtNodePos(node ast.Node) *token.File {
	return ctx.FileSet.File(node.Pos())
}

// NewIssue creates a new issue
func (ctx *Context) NewIssue(node ast.Node, ruleID, desc string,
	severity, confidence issue.Score,
) *issue.Issue {
	return issue.New(ctx.GetFileAtNodePos(node), node, ruleID, desc, severity, confidence)
}

// Metrics used when reporting information about a scanning run.
type Metrics struct {
	NumFiles int `json:"files"`
	NumLines int `json:"lines"`
	NumNosec int `json:"nosec"`
	NumFound int `json:"found"`
}

// Analyzer object is the main object of gosec. It has methods traverse an AST
// and invoke the correct checking rules as on each node as required.
type Analyzer struct {
	ignoreNosec       bool
	ruleset           RuleSet
	context           *Context
	config            Config
	logger            *log.Logger
	issues            []*issue.Issue
	stats             *Metrics
	errors            map[string][]Error // keys are file paths; values are the golang errors in those files
	tests             bool
	excludeGenerated  bool
	showIgnored       bool
	trackSuppressions bool
	concurrency       int
	analyzerSet       *analyzers.AnalyzerSet
	mu                sync.Mutex
}

// NewAnalyzer builds a new analyzer.
func NewAnalyzer(conf Config, tests bool, excludeGenerated bool, trackSuppressions bool, concurrency int, logger *log.Logger) *Analyzer {
	ignoreNoSec := false
	if enabled, err := conf.IsGlobalEnabled(Nosec); err == nil {
		ignoreNoSec = enabled
	}
	showIgnored := false
	if enabled, err := conf.IsGlobalEnabled(ShowIgnored); err == nil {
		showIgnored = enabled
	}
	if logger == nil {
		logger = log.New(os.Stderr, "[gosec]", log.LstdFlags)
	}
	return &Analyzer{
		ignoreNosec:       ignoreNoSec,
		showIgnored:       showIgnored,
		ruleset:           NewRuleSet(),
		context:           &Context{},
		config:            conf,
		logger:            logger,
		issues:            make([]*issue.Issue, 0, 16),
		stats:             &Metrics{},
		errors:            make(map[string][]Error),
		tests:             tests,
		concurrency:       concurrency,
		excludeGenerated:  excludeGenerated,
		trackSuppressions: trackSuppressions,
		analyzerSet:       analyzers.NewAnalyzerSet(),
	}
}

// SetConfig updates the analyzer configuration
func (gosec *Analyzer) SetConfig(conf Config) {
	gosec.config = conf
}

// Config returns the current configuration
func (gosec *Analyzer) Config() Config {
	return gosec.config
}

// LoadRules instantiates all the rules to be used when analyzing source
// packages
func (gosec *Analyzer) LoadRules(ruleDefinitions map[string]RuleBuilder, ruleSuppressed map[string]bool) {
	for id, def := range ruleDefinitions {
		r, nodes := def(id, gosec.config)
		gosec.ruleset.Register(r, ruleSuppressed[id], nodes...)
	}
}

// LoadAnalyzers instantiates all the analyzers to be used when analyzing source
// packages
func (gosec *Analyzer) LoadAnalyzers(analyzerDefinitions map[string]analyzers.AnalyzerDefinition, analyzerSuppressed map[string]bool) {
	for id, def := range analyzerDefinitions {
		r := def.Create(def.ID, def.Description)
		gosec.analyzerSet.Register(r, analyzerSuppressed[id])
	}
}

// Process kicks off the analysis process for a given package
func (gosec *Analyzer) Process(buildTags []string, packagePaths ...string) error {
	config := &packages.Config{
		Mode:       LoadMode,
		BuildFlags: buildTags,
		Tests:      gosec.tests,
	}

	type result struct {
		pkgPath string
		pkgs    []*packages.Package
		err     error
	}

	results := make(chan result)
	jobs := make(chan string, len(packagePaths))
	quit := make(chan struct{})

	var wg sync.WaitGroup

	worker := func(j chan string, r chan result, quit chan struct{}) {
		for {
			select {
			case s := <-j:
				pkgs, err := gosec.load(s, config)
				select {
				case r <- result{pkgPath: s, pkgs: pkgs, err: err}:
				case <-quit:
					// we've been told to stop, probably an error while
					// processing a previous result.
					wg.Done()
					return
				}
			default:
				// j is empty and there are no jobs left
				wg.Done()
				return
			}
		}
	}

	// fill the buffer
	for _, pkgPath := range packagePaths {
		jobs <- pkgPath
	}

	for i := 0; i < gosec.concurrency; i++ {
		wg.Add(1)
		go worker(jobs, results, quit)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		if r.err != nil {
			gosec.AppendError(r.pkgPath, r.err)
		}
		for _, pkg := range r.pkgs {
			if pkg.Name != "" {
				err := gosec.ParseErrors(pkg)
				if err != nil {
					close(quit)
					wg.Wait() // wait for the goroutines to stop
					return fmt.Errorf("parsing errors in pkg %q: %w", pkg.Name, err)
				}
				gosec.CheckRules(pkg)
				gosec.CheckAnalyzers(pkg)
			}
		}
	}
	sortErrors(gosec.errors)
	return nil
}

func (gosec *Analyzer) load(pkgPath string, conf *packages.Config) ([]*packages.Package, error) {
	abspath, err := GetPkgAbsPath(pkgPath)
	if err != nil {
		gosec.logger.Printf("Skipping: %s. Path doesn't exist.", abspath)
		return []*packages.Package{}, nil
	}

	gosec.logger.Println("Import directory:", abspath)
	// step 1/3 create build context.
	buildD := build.Default
	// step 2/3: add build tags to get env dependent files into basePackage.
	gosec.mu.Lock()
	buildD.BuildTags = conf.BuildFlags
	gosec.mu.Unlock()
	basePackage, err := buildD.ImportDir(pkgPath, build.ImportComment)
	if err != nil {
		return []*packages.Package{}, fmt.Errorf("importing dir %q: %w", pkgPath, err)
	}

	var packageFiles []string
	for _, filename := range basePackage.GoFiles {
		packageFiles = append(packageFiles, path.Join(pkgPath, filename))
	}
	for _, filename := range basePackage.CgoFiles {
		packageFiles = append(packageFiles, path.Join(pkgPath, filename))
	}

	if gosec.tests {
		testsFiles := make([]string, 0)
		testsFiles = append(testsFiles, basePackage.TestGoFiles...)
		testsFiles = append(testsFiles, basePackage.XTestGoFiles...)
		for _, filename := range testsFiles {
			packageFiles = append(packageFiles, path.Join(pkgPath, filename))
		}
	}

	// step 3/3 remove build tags from conf to proceed build correctly.
	gosec.mu.Lock()
	conf.BuildFlags = nil
	defer gosec.mu.Unlock()
	pkgs, err := packages.Load(conf, packageFiles...)
	if err != nil {
		return []*packages.Package{}, fmt.Errorf("loading files from package %q: %w", pkgPath, err)
	}
	return pkgs, nil
}

// CheckRules runs analysis on the given package.
func (gosec *Analyzer) CheckRules(pkg *packages.Package) {
	gosec.logger.Println("Checking package:", pkg.Name)
	for _, file := range pkg.Syntax {
		fp := pkg.Fset.File(file.Pos())
		if fp == nil {
			// skip files which cannot be located
			continue
		}
		checkedFile := fp.Name()
		// Skip the no-Go file from analysis (e.g. a Cgo files is expanded in 3 different files
		// stored in the cache which do not need to by analyzed)
		if filepath.Ext(checkedFile) != ".go" {
			continue
		}
		if gosec.excludeGenerated && ast.IsGenerated(file) {
			gosec.logger.Println("Ignoring generated file:", checkedFile)
			continue
		}

		gosec.logger.Println("Checking file:", checkedFile)
		gosec.context.FileSet = pkg.Fset
		gosec.context.Config = gosec.config
		gosec.context.Comments = ast.NewCommentMap(gosec.context.FileSet, file, file.Comments)
		gosec.context.Root = file
		gosec.context.Info = pkg.TypesInfo
		gosec.context.Pkg = pkg.Types
		gosec.context.PkgFiles = pkg.Syntax
		gosec.context.Imports = NewImportTracker()
		gosec.context.PassedValues = make(map[string]interface{})
		gosec.updateIgnores()
		ast.Walk(gosec, file)
		gosec.stats.NumFiles++
		gosec.stats.NumLines += pkg.Fset.File(file.Pos()).LineCount()
	}
}

// CheckAnalyzers runs analyzers on a given package.
func (gosec *Analyzer) CheckAnalyzers(pkg *packages.Package) {
	ssaResult, err := gosec.buildSSA(pkg)
	if err != nil || ssaResult == nil {
		gosec.logger.Printf("Error building the SSA representation of the package %q: %s", pkg.Name, err)
		return
	}

	resultMap := map[*analysis.Analyzer]interface{}{
		buildssa.Analyzer: &analyzers.SSAAnalyzerResult{
			Config: gosec.Config(),
			Logger: gosec.logger,
			SSA:    ssaResult.(*buildssa.SSA),
		},
	}

	generatedFiles := gosec.generatedFiles(pkg)

	for _, analyzer := range gosec.analyzerSet.Analyzers {
		pass := &analysis.Pass{
			Analyzer:          analyzer,
			Fset:              pkg.Fset,
			Files:             pkg.Syntax,
			OtherFiles:        pkg.OtherFiles,
			IgnoredFiles:      pkg.IgnoredFiles,
			Pkg:               pkg.Types,
			TypesInfo:         pkg.TypesInfo,
			TypesSizes:        pkg.TypesSizes,
			ResultOf:          resultMap,
			Report:            func(d analysis.Diagnostic) {},
			ImportObjectFact:  nil,
			ExportObjectFact:  nil,
			ImportPackageFact: nil,
			ExportPackageFact: nil,
			AllObjectFacts:    nil,
			AllPackageFacts:   nil,
		}
		result, err := pass.Analyzer.Run(pass)
		if err != nil {
			gosec.logger.Printf("Error running analyzer %s: %s\n", analyzer.Name, err)
			continue
		}
		if result != nil {
			if passIssues, ok := result.([]*issue.Issue); ok {
				for _, iss := range passIssues {
					if gosec.excludeGenerated {
						if _, ok := generatedFiles[iss.File]; ok {
							continue
						}
					}
					gosec.updateIssues(iss)
				}
			}
		}
	}
}

func (gosec *Analyzer) generatedFiles(pkg *packages.Package) map[string]bool {
	generatedFiles := map[string]bool{}
	for _, file := range pkg.Syntax {
		if ast.IsGenerated(file) {
			fp := pkg.Fset.File(file.Pos())
			if fp == nil {
				// skip files which cannot be located
				continue
			}
			generatedFiles[fp.Name()] = true
		}
	}
	return generatedFiles
}

// buildSSA runs the SSA pass which builds the SSA representation of the package. It handles gracefully any panic.
func (gosec *Analyzer) buildSSA(pkg *packages.Package) (interface{}, error) {
	defer func() {
		if r := recover(); r != nil {
			gosec.logger.Printf("Panic when running SSA analyser on package: %s", pkg.Name)
		}
	}()
	ssaPass := &analysis.Pass{
		Analyzer:          buildssa.Analyzer,
		Fset:              pkg.Fset,
		Files:             pkg.Syntax,
		OtherFiles:        pkg.OtherFiles,
		IgnoredFiles:      pkg.IgnoredFiles,
		Pkg:               pkg.Types,
		TypesInfo:         pkg.TypesInfo,
		TypesSizes:        pkg.TypesSizes,
		ResultOf:          nil,
		Report:            nil,
		ImportObjectFact:  nil,
		ExportObjectFact:  nil,
		ImportPackageFact: nil,
		ExportPackageFact: nil,
		AllObjectFacts:    nil,
		AllPackageFacts:   nil,
	}

	return ssaPass.Analyzer.Run(ssaPass)
}

// ParseErrors parses the errors from given package
func (gosec *Analyzer) ParseErrors(pkg *packages.Package) error {
	if len(pkg.Errors) == 0 {
		return nil
	}
	for _, pkgErr := range pkg.Errors {
		parts := strings.Split(pkgErr.Pos, ":")
		file := parts[0]
		var err error
		var line int
		if len(parts) > 1 {
			if line, err = strconv.Atoi(parts[1]); err != nil {
				return fmt.Errorf("parsing line: %w", err)
			}
		}
		var column int
		if len(parts) > 2 {
			if column, err = strconv.Atoi(parts[2]); err != nil {
				return fmt.Errorf("parsing column: %w", err)
			}
		}
		msg := strings.TrimSpace(pkgErr.Msg)
		newErr := NewError(line, column, msg)
		if errSlice, ok := gosec.errors[file]; ok {
			gosec.errors[file] = append(errSlice, *newErr)
		} else {
			errSlice = []Error{}
			gosec.errors[file] = append(errSlice, *newErr)
		}
	}
	return nil
}

// AppendError appends an error to the file errors
func (gosec *Analyzer) AppendError(file string, err error) {
	// Do not report the error for empty packages (e.g. files excluded from build with a tag)
	var noGoErr *build.NoGoError
	if errors.As(err, &noGoErr) {
		return
	}
	errors := make([]Error, 0)
	if ferrs, ok := gosec.errors[file]; ok {
		errors = ferrs
	}
	ferr := NewError(0, 0, err.Error())
	errors = append(errors, *ferr)
	gosec.errors[file] = errors
}

// ignore a node (and sub-tree) if it is tagged with a nosec tag comment
func (gosec *Analyzer) ignore(n ast.Node) map[string]issue.SuppressionInfo {
	if gosec.ignoreNosec {
		return nil
	}
	groups, ok := gosec.context.Comments[n]
	if !ok {
		return nil
	}

	// Checks if an alternative for #nosec is set and, if not, uses the default.
	noSecDefaultTag, err := gosec.config.GetGlobal(Nosec)
	if err != nil {
		noSecDefaultTag = NoSecTag(string(Nosec))
	} else {
		noSecDefaultTag = NoSecTag(noSecDefaultTag)
	}
	noSecAlternativeTag, err := gosec.config.GetGlobal(NoSecAlternative)
	if err != nil {
		noSecAlternativeTag = noSecDefaultTag
	} else {
		noSecAlternativeTag = NoSecTag(noSecAlternativeTag)
	}

	for _, group := range groups {
		comment := strings.TrimSpace(group.Text())
		foundDefaultTag := strings.HasPrefix(comment, noSecDefaultTag) || regexp.MustCompile("\n *"+noSecDefaultTag).MatchString(comment)
		foundAlternativeTag := strings.HasPrefix(comment, noSecAlternativeTag) || regexp.MustCompile("\n *"+noSecAlternativeTag).MatchString(comment)

		if foundDefaultTag || foundAlternativeTag {
			gosec.stats.NumNosec++

			// Discard what's in front of the nosec tag.
			if foundDefaultTag {
				comment = strings.SplitN(comment, noSecDefaultTag, 2)[1]
			} else {
				comment = strings.SplitN(comment, noSecAlternativeTag, 2)[1]
			}

			// Extract the directive and the justification.
			justification := ""
			commentParts := regexp.MustCompile(`-{2,}`).Split(comment, 2)
			directive := commentParts[0]
			if len(commentParts) > 1 {
				justification = strings.TrimSpace(strings.TrimRight(commentParts[1], "\n"))
			}

			// Pull out the specific rules that are listed to be ignored.
			re := regexp.MustCompile(`(G\d{3})`)
			matches := re.FindAllStringSubmatch(directive, -1)

			suppression := issue.SuppressionInfo{
				Kind:          "inSource",
				Justification: justification,
			}

			// Find the rule IDs to ignore.
			ignores := make(map[string]issue.SuppressionInfo)
			for _, v := range matches {
				ignores[v[1]] = suppression
			}

			// If no specific rules were given, ignore everything.
			if len(matches) == 0 {
				ignores[aliasOfAllRules] = suppression
			}
			return ignores
		}
	}
	return nil
}

// Visit runs the gosec visitor logic over an AST created by parsing go code.
// Rule methods added with AddRule will be invoked as necessary.
func (gosec *Analyzer) Visit(n ast.Node) ast.Visitor {
	// Using ast.File instead of ast.ImportSpec, so that we can track all imports at once.
	switch i := n.(type) {
	case *ast.File:
		gosec.context.Imports.TrackFile(i)
	}

	for _, rule := range gosec.ruleset.RegisteredFor(n) {
		issue, err := rule.Match(n, gosec.context)
		if err != nil {
			file, line := GetLocation(n, gosec.context)
			file = path.Base(file)
			gosec.logger.Printf("Rule error: %v => %s (%s:%d)\n", reflect.TypeOf(rule), err, file, line)
		}
		gosec.updateIssues(issue)
	}
	return gosec
}

func (gosec *Analyzer) updateIgnores() {
	for n := range gosec.context.Comments {
		gosec.updateIgnoredRulesForNode(n)
	}
}

func (gosec *Analyzer) updateIgnoredRulesForNode(n ast.Node) {
	ignoredRules := gosec.ignore(n)
	if len(ignoredRules) > 0 {
		if gosec.context.Ignores == nil {
			gosec.context.Ignores = newIgnores()
		}
		line := issue.GetLine(gosec.context.FileSet.File(n.Pos()), n)
		gosec.context.Ignores.add(
			gosec.context.FileSet.File(n.Pos()).Name(),
			line,
			ignoredRules,
		)
	}
}

func (gosec *Analyzer) getSuppressionsAtLineInFile(file string, line string, id string) ([]issue.SuppressionInfo, bool) {
	ignoredRules := gosec.context.Ignores.get(file, line)

	// Check if the rule was specifically suppressed at this location.
	generalSuppressions, generalIgnored := ignoredRules[aliasOfAllRules]
	ruleSuppressions, ruleIgnored := ignoredRules[id]
	ignored := generalIgnored || ruleIgnored
	suppressions := append(generalSuppressions, ruleSuppressions...)

	// Track external suppressions of this rule.
	if gosec.ruleset.IsRuleSuppressed(id) || gosec.analyzerSet.IsSuppressed(id) {
		ignored = true
		suppressions = append(suppressions, issue.SuppressionInfo{
			Kind:          "external",
			Justification: externalSuppressionJustification,
		})
	}
	return suppressions, ignored
}

func (gosec *Analyzer) updateIssues(issue *issue.Issue) {
	if issue != nil {
		suppressions, ignored := gosec.getSuppressionsAtLineInFile(issue.File, issue.Line, issue.RuleID)
		if gosec.showIgnored {
			issue.NoSec = ignored
		}
		if !ignored || !gosec.showIgnored {
			gosec.stats.NumFound++
		}
		if ignored && gosec.trackSuppressions {
			issue.WithSuppressions(suppressions)
			gosec.issues = append(gosec.issues, issue)
		} else if !ignored || gosec.showIgnored || gosec.ignoreNosec {
			gosec.issues = append(gosec.issues, issue)
		}
	}
}

// Report returns the current issues discovered and the metrics about the scan
func (gosec *Analyzer) Report() ([]*issue.Issue, *Metrics, map[string][]Error) {
	return gosec.issues, gosec.stats, gosec.errors
}

// Reset clears state such as context, issues and metrics from the configured analyzer
func (gosec *Analyzer) Reset() {
	gosec.context = &Context{}
	gosec.issues = make([]*issue.Issue, 0, 16)
	gosec.stats = &Metrics{}
	gosec.ruleset = NewRuleSet()
	gosec.analyzerSet = analyzers.NewAnalyzerSet()
}
