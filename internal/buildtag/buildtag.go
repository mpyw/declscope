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
// linux.go is not. Only the part before the first dot is read, so
// foo_linux.pb.go is constrained and foo.bar_linux.go is not.
func Constrained(name string, f *ast.File) bool {
	stem, _, _ := strings.Cut(filepath.Base(name), ".")
	_, suffixed := TrimOSArch(stem)
	return suffixed || constrainedByLine(f)
}

// TrimOSArch removes from stem the _GOOS_GOARCH, _GOOS or _GOARCH suffix that
// go/build reads as a build constraint, and reports whether there was one.
// stem is a file name cut at its first dot. It mirrors go/build's
// goodOSArchFile: a trailing _test is looked past and kept, and only the part
// after the first underscore is read, so a name that is all suffix constrains
// nothing.
func TrimOSArch(stem string) (string, bool) {
	i := strings.Index(stem, "_")
	if i < 0 {
		return stem, false
	}
	rest, test := strings.CutSuffix(stem[i:], "_test")
	l := strings.Split(rest, "_")
	n := len(l)
	var drop int
	switch {
	case n >= 2 && knownOS[l[n-2]] && knownArch[l[n-1]]:
		drop = len("_"+l[n-2]) + len("_"+l[n-1])
	case knownOS[l[n-1]] || knownArch[l[n-1]]:
		drop = len("_" + l[n-1])
	default:
		return stem, false
	}
	trimmed := stem[:i+len(rest)-drop]
	if test {
		trimmed += "_test"
	}
	return trimmed, true
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

// Mirror KnownOS and KnownArch in internal/syslist, which is not importable.
// TestKnownLists checks them against the Go that runs the tests.
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
	"mips64p32le": true, "ppc": true, "ppc64": true, "ppc64le": true,
	"riscv": true, "riscv64": true, "s390": true, "s390x": true, "sparc": true,
	"sparc64": true, "wasm": true,
}
