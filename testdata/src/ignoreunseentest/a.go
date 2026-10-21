package ignoreunseentest

// The only test file is b_tagged_test.go, which the build leaves out, so the
// package has no test variant. The non-test variant reads it for its names,
// as for any excluded file, and judges the ignores.

// aHelper is crossed into only by b_tagged_test.go. A configuration that
// reads that file reports the crossing, and the ignore answers it there, so
// it is not unused here.
//
//declscope:ignore boundary // b_tagged_test.go calls it behind the never tag
func aHelper() int { return 1 }

// No unseen file names aStale, so its ignore silences nothing anywhere.
//
//declscope:ignore boundary // want `unused //declscope:ignore boundary on aStale`
func aStale() int { return 2 }

func A() int { return aHelper() + aStale() }
