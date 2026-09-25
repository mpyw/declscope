package a

import (
	"fmt"
	"io"
)

// Used is named by another package, so nothing is said.
func Used() int { return Lonely() + Taken() + Count(1) + Dup() + DUP() + ExclQual + WinRef + GenRef }

func Lonely() int { return 1 } // want: func Lonely is exported, but nothing outside example.com/basic/internal/a uses it$

func Taken() int { return taken() } // want: func Taken is exported.*no fix: the unexported name is taken or would be captured

func taken() int { return 0 }

// Count is captured by the parameter of the same name at its one use.
var Count = func(count int) int { return count } // want: var Count is exported.*no fix: the unexported name is taken or would be captured

func useCount(count int) int { return count + Count(count) }

var _ = useCount

func Dup() int { return 1 } // want: func Dup is exported, but nothing.*uses it$

// DUP lowers to the name Dup's fix claims.
func DUP() int { return 2 } // want: func DUP is exported.*no fix: the unexported name is taken or would be captured

var OnlyExtTest = 1 // want: var OnlyExtTest is exported, but only the external tests.*no fix: external tests use it

// Hidden escapes into an interface through fmt.Println, so reflection may
// read its name and its fields.
type Hidden struct { // want: type Hidden is exported.*no fix: its type escapes
	Field int // want: field Field is exported.*no fix: its type escapes
}

func show() { fmt.Println(Hidden{Field: 1}) }

var _ = show

type Plain struct { // want: type Plain is exported, but nothing.*uses it$
	X int // want: field X is exported, but nothing.*uses it$
}

type Svc struct{} // want: type Svc is exported, but nothing.*uses it$

func (Svc) Run() int { return Plain{X: 1}.X } // want: method Run is exported, but nothing.*uses it$

var _ = Svc{}.Run

// Wr satisfies io.Writer where the compiler checks it, so Write is used. The
// check makes no value at run time, so nothing escapes and Wr itself is fixed.
type Wr struct{} // want: type Wr is exported, but nothing.*uses it$

func (Wr) Write(p []byte) (int, error) { return len(p), nil }

var _ io.Writer = Wr{}

// Exposed is handed out by pub, an importable package, so another module can
// call M and read F on a value of it without naming the type.
type Exposed struct {
	F int
}

func (Exposed) M() {}

// ExclQual is named only by a build-excluded file of package b.
var ExclQual = 1

// WinRef is named by a build-excluded file of this package, which a rename
// would leave behind.
var WinRef = 1 // want: var WinRef is exported.*no fix: a build-excluded file of its package names it

// GenRef is named by a generated file, which a regeneration would put back.
var GenRef = 1 // want: var GenRef is exported.*no fix: a generated file names it

//declscope:ignore overexported
func Silenced() {}

//declscope:ignore overexported // want: unused //declscope:ignore overexported
func Ignored() int { return Used() }

// An ignore naming another rule as well is judged by neither side: the
// analyzer cannot see overexported, and this run cannot see the analyzer's.
//
//declscope:ignore overexported,qualify
func Mixed() int { return Used() }

// A bare ignore does not reach overexported, so this is reported anyway, and
// the ignore is the analyzer's to judge.
//
//declscope:ignore
func Bare() {} // want: func Bare is exported, but nothing.*uses it$
