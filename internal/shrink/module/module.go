// Package module loads the main modules for declscope shrink: every package
// with its tests, every file the build left out, and every nested module,
// and says which internal packages have importers this run can all see.
//
// There is one main module, or several in a workspace. Every one is loaded,
// since each may import another's internal packages.
package module

import (
	"bytes"
	"errors"
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

	"github.com/mpyw/declscope/internal/pattern"
	"github.com/mpyw/declscope/internal/shrink/excluded"
)

// Module is every package of the main modules, loaded with its tests, and
// what the build left out of them.
type Module struct {
	// mains are the main modules, in the order the go command lists them.
	mains []mainModule
	Fset  *token.FileSet
	// Pkgs is every variant go/packages returned: a package, the package
	// with its in-package tests, and its external test package, each
	// separately. Paths lists their import paths in order.
	Pkgs  []*packages.Package
	Paths []string
	// Excluded is every Go file of the main modules the build did not
	// compile.
	Excluded []*excluded.File

	byPath map[string][]*packages.Package
	// nested is the module path of every go.mod below a main module's root
	// that is not a main module itself. Go checks internal/ by import path,
	// so a nested module whose path extends an internal parent may import
	// the package, and this run never loads it. One with an unrelated path
	// cannot, wherever it sits.
	nested []string
}

// mainModule is one main module: its import path and root directory.
type mainModule struct {
	path, dir string
}

// Load loads every package of the main modules the go command reports from
// dir: the module holding it, or every module of its workspace.
//
// A load error refuses the whole run. A package that does not type-check
// yields no references, which reads exactly like nothing using a declaration.
func Load(dir string) (*Module, error) {
	mains, err := mainModules(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Dir: dir,
		// No NeedDeps: dependencies come from export data. Only the module's
		// own packages are judged, and a generic declared outside the module
		// has no body to read either way.
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedForTest | packages.NeedModule,
		Tests: true,
	}
	// One load for every main module, so that they share one file set and
	// one universe of objects. A directory pattern per module reads what
	// ./... reads from its root, and needs no go command that knows "work".
	patterns := make([]string, 0, len(mains))
	for _, mm := range mains {
		patterns = append(patterns, filepath.Join(mm.dir, "..."))
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	m := &Module{mains: mains, byPath: map[string][]*packages.Package{}}
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
		return nil, fmt.Errorf("no package found in the module at %s", mains[0].dir)
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
// the main module declaring the package, and no nested module may have a
// path inside it: that module could import the package, and this run never
// loads it. The reason names that module, since nothing else would lead a
// reader to it. Another main module's path may lie inside the tree, since
// every main module is loaded.
func (m *Module) Range(path string) (parent, why string) {
	parent, ok := InternalParent(path)
	if !ok {
		return "", "not inside internal/, so another module may import it"
	}
	if owner := m.owner(path); parent != owner.path && !strings.HasPrefix(parent, owner.path+"/") {
		return "", fmt.Sprintf("its internal/ parent %s lies above the module %s", parent, owner.path)
	}
	for _, n := range m.nested {
		if n == parent || strings.HasPrefix(n, parent+"/") {
			return "", fmt.Sprintf("the nested module %s may import it, and this run does not load it", n)
		}
	}
	return parent, ""
}

// owner returns the main module declaring the package at path: the one with
// the longest module path that the import path lies in. A package this run
// loaded always has one.
func (m *Module) owner(path string) mainModule {
	var best mainModule
	for _, mm := range m.mains {
		if (path == mm.path || strings.HasPrefix(path, mm.path+"/")) && len(mm.path) > len(best.path) {
			best = mm
		}
	}
	return best
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

// Named is what the patterns name.
type Named struct {
	// Paths is every package they name in a main module.
	Paths map[string]bool
	// Outside counts the packages they name outside every main module: the
	// standard library, a dependency, anything all names.
	Outside int
	// Unmatched lists the patterns that named no package, in order.
	Unmatched []string
}

// Resolve resolves the patterns, from dir, to the packages they name, the way
// the go command does.
//
// A pattern the go command rejects is an error with the go command's own
// message, since go vet stops there too: a typo in CI must not pass as a run
// that found nothing. A pattern that names nothing is a warning, and an error
// when every pattern names nothing, which is what go vet does. Package
// pattern answers which ones those are, for every command alike.
func Resolve(dir string, patterns []string) (Named, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	cfg := &packages.Config{Dir: dir, Mode: packages.NeedName | packages.NeedModule}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return Named{}, err
	}
	named := Named{Paths: map[string]bool{}}
	outside := map[string]bool{}
	var problems []string
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			problems = append(problems, p.Errors[0].Msg)
			continue
		}
		if p.Module != nil && p.Module.Main {
			named.Paths[p.PkgPath] = true
		} else {
			outside[p.PkgPath] = true
		}
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return Named{}, fmt.Errorf("%s", strings.Join(slices.Compact(problems), "\n"))
	}
	if named.Unmatched, err = pattern.Unmatched(cfg, patterns, pkgs); err != nil {
		return Named{}, err
	}
	named.Outside = len(outside)
	return named, nil
}

