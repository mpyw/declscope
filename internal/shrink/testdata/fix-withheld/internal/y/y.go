// Package y is imported by build-excluded files of packages u and v. They
// are not type-checked, so any field or method of y, and any type of y a
// struct embeds, may be used there without its name showing. None of those
// is fixed. A package-level name counts only where it is written.
package y

// Dotted is named under a dot import by the file of package u.
var Dotted = 1 // want: var Dotted is exported.*no fix: a build-excluded file of another package may use it

// Unnamed is written by neither file, so it is fixed.
func Unnamed() {} // want: func Unnamed is exported, but nothing.*uses it$

// Picker's method is selected by name in the file of package u.
type Picker struct{} // want: type Picker is exported, but nothing.*uses it$

func (Picker) Pick() {} // want: method Pick is exported.*no fix: a build-excluded file of another package may use it

// Runner's method satisfies an interface in the file of package v, which
// never writes its name.
type Runner struct{} // want: type Runner is exported.*no fix: an exported declaration that keeps its name hands it out

func (Runner) Run() {} // want: method Run is exported.*no fix: a build-excluded file of another package may use it

func NewRunner() Runner { return Runner{} }

// Converted's field is matched by name when the file of package v converts
// an unnamed struct to it.
type Converted struct {
	Flag int // want: field Flag is exported.*no fix: a build-excluded file of another package may use it
}

// Holder embeds Embedded, and the file of package v selects the embedded
// field by the type's name.
type Holder struct{ Embedded } // want: type Holder is exported.*no fix: an exported declaration that keeps its name hands it out

type Embedded struct{} // want: type Embedded is exported, but nothing.*no fix: a build-excluded file of another package may use it

func NewHolder() Holder { return Holder{} }

// Keyed, Filled, InSlice, InMap and DotFilled are written in composite
// literals: by key, and by position, with the type elided or not.
type Keyed struct {
	Flag  int // want: field Flag is exported.*no fix: a build-excluded file of another package may use it
	Other int // want: field Other is exported.*no fix: a build-excluded file of another package may use it
}

type Filled struct {
	A int    // want: field A is exported.*no fix: a build-excluded file of another package may use it
	B string // want: field B is exported.*no fix: a build-excluded file of another package may use it
}

type InSlice struct {
	C int // want: field C is exported.*no fix: a build-excluded file of another package may use it
}

type InMap struct {
	D int // want: field D is exported.*no fix: a build-excluded file of another package may use it
}

type DotFilled struct { // want: type DotFilled is exported.*no fix: a build-excluded file of another package may use it
	E int // want: field E is exported.*no fix: a build-excluded file of another package may use it
}

var (
	_ = Picker{}.Pick
	_ = Dotted
)
