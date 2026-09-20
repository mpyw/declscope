package measure

import (
	"bytes"
	"strings"
	"testing"
)

// TestParseFormatNamesWhatItTakes checks that a rejected value is answered
// with the values the flag accepts, rather than with a bare failure.
func TestParseFormatNamesWhatItTakes(t *testing.T) {
	for _, want := range FormatSet {
		if got, err := ParseFormat(string(want)); err != nil || got != want {
			t.Errorf("ParseFormat(%q) = %q, %v", want, got, err)
		}
	}
	_, err := ParseFormat("yaml")
	if err == nil {
		t.Fatal("an unknown format was accepted")
	}
	for _, want := range []string{"yaml", "text", "json", "markdown"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not mention %q: %v", want, err)
		}
	}
}

// TestWriteFormatDispatches checks each format reaches its own renderer, by a
// mark only that renderer writes.
func TestWriteFormatDispatches(t *testing.T) {
	for format, mark := range map[Format]string{
		FormatText:     "Namespaces ",
		FormatJSON:     `"namespaces"`,
		FormatMarkdown: "## Namespaces",
	} {
		var buf bytes.Buffer
		if err := fixturePackage().WriteFormat(&buf, format); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), mark) {
			t.Errorf("%s did not render as itself:\n%s", format, buf.String())
		}
	}
}
