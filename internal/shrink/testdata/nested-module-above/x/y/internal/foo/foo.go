package foo

// Hello is what the nested module tools uses. Its path, example.com/nan/x,
// lies above this package's internal/ parent, example.com/nan/x/y, so its
// package example.com/nan/x/y/z may import this one.
func Hello() string { return "hi" }

func Bye() {} // want: func Bye is exported, but nothing outside example.com/nan/x/y/internal/foo uses it
