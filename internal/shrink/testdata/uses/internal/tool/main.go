// Package main is an exposure root though it sits under internal/, since a
// plugin host looks its exported symbols up. It is itself never judged.
package main

import "example.com/uses/internal/plug"

var V plug.Plug

func main() {}
