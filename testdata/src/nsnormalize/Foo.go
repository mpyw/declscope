package nsnormalize

// A PascalCase stem is lowered, so the suggested name stays unexported rather
// than becoming FooBar.
func bar() int { return 1 } // want `func bar does not carry namespace "foo" anywhere in its name; move it to the file it belongs to, or give it a name that carries "foo" \(fooBar is one candidate, not necessarily the best\)`

func fooOK() int { return bar() + fooBarOK() }
