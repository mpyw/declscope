package fixtesttaggedonly

// The only test file is user_tagged_test.go, which the build leaves out, so
// the package has no test variant. The non-test variant learns from the
// ignored files that no variant sees it, and reads it for its names.

// load is named from user_tagged_test.go, so it is not renamed.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; rename it to userLoad, or to another name that carries "user"`

// Named only in this file, so it is renamed.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; rename it to userFree, or to another name that carries "user"`

func Exported() int { return load() + free() }
