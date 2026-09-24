package directivestrictpkg

// Under defaults.unexported: package the private field narrows it, and is kept.
type implicit struct {
	//declscope:private
	x int
}

// A package directive on an unexported declaration restates the default.
//
//declscope:package // want `unused //declscope:package on helper: it already has package scope`
func helper() int { return 1 }

// So does a type's, and a field's restating it.
//
//declscope:package // want `unused //declscope:package on shape: it already has package scope`
type shape struct {
	//declscope:package // want `unused //declscope:package on shape.side: it already has package scope`
	side int
}

var _ = implicit{}.x + helper() + shape{}.side
