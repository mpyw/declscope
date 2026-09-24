package unusedstrict

// A type that declares no name is reached through its fields alone. Its
// private restates the default, and its only field states package, so the
// report says the field states its own scope.
//
//declscope:private // want `unused //declscope:private: every declaration it reaches states its own scope`
type _ struct {
	//declscope:package
	x int
}
