package surplusgenerator

// gen.go is a generator of another package, which the build excludes. It
// declares a userHelper of its own, not this one, so the directive is still
// reported.
//
//declscope:shared // want `//declscope:shared on userHelper: no use from another namespace is visible to declscope`
func userHelper() int { return 1 }
