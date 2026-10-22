//go:build never

package g

import "example.com/nbl/b/u"

// Run is compiled only under the never tag, so this run's load never
// compiles u. The build under that tag does.
func Run() { u.Run() }
