package rules

import (
	"go/ast"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type sshHostKey struct {
	issue.MetaData
	pkg   string
	calls []string
}

func (r *sshHostKey) ID() string {
	return r.MetaData.ID
}

func (r *sshHostKey) Match(n ast.Node, c *gosec.Context) (gi *issue.Issue, err error) {
	if _, matches := gosec.MatchCallByPackage(n, c, r.pkg, r.calls...); matches {
		return c.NewIssue(n, r.ID(), r.What, r.Severity, r.Confidence), nil
	}
	return nil, nil
}

// NewSSHHostKey rule detects the use of insecure ssh HostKeyCallback.
func NewSSHHostKey(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	return &sshHostKey{
		pkg:   "golang.org/x/crypto/ssh",
		calls: []string{"InsecureIgnoreHostKey"},
		MetaData: issue.MetaData{
			ID:         id,
			What:       "Use of ssh InsecureIgnoreHostKey should be audited",
			Severity:   issue.Medium,
			Confidence: issue.High,
		},
	}, []ast.Node{(*ast.CallExpr)(nil)}
}
