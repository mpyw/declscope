package a_test

import (
	"testing"

	"example.com/uses/internal/a"
)

func TestX(t *testing.T) { _ = a.OnlyExtTest + a.Ext{1}.A + a.ParseForTest() }

// ExampleShown names Shown, which go vet checks.
func ExampleShown() {}
