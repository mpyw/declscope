//go:build !never

package x_test

import (
	"testing"

	"example.com/ignores/internal/x"
)

func TestTagged(t *testing.T) {
	_ = x.Helper()
	_ = x.Kept()
	_ = x.New()
}
