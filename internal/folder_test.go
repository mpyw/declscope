package internal

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"golang.org/x/tools/go/analysis"
)

// TestListFolderFollowsChanges pins that a shared listing and a shared file
// read are kept only while the folder and the file keep their state (#300).
// The analyzer may live in a process that outlasts edits, so an added file or
// a changed one must be read again, never answered from the old state.
func TestListFolderFollowsChanges(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string, mtime time.Time) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mtime, mtime); err != nil {
			t.Fatal(err)
		}
	}
	// Distinct times, so the test does not depend on the file system's
	// resolution.
	base := time.Now().Add(-time.Hour)
	stampDir := func(at time.Time) {
		t.Helper()
		if err := os.Chtimes(dir, at, at); err != nil {
			t.Fatal(err)
		}
	}
	names := func(path string) []string {
		t.Helper()
		l, err := listFolder(dir)
		if err != nil {
			t.Fatal(err)
		}
		f, err := l.readFolderFile(path)
		if err != nil {
			t.Fatal(err)
		}
		got, err := l.readFolderFileNames(path, f)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	write("a.go", "package p\n\nfunc before() {}\n", base)
	stampDir(base)
	a := filepath.Join(dir, "a.go")
	if got := names(a); !slices.Contains(got, "before") {
		t.Fatalf("names = %v; want before", got)
	}

	write("b.go", "package p\n", base)
	stampDir(base.Add(time.Minute))
	l, err := listFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(l.goFiles, []string{"a.go", "b.go"}) {
		t.Errorf("goFiles = %v; want the added file listed", l.goFiles)
	}

	// The same size, so only the modification time tells the two apart.
	write("a.go", "package p\n\nfunc after_() {}\n", base.Add(time.Minute))
	if got := names(a); !slices.Contains(got, "after_") || slices.Contains(got, "before") {
		t.Errorf("names = %v; want the changed file read again", got)
	}

	if err := os.Remove(a); err != nil {
		t.Fatal(err)
	}
	if _, err := l.readFolderFile(a); err == nil {
		t.Error("a removed file was answered from the old read")
	}
}

// TestListFolderShared pins that passes asking about one folder at once share
// a single listing and a single read of each file, and agree on it.
func TestListFolderShared(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.go")
	if err := os.WriteFile(path, []byte("package p\n\nfunc a() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	const passes = 16
	files := make([]*folderFile, passes)
	var wg sync.WaitGroup
	for i := range passes {
		wg.Go(func() {
			l, err := listFolder(dir)
			if err != nil {
				t.Error(err)
				return
			}
			f, err := l.readFolderFile(path)
			if err != nil {
				t.Error(err)
				return
			}
			if _, err := l.readFolderFileNames(path, f); err != nil {
				t.Error(err)
			}
			files[i] = f
		})
	}
	wg.Wait()
	for _, f := range files[1:] {
		if f != files[0] {
			t.Fatal("the file was read more than once")
		}
	}
	if files[0].pkg != "p" || !slices.Contains(files[0].names, "a") {
		t.Errorf("pkg = %q, names = %v", files[0].pkg, files[0].names)
	}
}

// BenchmarkUnseen scans a folder of 3,000 build-excluded files of the package
// and 3,000 generators of package main from each variant one run analyzes
// (#300): the non-test variant, which stops at the test file it does not see,
// the test variant, which reads every excluded file, and the external test
// package, which reads only package clauses. The shared listing is forgotten
// before each iteration, as at the start of a run.
func BenchmarkUnseen(b *testing.B) {
	dir := b.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	write("a.go", "package p\n\nfunc a() {}\n")
	write("a_test.go", "package p\n\nfunc testA() { a() }\n")
	write("b_test.go", "package p_test\n")
	var ignored []string
	for i := range 3000 {
		excluded, generator := fmt.Sprintf("x%d.go", i), fmt.Sprintf("g%d.go", i)
		write(excluded, fmt.Sprintf("//go:build ignore\n\npackage p\n\nfunc x%d() { a() }\n", i))
		write(generator, fmt.Sprintf("//go:build ignore\n\npackage main\n\nfunc main() { println(%d) }\n", i))
		ignored = append(ignored, filepath.Join(dir, excluded), filepath.Join(dir, generator))
	}
	passOf := func(pkg string, names ...string) *analysis.Pass {
		fset := token.NewFileSet()
		var files []*ast.File
		for _, name := range names {
			f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
			if err != nil {
				b.Fatal(err)
			}
			files = append(files, f)
		}
		return &analysis.Pass{Fset: fset, Files: files, IgnoredFiles: ignored, Pkg: types.NewPackage(pkg, pkg)}
	}
	plain, test, xtest := passOf("p", "a.go"), passOf("p", "a.go", "a_test.go"), passOf("p_test", "b_test.go")
	for b.Loop() {
		forgetFolderListings()
		if u := (&collection{}).unseen(plain); !u.all {
			b.Fatal("the non-test variant read on past the test file")
		}
		if u := (&collection{}).unseen(test); u.all || len(u.names) == 0 {
			b.Fatalf("all = %v, %d names; want the excluded files read", u.all, len(u.names))
		}
		if u := (&collection{}).unseen(xtest); u.all || len(u.names) > 0 {
			b.Fatalf("all = %v, %d names; want nothing read", u.all, len(u.names))
		}
	}
}

// forgetFolderListings empties the listings every pass shares.
func forgetFolderListings() {
	folderMu.Lock()
	clear(folderListings)
	folderMu.Unlock()
}
