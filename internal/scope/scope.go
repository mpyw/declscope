// Package scope defines the two pseudo visibility levels that declscope layers
// on top of Go's exported/unexported distinction.
package scope

// Scope is a pseudo visibility level.
//
// Go only distinguishes exported from unexported, which means every unexported
// package-level identifier is visible to the whole package. Scope narrows that
// to a namespace, the way an unmodified item is narrow to its module in Rust.
// The name of a declaration never decides its scope: a directive states it, and
// the configured default applies otherwise.
//
// There is no public level. Everything is checked within a single package, so a
// use beyond the package edge is never seen, and a scope that says "visible
// outside" would name a distinction the analysis cannot make. What is reachable
// from outside the package is simply not declscope's subject.
type Scope int

const (
	// PackageInternal is visible anywhere in the package — what Go's unexported
	// already means. It is selected with //declscope:package, or by configuring
	// it as the default.
	PackageInternal Scope = iota

	// Private is visible only within its own namespace. It is the default.
	Private
)

func (s Scope) String() string {
	switch s {
	case PackageInternal:
		return "package-internal"
	case Private:
		return "private"
	default:
		return "unknown"
	}
}

// Directive returns the comment directive that selects s explicitly.
func (s Scope) Directive() string {
	switch s {
	case PackageInternal:
		return "//declscope:package"
	case Private:
		return "//declscope:private"
	default:
		return ""
	}
}

// Parse resolves a directive keyword to its scope.
func Parse(keyword string) (Scope, bool) {
	switch keyword {
	case "package":
		return PackageInternal, true
	case "private":
		return Private, true
	default:
		return 0, false
	}
}
