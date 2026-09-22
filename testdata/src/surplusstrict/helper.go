// The file's directive widens everything it declares. helperShared is called
// from order.go, so the directive is in use and loose says nothing. strict
// judges each declaration under it on its own.
//
//declscope:package

package surplusstrict

func helperShared() int { return helperLocal() }

// Called only here.
func helperLocal() int { return 1 } // want `func helperLocal takes package scope from the file's //declscope:package, but no use from another namespace is visible to declscope`

// One spec holding two names takes one directive, so both are reported.
var helperA, helperB = 1, 2 // want `var helperA takes package scope` `var helperB takes package scope`

// helperC is read from order.go, and a directive above the spec would narrow
// it too, so neither name is reported.
var helperC, helperD = 3, 4

// Nothing outside spells helperBox or reads n. Narrowing the type narrows n
// with it, so the type is reported and n is not reported again.
type helperBox struct { // want `type helperBox takes package scope from the file's //declscope:package`
	n int
}

// A method is its own declaration, and narrowing the type does not reach it.
func (b helperBox) helperSize() int { return b.n } // want `method helperBox.helperSize takes package scope`

// order.go reads x through helperMakePair without spelling the type, so
// narrowing the type would narrow x and start a boundary report. The type
// stays, and y, which nothing outside reads, is reported on its own.
type helperPair struct {
	x int
	y int // want `field helperPair.y takes package scope from the file's //declscope:package`
}

func helperMakePair() helperPair { return helperPair{} }

var _ = helperLocal() + helperA + helperB + helperD + helperBox{}.helperSize() + helperMakePair().y
