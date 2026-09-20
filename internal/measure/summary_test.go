package measure

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/rule"
)

// TestSummaryRanksByWhatIsUndecided pins the order the first question is
// answered in. A baselined finding is deferred rather than settled, so a
// package that deferred everything ranks with one that decided nothing —
// which is the package worth opening, and the one the analyzer alone makes
// look clean.
func TestSummaryRanksByWhatIsUndecided(t *testing.T) {
	deferred := Package{
		Path:     "example.com/deferred",
		Findings: map[rule.Rule]Count{rule.Boundary: {Asked: true, Keyable: true, Found: 30, Baselined: 30}},
	}
	settled := Package{
		Path:     "example.com/settled",
		Findings: map[rule.Rule]Count{rule.Boundary: {Asked: true, Keyable: true, Found: 2, Reported: 2}},
		Edges: []Edge{
			{From: "a", To: "b", Declaration: "x", State: EdgeDeclared},
			{From: "c", To: "b", Declaration: "x", State: EdgeDeclared},
		},
	}
	got := SummaryOf([]Package{settled, deferred}, Checks{})

	if got.Rows[0].Package != deferred.Path {
		t.Errorf("first row is %q, want the package with thirty deferred findings", got.Rows[0].Package)
	}
	if n := got.Rows[1].BoundaryDeclared; n != 1 {
		t.Errorf("declared counts %d, want 1: one declaration shared with two namespaces is one decision", n)
	}
	if n := got.Totals[rule.Boundary].Found; n != 32 {
		t.Errorf("the totals summed to %d, want 32", n)
	}
}

// TestSummaryAsksIfAnyPackageAsked checks the totals line for a rule in force
// in one package and inert in another. Reporting the total as not asked would
// hide the packages it did answer for.
func TestSummaryAsksIfAnyPackageAsked(t *testing.T) {
	got := SummaryOf([]Package{
		{Path: "a", Findings: map[rule.Rule]Count{rule.Qualify: {Keyable: true}}},
		{Path: "b", Findings: map[rule.Rule]Count{rule.Qualify: {Asked: true, Keyable: true, Found: 1, Reported: 1}}},
	}, Checks{})

	if !got.Totals[rule.Qualify].Asked {
		t.Error("the qualify total reads as not asked, though one package asked it")
	}
	for _, row := range got.Rows {
		if row.Package == "a" && row.QualifyAsked {
			t.Error("a package the rule is inert in reads as having been asked")
		}
	}
}

// TestSummaryKeepsBoundaryAskedPerPackage checks that a package where the rule
// was disabled cannot be rendered as a clean zero merely because another
// package in the run did ask it.
func TestSummaryKeepsBoundaryAskedPerPackage(t *testing.T) {
	got := SummaryOf([]Package{
		{Path: "unchecked", Findings: map[rule.Rule]Count{rule.Boundary: {Keyable: true}}},
		{Path: "checked", Findings: map[rule.Rule]Count{rule.Boundary: {Asked: true, Keyable: true}}},
	}, Checks{})

	for _, row := range got.Rows {
		if row.Package == "unchecked" && row.BoundaryAsked {
			t.Error("a package where boundary was disabled reads as checked")
		}
		if row.Package == "checked" && !row.BoundaryAsked {
			t.Error("a package where boundary ran reads as unchecked")
		}
	}

	var markdown bytes.Buffer
	if err := got.writeMarkdown(&markdown); err != nil {
		t.Fatal(err)
	}
	foundBoundaryRow := false
	for _, line := range strings.Split(markdown.String(), "\n") {
		cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
		if len(cells) != 5 || strings.TrimSpace(cells[0]) != "`unchecked`" {
			continue
		}
		foundBoundaryRow = true
		for _, cell := range cells[1:4] {
			if strings.TrimSpace(cell) != "-" {
				t.Errorf("unchecked package is rendered as clean boundary counts:\n%s", line)
			}
		}
		break
	}
	if !foundBoundaryRow {
		t.Fatal("no boundary row for the unchecked package")
	}

	var encoded bytes.Buffer
	if err := got.writeJSON(&encoded); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Packages []struct {
			Package  string `json:"package"`
			Boundary struct {
				Asked bool `json:"asked"`
			} `json:"boundary"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(encoded.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range decoded.Packages {
		if pkg.Package == "unchecked" && pkg.Boundary.Asked {
			t.Error("JSON says boundary was asked for the unchecked package")
		}
	}
}
