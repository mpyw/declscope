// An external test package is imported by nothing, so it is no root of the
// exposure walk.
package pub_test

import "example.com/more/pub"

var _ = pub.K
