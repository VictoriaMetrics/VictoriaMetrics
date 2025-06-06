package ifelse

// PreserveScope is a configuration argument that prevents suggestions
// that would enlarge variable scope
const PreserveScope = "preserveScope"

// AllowJump is a configuration argument that permits early-return to
// suggest introducing a new jump (return, continue, etc) statement
// to reduce nesting. By default, suggestions only bring existing jumps
// earlier.
const AllowJump = "allowJump"

// Args contains arguments common to the early-return, indent-error-flow
// and superfluous-else rules
type Args struct {
	PreserveScope bool
	AllowJump     bool
}
