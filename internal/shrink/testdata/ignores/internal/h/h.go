// Package h is imported only by c/c.go. Its field is used outside only by
// c/tagged.go, which is behind a build constraint and imports nothing: it
// reads the Held that c.go holds. Without tagged.go, the field is reported
// with its fix withheld, since c.go may hand tagged.go a Held, so the ignore
// is needed there.
package h

type Held struct {
	//declscope:ignore overexported
	Mark int
}

func Hold() Held { return Held{} }
