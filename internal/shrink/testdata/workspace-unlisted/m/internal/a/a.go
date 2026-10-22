package a

// Used is what the nested module tools uses.
func Used() {}

func Unused() {} // want: func Unused is exported, but nothing outside example.com/wu/internal/a uses it
