//go:build !never

package b

import (
	"fmt"

	"example.com/ignores/internal/i"
)

// This file is behind a build constraint, and -tags=never leaves it out.
var _ = i.Mode{}.Tagged + i.Mode{}.Plain

var _ = i.TaggedFunc

// Shown and Printed escape into fmt.Println, behind the constraint as well.
func show() { fmt.Println(i.Shown{Field: 1}, i.Printed{}) }

var _ = show
