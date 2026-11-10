package qualifyinflected

// string ends in -ing, but str holds no vowel, so it is not read as an
// inflected form and the message has nothing to add.
func quote() int { return 3 } // want `func quote does not carry namespace "string" anywhere in its name; move it to the file it belongs to, or give it a name that carries "string" \(stringQuote is one candidate, not necessarily the best\)$`

var _ = quote
