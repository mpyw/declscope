//declscope:package

package filescope

// A file whose whole contents are meant to be package-wide says so in one line,
// with a scope rather than an ignore. The difference is endorsing rather than
// suppressing: these declarations *are* package, so the label rule still reaches
// them, and one that turns out not to be shared can be narrowed back below.
func utilMust(e error) error { return e }

func utilFirst(s []int) int { return s[0] }

// A declaration may still state its own scope, which outranks the file's. Under
// an ignore this line would have meant nothing at all.
//
//declscope:private
var utilCache int // want `var utilCache is declared private by //declscope:private, but is used from namespace "caller"`
