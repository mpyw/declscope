// cell.go holds the cell formatters every renderer needs. They belong to none
// of the three: a dash that means "not asked" has to mean that in a terminal,
// in an issue and in a table pasted into a README, and a renderer that spelled
// its own would be free to disagree with the others about what a missing
// answer looks like.
//
// The file is shared on purpose, so it says so once rather than declaration by
// declaration.
//
//declscope:package

package measure

import "fmt"

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
