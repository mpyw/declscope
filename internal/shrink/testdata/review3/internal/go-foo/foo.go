// Package foo lives in go-foo, so an import of it with no name of its own
// binds foo, not go-foo.
package foo

func Foo() {}
