// Package t is used by its own tests only, which are in the package, so its
// fixes rename the test files too.
package t

// Helper is named by ExampleHelper, which go vet checks.
func Helper() int { return 1 } // want: func Helper is exported.*no fix: an example function names it

// Plain is named by nothing but its package.
func Plain() int { return 3 } // want: func Plain is exported, but nothing.*uses it$

// Shown is named only by an example of the external test package.
func Shown() {} // want: func Shown is exported, but only the external tests.*no fix: external tests use it

var _ = Plain

// Tool and Run are named by ExampleTool_Run.
type Tool struct{} // want: type Tool is exported.*no fix: an example function names it

func (Tool) Run() {} // want: method Run is exported.*no fix: an example function names it

var _ = Tool{}.Run
