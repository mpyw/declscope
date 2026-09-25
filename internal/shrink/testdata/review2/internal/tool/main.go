// Package main is looked up by a plugin host, so its exported variable hands
// out a.Plug though the package sits under internal/.
package main

import "example.com/review2/internal/a"

var V a.Plug

var _ = a.Answered() + a.FileAnswered()

func main() {}
