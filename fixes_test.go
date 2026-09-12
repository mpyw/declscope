package declscope_test

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"

	"github.com/mpyw/declscope"
)

func TestSuggestedFixes(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixes")
}

// TestSuggestedFixInlineStruct checks the directive fix for a field that
// shares its line with the struct type, which has to be broken onto a line of
// its own for the directive to bind to it.
func TestSuggestedFixInlineStruct(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixinline")
}

// TestSuggestedFixPromote checks the rename offered by the label rule. It
// never changes a declaration's reach, so it cannot conflict with the
// directive fix the way the old name-driven rename could.
func TestSuggestedFixPromote(t *testing.T) {
	analysistest.RunWithSuggestedFixes(t, analysistest.TestData(), declscope.Analyzer, "fixpromote")
}
