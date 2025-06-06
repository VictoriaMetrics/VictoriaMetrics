// (c) Copyright gosec's authors
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

package analyzers

import (
	"fmt"
	"go/token"
	"log"
	"os"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"

	"github.com/securego/gosec/v2/issue"
)

// SSAAnalyzerResult contains various information returned by the
// SSA analysis along with some configuration
type SSAAnalyzerResult struct {
	Config map[string]interface{}
	Logger *log.Logger
	SSA    *buildssa.SSA
}

// BuildDefaultAnalyzers returns the default list of analyzers
func BuildDefaultAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		newConversionOverflowAnalyzer("G115", "Type conversion which leads to integer overflow"),
		newSliceBoundsAnalyzer("G602", "Possible slice bounds out of range"),
		newHardCodedNonce("G407", "Use of hardcoded IV/nonce for encryption"),
	}
}

// getSSAResult retrieves the SSA result from analysis pass
func getSSAResult(pass *analysis.Pass) (*SSAAnalyzerResult, error) {
	result, ok := pass.ResultOf[buildssa.Analyzer]
	if !ok {
		return nil, fmt.Errorf("no SSA result found in the analysis pass")
	}
	ssaResult, ok := result.(*SSAAnalyzerResult)
	if !ok {
		return nil, fmt.Errorf("the analysis pass result is not of type SSA")
	}
	return ssaResult, nil
}

// newIssue creates a new gosec issue
func newIssue(analyzerID string, desc string, fileSet *token.FileSet,
	pos token.Pos, severity, confidence issue.Score,
) *issue.Issue {
	file := fileSet.File(pos)
	line := file.Line(pos)
	col := file.Position(pos).Column

	return &issue.Issue{
		RuleID:     analyzerID,
		File:       file.Name(),
		Line:       strconv.Itoa(line),
		Col:        strconv.Itoa(col),
		Severity:   severity,
		Confidence: confidence,
		What:       desc,
		Cwe:        issue.GetCweByRule(analyzerID),
		Code:       issueCodeSnippet(fileSet, pos),
	}
}

func issueCodeSnippet(fileSet *token.FileSet, pos token.Pos) string {
	file := fileSet.File(pos)

	start := (int64)(file.Line(pos))
	if start-issue.SnippetOffset > 0 {
		start = start - issue.SnippetOffset
	}
	end := (int64)(file.Line(pos))
	end = end + issue.SnippetOffset

	var code string
	if file, err := os.Open(file.Name()); err == nil {
		defer file.Close() // #nosec
		code, err = issue.CodeSnippet(file, start, end)
		if err != nil {
			return err.Error()
		}
	}
	return code
}
