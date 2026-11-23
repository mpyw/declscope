//go:build integration

package ignoreunseensurplus

// The build leaves this file out without -tags integration. It writes a
// name of the package, so surplus stands down in the run that leaves it out.
func extraRun() int { return 2 }

var _ = extraRun
