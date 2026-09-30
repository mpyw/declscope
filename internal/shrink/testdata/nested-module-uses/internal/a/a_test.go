package a

import "testing"

// TestCalled gives this package a test variant, which tools' load must not
// read as what tools imports.
func TestCalled(t *testing.T) { Called() }
