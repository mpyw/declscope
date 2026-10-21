//go:build !never

package c

// This file imports nothing, and reads h through the variable c.go declares.
var _ = held.Mark
