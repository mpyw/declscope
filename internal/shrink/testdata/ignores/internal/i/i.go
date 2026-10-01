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

// Hand is called by package b, so the Handed it returns is used there, and
// the ignore silences nothing.
//
//declscope:ignore overexported // want: unused //declscope:ignore overexported
type Handed struct{}

func Hand() *Handed { return nil }

// Build is silenced, so it keeps its name, and the type it returns keeps its
// own, with its report.
//
//declscope:ignore overexported
func Build() *Built { return nil }

type Built struct{} // want: type Built is exported.*no fix: an exported declaration that keeps its name hands it out

var (
	_ = Braced{}.n
	_ = stray
)

// Mode's fields are used outside only by b/tagged.go, which is behind a build
// constraint, except Plain, which b.go uses too. A configuration that leaves
// tagged.go out reports Tagged, with its fix withheld, and the ignore answers
// that report there, so it is not unused here either. Plain's ignore is,
// since b.go uses Plain in every configuration.
type Mode struct {
	//declscope:ignore overexported
	Tagged int
	//declscope:ignore overexported // want: unused //declscope:ignore overexported
	Plain int
}

// TaggedFunc is used only by b/tagged.go too, but tagged.go spells
// i.TaggedFunc, which a configuration leaving it out still reads as a use.
// The ignore is needed in none of them.
//
//declscope:ignore overexported // want: unused //declscope:ignore overexported
func TaggedFunc() {}

// Shown's field is used outside only by b/tagged.go, which also passes a
// Shown to fmt.Println. The escape is behind the constraint too, so the run
// without tagged.go reports Field and needs the ignore.
type Shown struct {
	//declscope:ignore overexported
	Field int
}

// Printed's field is never named outside, but b/tagged.go passes a Printed to
// fmt.Println, which reads its fields. Without tagged.go, the field is
// reported with its fix withheld, so the ignore is needed there.
type Printed struct {
	//declscope:ignore overexported
	Hidden int
}
