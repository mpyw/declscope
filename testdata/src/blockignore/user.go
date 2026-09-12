package blockignore

// A directive on a block is one comment, however many specs it reaches. It
// is used by userA's crossing and not needed by userB, and that is enough:
// it silenced something, so it is not unused.
//
//declscope:ignore escape
var (
	userA = 1
	userB = 2
)

// One directive shared by both names on a line, needed by userC alone.
var userC, userD = 3, 4 //declscope:ignore escape

// A block directive that silenced nothing is reported once, at the comment,
// naming every declaration it reached — not once per spec.
//
//declscope:ignore escape // want `unused //declscope:ignore escape on userE, userF`
var (
	userE = 5
	userF = 6
)

type User struct {
	// Shared by both fields, needed by x alone.
	x, y int //declscope:ignore escape
	// Silenced nothing for either.
	p, q int //declscope:ignore demote // want `unused //declscope:ignore demote on User.p, User.q`
}

// A spec's own ignore is judged as its own comment, apart from the block's:
// the block's is used by userG, the spec's demote is not.
//
//declscope:ignore escape
var (
	userG = 7
	//declscope:ignore demote // want `unused //declscope:ignore demote on userH`
	userH = 8
)
