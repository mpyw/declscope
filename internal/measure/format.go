package measure

import (
	"fmt"
	"io"
	"strings"
)

// Format is how a report is rendered. One flag with three values rather than a
// -json boolean beside a -markdown one: the three are one choice, and a
// rejected value can then be answered with the values the flag accepts, the
// way a rejected rules.naming.qualify is.
type Format string

const (
	// FormatMarkdown is the default: GitHub-flavoured tables, and a diagram
	// where one fits. Its cells are padded, so it reads in a terminal as an
	// aligned table and renders in an issue, a pull request or a README as
	// the same table.
	//
	// There was a third format, plain aligned text, and keeping the two in
	// step turned out to be the most common defect in this package: a note
	// added to one, a count spelled by hand in one, a line printed after an
	// early return in the other. One renderer cannot disagree with itself.
	//
	// It is a rendering and never a different data set. Anything it shows has
	// to be derivable from the JSON of the same run; a number that exists only
	// in markdown is a bug.
	FormatMarkdown Format = "markdown"

	// FormatJSON is for an agent and for anything scripted. It is the
	// interface the adoption skill reads.
	FormatJSON Format = "json"
)

// FormatSet is the values the flag accepts, in the order an error names them.
var FormatSet = []Format{FormatMarkdown, FormatJSON}

// ParseFormat resolves the flag value, and names what it takes when it cannot.
func ParseFormat(s string) (Format, error) {
	for _, f := range FormatSet {
		if Format(s) == f {
			return f, nil
		}
	}
	names := make([]string, 0, len(FormatSet))
	for _, f := range FormatSet {
		names = append(names, string(f))
	}
	return "", fmt.Errorf("unknown format %q: this flag takes %s", s, strings.Join(names, ", "))
}

// WriteFormat renders one package in the chosen format. It is the one entry:
// the renderers are reached through it, so that a caller cannot pick one and
// bypass the choice the flag records.
func (p Package) WriteFormat(w io.Writer, f Format) error {
	if f == FormatJSON {
		return p.writeJSON(w)
	}
	return p.writeMarkdown(w)
}

// WriteFormat renders a whole run in the chosen format.
func (s Summary) WriteFormat(w io.Writer, f Format) error {
	if f == FormatJSON {
		return s.writeJSON(w)
	}
	return s.writeMarkdown(w)
}
