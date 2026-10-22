package i

// The doc and the trailing comment of a member are one binding, whatever
// the type: an interface's method and an alias struct type's field are no
// candidates here, but the ignore naming unused still answers the doc's
// report, as it does for the analyzer.
type iface interface {
	//declscope:ignore overexported
	M() //declscope:ignore unused
}

type aliased = struct {
	//declscope:ignore overexported
	F int //declscope:ignore unused
}

type fielded struct {
	//declscope:ignore overexported
	F int //declscope:ignore unused
}
