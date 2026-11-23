// Command asmprog reads a.FromAssembly from assembly only. ./... leaves it
// out, and go build compiles it when it is named.
package main

import (
	"fmt"

	_ "example.com/skipped/internal/a" // links the symbol the assembly reads
)

func main() { fmt.Println(read()) }
