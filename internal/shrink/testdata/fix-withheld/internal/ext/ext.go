// Package ext holds declarations named only by example functions in an
// external test file the build leaves out. The configuration that builds
// the file reads them as used only by external tests, and so does this one.
package ext

// Gizmo is named by ExampleGizmo, and nothing else names it.
type Gizmo struct{} // want: type Gizmo is exported, but only the external tests of \S+ use it \(no fix: external tests use it\)$

// Run is named by ExampleRun_twice: the suffix after _ starts with a
// lower-case letter, so it names no method.
func Run() int { return 1 } // want: func Run is exported, but only the external tests of \S+ use it \(no fix: external tests use it\)$

// Spare is named by nothing, and is fixed.
func Spare() int { return 2 } // want: func Spare is exported, but nothing outside \S+ uses it$
