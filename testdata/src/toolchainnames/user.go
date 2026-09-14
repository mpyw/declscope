package toolchainnames

// TestHelper is not in a _test.go file, so the toolchain never finds it by
// name and the prefix is asked for like any other.
func TestHelper() int { return 1 } // want `func TestHelper does not carry namespace "user" anywhere in its name; rename it to UserTestHelper`

func userLoad() int { return 2 }
