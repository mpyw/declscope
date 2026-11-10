package nsnormalize

// Any separator yields a lowerCamelCase namespace, so the suggested name is
// an identifier rather than foo-barBaz.
func baz() int { return 1 } // want `func baz does not carry namespace "fooBar" anywhere in its name; give it a name that carries "fooBar", such as fooBarBaz, or move it to the file it belongs to`

// Used from Foo.go, which is namespace foo: the two stems are distinct
// identities, and the boundary is reported here at the declaration.
func fooBarOK() int { return baz() } // want `func fooBarOK is private to namespace "fooBar", but is used from namespace "foo"`
