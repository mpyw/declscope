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

// Made keeps its name, since an example function names it, so the type it
// returns must stay nameable. Deeper keeps its name for the same reason, so
// Deep, which it hands out, keeps its name too, with its report. Unused, a
// method like Deeper would be unexported with Deep in the same run.
func Made() Produced { return Produced{} } // want: func Made is exported.*no fix: an example function names it

type Produced struct{} // want: type Produced is exported.*no fix: an example function names it

func (Produced) Deeper() *Deep { return nil } // want: method Deeper is exported.*no fix: an example function names it

type Deep struct{} // want: type Deep is exported.*no fix: an exported declaration that keeps its name hands it out

// Parent keeps its name, and so does its method Kid, since an example names
// both, so Child, which Kid hands out, keeps its name with its report.
// Child's method Back hands Parent back, and keeps its name because the
// unexported one is taken. A type kept only through what it keeps holds
// nothing up, so Parent's own report stays.
type Parent struct{} // want: type Parent is exported.*no fix: an example function names it

func (Parent) Kid() *Child { return nil } // want: method Kid is exported.*no fix: an example function names it

type Child struct { // want: type Child is exported.*no fix: an exported declaration that keeps its name hands it out
	back int
}

func (Child) Back() *Parent { return nil } // want: method Back is exported.*no fix: the unexported name is taken on its type

// Outer keeps its name, since an example names it, and embeds Inner. Code
// holding an Outer names Inner only by spelling the field, which would be a
// use of Inner, or writes it by position, which is one too (Framed below).
// Neither happens, so Inner is fixed, and the field with it.
type Outer struct { // want: type Outer is exported.*no fix: an example function names it
	Inner
}

type Inner struct{} // want: type Inner is exported, but nothing.*uses it$

// The external tests write a Framed by position, which writes its embedded
// Frame too: unexported, it could not be written there.
type Framed struct{ Frame } // want: type Framed is exported, but only the external tests.*no fix: external tests use it

type Frame int // want: type Frame is exported, but only the external tests.*no fix: external tests use it

var (
	_ = Parent{}.Kid
	_ = Outer{}.Inner
	_ = Child{}.Back
	_ = Child{}.back
	_ = Made
	_ = Lonely() + WinRef + GenRef + Dotted + Helper()
	_ = Picker{}.Pick
	_ = Tool{}.Run
	_ = current
)
