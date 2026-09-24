package fixunusedstrictsurplus

// aHelper's directive restates the default. surplus reports it too, and the
// deletion settles both.
//
//declscope:package // want `unused //declscope:package on aHelper: it already has package scope` `//declscope:package on aHelper: no use from another namespace is visible to declscope`
func aHelper() int { return 1 }

var _ = aHelper()
