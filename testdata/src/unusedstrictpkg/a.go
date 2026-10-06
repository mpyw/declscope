package unusedstrictpkg

// Under defaults.unexported: shared the private field narrows it, and is kept.
type implicit struct {
	//declscope:private
	x int
}

// A shared directive on an unexported declaration restates the default.
//
//declscope:shared // want `unused //declscope:shared on helper: it already has shared scope`
func helper() int { return 1 }

// So does a type's, and a field's restating it.
//
//declscope:shared // want `unused //declscope:shared on shape: it already has shared scope`
type shape struct {
	//declscope:shared // want `unused //declscope:shared on shape.side: it already has shared scope`
	side int
}

var _ = implicit{}.x + helper() + shape{}.side
