package testonlyignore

// userHelper is used from order_test.go, in namespace "order", and from
// nowhere else. Only the test variant of the package sees that reference, so
// only it can tell whether the directive did anything; the ordinary variant,
// which cannot see order_test.go, must not call it unused. Neither variant
// reports it: the test variant because it was used, the ordinary one because
// it defers to the test variant.
//
//declscope:ignore escape
func userHelper() int { return 1 }
