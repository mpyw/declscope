package surplusstrictopaque

// note's only outside use is in gen.go, which the analysis excludes as a
// reference site. The package holds a source the rule cannot read, so the
// rule switches off for it.
//
//declscope:package
type userCard struct {
	id   int
	note int
}
