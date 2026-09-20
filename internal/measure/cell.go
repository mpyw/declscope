// cell.go holds the shared Markdown cell formatters. A dash that means "not
// asked" has to mean that in every table, and a table that spelled its own
// would be free to disagree about what a missing answer looks like.
//
// The file is shared on purpose, so it says so once rather than declaration by
// declaration.
//
//declscope:package

package measure

import (
	"fmt"
	"strings"
)

// cellCount prints a number, or a dash where the question was not asked. The
// two are different answers, and a zero can only give one of them.
func cellCount(n int, asked bool) string {
	if !asked {
		return "-"
	}
	return fmt.Sprint(n)
}

// cellKeyable prints a dash where no baseline could ever suppress the rule,
// which is the directive rule and the filter rule. A zero there would read as
// "suppressible, and none suppressed".
func cellKeyable(n int, keyable bool) string {
	if !keyable {
		return "-"
	}
	return fmt.Sprint(n)
}

func cellYes(b bool) string {
	if b {
		return "yes"
	}
	return ""
}

func cellOnOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func cellTypeCheck(t TypeCheck) string {
	return fmt.Sprintf("%s ok, %d failed", cellPackages(t.Packages-len(t.Failed)), len(t.Failed))
}

func cellPackages(n int) string { return cellPlural(n, "package", "packages") }

func cellPlural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// cellTable renders a padded markdown table: the header, the alignment row,
// then the data, every cell widened to its column so that the markdown a
// reader pastes into an issue is also the table they can read in the terminal
// they produced it in. GitHub renders padded and unpadded cells the same.
//
// A column is right-aligned when every value in it is one — a count, a ratio,
// or a dash — and left-aligned otherwise, so figures line up under each other
// while names and file lists start where the eye looks for them.
func cellTable(rows [][]string) []string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			for len(widths) <= i {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len([]rune(cell)))
		}
	}

	numeric := make([]bool, len(widths))
	for i := range numeric {
		numeric[i] = i > 0 && cellColumnIsNumeric(rows[1:], i)
	}

	out := make([]string, 0, len(rows)+1)
	for n, row := range rows {
		line := ""
		for i, cell := range row {
			pad := strings.Repeat(" ", widths[i]-len([]rune(cell)))
			if numeric[i] {
				line += "| " + pad + cell + " "
			} else {
				line += "| " + cell + pad + " "
			}
		}
		out = append(out, line+"|")
		if n == 0 {
			out = append(out, cellRule(widths, numeric))
		}
	}
	return out
}

// cellColumnIsNumeric reports whether every value in a column is a figure: a
// count, a "3 of 7" ratio, or the dash that stands for a question not asked.
func cellColumnIsNumeric(rows [][]string, i int) bool {
	for _, row := range rows {
		if i >= len(row) {
			continue
		}
		cell := strings.TrimSpace(row[i])
		if cell == "" || cell == "-" {
			continue
		}
		for _, word := range strings.Fields(strings.ReplaceAll(cell, "of", " ")) {
			if strings.TrimLeft(word, "0123456789") != "" {
				return false
			}
		}
	}
	return true
}

// cellRule is the alignment row, widened like the cells above it and carrying
// the same alignment.
func cellRule(widths []int, numeric []bool) string {
	line := ""
	for i, width := range widths {
		dashes := strings.Repeat("-", max(width, 3))
		if numeric[i] {
			line += "| " + dashes[:len(dashes)-1] + ": "
		} else {
			line += "| " + dashes + " "
		}
	}
	return line + "|"
}
