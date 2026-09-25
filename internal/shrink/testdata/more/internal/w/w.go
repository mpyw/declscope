package w

import (
	_ "unsafe"

	"example.com/more/internal/r"
)

var _ = &r.Point{1, 2}

//go:linkname pulled example.com/more/internal/r.Linked.Pulled
func pulled()

//go:linkname nodot nodot
func nodot()

//go:linkname missing example.com/more/internal/r.Missing
func missing()

var _, _, _ = pulled, nodot, missing
