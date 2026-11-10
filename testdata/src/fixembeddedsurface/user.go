package fixembeddedsurface

// item is embedded in userBox, where its field hides the method item that
// userItemer promotes from one level down. Renamed, the field would stop
// hiding it: userBox would gain the method and satisfy userItemer, and
// userSatisfies would answer true. That compiles, so the rename is withheld.
type item struct{ n int } // want `type item does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userItem, or move it to the file it belongs to`

type userItemer interface{ item() string }

type userBox struct {
	item
	userItemer
}

func userSatisfies() bool {
	_, ok := any(userBox{}).(userItemer)
	return ok
}

// spare is embedded where no interface of the package asks for its name, so
// no method can surface and it is renamed.
type spare struct{ n int } // want `type spare does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userSpare, or move it to the file it belongs to`

type userFree struct {
	spare
	userItemer
}

func userReadFree(h userFree) int { return h.spare.n }
