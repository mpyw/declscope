package x

// Foo is what the copy of t that the nested module s/n builds with uses.
// Nothing in this tree does, and unexporting it breaks go build ./... of s/n,
// so its fix is withheld.
func Foo() {} // want: func Foo is exported, but nothing outside example.com/nbl/s/internal/x uses it \(no fix: the build of the nested module example.com/nbl/s/n uses example.com/nbl/s/t from .*, which imports example.com/nbl/s/internal/x\)

// Bar is used by nothing, but the copy may use it too, so its fix is
// withheld all the same.
func Bar() {} // want: func Bar is exported, but nothing outside example.com/nbl/s/internal/x uses it \(no fix: the build of the nested module example.com/nbl/s/n uses example.com/nbl/s/t from
