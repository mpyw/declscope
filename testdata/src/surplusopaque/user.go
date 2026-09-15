package surplusopaque

// The only use is in gen.go, which the analysis excludes as a reference site.
// The package holds a source the rule cannot read, so the rule switches off
// rather than read the absence as evidence.
//
//declscope:package
func userMagic() int { return 7 }
