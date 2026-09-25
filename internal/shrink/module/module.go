// Package module loads the main module for declscope shrink: every package
// with its tests, every file the build left out, and every nested module,
// and says which internal packages have importers this run can all see.
package module

import (
	"bytes"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/shrink/excluded"
)

// Module is every package of the main module, loaded with its tests, and
// what the build left out of it.
type Module struct {
	// path and dir are the module's import path and root directory.
	path, dir string
	Fset      *token.FileSet
	// Pkgs is every variant go/packages returned: a package, the package
	// with its in-package tests, and its external test package, each
	// separately. Paths lists their import paths in order.
	Pkgs  []*packages.Package
	Paths []string
	// Excluded is every Go file of the module the build did not compile.
	Excluded []*excluded.File

	byPath map[string][]*packages.Package
	// nested is the module path of every go.mod below the root. Go checks
	// internal/ by import path, so a nested module whose path extends an
	// internal parent may import the package, and this run never loads it.
	// One with an unrelated path cannot, wherever it sits.
	nested []string
}

// Load loads every package of the main module holding dir.
//
// A load error refuses the whole run. A package that does not type-check
// yields no references, which reads exactly like nothing using a declaration.
func Load(dir string) (*Module, error) {
	root, modPath, err := mainModule(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Dir: root,
		// No NeedDeps: dependencies come from export data. Only the module's
		// own packages are judged, and a generic declared outside the module
		// has no body to read either way.
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedForTest | packages.NeedModule,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	m := &Module{path: modPath, dir: root, byPath: map[string][]*packages.Package{}}
	var failed []string
	for _, p := range pkgs {
		if isTestMain(p) {
			continue
		}
		if len(p.Errors) > 0 {
			failed = append(failed, fmt.Sprintf("%s: %s", p.PkgPath, p.Errors[0]))
			continue
		}
		if m.Fset == nil {
			m.Fset = p.Fset
		}
		m.Pkgs = append(m.Pkgs, p)
		if _, ok := m.byPath[p.PkgPath]; !ok {
			m.Paths = append(m.Paths, p.PkgPath)
		}
		m.byPath[p.PkgPath] = append(m.byPath[p.PkgPath], p)
	}
	if len(failed) > 0 {
		slices.Sort(failed)
		return nil, fmt.Errorf("these packages do not type-check, so no absence of a use means anything:\n  %s",
			strings.Join(slices.Compact(failed), "\n  "))
	}
	if m.Fset == nil {
		return nil, fmt.Errorf("no package found in the module at %s", root)
	}
	slices.Sort(m.Paths)
	for _, path := range m.Paths {
		slices.SortStableFunc(m.byPath[path], func(a, b *packages.Package) int {
			return len(b.Syntax) - len(a.Syntax)
		})
	}
	if err := m.walk(); err != nil {
		return nil, err
	}
	return m, nil
}

// Variants returns the loaded variants of the package at path, the widest
// first: the one with the most files, which holds every file the others do.
func (m *Module) Variants(path string) []*packages.Package { return m.byPath[path] }

// Widest returns one variant per import path, the widest. The variants share
// their parsed files, so reading the others would find nothing new.
func (m *Module) Widest() []*packages.Package {
	out := make([]*packages.Package, 0, len(m.Paths))
	for _, path := range m.Paths {
		out = append(out, m.byPath[path][0])
	}
	return out
}

// Range returns the import path of the tree allowed to import the package at
// path, or why this run cannot load every package that tree could hold.
//
// The deepest internal element is the one that restricts: a/internal/b/
// internal/c is importable only from a/internal/b. The tree must lie inside
// the module, and no nested module may have a path inside it: that module
// could import the package, and this run never loads it. The reason names
// that module, since nothing else would lead a reader to it.
func (m *Module) Range(path string) (parent, why string) {
	parent, ok := InternalParent(path)
	if !ok {
		return "", "not inside internal/, so another module may import it"
	}
	if parent != m.path && !strings.HasPrefix(parent, m.path+"/") {
		return "", fmt.Sprintf("its internal/ parent %s lies above the module %s", parent, m.path)
	}
	for _, n := range m.nested {
		if n == parent || strings.HasPrefix(n, parent+"/") {
			return "", fmt.Sprintf("the nested module %s may import it, and this run does not load it", n)
		}
	}
	return parent, ""
}

// InternalParent returns the import path above the deepest internal element
// of path, and whether there is one.
func InternalParent(path string) (string, bool) {
	elems := strings.Split(path, "/")
	for i := len(elems) - 1; i >= 0; i-- {
		if elems[i] == "internal" {
			return strings.Join(elems[:i], "/"), true
		}
	}
	return "", false
}

// Wanted resolves the patterns, from dir, to the import paths they name.
func Wanted(dir string, patterns []string) (map[string]bool, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	pkgs, err := packages.Load(&packages.Config{Dir: dir, Mode: packages.NeedName}, patterns...)
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, p := range pkgs {
		wanted[p.PkgPath] = true
	}
	return wanted, nil
}

// mainModule asks the go command for the module holding dir.
func mainModule(dir string) (root, path string, err error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}\t{{.Path}}")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("finding the module: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 1 {
		// A workspace lists every module it uses. Which one to shrink would be
		// a guess, and the others may import this one's internal packages.
		return "", "", fmt.Errorf("the go command reports %d main modules; run declscope shrink with GOWORK=off", len(lines))
	}
	root, path, ok := strings.Cut(lines[0], "\t")
	if !ok || root == "" {
		return "", "", fmt.Errorf("not inside a module")
	}
	return root, path, nil
}

