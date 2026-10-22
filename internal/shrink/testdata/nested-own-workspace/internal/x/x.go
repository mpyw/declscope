package x

// Foo is what the nested module n uses, through its own go.work.
func Foo() {}

func Bar() {} // want: func Bar is exported, but nothing outside example.com/now/internal/x uses it
