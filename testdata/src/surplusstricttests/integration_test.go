package surplusstricttests

import "testing"

func TestIntegration(t *testing.T) {
	if (userCard{note: 1}).note != 1 {
		t.Fatal("no")
	}
}

var _ = userFixture{}.name
