// Command prog is a package of the module that ./... leaves out. A test of a
// builds it with go run ./testdata/prog.
package main

import (
	"fmt"

	"example.com/skipped/internal/a"
)

func main() { fmt.Println(a.Greeting(), a.New()) }
