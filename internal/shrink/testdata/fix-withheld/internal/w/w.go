// Package w holds declarations nothing outside the package uses, where the
// fix is withheld for a reason other than the rename itself. Each is
// reported, and says why.
package w

// Lonely has no reason to keep its name, and is fixed.
func Lonely() int { return 1 } // want: func Lonely is exported, but nothing.*uses it$

// WinRef is named by a build-excluded file of this package, which the rename
// would leave behind.
var WinRef = 1 // want: var WinRef is exported.*no fix: a build-excluded file of its package names it

// GenRef is named by a generated file, which a regeneration would put back.
var GenRef = 1 // want: var GenRef is exported.*no fix: a generated file names it

// Dotted is named under a dot import by a build-excluded file of package u.
var Dotted = 1 // want: var Dotted is exported.*no fix: a build-excluded file of another package may use it

// Picker's method is selected by name in a build-excluded file of package u.
type Picker struct{} // want: type Picker is exported, but nothing.*uses it$

func (Picker) Pick() {} // want: method Pick is exported.*no fix: a build-excluded file of another package may use it

// Helper, Tool and Run are named by example functions, which go vet checks.
func Helper() int { return 1 } // want: func Helper is exported.*no fix: an example function names it

type Tool struct{} // want: type Tool is exported.*no fix: an example function names it

func (Tool) Run() {} // want: method Run is exported.*no fix: an example function names it

// Version may be set by -ldflags -X, which names it where go/types never
// looks.
var Version = "dev" // want: var Version is exported.*no fix: a string variable may be set by -ldflags -X

func current() string { return Version }

// Made stays exported, since an example function names it, so the type it
// returns stays nameable: Produced is not reported, though an example names
// it too. Deeper stays exported for the same reason, so Deep, which it hands
// out, is kept as well. Unused, a method
// like Deeper would be unexported with Deep in the same run.
func Made() Produced { return Produced{} } // want: func Made is exported.*no fix: an example function names it

type Produced struct{}

func (Produced) Deeper() *Deep { return nil } // want: method Deeper is exported.*no fix: an example function names it

type Deep struct{}

var (
	_ = Made
	_ = Lonely() + WinRef + GenRef + Dotted + Helper()
	_ = Picker{}.Pick
	_ = Tool{}.Run
	_ = current
)
