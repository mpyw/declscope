package t

import (
	"os"
	"testing"
)

// The test functions are found by name, so none of them is reported.
func TestMain(m *testing.M) { os.Exit(m.Run()) }

func TestHelper(t *testing.T) { _ = Helper() + Exported() }

func BenchmarkHelper(b *testing.B) {}

func FuzzHelper(f *testing.F) {}

func ExampleHelper() {}

// Exported is declared in a test file, and only tests use it.
func Exported() int { return 2 } // want: func Exported is exported, but nothing.*uses it$

func ExampleTool_Run() {}

func ExampleTool_suffix() {}
