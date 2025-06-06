package interfaces

import (
	"go/token"
	gotypes "go/types"
)

var (
	errorType         *gotypes.Interface
	gomegaMatcherType *gotypes.Interface
)

func init() {
	errorType = gotypes.Universe.Lookup("error").Type().Underlying().(*gotypes.Interface)
	gomegaMatcherType = generateTheGomegaMatcherInfType()
}

// generateTheGomegaMatcherInfType generates a types.Interface instance that represents the
// GomegaMatcher interface.
// The original code is (copied from https://github.com/nunnatsa/ginkgolinter/blob/8fdd05eee922578d4699f49d267001c01e0b9f1e/testdata/src/a/vendor/github.com/onsi/gomega/types/types.go)
//
//	type GomegaMatcher interface {
//		Match(actual interface{}) (success bool, err error)
//		FailureMessage(actual interface{}) (message string)
//		NegatedFailureMessage(actual interface{}) (message string)
//	}
func generateTheGomegaMatcherInfType() *gotypes.Interface {
	err := gotypes.Universe.Lookup("error").Type()
	bl := gotypes.Typ[gotypes.Bool]
	str := gotypes.Typ[gotypes.String]
	anyType := gotypes.Universe.Lookup("any").Type()

	return gotypes.NewInterfaceType([]*gotypes.Func{
		// Match(actual interface{}) (success bool, err error)
		gotypes.NewFunc(token.NoPos, nil, "Match", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "actual", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", bl),
				gotypes.NewVar(token.NoPos, nil, "", err),
			), false),
		),
		// FailureMessage(actual interface{}) (message string)
		gotypes.NewFunc(token.NoPos, nil, "FailureMessage", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", str),
			),
			false),
		),
		//NegatedFailureMessage(actual interface{}) (message string)
		gotypes.NewFunc(token.NoPos, nil, "NegatedFailureMessage", gotypes.NewSignatureType(
			nil, nil, nil,
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", anyType),
			),
			gotypes.NewTuple(
				gotypes.NewVar(token.NoPos, nil, "", str),
			),
			false),
		),
	}, nil)
}

func ImplementsError(t gotypes.Type) bool {
	return gotypes.Implements(t, errorType)
}

func ImplementsGomegaMatcher(t gotypes.Type) bool {
	return t != nil && gotypes.Implements(t, gomegaMatcherType)
}
