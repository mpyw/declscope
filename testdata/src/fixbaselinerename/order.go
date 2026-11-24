package fixbaselinerename

// The baseline records this method, so it stays quiet.
func (item) help() int { return count }

var _ = item{}.help
