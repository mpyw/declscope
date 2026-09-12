// Package scope defines the three pseudo visibility levels that declscope
// layers on top of Go's exported/unexported distinction.
package scope

// Scope is a pseudo visibility level.
//
// Go only distinguishes exported from unexported, which means every unexported
// package-level identifier is visible to the whole package. Scope narrows that
// to a namespace, the way pub(crate) narrows pub in Rust. The name of a
// declaration never decides its scope: a directive states it, and the
// configured defaults apply otherwise.
type Scope int

const (
	// Public is visible outside the package. Exported identifiers default here.
	Public Scope = iota

	// PackageInternal is visible anywhere in the package. It is selected with
	// //declscope:package, or by configuring it as a default.
	PackageInternal

	// FilePrivate is visible only within its own namespace. Unexported
	// identifiers default here.
	FilePrivate
)

func (s Scope) String() string {
	switch s {
	case Public:
		return "public"
	case PackageInternal:
		return "package-internal"
	case FilePrivate:
		return "file-private"
	default:
		return "unknown"
	}
}

// Directive returns the comment directive that selects s explicitly.
func (s Scope) Directive() string {
	switch s {
	case Public:
		return "//declscope:public"
	case PackageInternal:
		return "//declscope:package"
	case FilePrivate:
		return "//declscope:file"
	default:
		return ""
	}
}

// Parse resolves a directive keyword to its scope.
func Parse(keyword string) (Scope, bool) {
	switch keyword {
	case "public":
		return Public, true
	case "package":
		return PackageInternal, true
	case "file":
		return FilePrivate, true
	default:
		return 0, false
	}
}
