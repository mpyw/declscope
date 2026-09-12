package nsnormalize

// A PascalCase stem is lowered, so the suggested name stays unexported rather
// than becoming FooBar.
func bar() int { return 1 } // want `func bar does not carry the prefix of namespace "foo"; rename it to fooBar`

func fooOK() int { return bar() + fooBarOK() }
