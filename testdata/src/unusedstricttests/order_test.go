package directivestricttests

import "testing"

func TestOrder(t *testing.T) {
	if userShared() != 1 {
		t.Fatal("userShared")
	}
}
