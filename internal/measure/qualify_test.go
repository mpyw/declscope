package measure

import "testing"

// TestWorstQualifiedComparesRatios checks that the namespace a package row
// names is the one the rule is least satisfied by, as a share rather than a
// count: a namespace failing two of two is worse than one failing three of
// thirty, and naming the larger count would send the reader to the wrong file.
func TestWorstQualifiedComparesRatios(t *testing.T) {
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
	if worst.Namespace != "small" {
		t.Errorf("worst is %q, want small: 2 of 2 is worse than 3 of 30", worst.Namespace)
	}
	if worst.Saturation() != 2 {
		t.Errorf("saturation counts %d, want both reported findings", worst.Saturation())
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
