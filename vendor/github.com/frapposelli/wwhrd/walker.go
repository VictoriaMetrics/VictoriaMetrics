package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/emicklei/dot"
	"github.com/google/licensecheck"
	log "github.com/sirupsen/logrus"
)

const (
	unknownLicense string = "UNKNOWN"
)

var (
	// FileNames used to search for licenses
	// "LICENSE.docs" and "LICENCE.docs" are excluded from the list as we only care about source code in the repo.
	FileNames = []string{
		"COPYING",
		"COPYING.md",
		"COPYING.markdown",
		"COPYING.txt",
		"LICENCE",
		"LICENCE.md",
		"LICENCE.markdown",
		"LICENCE.txt",
		"LICENSE",
		"LICENSE.md",
		"LICENSE.markdown",
		"LICENSE.txt",
		"LICENSE-2.0.txt",
		"LICENCE-2.0.txt",
		"LICENSE-APACHE",
		"LICENCE-APACHE",
		"LICENSE-APACHE-2.0.txt",
		"LICENCE-APACHE-2.0.txt",
		"LICENSE-MIT",
		"LICENCE-MIT",
		"LICENSE.MIT",
		"LICENCE.MIT",
		"LICENSE.code",
		"LICENCE.code",
		"LICENSE.rst",
		"LICENCE.rst",
		"MIT-LICENSE",
		"MIT-LICENCE",
		"MIT-LICENSE.md",
		"MIT-LICENCE.md",
		"MIT-LICENSE.markdown",
		"MIT-LICENCE.markdown",
		"MIT-LICENSE.txt",
		"MIT-LICENCE.txt",
		"MIT_LICENSE",
		"MIT_LICENCE",
		"UNLICENSE",
		"UNLICENCE",
	}
)

// fileNamesLowercase has all the entries of FileNames, lower cased and made a set
// for fast case-insensitive matching.
var fileNamesLowercase = map[string]bool{}

func init() {
	for _, f := range FileNames {
		fileNamesLowercase[strings.ToLower(f)] = true
	}
}

// dependencies are tracked as a graph, but the graph itself is not used to build the node list
type dependencies struct {
	nodes     []*node
	nodesList map[string]bool
	edges     map[node][]*node
	dotGraph  *dot.Graph
	checkTest bool
	sync.RWMutex
}

type node struct {
	pkg    string
	dir    string
	vendor string
}

func newGraph(checkTest bool) *dependencies {
	var g dependencies
	g.nodesList = make(map[string]bool)
	g.checkTest = checkTest
	return &g
}

// AddNode adds a node to the graph
func (g *dependencies) addNode(n *node) error {
	log.Debugf("[%s] current nodesList status %+v", n.pkg, g.nodesList)
	// check if Node has been visited, this is done raw by caching it in a global hashtable
	if !g.nodesList[n.pkg] {
		g.Lock()
		g.nodes = append(g.nodes, n)
		g.nodesList[n.pkg] = true
		g.Unlock()
		return nil
	}
	return fmt.Errorf("[%s] node already visited", n.pkg)
}

// addEdge adds an edge to the graph
func (g *dependencies) addEdge(n1, n2 *node) {
	g.Lock()
	if g.edges == nil {
		g.edges = make(map[node][]*node)
	}
	g.edges[*n1] = append(g.edges[*n1], n2)
	g.edges[*n2] = append(g.edges[*n2], n1)
	g.Unlock()
}

func (g *dependencies) getDotGraph() string {
	g.dotGraph = dot.NewGraph(dot.Directed)
	g.generateDotGraph(func(n *node) {})
	return g.dotGraph.String()
}

type nodeQueue struct {
	items []node
	sync.RWMutex
}

func (s *nodeQueue) new() *nodeQueue {
	s.Lock()
	s.items = []node{}
	s.Unlock()
	return s
}

func (s *nodeQueue) enqueue(t node) {
	s.Lock()
	s.items = append(s.items, t)
	s.Unlock()
}

func (s *nodeQueue) dequeue() *node {
	s.Lock()
	item := s.items[0]
	s.items = s.items[1:len(s.items)]
	s.Unlock()
	return &item
}

func (s *nodeQueue) isEmpty() bool {
	s.RLock()
	defer s.RUnlock()
	return len(s.items) == 0
}

// do a BFS on the graph and generate dot.Graph
func (g *dependencies) generateDotGraph(f func(*node)) {
	g.RLock()
	q := nodeQueue{}
	q.new()
	n := g.nodes[0]
	q.enqueue(*n)
	visited := make(map[*node]bool)
	for {
		if q.isEmpty() {
			break
		}
		node := q.dequeue()
		// add dotGraph node after dequeing
		dGN := g.dotGraph.Node(node.pkg)

		visited[node] = true
		near := g.edges[*node]

		for i := 0; i < len(near); i++ {
			j := near[i]
			if !visited[j] {
				// add unvisited node to dotGraph
				edGN := g.dotGraph.Node(j.pkg)
				// add an edge in the dotGraph between ancestor and descendant
				g.dotGraph.Edge(dGN, edGN)
				q.enqueue(*j)
				visited[j] = true
			}
		}
		if f != nil {
			f(node)
		}
	}
	g.RUnlock()
}

