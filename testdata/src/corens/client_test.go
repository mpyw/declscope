package corens

import "testing"

// A test file joins its subject's namespace, and the core is a namespace like
// any other: client.go is core, so this file is too and reaches what it
// declares. Deriving the join from the stem alone would leave the core out of
// it, and the only repair would be to widen everything the core holds.
func TestClient(t *testing.T) {
	if helper() == 0 {
		t.Fatal("helper answered zero")
	}
}
