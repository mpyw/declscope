package nstestsuffix

// foo_test_linux.go is an ordinary linux file, not a test of this one: only a
// name ending in _test.go is a test file. It is namespace fooTest, so its use
// is a crossing.
func fooHelper() int { return 1 } // want `func fooHelper is private to namespace "foo", but is used from namespace "fooTest"`

func fooOK() int { return fooHelper() }
