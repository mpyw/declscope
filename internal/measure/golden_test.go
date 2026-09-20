package measure

import (
	"bytes"
	"cmp"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mpyw/declscope/internal/rule"
)

// update rewrites the goldens instead of comparing against them. The rendered
// tables are the command's interface, so they are reviewed as a diff rather
// than asserted line by line in Go.
var goldenUpdate = flag.Bool("update", false, "rewrite the golden files")

// TestGoldens pins every format of both reports.
//
// The markdown golden is where the diagram is pinned, including the arrow
// precedence on an edge that carries two states: without a fixed precedence
// the same package would draw differently from one run to the next.
func TestGoldens(t *testing.T) {
	pkg := fixturePackage()
	summary := fixtureSummary()
	unchecked := fixtureUncheckedPackage()
	quiet := fixtureQuietPackage()

	for _, tc := range []struct {
		name   string
		render func(*bytes.Buffer) error
	}{
		{"inspect.text", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatText) }},
		{"inspect.json", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatJSON) }},
		{"inspect.md", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatMarkdown) }},
		{"survey.text", func(b *bytes.Buffer) error { return summary.WriteFormat(b, FormatText) }},
		{"survey.json", func(b *bytes.Buffer) error { return summary.WriteFormat(b, FormatJSON) }},
		{"survey.md", func(b *bytes.Buffer) error { return summary.WriteFormat(b, FormatMarkdown) }},
		// A package where both rules were switched off, and one where every
		// file joined the core: between them they reach the branches that
		// print a dash, the note about an unchecked crossing, and the two
		// "nothing to report" paths.
		{"unchecked.text", func(b *bytes.Buffer) error { return unchecked.WriteFormat(b, FormatText) }},
		{"unchecked.md", func(b *bytes.Buffer) error { return unchecked.WriteFormat(b, FormatMarkdown) }},
		{"quiet.text", func(b *bytes.Buffer) error { return quiet.WriteFormat(b, FormatText) }},
		{"quiet.md", func(b *bytes.Buffer) error { return quiet.WriteFormat(b, FormatMarkdown) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.render(&buf); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tc.name+".golden")
			if *goldenUpdate {
				if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := buf.String(); got != string(want) {
				t.Errorf("%s does not match the golden:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
			}
		})
	}
}

// TestMarkdownIsDerivableFromJSON holds markdown to what it claims to be: a
// rendering, never a different data set.
//
// The check that matters is the dash. Where a renderer prints one it is saying
// the rule was not asked, and a consumer reading the JSON has to be able to
// reach the same conclusion — which it could not while the package JSON
// carried no findings at all, and printed qualifyTargets as a live number for
// a rule that never ran.
func TestMarkdownIsDerivableFromJSON(t *testing.T) {
	for _, pkg := range []Package{fixturePackage(), fixtureUncheckedPackage(), fixtureQuietPackage()} {
		t.Run(pkg.Path, func(t *testing.T) {
			var md, js bytes.Buffer
			if err := pkg.WriteMarkdown(&md); err != nil {
				t.Fatal(err)
			}
			if err := pkg.WriteJSON(&js); err != nil {
				t.Fatal(err)
			}

			var decoded struct {
				Findings map[string]struct {
					Asked    bool `json:"asked"`
					Reported int  `json:"reported"`
				} `json:"findings"`
				Edges []struct {
					State string `json:"state"`
				} `json:"edges"`
			}
			if err := json.Unmarshal(js.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}

			for _, r := range []rule.Rule{rule.Boundary, rule.Qualify} {
				want := pkg.Findings[r].Asked
				if got := decoded.Findings[string(r)].Asked; got != want {
					t.Errorf("the JSON says %s asked=%v, the model says %v", r, got, want)
				}
			}
			// Markdown says so in words where the rule was not asked, and the
			// JSON has to agree rather than leave a count to be misread.
			saysNotAsked := bytes.Contains(md.Bytes(), []byte("Not asked"))
			if saysNotAsked == decoded.Findings["qualify"].Asked {
				t.Errorf("markdown says not-asked=%v while the JSON says asked=%v",
					saysNotAsked, decoded.Findings["qualify"].Asked)
			}

			// What markdown leaves out on purpose is still in the JSON.
			open := 0
			for _, e := range decoded.Edges {
				if e.State == string(EdgeOpen) {
					open++
				}
			}
			if open != pkg.OpenCrossings() {
				t.Errorf("the JSON carries %d open crossings, the model holds %d", open, pkg.OpenCrossings())
			}
		})
	}
}

// TestMostReachedHonoursItsLimit pins the truncation, which no golden reaches.
func TestMostReachedHonoursItsLimit(t *testing.T) {
	pkg := fixturePackage()
	if got := len(pkg.MostReached(2)); got != 2 {
		t.Errorf("MostReached(2) returned %d rows", got)
	}
	if all, none := len(pkg.MostReached(0)), len(pkg.MostReached(99)); all != none || all < 3 {
		t.Errorf("MostReached(0) returned %d rows and MostReached(99) returned %d", all, none)
	}
}

// TestSortedIsDeterministic pins the order the renderers and the goldens rely
// on, and that it is taken on a copy.
func TestSortedIsDeterministic(t *testing.T) {
	pkg := fixturePackage()
	slices.Reverse(pkg.Edges)
	slices.Reverse(pkg.Namespaces)

	sorted := pkg.Sorted()
	if !sorted.Namespaces[0].Core {
		t.Error("the core namespace does not sort first")
	}
	if !slices.IsSortedFunc(sorted.Edges, func(a, b Edge) int {
		return cmp.Or(cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To), cmp.Compare(a.Declaration, b.Declaration))
	}) {
		t.Error("edges are not in order")
	}
	if pkg.Namespaces[0].Core {
		t.Error("Sorted reordered the value it was called on")
	}
}
