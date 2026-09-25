package w

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

func ExampleTool_Run() {}

func ExampleTool_suffix() {}

func ExampleMade() {}

func ExampleProduced_Deeper() {}

func ExampleParent_Kid() {}

func ExampleOuter() {}

// Exported is declared in a test file and used only by the package's own
// tests, so it is fixed, and the fix renames the test that calls it.
func Exported() int { return 2 } // want: func Exported is exported, but nothing.*uses it$
