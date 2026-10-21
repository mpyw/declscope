package corenamedtest

import "testing"

// No directive, so this file joins its core subject.
func TestErrClosed(t *testing.T) {
	if ErrClosed != probeHelper() {
		t.Fatal("unexpected")
	}
}
