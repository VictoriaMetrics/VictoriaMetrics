package rules

import (
	"go/ast"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/issue"
)

type usingOldMathBig struct {
	issue.MetaData
	calls gosec.CallList
}

func (r *usingOldMathBig) ID() string {
	return r.MetaData.ID
}

func (r *usingOldMathBig) Match(node ast.Node, ctx *gosec.Context) (gi *issue.Issue, err error) {
	if callExpr := r.calls.ContainsPkgCallExpr(node, ctx, false); callExpr == nil {
		return nil, nil
	}

	confidence := issue.Low
	major, minor, build := gosec.GoVersion()
	if major == 1 && (minor == 16 && build < 14 || minor == 17 && build < 7) {
		confidence = issue.Medium
	}

	return ctx.NewIssue(node, r.ID(), r.What, r.Severity, confidence), nil
}

// NewUsingOldMathBig rule detects the use of Rat.SetString from math/big.
func NewUsingOldMathBig(id string, _ gosec.Config) (gosec.Rule, []ast.Node) {
	calls := gosec.NewCallList()
	calls.Add("math/big.Rat", "SetString")
	return &usingOldMathBig{
		calls: calls,
		MetaData: issue.MetaData{
			ID:       id,
			What:     "Potential uncontrolled memory consumption in Rat.SetString (CVE-2022-23772)",
			Severity: issue.High,
		},
	}, []ast.Node{(*ast.CallExpr)(nil)}
}
