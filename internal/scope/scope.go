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
	// Shared is visible anywhere in the package — what Go's unexported
	// already means. It is selected with //declscope:shared, or by configuring
	// it as the default.
	Shared Scope = iota

	// Private is visible only within its own namespace. It is the default.
	Private
)

func (s Scope) String() string {
	switch s {
	case Shared:
		return "shared"
	case Private:
		return "private"
	default:
		return "unknown"
	}
}

// Directive returns the comment directive that selects s explicitly.
func (s Scope) Directive() string {
	switch s {
	case Shared:
		return "//declscope:shared"
	case Private:
		return "//declscope:private"
	default:
		return ""
	}
}

// Parse resolves a directive keyword to its scope.
func Parse(keyword string) (Scope, bool) {
	switch keyword {
	case "shared", Renamed: // TODO(#185): drop Renamed with the alias.
		return Shared, true
	case "private":
		return Private, true
	default:
		return 0, false
	}
}

// Renamed is the keyword that selected Shared before it was named shared.
// Parse still reads it, so that code and config written for an older release
// keep their meaning. A directive spelled with it is reported with a fix that
// writes the new keyword.
//
// TODO(#185): delete with the alias, together with every use of it.
const Renamed = "package"
