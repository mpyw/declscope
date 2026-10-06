package fixunusedstrictsurplus

// aHelper's directive restates the default. surplus reports it too, and the
// deletion settles both.
//
//declscope:shared // want `unused //declscope:shared on aHelper: it already has shared scope` `//declscope:shared on aHelper: no use from another namespace is visible to declscope`
func aHelper() int { return 1 }

var _ = aHelper()
