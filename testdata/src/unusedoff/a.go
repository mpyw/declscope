package unusedoff

// Under rules.unused: off nothing is reported unused. Not an ignore that
// silences nothing,
//
//declscope:ignore boundary
func aIgnore() int { return 1 }

// nor a scope directive no configuration could make bind,
//
//declscope:package
func AExported() int { return 2 }

// nor one naming the scope the declaration would have without it.
type aImplicit struct {
	//declscope:private
	x int
}

// The directive rule has no switch, so a malformed directive is still
// reported.
//
//declscope:bogus // want `unknown directive declscope:bogus`
func aTypo() int { return 3 }

var _ = aIgnore() + AExported() + aImplicit{}.x + aTypo()
