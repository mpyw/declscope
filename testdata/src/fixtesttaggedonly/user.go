package fixtesttaggedonly

// The only test file is user_tagged_test.go, which the build leaves out, so
// the package has no test variant. The non-test variant learns from the
// ignored files that no variant sees it, and reads it for its names.

// load is named from user_tagged_test.go, so it is not renamed.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userLoad is one candidate, not necessarily the best\)`

// Named only in this file, so it is renamed.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userFree is one candidate, not necessarily the best\)`

func Exported() int { return load() + free() }
