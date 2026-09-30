// Package buildtag answers which files a build configuration may leave out:
// the ones behind a build constraint, written as a //go:build line or as a
// GOOS or GOARCH suffix of the file name.
package buildtag

import (
	"go/ast"
	"go/build/constraint"
	"path/filepath"
	"strings"
)

// Constrained reports whether the file named name, parsed as f, is behind a
// build constraint, so that some configuration leaves it out. The name is
// read the way go/build reads it: name_GOOS.go, name_GOARCH.go and
// name_GOOS_GOARCH.go are constrained, each with an optional _test, and
// linux.go is not.
func Constrained(name string, f *ast.File) bool {
	return suffixed(filepath.Base(name)) || constrainedByLine(f)
}

// KnownOS reports whether s is a GOOS go/build recognizes in a file name.
func KnownOS(s string) bool { return knownOS[s] }

// KnownArch reports whether s is a GOARCH go/build recognizes in a file name.
func KnownArch(s string) bool { return knownArch[s] }

// suffixed mirrors go/build's goodOSArchFile: only the part after the first
// underscore is read, so a name that is all suffix constrains nothing.
func suffixed(base string) bool {
	base = strings.TrimSuffix(base, ".go")
	i := strings.Index(base, "_")
	if i < 0 {
		return false
	}
	l := strings.Split(base[i:], "_")
	if n := len(l); n > 0 && l[n-1] == "test" {
		l = l[:n-1]
	}
	n := len(l)
	if n >= 2 && KnownOS(l[n-2]) && KnownArch(l[n-1]) {
		return true
	}
	return n >= 1 && (KnownOS(l[n-1]) || KnownArch(l[n-1]))
}

// constrainedByLine reports whether a //go:build or // +build line comes
// before the package clause, where the go command reads one.
func constrainedByLine(f *ast.File) bool {
	for _, g := range f.Comments {
		if g.Pos() >= f.Package {
			break
		}
		for _, c := range g.List {
			if constraint.IsGoBuild(c.Text) || constraint.IsPlusBuild(c.Text) {
				return true
			}
		}
	}
	return false
}

// Mirrors the lists in go/internal/syslist, which is not importable.
var knownOS = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true,
	"freebsd": true, "hurd": true, "illumos": true, "ios": true, "js": true,
	"linux": true, "nacl": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "wasip1": true, "windows": true, "zos": true,
}

var knownArch = map[string]bool{
	"386": true, "amd64": true, "amd64p32": true, "arm": true, "armbe": true,
	"arm64": true, "arm64be": true, "loong64": true, "mips": true,
	"mipsle": true, "mips64": true, "mips64le": true, "mips64p32": true,
	"mips64p32le": true, "mips8": true, "ppc": true, "ppc64": true,
	"ppc64le": true, "riscv": true, "riscv64": true, "s390": true,
	"s390x": true, "sparc": true, "sparc64": true, "wasm": true,
}
