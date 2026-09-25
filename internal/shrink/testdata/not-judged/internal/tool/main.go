// Package main is never judged: -buildmode=plugin looks its exported symbols
// up.
package main

func Plugged() {}

func main() {}