func (g *dependencies) WalkNode(n *node) {
	var walkFn = func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		log.Debugf("walking %q", path)

		// check if we need to skip this
		if ok, err := shouldSkip(path, info, g.checkTest); ok {
			return err
		}

		fs := token.NewFileSet()
		f, err := parser.ParseFile(fs, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}

		for _, s := range f.Imports {
			vendorpkg := strings.Replace(s.Path.Value, "\"", "", -1)
			log.Debugf("found import %q", vendorpkg)
			pkgdir := filepath.Join(n.vendor, "vendor", vendorpkg)
			if _, err := os.Stat(pkgdir); !os.IsNotExist(err) {

				// Add imported pkg to the graph
				var vendornode = node{pkg: vendorpkg, dir: pkgdir, vendor: n.vendor}
				log.Debugf("[%s] adding node", vendornode.pkg)
				if err := g.addNode(&vendornode); err != nil {
					log.Debug(err.Error())
					continue
				}
				log.Debugf("[%s] adding node as edge of %s", vendornode.pkg, n.pkg)
				g.addEdge(n, &vendornode)
				log.Debugf("[%s] walking node", vendornode.pkg)
				g.WalkNode(&vendornode)
			}

		}
		return nil
	}

	if err := filepath.Walk(n.dir, walkFn); err != nil {
		return
	}

}

func WalkImports(root string, checkTest bool) (map[string]bool, error) {

	graph := newGraph(checkTest)
	rootNode := node{pkg: "root", dir: root, vendor: root}
	if err := graph.addNode(&rootNode); err != nil {
		log.Debug(err.Error())
	}

	log.Debugf("[%s] walking root node", rootNode.pkg)
	graph.WalkNode(&rootNode)

	return graph.nodesList, nil
}

func GraphImports(root string, checkTest bool) (string, error) {

	graph := newGraph(checkTest)
	rootNode := node{pkg: "root", dir: root, vendor: root}
	if err := graph.addNode(&rootNode); err != nil {
		log.Debug(err.Error())
	}

	log.Debugf("[%s] walking root node", rootNode.pkg)
	graph.WalkNode(&rootNode)

	return graph.getDotGraph(), nil
}

func GetLicenses(root string, list map[string]bool, threshold float64) map[string]string {

	checker, err := licensecheck.NewScanner(licensecheck.BuiltinLicenses())
	if err != nil {
		log.Fatal("Cannot initialize LicenseChecker")
	}

	var lics = make(map[string]string)

	if !strings.HasSuffix(root, "vendor") {
		root = filepath.Join(root, "vendor")
	}
	log.Debug("Start walking paths for LICENSE discovery")
	for k := range list {

		var fpath = filepath.Join(root, k)
		pkg, err := os.Stat(fpath)

		if err != nil {
			continue
		}
		if pkg.IsDir() {
			log.Debugf("Walking path: %s", fpath)

			waa := scanDir(checker, fpath, threshold)
			if waa != "" {
				lics[k] = waa
			}

		}

	}

	return lics
}

func scanDir(checker *licensecheck.Scanner, fpath string, threshold float64) string {
	var license = ""

	filesInDir, err := ioutil.ReadDir(fpath)
	if err != nil {
		return ""
	}
	for _, f := range filesInDir {
		log.Debugf("Evaluating: %s", f.Name())
		// if it's a directory or not in the list of well-known license files, we skip
		if f.IsDir() || !fileNamesLowercase[strings.ToLower(f.Name())] {
			log.Debugf("Skipping...")
			continue
		}

		// Read the license file
		text, err := ioutil.ReadFile(filepath.Join(fpath, f.Name()))
		if err != nil {
			log.Errorf("Cannot read file: %s because: %s", filepath.Join(fpath, f.Name()), err.Error())
			continue
		}

		// Verify against the checker
		cov := checker.Scan(text)
		log.Debugf("%.1f%% of text covered by licenses:\n", cov.Percent)
		for _, m := range cov.Match {
			log.Debugf("%s at [%d:%d] IsURL=%v\n", m.ID, m.Start, m.End, m.IsURL)
		}

		// If the threshold is met, we qualify the license
		if cov.Percent >= threshold {
			license = cov.Match[0].ID
		}

	}

	// if we didn't find any licenses after walking the path, we pop one out from it
	if license == "" {
		pak := strings.Split(filepath.ToSlash(fpath), "/")
		// if we're 1 directories removed from vendor/ that means we couldn't find a decent license file
		if pak[len(pak)-2] != "vendor" {
			log.Debugf("Recursive call to scanDir starting from: %s going to: %s", fpath, filepath.FromSlash(strings.Join(pak[:len(pak)-1], "/")))
			license = scanDir(checker, filepath.FromSlash(strings.Join(pak[:len(pak)-1], "/")), threshold)
		}
	}

	if license == "" {
		license = unknownLicense
	}

	return license
}

func shouldSkip(path string, info os.FileInfo, checkTest bool) (bool, error) {
	if info.IsDir() {
		name := info.Name()
		// check if directory is in the blocklist
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" {
			log.Debugf("skipping %q: directory in blocklist", path)
			return true, filepath.SkipDir
		}
		return true, nil
	}
	// if it's not a .go file, skip
	if filepath.Ext(path) != ".go" {
		log.Debugf("skipping %q: not a go file", path)
		return true, nil
	}
	// if it's a test file, skip
	if strings.HasSuffix(path, "_test.go") && !checkTest {
		log.Debugf("skipping %q: test file", path)
		return true, nil
	}
	return false, nil
}
