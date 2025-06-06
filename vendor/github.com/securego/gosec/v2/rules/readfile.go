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

package rules

import (
	"go/ast"
	"go/types"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type readfile struct {
	issue.MetaData
	gosec.CallList
	pathJoin   gosec.CallList
	clean      gosec.CallList
	cleanedVar map[any]ast.Node
}

// ID returns the identifier for this rule
func (r *readfile) ID() string {
	return r.MetaData.ID
}

// isJoinFunc checks if there is a filepath.Join or other join function
func (r *readfile) isJoinFunc(n ast.Node, c *gosec.Context) bool {
	if call := r.pathJoin.ContainsPkgCallExpr(n, c, false); call != nil {
		for _, arg := range call.Args {
			// edge case: check if one of the args is a BinaryExpr
			if binExp, ok := arg.(*ast.BinaryExpr); ok {
				// iterate and resolve all found identities from the BinaryExpr
				if _, ok := gosec.FindVarIdentities(binExp, c); ok {
					return true
				}
			}

			// try and resolve identity
			if ident, ok := arg.(*ast.Ident); ok {
				obj := c.Info.ObjectOf(ident)
				if _, ok := obj.(*types.Var); ok && !gosec.TryResolve(ident, c) {
					return true
				}
			}
		}
	}
	return false
}

// isFilepathClean checks if there is a filepath.Clean for given variable
func (r *readfile) isFilepathClean(n *ast.Ident, c *gosec.Context) bool {
	if _, ok := r.cleanedVar[n.Obj.Decl]; ok {
		return true
	}
	if n.Obj.Kind != ast.Var {
		return false
	}
	if node, ok := n.Obj.Decl.(*ast.AssignStmt); ok {
		if call, ok := node.Rhs[0].(*ast.CallExpr); ok {
			if clean := r.clean.ContainsPkgCallExpr(call, c, false); clean != nil {
				return true
			}
		}
	}
	return false
}

// trackFilepathClean tracks back the declaration of variable from filepath.Clean argument
func (r *readfile) trackFilepathClean(n ast.Node) {
	if clean, ok := n.(*ast.CallExpr); ok && len(clean.Args) > 0 {
		if ident, ok := clean.Args[0].(*ast.Ident); ok {
			// ident.Obj may be nil if the referenced declaration is in another file. It also may be incorrect.
			// if it is nil, do not follow it.
			if ident.Obj != nil {
				r.cleanedVar[ident.Obj.Decl] = n
			}
		}
	}
}

// Match inspects AST nodes to determine if the match the methods `os.Open` or `ioutil.ReadFile`
func (r *readfile) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if node := r.clean.ContainsPkgCallExpr(n, c, false); node != nil {
		r.trackFilepathClean(n)
		return nil, nil
	} else if node := r.ContainsPkgCallExpr(n, c, false); node != nil {
		for _, arg := range node.Args {
			// handles path joining functions in Arg
			// eg. os.Open(filepath.Join("/tmp/", file))
			if callExpr, ok := arg.(*ast.CallExpr); ok {
				if r.isJoinFunc(callExpr, c) {
					return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
				}
			}
			// handles binary string concatenation eg. ioutil.Readfile("/tmp/" + file + "/blob")
			if binExp, ok := arg.(*ast.BinaryExpr); ok {
				// resolve all found identities from the BinaryExpr
				if _, ok := gosec.FindVarIdentities(binExp, c); ok {
					return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
				}
			}

			if ident, ok := arg.(*ast.Ident); ok {
				obj := c.Info.ObjectOf(ident)
				if _, ok := obj.(*types.Var); ok &&
					!gosec.TryResolve(ident, c) &&
					!r.isFilepathClean(ident, c) {
					return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
				}
			}
		}
	}
	return nil, nil
}

// NewReadFile detects cases where we read files
func NewReadFile(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	rule := &readfile{
		pathJoin: gosec.NewCallList(),
		clean:    gosec.NewCallList(),
		CallList: gosec.NewCallList(),
		MetaData: issue.MetaData{
			ID:         id,
			What:       "Potential file inclusion via variable",
			Severity:   issue.Medium,
			Confidence: issue.High,
		},
		cleanedVar: map[any]ast.Node{},
	}
	rule.pathJoin.Add("path/filepath", "Join")
	rule.pathJoin.Add("path", "Join")
	rule.clean.Add("path/filepath", "Clean")
	rule.clean.Add("path/filepath", "Rel")
	rule.clean.Add("path/filepath", "EvalSymlinks")
	rule.Add("io/ioutil", "ReadFile")
	rule.Add("os", "ReadFile")
	rule.Add("os", "Open")
	rule.Add("os", "OpenFile")
	rule.Add("os", "Create")
	return rule, []ast.Node{(*ast.CallExpr)(nil)}
}
