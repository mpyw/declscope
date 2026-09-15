package qualifyrule

// Unexported package-level declarations must say which unit owns them.
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper, or to another name that carries "user"`

// The right edge of the match is free, so a plural spells the namespace as
// plainly as the singular would.
func users() int { return 2 }

var count int // want `var count does not carry namespace "user" anywhere in its name; rename it to userCount, or to another name that carries "user"`

const limit = 3 // want `const limit does not carry namespace "user" anywhere in its name; rename it to userLimit, or to another name that carries "user"`

type payload struct { // want `type payload does not carry namespace "user" anywhere in its name; rename it to userPayload, or to another name that carries "user"`
	// Members are exempt: they are already namespaced by the type that owns
	// them, and a prefix here would be the stutter Go idiom avoids.
	field int
}

func (p *payload) method() int { return p.field }

// Exported identifiers need no prefix by default; rules.naming.exported asks
// them for one.
func Exported() int {
	p := &payload{}
	return helper() + users() + count + limit + p.method() + userOK() + newUser() + deliberatelyUnprefixed()
}

// Already prefixed.
func userOK() int { return 4 }

// The namespace need not lead the name: Go spells a constructor newX, and the
// namespace inside it marks the owner as plainly as a prefix would.
func newUser() int { return 6 }

//declscope:ignore
func deliberatelyUnprefixed() int { return 5 }
