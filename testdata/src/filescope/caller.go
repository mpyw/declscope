package filescope

func callerRun() int {
	_ = utilMust(nil)
	return utilFirst([]int{1}) + utilCache
}

var _ = callerRun
