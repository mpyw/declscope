package toolchainunqualify

import "fmt"

// Dropping the prefix here would spell Load, which is taken, and would stop
// being an example at all.
func ExampleLoad() {
	fmt.Println(Load())
	// Output: 1
}
