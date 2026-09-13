package qualifyrule

// Unexported package-level declarations must say which unit owns them.
func helper() int { return 1 } // want `func helper does not carry the prefix of namespace "user"; rename it to userHelper`

// The character after the prefix must start a new word, so this is not
// prefixed with "user" either.
func users() int { return 2 } // want `func users does not carry the prefix of namespace "user"; rename it to userUsers`

var count int // want `var count does not carry the prefix of namespace "user"; rename it to userCount`

const limit = 3 // want `const limit does not carry the prefix of namespace "user"; rename it to userLimit`

type payload struct { // want `type payload does not carry the prefix of namespace "user"; rename it to userPayload`
	// Members are exempt: they are already namespaced by the type that owns
	// them, and a prefix here would be the stutter Go idiom avoids.
	field int
}

func (p *payload) method() int { return p.field }

// Exported identifiers need no label by default; rules.exportedLabels asks
// them for one.
func Exported() int {
	p := &payload{}
	return helper() + users() + count + limit + p.method() + userOK() + deliberatelyUnlabeled()
}

// Already labeled.
func userOK() int { return 4 }

//declscope:ignore
func deliberatelyUnlabeled() int { return 5 }
