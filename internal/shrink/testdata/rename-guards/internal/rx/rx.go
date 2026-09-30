// Package rx has a build-excluded file of its own, so it is apart from r:
// that file holds back every field and method of its package.
package rx

// The unexported name is written by the build-excluded file.
func Clash() {} // want: func Clash is exported.*no fix: a build-excluded file of its package writes the unexported name
