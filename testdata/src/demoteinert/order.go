package demoteinert

// A second namespace, which makes ondemand require the label and so makes
// demote inert. It touches nothing in namespace "user".
func orderRun() int { return 2 }

var _ = orderRun
