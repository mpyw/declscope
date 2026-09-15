package wideningtests

// The only use is in other_test.go. The test variant sees it and stays quiet;
// the ordinary variant does not see every file, so the rule switches off there
// instead of reporting.
//
//declscope:package
func userHelp() int { return 1 }
