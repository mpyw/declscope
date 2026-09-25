// Package i exercises //declscope:ignore overexported: where it binds, what
// it silences, and when it is itself reported unused.
package i

//declscope:ignore overexported
func Silenced() {}

// The declaration is used by package b, so the ignore silences nothing.
//
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

// A comment trailing a func's first or last line binds, as it does for the
// analyzer.
func Trailing() {} //declscope:ignore overexported

func Multi() {
} //declscope:ignore overexported

// So does one after `struct {` or `var (`.
type Braced struct { //declscope:ignore overexported
	n int
}

var ( //declscope:ignore overexported
	Parened = 1
)

// F is used by package b, so the trailing ignore silences nothing. The doc
// ignore naming unused answers its unused report: both are bound to F.
//
//declscope:ignore unused
func F() int { return 1 } //declscope:ignore overexported

// An ignore beside it naming unused answers the same report.
//
//declscope:ignore overexported
//declscope:ignore unused
func Answered() int { return Used() }

// A silenced declaration claims no name, so FOO's fix is offered.
//
//declscope:ignore overexported
func Foo() {}

func FOO() {} // want: func FOO is exported, but nothing.*uses it$

// An ignore bound to no declaration silences nothing, and nothing beside it
// answers its unused report.
func stray() {
	//declscope:ignore overexported // want: unused //declscope:ignore overexported
}

func Used() int { return 1 }

var (
	_ = Braced{}.n
	_ = stray
)
