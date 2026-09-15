package nsnormalize

// A PascalCase stem is lowered, so the suggested name stays unexported rather
// than becoming FooBar.
func bar() int { return 1 } // want `func bar does not carry namespace "foo" anywhere in its name; rename it to fooBar, or to another name that carries "foo"`

func fooOK() int { return bar() + fooBarOK() }
