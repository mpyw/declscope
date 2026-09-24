package directivestricttests

// userShared restates the default. order_test.go uses it from another
// namespace, which only the test variant sees.
//
//declscope:private
func userShared() int { return 1 }
