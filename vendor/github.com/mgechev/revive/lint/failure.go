package lint

import (
	"go/ast"
	"go/token"
)

const (
	// FailureCategoryArgOrder indicates argument order issues.
	FailureCategoryArgOrder FailureCategory = "arg-order"
	// FailureCategoryBadPractice indicates bad practice issues.
	FailureCategoryBadPractice FailureCategory = "bad practice"
	// FailureCategoryCodeStyle indicates code style issues.
	FailureCategoryCodeStyle FailureCategory = "code-style"
	// FailureCategoryComments indicates comment issues.
	FailureCategoryComments FailureCategory = "comments"
	// FailureCategoryComplexity indicates complexity issues.
	FailureCategoryComplexity FailureCategory = "complexity"
	// FailureCategoryContent indicates content issues.
	FailureCategoryContent FailureCategory = "content"
	// FailureCategoryErrors indicates error handling issues.
	FailureCategoryErrors FailureCategory = "errors"
	// FailureCategoryImports indicates import issues.
	FailureCategoryImports FailureCategory = "imports"
	// FailureCategoryLogic indicates logic issues.
	FailureCategoryLogic FailureCategory = "logic"
	// FailureCategoryMaintenance indicates maintenance issues.
	FailureCategoryMaintenance FailureCategory = "maintenance"
	// FailureCategoryNaming indicates naming issues.
	FailureCategoryNaming FailureCategory = "naming"
	// FailureCategoryOptimization indicates optimization issues.
	FailureCategoryOptimization FailureCategory = "optimization"
	// FailureCategoryStyle indicates style issues.
	FailureCategoryStyle FailureCategory = "style"
	// FailureCategoryTime indicates time-related issues.
	FailureCategoryTime FailureCategory = "time"
	// FailureCategoryTypeInference indicates type inference issues.
	FailureCategoryTypeInference FailureCategory = "type-inference"
	// FailureCategoryUnaryOp indicates unary operation issues.
	FailureCategoryUnaryOp FailureCategory = "unary-op"
	// FailureCategoryUnexportedTypeInAPI indicates unexported type in API issues.
	FailureCategoryUnexportedTypeInAPI FailureCategory = "unexported-type-in-api"
	// FailureCategoryZeroValue indicates zero value issues.
	FailureCategoryZeroValue FailureCategory = "zero-value"

	// failureCategoryInternal indicates internal failures.
	failureCategoryInternal FailureCategory = "REVIVE_INTERNAL"
	// failureCategoryValidity indicates validity issues.
	failureCategoryValidity FailureCategory = "validity"
)

// FailureCategory is the type for the failure categories.
type FailureCategory string

const (
	// SeverityWarning declares failures of type warning
	SeverityWarning = "warning"
	// SeverityError declares failures of type error.
	SeverityError = "error"
)

// Severity is the type for the failure types.
type Severity string

// FailurePosition returns the failure position
type FailurePosition struct {
	Start token.Position
	End   token.Position
}

// Failure defines a struct for a linting failure.
type Failure struct {
	Failure    string
	RuleName   string
	Category   FailureCategory
	Position   FailurePosition
	Node       ast.Node `json:"-"`
	Confidence float64
	// For future use
	ReplacementLine string
}

// GetFilename returns the filename.
func (f *Failure) GetFilename() string {
	return f.Position.Start.Filename
}

// IsInternal returns true if this failure is internal, false otherwise.
func (f *Failure) IsInternal() bool {
	return f.Category == failureCategoryInternal
}

// NewInternalFailure yields an internal failure with the given message as failure message.
func NewInternalFailure(message string) Failure {
	return Failure{
		Category: failureCategoryInternal,
		Failure:  message,
	}
}
