package aliases

// al is reachable: the directive in alias.go widened it. base.n is not: the
// alias never reached it. entry is not: inline.go narrowed it.
func useIt(x al, e entry) int { return x.n + len(e.key) }

var _ = useIt
