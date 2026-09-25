//go:build never

package b

import (
	"example.com/uses/internal/a"
	"example.com/uses/internal/go-foo"
)

var _ = a.ExclQual

var _ = foo.Foo
