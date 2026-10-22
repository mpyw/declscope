package typeparamsets

func buildNamed[P interface{ Named }]() []P { return []P{P{1}, {2}} }

func buildNarrow[P interface {
	~*Narrow | ~*Wide
	~*Narrow
}]() []P {
	return []P{{1}}
}

func buildVia[P interface{ PtrTo[Via] | *Via }]() []P { return []P{{1}} }

func buildTwins[P interface{ Left | Right }]() P { return P{1} }

type buildMirror struct{ count int }

func buildConverted[P interface{ Converted }](u buildMirror) P { return P(u) }

var (
	_ = buildNamed[Named]
	_ = buildNarrow[*Narrow]
	_ = buildVia[*Via]
	_ = buildTwins[Left]
	_ = buildConverted[Converted]
)
