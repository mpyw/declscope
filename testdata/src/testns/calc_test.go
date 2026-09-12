package testns

import "testing"

func TestHelper(t *testing.T) {
	if helper() != 1 {
		t.Fatal("unexpected")
	}
	if calcShared() != 1 {
		t.Fatal("unexpected")
	}
}
