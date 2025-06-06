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
	"errors"
	"fmt"
	"go/token"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"

	"github.com/securego/gosec/v2/issue"
)

const defaultIssueDescription = "Use of hardcoded IV/nonce for encryption"

func newHardCodedNonce(id string, description string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name:     id,
		Doc:      description,
		Run:      runHardCodedNonce,
		Requires: []*analysis.Analyzer{buildssa.Analyzer},
	}
}

func runHardCodedNonce(pass *analysis.Pass) (interface{}, error) {
	ssaResult, err := getSSAResult(pass)
	if err != nil {
		return nil, fmt.Errorf("building ssa representation: %w", err)
	}

	// Holds the function name as key, the number of arguments that the function accepts, and at which index of those accepted arguments is the nonce/IV
	// Example "Test" 3, 1 -- means the function "Test" which accepts 3 arguments, and has the nonce arg as second argument
	calls := map[string][]int{
		"(crypto/cipher.AEAD).Seal":     {4, 1},
		"crypto/cipher.NewCBCEncrypter": {2, 1},
		"crypto/cipher.NewCFBEncrypter": {2, 1},
		"crypto/cipher.NewCTR":          {2, 1},
		"crypto/cipher.NewOFB":          {2, 1},
	}
	ssaPkgFunctions := ssaResult.SSA.SrcFuncs
	args := getArgsFromTrackedFunctions(ssaPkgFunctions, calls)
	if args == nil {
		return nil, errors.New("no tracked functions found, resulting in no variables to track")
	}
	var issues []*issue.Issue
	for _, arg := range args {
		if arg == nil {
			continue
		}
		i, err := raiseIssue(*arg, calls, ssaPkgFunctions, pass, "")
		if err != nil {
			return issues, fmt.Errorf("raising issue error: %w", err)
		}
		issues = append(issues, i...)
	}
	return issues, nil
}

func raiseIssue(val ssa.Value, funcsToTrack map[string][]int, ssaFuncs []*ssa.Function,
	pass *analysis.Pass, issueDescription string,
) ([]*issue.Issue, error) {
	if issueDescription == "" {
		issueDescription = defaultIssueDescription
	}
	var err error
	var allIssues []*issue.Issue
	var issues []*issue.Issue
	switch valType := (val).(type) {
	case *ssa.Slice:
		issueDescription += " by passing hardcoded slice/array"
		issues, err = iterateThroughReferrers(val, funcsToTrack, pass.Analyzer.Name, issueDescription, pass.Fset, issue.High)
		allIssues = append(allIssues, issues...)
	case *ssa.UnOp:
		// Check if it's a dereference operation (a.k.a pointer)
		if valType.Op == token.MUL {
			issueDescription += " by passing pointer which points to hardcoded variable"
			issues, err = iterateThroughReferrers(val, funcsToTrack, pass.Analyzer.Name, issueDescription, pass.Fset, issue.Low)
			allIssues = append(allIssues, issues...)
		}
	// When the value assigned to a variable is a function call.
	// It goes and check if this function contains call to crypto/rand.Read
	// in it's body(Assuming that calling crypto/rand.Read in a function,
	// is used for the generation of nonce/iv )
	case *ssa.Call:
		if callValue := valType.Call.Value; callValue != nil {
			if calledFunction, ok := callValue.(*ssa.Function); ok {
				if contains, funcErr := isFuncContainsCryptoRand(calledFunction); !contains && funcErr == nil {
					issueDescription += " by passing a value from function which doesn't use crypto/rand"
					issues, err = iterateThroughReferrers(val, funcsToTrack, pass.Analyzer.Name, issueDescription, pass.Fset, issue.Medium)
					allIssues = append(allIssues, issues...)
				} else if funcErr != nil {
					err = funcErr
				}
			}
		}
	// only checks from strings->[]byte
	// might need to add additional types
	case *ssa.Convert:
		if valType.Type().String() == "[]byte" && valType.X.Type().String() == "string" {
			issueDescription += " by passing converted string"
			issues, err = iterateThroughReferrers(val, funcsToTrack, pass.Analyzer.Name, issueDescription, pass.Fset, issue.High)
			allIssues = append(allIssues, issues...)
		}
	case *ssa.Parameter:
		// arg given to tracked function is wrapped in another function, example:
		// func encrypt(..,nonce,...){
		// 		aesgcm.Seal(nonce)
		// }
		// save parameter position, by checking the name of the variable used in
		// tracked functions and comparing it with the name of the arg
		if valType.Parent() != nil {
			trackedFunctions := make(map[string][]int)
			for index, funcArgs := range valType.Parent().Params {
				if funcArgs.Name() == valType.Name() && funcArgs.Type() == valType.Type() {
					trackedFunctions[valType.Parent().String()] = []int{len(valType.Parent().Params), index}
				}
			}
			args := getArgsFromTrackedFunctions(ssaFuncs, trackedFunctions)

			issueDescription += " by passing a parameter to a function and"
			// recursively backtrack to where the origin of a variable passed to multiple functions is
			for _, arg := range args {
				if arg == nil {
					continue
				}
				issues, err = raiseIssue(*arg, trackedFunctions, ssaFuncs, pass, issueDescription)
				allIssues = append(allIssues, issues...)
			}
		}
	}
	return allIssues, err
}

