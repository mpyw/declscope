package nsnormalize

// Any separator yields a lowerCamelCase namespace, so the suggested name is
// an identifier rather than foo-barBaz.
func baz() int { return 1 } // want `func baz does not carry the prefix of namespace "fooBar"; rename it to fooBarBaz`

// Used from Foo.go, which is namespace foo: the two stems are distinct
// identities, and the boundary is reported here at the declaration.
func fooBarOK() int { return baz() } // want `func fooBarOK is file-private to namespace "fooBar", but is used from namespace "foo"`