// mainModules asks the go command for the main modules seen from dir: the
// module holding it, or every module a workspace uses.
func mainModules(dir string) ([]mainModule, error) {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}\t{{.Path}}")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("finding the module: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	// Split rather than Lines: an empty answer is one empty line, which the
	// check below refuses like any line with no directory.
	var mains []mainModule
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		root, path, ok := strings.Cut(line, "\t")
		if !ok || root == "" {
			// Outside a module, go list -m answers command-line-arguments,
			// with no directory.
			return nil, fmt.Errorf("not inside a module")
		}
		mains = append(mains, mainModule{path: path, dir: root})
	}
	return mains, nil
}

// isTestMain identifies the synthesized main package of a test binary, which
// has nothing of its own to judge.
func isTestMain(p *packages.Package) bool {
	return p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") &&
		!slices.ContainsFunc(p.GoFiles, func(f string) bool { return strings.HasSuffix(f, ".go") && filepath.Base(f) != "_testmain.go" })
}

// walk reads every Go file no loaded package holds, and every nested module,
// in each main module's tree.
//
// A package whose files are all excluded under this GOOS is not in the load
// at all, so reading only IgnoredFiles would miss it. Files are read only
// where ./... reads them, which go help packages lists: not under a directory
// named vendor or testdata, one starting with . or _, a nested module, or a
// directory an ignore directive in the module's go.mod names. A file outside
// those is code another build configuration compiles. A file inside them is
// not part of the module's build at all: nothing may import a vendored path,
// and nothing under testdata or an ignored directory is a package of it.
//
// The walk itself goes everywhere but .git, since it is also looking for
// go.mod files. A nested module in testdata/tools or _tools/gen is no part
// of this module's build, but it can import the module's internal packages
// all the same, at any depth. A nested module that is itself a main module
// is walked on its own, and loaded, so it is not recorded.
func (m *Module) walk() error {
	compiled := map[string]bool{}
	for _, p := range m.Pkgs {
		for _, f := range slices.Concat(p.GoFiles, p.CompiledGoFiles) {
			compiled[f] = true
		}
	}
	mainDirs := map[string]bool{}
	for _, mm := range m.mains {
		mainDirs[mm.dir] = true
	}
	fset := token.NewFileSet()
	for _, mm := range m.mains {
		if err := m.walkModule(mm, mainDirs, compiled, fset); err != nil {
			return err
		}
	}
	return nil
}

func (m *Module) walkModule(mm mainModule, mainDirs, compiled map[string]bool, fset *token.FileSet) error {
	ignored, err := ignoreDirectives(mm.dir)
	if err != nil {
		return err
	}
	// unread holds the directories whose Go files ./... does not read.
	var unread []string
	under := func(path string) bool {
		return slices.ContainsFunc(unread, func(dir string) bool {
			return strings.HasPrefix(path, dir+string(filepath.Separator))
		})
	}
	return filepath.WalkDir(mm.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == mm.dir {
				return nil
			}
			name := d.Name()
			if name == ".git" || mainDirs[path] {
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
			rel, _ := filepath.Rel(mm.dir, path)
			if name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || ignored.matches(rel) {
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

// ignorePatterns are the ignore directives of one go.mod, spelled the way
// the go command's search.IgnorePatterns compares them: with a slash at each
// end, so that a directory matches only whole elements.
type ignorePatterns struct {
	// fromRoot are the ./x directives, which name x at the module root only.
	// anywhere are the x directives, which name any directory x.
	fromRoot, anywhere []string
}

// ignoreDirectives reads the ignore directives of the go.mod in dir.
func ignoreDirectives(dir string) (ignorePatterns, error) {
	name := filepath.Join(dir, "go.mod")
	// A file that cannot be read parses as empty, so only the read error is
	// reported for it.
	data, readErr := os.ReadFile(name)
	f, parseErr := modfile.ParseLax(name, data, nil)
	if err := errors.Join(readErr, parseErr); err != nil {
		return ignorePatterns{}, err
	}
	var ps ignorePatterns
	for _, ig := range f.Ignore {
		if rest, ok := strings.CutPrefix(ig.Path, "./"); ok {
			ps.fromRoot = append(ps.fromRoot, slashed(rest))
		} else {
			ps.anywhere = append(ps.anywhere, slashed(ig.Path))
		}
	}
	return ps, nil
}

// matches reports whether an ignore directive names rel, a directory
// relative to the module root.
func (ps ignorePatterns) matches(rel string) bool {
	dir := slashed(rel)
	return slices.ContainsFunc(ps.fromRoot, func(p string) bool { return strings.HasPrefix(dir, p) }) ||
		slices.ContainsFunc(ps.anywhere, func(p string) bool { return strings.Contains(dir, p) })
}

// slashed spells a path with forward slashes and a slash at each end.
func slashed(path string) string {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if !strings.HasSuffix(path, "/") {
		path += "/"
	}
	return path
}
