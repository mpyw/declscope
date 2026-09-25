// Package a holds one case per finding of the third review.
package a

// Version may be set by -ldflags -X, which names it where go/types never
// looks.
var Version = "dev" // want: var Version is exported.*no fix: a string variable may be set by -ldflags -X

func current() string { return Version }

var _ = current

// An ignore after `struct {` or `var (` binds, as it does for the analyzer.
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

// T.F and U.F are both promoted into Outer, so Outer's F is ambiguous, and
// renaming T.F to f would make Outer's f ambiguous too.
type T struct { // want: type T is exported, but nothing.*uses it$
	F int // want: field F is exported.*no fix: the unexported name is taken on its type
}

type U struct { // want: type U is exported, but nothing.*uses it$
	F int // want: field F is exported.*no fix: the unexported name is taken on its type
	f int
}

type outer struct {
	T
	U
}

var _ = outer{}.f + T{}.F + U{}.F

// Names Go would spell differently once unexported.
func HTTPServer() {} // want: func HTTPServer is exported, but nothing.*uses it$

const MAX_RETRIES = 3 // want: const MAX_RETRIES is exported.*no fix: the name has no unexported spelling Go would use

var _ = MAX_RETRIES

func parse() int { return 2 }
