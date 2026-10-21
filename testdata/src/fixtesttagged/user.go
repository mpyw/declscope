package fixtesttagged

// user_test.go is in the test variant. zz_tagged_test.go is an in-package
// test file the build leaves out, so the test variant lacks it too. The
// non-test variant stops at user_test.go and offers no rename. The test
// variant reads the tagged file for its names, as for any excluded file, and
// withholds only the rename that file would notice.

// load is named from zz_tagged_test.go, so neither variant renames it.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; rename it to userLoad, or to another name that carries "user"`

// Named only in files the fix rewrites, so the test variant renames it here
// and in user_test.go.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; rename it to userFree, or to another name that carries "user"`

func Exported() int { return load() + free() }
