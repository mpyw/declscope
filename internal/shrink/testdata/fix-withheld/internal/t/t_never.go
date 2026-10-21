//go:build never

package t

func w() string { return v.Method() + string(rune(v.Field)) }
