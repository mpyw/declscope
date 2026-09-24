package measure

import (
	"bytes"
	"strings"
	"testing"
)

// TestCellTableAlignsARaggedColumn checks a table whose rows are not all as
// long as the header. A missing cell neither breaks the table nor stops the
// column it belongs to from reading as numeric, so the figures still align
// right.
func TestCellTableAlignsARaggedColumn(t *testing.T) {
	got := cellTable([][]string{
		{"name", "count"},
		{"a"},
		{"b", "12"},
	})
	want := []string{
		"| name | count |",
		"| ---- | ----: |",
		"| a    |",
		"| b    |    12 |",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestMarkdownNamesARuleSwitchedOff checks that a rule the config turned off
// reads as off in the checks in force, rather than disappearing.
func TestMarkdownNamesARuleSwitchedOff(t *testing.T) {
	var buf bytes.Buffer
	s := Summary{Checks: Checks{Configs: []ConfigUse{{
		Packages: 1, Boundary: false, Surplus: "loose", Unused: "off", Qualify: "never",
	}}}}
	if err := s.writeMarkdown(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "boundary off, qualify never, surplus loose, unused off") {
		t.Errorf("the rules row does not read boundary off:\n%s", buf.String())
	}
}
