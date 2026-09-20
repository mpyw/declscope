package measure

import "testing"

// TestWorstQualifiedNamesTheWork checks that the namespace a package row names
// is where the work is, with the ratio beside it rather than deciding it.
//
// Ratio first was tried and measured: on go/printer it named math 2 of 2 while
// nodes 56 of 57 was the whole job, and on encoding/json it named doc 2 of 2.
// A fully saturated two-declaration file is a real shape and a trivial amount
// of work, and the first screen of a report should not point at it.
func TestWorstQualifiedNamesTheWork(t *testing.T) {
	pkg := Package{
		Namespaces: []Namespace{
			{Name: "small", QualifyTargets: 2},
			{Name: "large", QualifyTargets: 30},
		},
		Names: []NameFinding{
			{Namespace: "small", Declaration: "a", State: NameReported},
			{Namespace: "small", Declaration: "b", State: NameReported},
			{Namespace: "large", Declaration: "c", State: NameReported},
			{Namespace: "large", Declaration: "d", State: NameReported},
			{Namespace: "large", Declaration: "e", State: NameBaselined},
		},
	}
	worst, ok := pkg.WorstQualified()
	if !ok {
		t.Fatal("no worst namespace where two of them fail")
	}
	if worst.Namespace != "large" {
		t.Errorf("worst is %q, want large: three failing names are more work than two", worst.Namespace)
	}
	if worst.Saturation() != 3 || worst.Targets != 30 {
		t.Errorf("worst reads %d of %d, want 3 of 30 — the ratio is reported, not the ranking",
			worst.Saturation(), worst.Targets)
	}
}

// TestWorstQualifiedBreaksTiesByShare checks the other half: between two
// namespaces holding the same amount of work, the saturated one is named,
// since that is the one a single rename can clear.
func TestWorstQualifiedBreaksTiesByShare(t *testing.T) {
	pkg := Package{
		Namespaces: []Namespace{
			{Name: "whole", QualifyTargets: 2},
			{Name: "part", QualifyTargets: 30},
		},
		Names: []NameFinding{
			{Namespace: "whole", Declaration: "a", State: NameReported},
			{Namespace: "whole", Declaration: "b", State: NameReported},
			{Namespace: "part", Declaration: "c", State: NameReported},
			{Namespace: "part", Declaration: "d", State: NameReported},
		},
	}
	worst, ok := pkg.WorstQualified()
	if !ok || worst.Namespace != "whole" {
		t.Errorf("worst is %q, want whole: two of two is cleared by renaming the file", worst.Namespace)
	}
}

// TestWorstQualifiedSkipsWhatWasNotAsked checks the two namespaces that have
// no ratio at all: the core, which the rule asks nothing of, and one with no
// targets, where a zero would read as "asked and satisfied".
func TestWorstQualifiedSkipsWhatWasNotAsked(t *testing.T) {
	pkg := Package{
		Namespaces: []Namespace{
			{Name: "(core)", Core: true, QualifyTargets: 4},
			{Name: "quiet", QualifyTargets: 0},
		},
		Names: []NameFinding{{Namespace: "(core)", Declaration: "a", State: NameReported}},
	}
	if worst, ok := pkg.WorstQualified(); ok {
		t.Errorf("WorstQualified named %q, where nothing was asked", worst.Namespace)
	}
}

// TestQualifyRowsCountExemptApart checks that a name a directive excused is
// not counted as failing: the two are different answers, and the exempt column
// exists so a reader can tell them apart.
func TestQualifyRowsCountExemptApart(t *testing.T) {
	rows := fixturePackage().QualifyRows()
	for _, row := range rows {
		if row.Namespace != "completion" {
			continue
		}
		if row.Exempt != 1 {
			t.Errorf("completion has %d exempt names, want 1", row.Exempt)
		}
		if row.Saturation() != row.Baselined+row.Reported {
			t.Error("an exempt name was counted as failing")
		}
		return
	}
	t.Fatalf("no row for completion: %+v", rows)
}
