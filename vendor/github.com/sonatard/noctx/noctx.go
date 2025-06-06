package noctx

import (
	"fmt"

	"github.com/sonatard/noctx/ngfunc"
	"github.com/sonatard/noctx/reqwithoutctx"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
)

var Analyzer = &analysis.Analyzer{
	Name:             "noctx",
	Doc:              Doc,
	Run:              run,
	RunDespiteErrors: false,
	Requires: []*analysis.Analyzer{
		buildssa.Analyzer,
	},
	ResultType: nil,
	FactTypes:  nil,
}

const Doc = "noctx finds sending http request without context.Context"

func run(pass *analysis.Pass) (interface{}, error) {
	if _, err := ngfunc.Run(pass); err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}

	if _, err := reqwithoutctx.Run(pass); err != nil {
		return nil, fmt.Errorf("run: %w", err)
	}

	return nil, nil
}
