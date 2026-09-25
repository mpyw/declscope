package t_test

// go vet resolves an example against the packages the test imports.
import _ "example.com/more/internal/t"

func ExampleShown() {}
