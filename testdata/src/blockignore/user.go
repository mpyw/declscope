package blockignore

// A directive on a block is one comment, however many specs it reaches. It
// is used by userA's crossing and not needed by userB, and that is enough:
// it silenced something, so it is not unused.
//
//declscope:ignore boundary
var (
	userA = 1
	userB = 2
)

// One directive shared by both names on a line, needed by userC alone.
var userC, userD = 3, 4 //declscope:ignore boundary

// A block directive that silenced nothing is reported once, at the comment,
// naming every declaration it reached — not once per spec.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on userE, userF`
var (
	userE = 5
	userF = 6
)

type User struct {
	// Shared by both fields, needed by x alone.
	x, y int //declscope:ignore boundary
	// Silenced nothing for either: the naming rule never reaches a member.
	p, q int //declscope:ignore qualify // want `unused //declscope:ignore qualify on User.p, User.q`
}

// A spec's own ignore is judged as its own comment, apart from the block's:
// the block's is used by userG, the spec's qualify — needless on a name that
// carries its namespace — is not.
//
//declscope:ignore boundary
var (
	userG = 7
	//declscope:ignore qualify // want `unused //declscope:ignore qualify on userH`
	userH = 8
)
