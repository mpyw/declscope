package surplusstricttests

// Only the test variant sees this file, so only it can report here, and it
// does: a want in a _test.go file is checked in that variant alone.
//
//declscope:package
type userFixture struct {
	name  string
	extra int // want `field userFixture.extra takes package scope from //declscope:package on userFixture, but no use from another namespace is visible to declscope`
}

func userFixtureExtra(f userFixture) int { return f.extra }

var _ = userFixtureExtra
