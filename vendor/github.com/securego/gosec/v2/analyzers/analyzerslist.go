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
	"golang.org/x/tools/go/analysis"
)

// AnalyzerDefinition contains the description of an analyzer and a mechanism to
// create it.
type AnalyzerDefinition struct {
	ID          string
	Description string
	Create      AnalyzerBuilder
}

// AnalyzerBuilder is used to register an analyzer definition with the analyzer
type AnalyzerBuilder func(id string, description string) *analysis.Analyzer

// AnalyzerList contains a mapping of analyzer ID's to analyzer definitions and a mapping
// of analyzer ID's to whether analyzers are suppressed.
type AnalyzerList struct {
	Analyzers          map[string]AnalyzerDefinition
	AnalyzerSuppressed map[string]bool
}

// AnalyzersInfo returns all the create methods and the analyzer suppressed map for a
// given list
func (al *AnalyzerList) AnalyzersInfo() (map[string]AnalyzerDefinition, map[string]bool) {
	builders := make(map[string]AnalyzerDefinition)
	for _, def := range al.Analyzers {
		builders[def.ID] = def
	}
	return builders, al.AnalyzerSuppressed
}

// AnalyzerFilter can be used to include or exclude an analyzer depending on the return
// value of the function
type AnalyzerFilter func(string) bool

// NewAnalyzerFilter is a closure that will include/exclude the analyzer ID's based on
// the supplied boolean value (false means don't remove, true means exclude).
func NewAnalyzerFilter(action bool, analyzerIDs ...string) AnalyzerFilter {
	analyzerlist := make(map[string]bool)
	for _, analyzer := range analyzerIDs {
		analyzerlist[analyzer] = true
	}
	return func(analyzer string) bool {
		if _, found := analyzerlist[analyzer]; found {
			return action
		}
		return !action
	}
}

var defaultAnalyzers = []AnalyzerDefinition{
	{"G115", "Type conversion which leads to integer overflow", newConversionOverflowAnalyzer},
	{"G602", "Possible slice bounds out of range", newSliceBoundsAnalyzer},
	{"G407", "Use of hardcoded IV/nonce for encryption", newHardCodedNonce},
}

// Generate the list of analyzers to use
func Generate(trackSuppressions bool, filters ...AnalyzerFilter) *AnalyzerList {
	analyzerMap := make(map[string]AnalyzerDefinition)
	analyzerSuppressedMap := make(map[string]bool)

	for _, analyzer := range defaultAnalyzers {
		analyzerSuppressedMap[analyzer.ID] = false
		addToAnalyzerList := true
		for _, filter := range filters {
			if filter(analyzer.ID) {
				analyzerSuppressedMap[analyzer.ID] = true
				if !trackSuppressions {
					addToAnalyzerList = false
				}
			}
		}
		if addToAnalyzerList {
			analyzerMap[analyzer.ID] = analyzer
		}
	}
	return &AnalyzerList{Analyzers: analyzerMap, AnalyzerSuppressed: analyzerSuppressedMap}
}
