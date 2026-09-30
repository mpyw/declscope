//go:build never

package b

import (
	"example.com/uses/internal/go-foo"
	"example.com/uses/internal/q"
)

var _ = q.ExclQual

var _ = foo.Foo
