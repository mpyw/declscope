package braceignore

// A directive after the opening brace binds to the type, the way one after a
// function's opening brace binds to the function, and covers the fields like
// any directive on the type.
type UserT struct { //declscope:ignore boundary
	x int
}

// It is parsed there too, so a bogus one is reported rather than dropped.
type UserV struct { //declscope:bogus // want `unknown directive declscope:bogus`
	z int // want `field UserV.z is private to namespace "user", but is used from namespace "order"`
}

// A comment the parser hung on a field is the field's even on the brace
// line: only w is covered. This is the one shape here gofmt would rewrite —
// it moves w to its own line — and it is kept as written because that is
// the shape the rule is about.
type UserW struct { w int //declscope:ignore boundary
	v int // want `field UserW.v is private to namespace "user", but is used from namespace "order"`
}

// After "(" a directive reaches every spec of the block...
var ( //declscope:ignore boundary
	userA = 1
	userB = 2
)

// ...and after ")" as well.
const (
	userC = 3
) //declscope:ignore boundary

// A function's closing brace is its last line.
func userHelper() int {
	return 4
} //declscope:ignore boundary

// A directive that binds nowhere is reported as misplaced instead of being
// dropped: one separated from its declaration by a blank line...
//
//declscope:ignore boundary // want `misplaced declscope:ignore: no declaration here for it to bind to`

func userLoose() int { return 5 } // want `func userLoose is private to namespace "user", but is used from namespace "order"`

// ...one inside a function body...
func userBody() int {
	//declscope:package // want `misplaced declscope:package: no declaration here for it to bind to`
	return 6
}

var _ = userBody

// ...and one on a line in the middle of a declaration.
type UserM struct {
	//declscope:ignore boundary // want `misplaced declscope:ignore: no declaration here for it to bind to`

	m int // want `field UserM.m is private to namespace "user", but is used from namespace "order"`
}
