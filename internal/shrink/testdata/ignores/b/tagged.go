//go:build !never

package b

import "example.com/ignores/internal/i"

// This file is behind a build constraint, and -tags=never leaves it out.
var _ = i.Mode{}.Tagged + i.Mode{}.Plain

var _ = i.TaggedFunc
