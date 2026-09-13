package toolchainnames

// TestHelper is not in a _test.go file, so the toolchain never finds it by
// name and the prefix is asked for like any other.
func TestHelper() int { return 1 } // want `func TestHelper does not carry the prefix of namespace "user"; rename it to UserTestHelper`

func userLoad() int { return 2 }
