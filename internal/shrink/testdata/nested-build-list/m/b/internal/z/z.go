package z

// Baz is what the copy of u that the nested module b/g builds with under the
// never tag uses. Nothing in this tree does, and unexporting it breaks
// go build -tags=never ./... of b/g, so its fix is withheld.
func Baz() {} // want: func Baz is exported, but nothing outside example.com/nbl/b/internal/z uses it \(no fix: the build of the nested module example.com/nbl/b/g uses example.com/nbl/b/u from .*u-copy, which imports example.com/nbl/b/internal/z\)
