package qualifyrule

// A second namespace, which is what gives the labels something to distinguish.
// It deliberately touches nothing in namespace "user", so the only rule under
// test here is the label rule.
func orderRun() int { return 1 }

var _ = orderRun
