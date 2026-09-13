package unqualifyinert

// A second namespace, which makes ondemand require the prefix and so makes
// unqualify inert. It touches nothing in namespace "user".
func orderRun() int { return 2 }

var _ = orderRun
