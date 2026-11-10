package fixoutside

// helper is also named from gen.go, which is generated and so not a file the
// pass rewrites; the rename would leave that use dangling.
//
//declscope:shared
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userHelper, or move it to the file it belongs to`

// aux is named from ext.go, which the configuration excludes.
//
//declscope:shared
func aux() int { return 2 } // want `func aux does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userAux, or move it to the file it belongs to`

// Every use is in a collected file, so this one is renamed.
func local() int { return 3 } // want `func local does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userLocal, or move it to the file it belongs to`

var _ = local

// box is named from gen.go only through the field embedding it, which is
// spelled with the type's name, so the rename would leave that use dangling.
type box struct{ n int } // want `type box does not carry namespace "user" anywhere in its name; give it a name that carries "user", such as userBox, or move it to the file it belongs to`

// UserHolder embeds box.
type UserHolder struct{ box }
