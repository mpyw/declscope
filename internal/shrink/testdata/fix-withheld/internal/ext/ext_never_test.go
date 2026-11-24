//go:build never

package ext_test

import (
	"fmt"

	_ "example.com/withheld/internal/ext" // go vet looks the example names up in it
)

func ExampleGizmo() {
	fmt.Println("gizmo")
	// Output: gizmo
}

func ExampleRun_twice() {}

// Example names the package itself, and so no declaration.
func Example() {}

type helper struct{}

// A method named like an example is no example.
func (helper) ExampleSpare() {}
