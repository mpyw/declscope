package b

import "example.com/typeparamsets/internal/a"

type U struct{ X, Y int }

type PtrTo[T any] interface{ *T }

func Named[P interface{ a.Named }]() []P { return []P{P{1, 2}, {3, 4}} }

func Converted[P interface{ a.Converted }](u U) P { return P(u) }

func Narrow[P interface {
	~*a.Narrow | ~*a.Wide
	~*a.Narrow
}]() []P {
	return []P{{1, 2}}
}

func Via[P interface{ PtrTo[a.Via] | *a.Via }]() []P { return []P{{3, 4}} }

func Twins[P interface{ a.Left | a.Right }]() P { return P{1, 2} }
