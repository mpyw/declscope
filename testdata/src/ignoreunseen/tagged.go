//go:build never

package ignoreunseen

// This file is left out of the build. It crosses into a.go's declarations.
func Tagged() int { return aHelper() + aBare() + aQualify() + filelevelHelper() }

// Unkeyed literals fill the fields by position.
func Positional() (aPoint, aAlias) { return aPoint{1}, aAlias{2} }
