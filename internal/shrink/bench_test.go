package shrink_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mpyw/declscope/internal/shrink"
)

// The benchmarks write a synthetic module, each shaped so that judging grows
// with the candidates times another count (#217). Loading the module is part
// of every run, and the same in both, so what changes between them is the
// judgment.

// BenchmarkShrinkExcluded judges 3,000 exported funcs in a module with 1,500
// build-excluded files in other packages, a few of which import the judged
// one.
func BenchmarkShrinkExcluded(b *testing.B) {
	files := map[string]string{}
	for i := range 30 {
		var src strings.Builder
		src.WriteString("package a\n")
		for j := range 100 {
			fmt.Fprintf(&src, "\nfunc F%d() {}\n", i*100+j)
		}
		files[fmt.Sprintf("internal/a/a%d.go", i)] = src.String()
	}
	for i := range 50 {
		files[fmt.Sprintf("internal/x/x%d/doc.go", i)] = fmt.Sprintf("package x%d\n", i)
		for j := range 30 {
			n := i*30 + j
			src := fmt.Sprintf("//go:build never\n\npackage x%d\n\nimport \"strings\"\n\nvar _ = strings.ToUpper(\"\")\n", i)
			if n%150 == 0 {
				src = fmt.Sprintf("//go:build never\n\npackage x%d\n\nimport \"example.com/bench/internal/a\"\n\nvar _ = a.F%d\n", i, n)
			}
			files[fmt.Sprintf("internal/x/x%d/f%d.go", i, j)] = src
		}
	}
	benchShrinkRun(b, files)
}

// BenchmarkShrinkStructs judges 600 exported structs of 10 exported fields
// each, every field a member the rename guards ask about.
func BenchmarkShrinkStructs(b *testing.B) {
	files := map[string]string{}
	for i := range 60 {
		var src strings.Builder
		src.WriteString("package s\n")
		for j := range 10 {
			fmt.Fprintf(&src, "\ntype S%d struct {\n", i*10+j)
			for k := range 10 {
				fmt.Fprintf(&src, "\tField%d int\n", k)
			}
			src.WriteString("}\n")
		}
		files[fmt.Sprintf("internal/s/s%d.go", i)] = src.String()
	}
	benchShrinkRun(b, files)
}

// BenchmarkShrinkNestedModules loads 40 nested modules, each importing the
// judged package and holding 6 build-excluded files (#301). Each nested
// module is loaded on its own, so this measures wall time spent on the go
// command rather than the judgment.
func BenchmarkShrinkNestedModules(b *testing.B) {
	var src strings.Builder
	src.WriteString("package a\n")
	files := map[string]string{}
	for i := range 40 {
		fmt.Fprintf(&src, "\nfunc F%d() {}\n", i)
		dir := fmt.Sprintf("tools/t%d/", i)
		files[dir+"go.mod"] = fmt.Sprintf("module example.com/bench/tools/t%d\n\ngo 1.24\n\nrequire example.com/bench v0.0.0\n\nreplace example.com/bench => ../..\n", i)
		files[dir+"main.go"] = fmt.Sprintf("package main\n\nimport \"example.com/bench/internal/a\"\n\nfunc main() { a.F%d() }\n", i)
		for j := range 6 {
			files[fmt.Sprintf("%sx%d.go", dir, j)] = fmt.Sprintf("//go:build never\n\npackage main\n\nimport \"example.com/bench/internal/a\"\n\nvar _ = a.F%d\n", i)
		}
	}
	files["internal/a/a.go"] = src.String()
	benchShrinkRun(b, files)
}

// benchShrinkRun writes files into a new module and runs shrink on it.
func benchShrinkRun(b *testing.B, files map[string]string) {
	b.Helper()
	dir := b.TempDir()
	files["go.mod"] = "module example.com/bench\n\ngo 1.24\n"
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			b.Fatal(err)
		}
	}
	dir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		b.Fatal(err)
	}
	var findings int
	for b.Loop() {
		res, err := shrink.Run(dir, nil, nil)
		if err != nil {
			b.Fatal(err)
		}
		findings = len(res.Findings)
	}
	b.ReportMetric(float64(findings), "findings")
}
