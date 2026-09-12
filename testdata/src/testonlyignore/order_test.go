package testonlyignore

import "testing"

func TestHelper(t *testing.T) {
	if userHelper() != 1 {
		t.Fatal("unexpected")
	}
}

// The test variant sees every file in the package, so it does judge, and a
// directive written in a test file is checked by that variant alone.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on orderUnused`
func orderUnused() int { return 3 }

var _ = orderUnused
