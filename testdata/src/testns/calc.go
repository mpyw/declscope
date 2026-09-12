package testns

// helper is file-private to namespace "calc". A _test.go file shares its
// subject's namespace, so using it from calc_test.go is not a violation.
func helper() int { return 1 }

func calcShared() int { return helper() }
