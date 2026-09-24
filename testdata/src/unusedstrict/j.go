package unusedstrict

// A type that declares no name, one field taking its private and one stating
// package. The private restates the default, so strict reports it. The report
// must not say that every field states its own scope: jTaken states none.
//
//declscope:private // want `unused //declscope:private: every declaration it reaches states its own scope or already has private scope$`
type _ struct {
	jTaken int
	//declscope:package
	jWidened int
}
