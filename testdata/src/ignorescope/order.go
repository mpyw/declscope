package ignorescope

func orderRun() int {
	return helper() + helper2() + userKept() + userTypo() + helper3() + userBlockA + userBlockB
}

var _ = orderRun
