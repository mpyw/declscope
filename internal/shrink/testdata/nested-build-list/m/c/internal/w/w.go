package w

// Qux is what the copy of v that the nested module k builds with uses. k
// lies under no internal parent, so this run never loads it, but its build
// list still names that copy. Unexporting Qux breaks go build ./... of k, so
// its fix is withheld.
func Qux() {} // want: func Qux is exported, but nothing outside example.com/nbl/c/internal/w uses it \(no fix: the build of the nested module example.com/nbl/k uses example.com/nbl/c/v from .*v-copy, which imports example.com/nbl/c/internal/w\)
