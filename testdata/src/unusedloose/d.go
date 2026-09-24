package unusedloose

// A type that declares no name is reached through its fields alone. DWide
// takes the type's package, which it has anyway as an exported name, and
// dNarrow states private. The report must not say that every field states
// its own scope: DWide states none.
//
//declscope:package // want `unused //declscope:package: nothing it reaches takes a scope$`
type _ struct {
	DWide int
	//declscope:private
	dNarrow int
}

// Its only field takes the type's package, so the directive reaches a checked
// declaration and the report must not say that none carries it.
//
//declscope:package // want `unused //declscope:package: nothing it reaches takes a scope$`
type _ struct {
	DOnly int
}
