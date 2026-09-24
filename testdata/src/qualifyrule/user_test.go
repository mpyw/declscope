package qualifyrule

import "testing"

// The toolchain finds a test by its name, so no rename is asked of it.
func TestUser(t *testing.T) { _ = fixture() }

// Any other function in a test file is an ordinary declaration.
func fixture() int { return 1 } // want `func fixture does not carry namespace "user" anywhere in its name; rename it to userFixture, or to another name that carries "user"`
