package ignorescope

func orderRun() int {
	return helper() + helper2() + kept() + userTypo() + helper3()
}

var _ = orderRun
