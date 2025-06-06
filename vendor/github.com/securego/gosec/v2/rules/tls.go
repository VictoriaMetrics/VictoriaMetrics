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

//go:generate tlsconfig

package rules

import (
	"crypto/tls"
	"fmt"
	"go/ast"
	"go/types"
	"strconv"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type insecureConfigTLS struct {
	issue.MetaData
	MinVersion       int64
	MaxVersion       int64
	requiredType     string
	goodCiphers      []string
	actualMinVersion int64
	actualMaxVersion int64
}

func (t *insecureConfigTLS) ID() string {
	return t.MetaData.ID
}

func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func (t *insecureConfigTLS) processTLSCipherSuites(n ast.Node, c *gosec.Context) *issue.Issue {
	if ciphers, ok := n.(*ast.CompositeLit); ok {
		for _, cipher := range ciphers.Elts {
			if ident, ok := cipher.(*ast.SelectorExpr); ok {
				if !stringInSlice(ident.Sel.Name, t.goodCiphers) {
					err := fmt.Sprintf("TLS Bad Cipher Suite: %s", ident.Sel.Name)
					return c.NewIssue(ident, t.ID(), err, issue.High, issue.High)
				}
			}
		}
	}
	return nil
}

func (t *insecureConfigTLS) processTLSConf(n ast.Node, c *gosec.Context) *issue.Issue {
	if kve, ok := n.(*ast.KeyValueExpr); ok {
		issue := t.processTLSConfVal(kve.Key, kve.Value, c)
		if issue != nil {
			return issue
		}
	} else if assign, ok := n.(*ast.AssignStmt); ok {
		if len(assign.Lhs) < 1 || len(assign.Rhs) < 1 {
			return nil
		}
		if selector, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
			issue := t.processTLSConfVal(selector.Sel, assign.Rhs[0], c)
			if issue != nil {
				return issue
			}
		}
	}
	return nil
}

func (t *insecureConfigTLS) processTLSConfVal(key ast.Expr, value ast.Expr, c *gosec.Context) *issue.Issue {
	if ident, ok := key.(*ast.Ident); ok {
		switch ident.Name {
		case "InsecureSkipVerify":
			if node, ok := value.(*ast.Ident); ok {
				if node.Name != "false" {
					return c.NewIssue(value, t.ID(), "TLS InsecureSkipVerify set true.", issue.High, issue.High)
				}
			} else {
				// TODO(tk): symbol tab look up to get the actual value
				return c.NewIssue(value, t.ID(), "TLS InsecureSkipVerify may be true.", issue.High, issue.Low)
			}

		case "PreferServerCipherSuites":
			if node, ok := value.(*ast.Ident); ok {
				if node.Name == "false" {
					return c.NewIssue(value, t.ID(), "TLS PreferServerCipherSuites set false.", issue.Medium, issue.High)
				}
			} else {
				// TODO(tk): symbol tab look up to get the actual value
				return c.NewIssue(value, t.ID(), "TLS PreferServerCipherSuites may be false.", issue.Medium, issue.Low)
			}

		case "MinVersion":
			if d, ok := value.(*ast.Ident); ok {
				obj := d.Obj
				if obj == nil {
					for _, f := range c.PkgFiles {
						obj = f.Scope.Lookup(d.Name)
						if obj != nil {
							break
						}
					}
				}
				if vs, ok := obj.Decl.(*ast.ValueSpec); ok && len(vs.Values) > 0 {
					if s, ok := vs.Values[0].(*ast.SelectorExpr); ok {
						x := s.X.(*ast.Ident).Name
						sel := s.Sel.Name

						for _, imp := range c.Pkg.Imports() {
							if imp.Name() == x {
								tObj := imp.Scope().Lookup(sel)
								if cst, ok := tObj.(*types.Const); ok {
									// ..got the value check if this can be translated
									if minVersion, err := strconv.ParseInt(cst.Val().String(), 0, 64); err == nil {
										t.actualMinVersion = minVersion
									}
								}
							}
						}
					}
					if ival, ierr := gosec.GetInt(vs.Values[0]); ierr == nil {
						t.actualMinVersion = ival
					}
				}
			} else if ival, ierr := gosec.GetInt(value); ierr == nil {
				t.actualMinVersion = ival
			} else {
				if se, ok := value.(*ast.SelectorExpr); ok {
					if pkg, ok := se.X.(*ast.Ident); ok {
						if ip, ok := gosec.GetImportPath(pkg.Name, c); ok && ip == "crypto/tls" {
							t.actualMinVersion = t.mapVersion(se.Sel.Name)
						}
					}
				}
			}

		case "MaxVersion":
			if ival, ierr := gosec.GetInt(value); ierr == nil {
				t.actualMaxVersion = ival
			} else {
				if se, ok := value.(*ast.SelectorExpr); ok {
					if pkg, ok := se.X.(*ast.Ident); ok {
						if ip, ok := gosec.GetImportPath(pkg.Name, c); ok && ip == "crypto/tls" {
							t.actualMaxVersion = t.mapVersion(se.Sel.Name)
						}
					}
				}
			}

		case "CipherSuites":
			if ret := t.processTLSCipherSuites(value, c); ret != nil {
				return ret
			}

		}
	}
	return nil
}

func (t *insecureConfigTLS) mapVersion(version string) int64 {
	var v int64
	switch version {
	case "VersionTLS13":
		v = tls.VersionTLS13
	case "VersionTLS12":
		v = tls.VersionTLS12
	case "VersionTLS11":
		v = tls.VersionTLS11
	case "VersionTLS10":
		v = tls.VersionTLS10
	}
	return v
}

func (t *insecureConfigTLS) checkVersion(n ast.Node, c *gosec.Context) *issue.Issue {
	if t.actualMaxVersion == 0 && t.actualMinVersion >= t.MinVersion {
		// no warning is generated since the min version is greater than the secure min version
		return nil
	}
	if t.actualMinVersion < t.MinVersion {
		return c.NewIssue(n, t.ID(), "TLS MinVersion too low.", issue.High, issue.High)
	}
	if t.actualMaxVersion < t.MaxVersion {
		return c.NewIssue(n, t.ID(), "TLS MaxVersion too low.", issue.High, issue.High)
	}
	return nil
}

func (t *insecureConfigTLS) resetVersion() {
	t.actualMaxVersion = 0
	t.actualMinVersion = 0
}

func (t *insecureConfigTLS) Match(n ast.Node, c *gosec.Context) (*issue.Issue, error) {
	if complit, ok := n.(*ast.CompositeLit); ok && complit.Type != nil {
		actualType := c.Info.TypeOf(complit.Type)
		if actualType != nil && actualType.String() == t.requiredType {
			for _, elt := range complit.Elts {
				issue := t.processTLSConf(elt, c)
				if issue != nil {
					return issue, nil
				}
			}
			issue := t.checkVersion(complit, c)
			t.resetVersion()
			return issue, nil
		}
	} else {
		if assign, ok := n.(*ast.AssignStmt); ok && len(assign.Lhs) > 0 {
			if selector, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
				actualType := c.Info.TypeOf(selector.X)
				if actualType != nil && actualType.String() == t.requiredType {
					issue := t.processTLSConf(assign, c)
					if issue != nil {
						return issue, nil
					}
				}
			}
		}
	}
	return nil, nil
}
