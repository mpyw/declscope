package surplusstrictopaque

// userBox crosses, so it is reported and its fix widens it. Under strict the
// fix would also narrow n, which nothing outside reads, but the rule cannot
// read gen.go, so it predicts nothing and the fix narrows no member.
type userBox struct { // want `type userBox is private to namespace "box", but is used from namespace "order"`
	n int
}

func boxRead(b userBox) int { return b.n }

var _ = boxRead
