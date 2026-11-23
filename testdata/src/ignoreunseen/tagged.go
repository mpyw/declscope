//go:build never

package ignoreunseen

// This file is left out of the build. It crosses into a.go's declarations.
func Tagged() int { return aHelper() + aBare() + aQualify() + filelevelHelper() }

// Unkeyed literals fill the fields by position.
func Positional() (aPoint, aAlias) { return aPoint{1}, aAlias{2} }

// The same, through a named pointer type, a defined type and a constraint.
func Indirect() ([]aPSlot, aDerived) { return []aPSlot{{1}}, aDerived{2} }

func Constrained[P aBoxes]() P { return P{1} }

// Inner literals that leave out their type, inside a named slice, a named
// map's key and a named slice of slices of pointers.
func Elided() (aItems, aKeyed, aGrid) { return aItems{{1}}, aKeyed{{1}: true}, aGrid{{{1}}} }
