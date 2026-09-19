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
	// FormatText is aligned tables for a terminal.
	FormatText Format = "text"

	// FormatJSON is for an agent and for anything scripted. It is the
	// interface the adoption skill reads.
	FormatJSON Format = "json"

	// FormatMarkdown is for pasting into an issue, a pull request, a README
	// or an article: GitHub-flavoured tables, and a diagram where one fits.
	//
	// It is a rendering and never a different data set. Anything it shows has
	// to be derivable from the JSON of the same run; a number that exists only
	// in markdown is a bug.
	FormatMarkdown Format = "markdown"
)

// FormatSet is the values the flag accepts, in the order an error names them.
var FormatSet = []Format{FormatText, FormatJSON, FormatMarkdown}

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

// WriteFormat renders one package in the chosen format.
func (p Package) WriteFormat(w io.Writer, f Format) error {
	switch f {
	case FormatJSON:
		return p.WriteJSON(w)
	case FormatMarkdown:
		return p.WriteMarkdown(w)
	default:
		return p.WriteText(w)
	}
}

// WriteSummaryFormat renders a whole run in the chosen format.
func (s Summary) WriteSummaryFormat(w io.Writer, f Format) error {
	switch f {
	case FormatJSON:
		return s.WriteSummaryJSON(w)
	case FormatMarkdown:
		return s.WriteSummaryMarkdown(w)
	default:
		return s.WriteSummaryText(w)
	}
}
