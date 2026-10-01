package buildtag

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestConstrained(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want bool
	}{
		{"user.go", "package p\n", false},

		// A GOOS or GOARCH suffix, as go/build reads it.
		{"run_linux.go", "package p\n", true},
		{"run_amd64.go", "package p\n", true},
		{"run_linux_amd64.go", "package p\n", true},
		{"run_linux_test.go", "package p\n", true},
		{"a/b/run_windows.go", "package p\n", true},

		// Only the part after the first underscore is read.
		{"linux.go", "package p\n", false},
		{"linux_test.go", "package p\n", false},
		{"amd64_linux.go", "package p\n", true},
		{"run_linuxish.go", "package p\n", false},

		// A constraint line before the package clause.
		{"gui.go", "//go:build production || dev\n\npackage p\n", true},
		{"gui.go", "// +build production\n\npackage p\n", true},
		{"gui.go", "// Package p is doc.\n//go:build !never\n\npackage p\n", true},

		// After the package clause, the go command reads none.
		{"gui.go", "package p\n\n//go:build never\n", false},
		{"gui.go", "// go:build never\n\npackage p\n", false},
	}
	for _, tt := range tests {
		f, err := parser.ParseFile(token.NewFileSet(), tt.name, tt.src, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		if got := Constrained(tt.name, f); got != tt.want {
			t.Errorf("Constrained(%q, %q) = %v, want %v", tt.name, tt.src, got, tt.want)
		}
	}
}
