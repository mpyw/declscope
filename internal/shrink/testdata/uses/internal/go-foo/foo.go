// Package foo lives in go-foo, so an import of it with no name of its own
// binds foo. Foo is used under that name by a build-excluded file.
package foo

func Foo() {}
