package b

import (
	_ "unsafe"

	"example.com/uses/internal/a"
)

//go:linkname linked example.com/uses/internal/a.Linked
func linked() int

//go:linkname byValue example.com/uses/internal/a.ByValue.Pulled
func byValue()

//go:linkname byPointer example.com/uses/internal/a.(*ByPointer).Pulled
func byPointer()

//go:linkname nodot nodot
func nodot()

//go:linkname missing example.com/uses/internal/a.Missing
func missing()

var (
	_, _, _, _, _ = linked, byValue, byPointer, nodot, missing
	_             = &a.Point{1, 2}
	_             = a.Make()
	_             = a.MakeHandle()
	_             a.Spelled2
)

func init() { a.Spelled() }
