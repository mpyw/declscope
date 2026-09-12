package nsidentity

import "testing"

func TestTOTP(t *testing.T) {
	if totp() != 1 {
		t.Fatal("unexpected")
	}
}
