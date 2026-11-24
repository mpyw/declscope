// Command w sits in a vendor directory below the module root. ./... leaves
// it out, and the go command vendors nothing there, but go build
// ./internal/a/vendor/w compiles it.
package main

import (
	"fmt"

	"example.com/skipped/internal/a"
)

func main() { fmt.Println(a.FromVendor()) }
