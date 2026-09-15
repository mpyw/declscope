package surplustests

import "testing"

func TestOther(t *testing.T) {
	if userHelp() != 1 {
		t.Fatal("no")
	}
}
