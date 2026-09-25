package u

import (
	_ "unsafe"

	"example.com/edge/internal/m"
)

//go:linkname linked example.com/edge/internal/m.Linked
func linked() int

var _ = linked

var _ = m.Point{1, 2}
