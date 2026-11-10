package fixtestexcluded

// The directory lists user_test.go before zz_excluded.go. The non-test
// variant stops reading at the test file, which already withholds every
// rename there. The test variant sees the test file and still reads the
// excluded one, so only a rename that file does not disturb is offered.

// load is named from zz_excluded.go, so neither variant renames it.
func load() int { return 1 } // want `func load does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userLoad is one candidate, not necessarily the best\)`

// Named only in files the fix rewrites, so the test variant renames it here
// and in user_test.go.
func free() int { return 3 } // want `func free does not carry namespace "user" anywhere in its name; move it to the file it belongs to, or give it a name that carries "user" \(userFree is one candidate, not necessarily the best\)`

func Exported() int { return load() + free() }
