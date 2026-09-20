package measure

import "testing"

// TestCrossingsFoldsEachDirectionSeparately pins the shape the crossing table
// rests on: a mutual pair stays two rows, each carrying its own counts, and
// both are marked. Folding them would lose the asymmetry that says one
// namespace holds the working parts of the other.
func TestCrossingsFoldsEachDirectionSeparately(t *testing.T) {
	got := fixturePackage().Crossings()

	byPair := map[[2]string]Crossing{}
	for _, c := range got {
		byPair[[2]string{c.From, c.To}] = c
	}
	forward, ok := byPair[[2]string{"completion", "flags"}]
	if !ok {
		t.Fatalf("the crossing table lost a pair: %+v", got)
	}
	back := byPair[[2]string{"flags", "completion"}]
	if !forward.Mutual || !back.Mutual {
		t.Error("a mutual pair is not marked on both rows")
	}
	if forward.Reached == back.Reached && forward.Uses == back.Uses {
		t.Error("the two directions were folded into one count")
	}
	if forward.Declarations != 5 {
		t.Errorf("reached divides by %d, want the reached namespace's own declaration count", forward.Declarations)
	}
}

// TestCrossingsLeaveOpenOut checks that a crossing nobody was asked about is
// neither a row nor part of a ratio, and is counted where the note reads it.
func TestCrossingsLeaveOpenOut(t *testing.T) {
	pkg := fixturePackage()
	for _, c := range pkg.Crossings() {
		if c.From == "completion" && c.To == "(core)" {
			t.Error("an open crossing was listed as a row")
		}
	}
	if got := pkg.DeclarationsCrossing(EdgeOpen); got != 1 {
		t.Errorf("OpenCrossings = %d, want 1", got)
	}
}

// TestCrossingsAreHeaviestFirst pins the order the whole report reads by: the
// first row is where to look, and breadth decides it rather than depth.
func TestCrossingsAreHeaviestFirst(t *testing.T) {
	got := fixturePackage().Crossings()
	for i := 1; i < len(got); i++ {
		if got[i-1].Reached < got[i].Reached {
			t.Errorf("row %d reaches fewer declarations than row %d", i-1, i)
		}
	}
}
