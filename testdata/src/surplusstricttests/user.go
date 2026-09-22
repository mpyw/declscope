package surplusstricttests

// note's only outside use is in integration_test.go, a namespace of its own.
// The test variant sees it and stays quiet. The ordinary variant does not see
// every file, so the rule switches off there instead of reporting.
//
//declscope:package
type userCard struct {
	id   int
	note int
}
