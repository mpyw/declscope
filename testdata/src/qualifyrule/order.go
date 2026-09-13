package qualifyrule

// A second namespace, which is what gives the prefixes something to distinguish.
// It deliberately touches nothing in namespace "user", so the only rule under
// test here is the naming rule.
func orderRun() int { return 1 }

var _ = orderRun
