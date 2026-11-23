package ignoreunseensurplus

// Nothing outside this file uses userHelper, in any build. A run with
// -tags integration reads every file, so surplus judges the directive, and
// the ignore silences its report. A run without the tag leaves
// extra_integration.go out, and that file alone stands surplus down for the
// package. The ignore is kept there for the run that reads the file, so
// neither run reports anything. The silence is the assertion.
//
//declscope:shared
//declscope:ignore surplus
func userHelper() int { return 1 }

var _ = userHelper
