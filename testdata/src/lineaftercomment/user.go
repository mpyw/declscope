package lineaftercomment

// A directive trailing a declaration's first or last line binds even when an
// earlier comment shares that line. The earlier comment is not a directive,
// so it is not reported either.

func userHelper(a int /* units */) int { //declscope:shared
	return a
}

func userOne() int { return 1 /* one */ } //declscope:ignore boundary

type UserT[E any /* elem */] struct { //declscope:ignore boundary
	x E
}

var /* group */ ( //declscope:ignore boundary
	userA = 1
)