// isTestMain identifies the synthesized main package of a test binary, which
// has nothing of its own to judge.
func isTestMain(p *packages.Package) bool {
	return p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") &&
		!slices.ContainsFunc(p.GoFiles, func(f string) bool { return strings.HasSuffix(f, ".go") && filepath.Base(f) != "_testmain.go" })
}

// walk reads every Go file no loaded package holds, and every nested module.
//
// A package whose files are all excluded under this GOOS is not in the load
// at all, so reading only IgnoredFiles would miss it. Files are read only
// where ./... reads them: not under testdata, a directory starting with . or
// _, the vendor directory at the root, or a nested module.
//
// The walk itself goes everywhere but .git, since it is also looking for
// go.mod files. A nested module in testdata/tools or _tools/gen is no part
// of this module's build, but it can import the module's internal packages
// all the same, at any depth.
func (m *Module) walk() error {
	compiled := map[string]bool{}
	for _, p := range m.Pkgs {
		for _, f := range slices.Concat(p.GoFiles, p.CompiledGoFiles) {
			compiled[f] = true
		}
	}
	fset := token.NewFileSet()
	// unread holds the directories whose Go files ./... does not read.
	var unread []string
	under := func(path string) bool {
		return slices.ContainsFunc(unread, func(dir string) bool {
			return strings.HasPrefix(path, dir+string(filepath.Separator))
		})
	}
	return filepath.WalkDir(m.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == m.dir {
				return nil
			}
			name := d.Name()
			if name == ".git" {
				return filepath.SkipDir
			}
			if data, err := os.ReadFile(filepath.Join(path, "go.mod")); err == nil {
				modPath := modfile.ModulePath(data)
				if modPath == "" {
					return fmt.Errorf("reading %s: no module path", filepath.Join(path, "go.mod"))
				}
				m.nested = append(m.nested, modPath)
				unread = append(unread, path)
				return nil
			}
			rootVendor := name == "vendor" && filepath.Dir(path) == m.dir
			if name == "testdata" || rootVendor || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") {
				unread = append(unread, path)
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || compiled[path] || under(path) {
			return nil
		}
		f, err := excluded.Read(fset, path)
		if err != nil {
			return fmt.Errorf("reading %s, which the build excludes: %w", path, err)
		}
		m.Excluded = append(m.Excluded, f)
		return nil
	})
}
