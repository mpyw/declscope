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
	s := Summary{checks: Checks{Configs: []ConfigUse{{
		Packages: 1, Boundary: false, Surplus: "loose", Unused: "off", Qualify: "never",
	}}}}
	if err := s.writeMarkdown(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "boundary off, qualify never, surplus loose, unused off") {
		t.Errorf("the rules row does not read boundary off:\n%s", buf.String())
	}
}

// TestMarkdownEscapesAPipeInACell checks a file name holding a "|", which Go
// accepts. GitHub splits a cell at one even inside backticks, so an unescaped
// one adds a column to the namespace table, and to the survey's config and
// baseline rows.
func TestMarkdownEscapesAPipeInACell(t *testing.T) {
	var pkg bytes.Buffer
	p := Package{
		Path:       "example.com/x",
		Namespaces: []Namespace{{Name: "aB", Files: []string{"a|b.go"}, Declarations: 1}},
	}
	if err := p.writeMarkdown(&pkg); err != nil {
		t.Fatal(err)
	}
	if want := "| aB        | `a\\|b.go` |            1 | -               |"; !strings.Contains(pkg.String(), want) {
		t.Errorf("the namespace row is not escaped, want %q:\n%s", want, pkg.String())
	}

	var survey bytes.Buffer
	s := Summary{checks: Checks{
		Configs:   []ConfigUse{{Chain: []string{"a|b/.declscope.yaml"}, Packages: 1, Qualify: "never"}},
		Baselines: []BaselineUse{{Path: "a|b/.declscope-baseline.yaml", Entries: 1}},
	}}
	if err := s.writeMarkdown(&survey); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"`a\\|b/.declscope.yaml`", "`a\\|b/.declscope-baseline.yaml`"} {
		if !strings.Contains(survey.String(), want) {
			t.Errorf("the survey does not carry %q:\n%s", want, survey.String())
		}
	}
}

// TestCellTableAlignsOnlyFigures checks which columns read as numeric. A
// column of dashes and empty cells holds no figure, so it stays left-aligned
// rather than taking its alignment from a rule that was not asked; and a cell
// of exactly "of" is a word, not a ratio.
func TestCellTableAlignsOnlyFigures(t *testing.T) {
	got := cellTable([][]string{
		{"name", "dash", "word", "ratio"},
		{"a", "-", "of", "3 of 7"},
		{"b", "", "of", "-"},
	})
	want := []string{
		"| name | dash | word |  ratio |",
		"| ---- | ---- | ---- | -----: |",
		"| a    | -    | of   | 3 of 7 |",
		"| b    |      | of   |      - |",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestJSONCarriesAnEmptyConfig checks that a package no config file governs
// says so with an empty array, as the survey's chain does, rather than by
// leaving the key out.
func TestJSONCarriesAnEmptyConfig(t *testing.T) {
	var buf bytes.Buffer
	if err := fixtureQuietPackage().writeJSON(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"config": [],`) {
		t.Errorf("the package JSON does not carry an empty config:\n%s", buf.String())
	}
}