// iterateThroughReferrers iterates through all places that use the `variable` argument and check if it's used in one of the tracked functions.
func iterateThroughReferrers(variable ssa.Value, funcsToTrack map[string][]int,
	analyzerID string, issueDescription string,
	fileSet *token.FileSet, issueConfidence issue.Score,
) ([]*issue.Issue, error) {
	if funcsToTrack == nil || variable == nil || analyzerID == "" || issueDescription == "" || fileSet == nil {
		return nil, errors.New("received a nil object")
	}
	var gosecIssues []*issue.Issue
	refs := variable.Referrers()
	if refs == nil {
		return gosecIssues, nil
	}
	// Go through all functions that use the given arg variable
	for _, ref := range *refs {
		// Iterate through the functions we are interested
		for trackedFunc := range funcsToTrack {

			// Split the functions we are interested in, by the '.' because we will use the function name to do the comparison
			// MIGHT GIVE SOME FALSE POSITIVES THIS WAY
			trackedFuncParts := strings.Split(trackedFunc, ".")
			trackedFuncPartsName := trackedFuncParts[len(trackedFuncParts)-1]
			if strings.Contains(ref.String(), trackedFuncPartsName) {
				gosecIssues = append(gosecIssues, newIssue(analyzerID, issueDescription, fileSet, ref.Pos(), issue.High, issueConfidence))
			}
		}
	}
	return gosecIssues, nil
}

// isFuncContainsCryptoRand checks whether a function contains a call to crypto/rand.Read in it's function body.
func isFuncContainsCryptoRand(funcCall *ssa.Function) (bool, error) {
	if funcCall == nil {
		return false, errors.New("passed ssa.Function object is nil")
	}
	for _, block := range funcCall.Blocks {
		for _, instr := range block.Instrs {
			if call, ok := instr.(*ssa.Call); ok {
				if calledFunction, ok := call.Call.Value.(*ssa.Function); ok {
					if calledFunction.Pkg != nil && calledFunction.Pkg.Pkg.Path() == "crypto/rand" && calledFunction.Name() == "Read" {
						return true, nil
					}
				}
			}
		}
	}
	return false, nil
}

func addToVarsMap(value ssa.Value, mapToAddTo map[string]*ssa.Value) {
	key := value.Name() + value.Type().String() + value.String() + value.Parent().String()
	mapToAddTo[key] = &value
}

func isContainedInMap(value ssa.Value, mapToCheck map[string]*ssa.Value) bool {
	key := value.Name() + value.Type().String() + value.String() + value.Parent().String()
	_, contained := mapToCheck[key]
	return contained
}

func getArgsFromTrackedFunctions(ssaFuncs []*ssa.Function, trackedFunc map[string][]int) map[string]*ssa.Value {
	values := make(map[string]*ssa.Value)
	for _, pkgFunc := range ssaFuncs {
		for _, funcBlock := range pkgFunc.Blocks {
			for _, funcBlocInstr := range funcBlock.Instrs {
				iterateTrackedFunctionsAndAddArgs(funcBlocInstr, trackedFunc, values)
			}
		}
	}
	return values
}

func iterateTrackedFunctionsAndAddArgs(funcBlocInstr ssa.Instruction, trackedFunc map[string][]int, values map[string]*ssa.Value) {
	if funcCall, ok := (funcBlocInstr).(*ssa.Call); ok {
		for trackedFuncName, trackedFuncArgsInfo := range trackedFunc {
			// only process functions that have the same number of arguments as the ones we track
			if len(funcCall.Call.Args) == trackedFuncArgsInfo[0] {
				tmpArg := funcCall.Call.Args[trackedFuncArgsInfo[1]]
				// check if the function is called from an object or directly from the package
				if funcCall.Call.Method != nil {
					if methodFullname := funcCall.Call.Method.FullName(); methodFullname == trackedFuncName {
						if !isContainedInMap(tmpArg, values) {
							addToVarsMap(tmpArg, values)
						}
					}
				} else if funcCall.Call.Value.String() == trackedFuncName {
					if !isContainedInMap(tmpArg, values) {
						addToVarsMap(tmpArg, values)
					}
				}
			}
		}
	}
}
