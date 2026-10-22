package y

// Unused lies in a range that no copy of t has a path in, so it is judged.
func Unused() {} // want: func Unused is exported, but nothing outside example.com/nbl/a/internal/y uses it
