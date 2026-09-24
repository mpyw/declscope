package fixoutside

// helper is also named from gen.go, which is generated and so not a file the
// pass rewrites; the rename would leave that use dangling.
//
//declscope:package
func helper() int { return 1 } // want `func helper does not carry namespace "user" anywhere in its name; rename it to userHelper, or to another name that carries "user"`

// aux is named from ext.go, which the configuration excludes.
//
//declscope:package
func aux() int { return 2 } // want `func aux does not carry namespace "user" anywhere in its name; rename it to userAux, or to another name that carries "user"`

// Every use is in a collected file, so this one is renamed.
func local() int { return 3 } // want `func local does not carry namespace "user" anywhere in its name; rename it to userLocal, or to another name that carries "user"`

var _ = local

// box is named from gen.go only through the field embedding it, which is
// spelled with the type's name, so the rename would leave that use dangling.
type box struct{ n int } // want `type box does not carry namespace "user" anywhere in its name; rename it to userBox, or to another name that carries "user"`

// UserHolder embeds box.
type UserHolder struct{ box }
