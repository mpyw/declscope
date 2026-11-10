package qualifyinflected

// string ends in -ing, but str holds no vowel, so it is not read as an
// inflected form and the message has nothing to add.
func quote() int { return 3 } // want `func quote does not carry namespace "string" anywhere in its name; give it a name that carries "string", such as stringQuote, or move it to the file it belongs to$`

var _ = quote
