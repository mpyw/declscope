//declscope:package

package fixsurplusstrictunusedstrict

// userHelper takes the file's package, and order.go calls it.
func userHelper() int { return 3 }

// userSpare takes it too, and nothing else calls it. Its narrowing gives
// this file an edit whatever the rest gets, so the golden is compared.
func userSpare() int { return 4 } // want `func userSpare takes package scope`

var _ = userSpare

// The type restates the file's package, and its report names the type alone.
// Narrowing spare leaves it reading the same, so the fix is offered. The
// directive's own deletion is withheld, since surplus reads it.
//
//declscope:package // want `unused //declscope:package on userEntry: it already has package scope`
type userEntry struct {
	key   int
	spare int // want `field userEntry.spare takes package scope`
}

var _ = userEntry{}.spare

// A type that declares no name has a report that says what everything it
// reaches has. Narrowing spare would make it say that one field states its
// own scope, so the fix is withheld.
//
//declscope:package // want `unused //declscope:package: every declaration it reaches already has package scope`
type _ struct {
	Wide  int
	spare int // want `field _.spare takes package scope`
}
