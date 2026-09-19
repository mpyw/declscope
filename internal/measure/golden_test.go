//declscope:core

package measure

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites the goldens instead of comparing against them. The rendered
// tables are the command's interface, so they are reviewed as a diff rather
// than asserted line by line in Go.
var update = flag.Bool("update", false, "rewrite the golden files")

// TestGoldens pins every format of both reports.
//
// The markdown golden is where the diagram is pinned, including the arrow
// precedence on an edge that carries two states: without a fixed precedence
// the same package would draw differently from one run to the next.
func TestGoldens(t *testing.T) {
	pkg := fixturePackage()
	summary := fixtureSummary()

	for _, tc := range []struct {
		name   string
		render func(*bytes.Buffer) error
	}{
		{"inspect.text", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatText) }},
		{"inspect.json", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatJSON) }},
		{"inspect.md", func(b *bytes.Buffer) error { return pkg.WriteFormat(b, FormatMarkdown) }},
		{"survey.text", func(b *bytes.Buffer) error { return summary.WriteSummaryFormat(b, FormatText) }},
		{"survey.json", func(b *bytes.Buffer) error { return summary.WriteSummaryFormat(b, FormatJSON) }},
		{"survey.md", func(b *bytes.Buffer) error { return summary.WriteSummaryFormat(b, FormatMarkdown) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := tc.render(&buf); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join("testdata", tc.name+".golden")
			if *update {
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
// rendering, never a different data set. Every number it prints has to exist
// in the JSON of the same model, so the check is that the counts markdown
// spells are the ones the JSON carries.
func TestMarkdownIsDerivableFromJSON(t *testing.T) {
	pkg := fixturePackage()

	var md, js bytes.Buffer
	if err := pkg.WriteMarkdown(&md); err != nil {
		t.Fatal(err)
	}
	if err := pkg.WriteJSON(&js); err != nil {
		t.Fatal(err)
	}
	for _, decl := range []string{"writeFlag", "Flags.parsed", "compName", "parseAll"} {
		if !bytes.Contains(js.Bytes(), []byte(decl)) {
			t.Errorf("the JSON does not name %s, which the model holds", decl)
		}
	}
	// An open crossing is left out of both the table and the diagram, and
	// carried in the JSON, where a consumer can still find it.
	if bytes.Contains(md.Bytes(), []byte("| (core) → flags |")) && !bytes.Contains(md.Bytes(), []byte("open")) {
		t.Error("markdown lists an open crossing without saying that it is open")
	}
	if !bytes.Contains(js.Bytes(), []byte(`"state": "open"`)) {
		t.Error("the JSON drops the open crossing the markdown leaves out")
	}
}
