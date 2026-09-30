// Package module loads the main modules for declscope shrink: every package
// with its tests, every file the build left out, and every nested module,
// and says which internal packages have importers this run can all see.
//
// There is one main module, or several in a workspace. Every one is loaded,
// since each may import another's internal packages. A nested module that may
// import them is loaded too, on its own, only to read what it uses.
package module

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"maps"
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

	// Importers are the nested modules this run loaded to read what they use.
	// Their packages are never judged.
	Importers []Importer

	byPath map[string][]*packages.Package
	// nested is every go.mod below a main module's root that is not a main
	// module itself. Go checks internal/ by import path, so a nested module
	// whose path extends an internal parent may import the package. One with
	// an unrelated path cannot, wherever it sits.
	nested []nestedModule
}

// nestedModule is one module below a main module's root.
type nestedModule struct {
	path, dir string
	// why says what keeps this run from reading every use the module makes,
	// or is empty once it has read them.
	why string
}

// Importer is one nested module this run loaded to read what it uses.
//
// It is loaded on its own, from its own directory, since its go.mod decides
// what it builds with. So its types are its own: a type of the main modules
// it names is another object than the main load's. Declarations still match
// by keyOf, since the load shares the main load's file set and reads the
// main modules' packages it imports from their source. A check that compares
// types, such as interface satisfaction, runs within one Importer.
type Importer struct {
	// Pkgs are its own packages, the widest variant of each import path.
	Pkgs []*packages.Package
	// Imported are the main modules' packages it imports, as its own load
	// type-checked them from their source.
	Imported []*packages.Package
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
	for i := range m.nested {
		m.loadImporter(&m.nested[i], cfg.Mode)
	}
	return m, nil
}

// loadImporter loads a nested module that may import a main module's
// internal packages, so that its uses count as any other importer's. A
// nested module that no internal parent covers imports nothing that matters,
// and is left alone.
//
// It takes two loads. The first asks what the module imports, the way go
// list -deps does. The second type-checks its packages together with the
// main modules' packages it imports, as roots: a dependency comes from
// export data, which keeps a declaration's file and line but not its offset,
// so only a root's declarations match the main load's by keyOf. Every
// internal package cannot be a root instead. The module's go.mod need not
// require what those import, and loading them fails.
//
// Whatever it cannot read leaves why set, and the ranges it may import stay
// unjudged: a load that fails, and a main module's package read from
// anywhere but that module's directory, such as a published version. Uses of
// other source say nothing about this one.
func (m *Module) loadImporter(n *nestedModule, mode packages.LoadMode) {
	if !m.covers(n.path) {
		return
	}
	n.why = "this run does not load it"
	deps, err := packages.Load(&packages.Config{
		Dir:   n.dir,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Tests: true,
	}, "./...")
	problem := loadFailure(err)
	imported := map[string]bool{}
	packages.Visit(deps, nil, func(p *packages.Package) {
		if len(p.Errors) > 0 && problem == "" {
			problem = fmt.Sprintf("loading it failed: %s: %s", p.PkgPath, p.Errors[0])
		}
		if m.byPath[p.PkgPath] != nil && p.PkgPath != n.path {
			if where := m.elsewhere(p); where != "" && problem == "" {
				problem = fmt.Sprintf("it reads %s from %s", p.PkgPath, where)
			}
			imported[p.PkgPath] = true
		}
	})
	if problem != "" {
		n.why = problem
		return
	}
	var imp Importer
	own := deps
	if len(imported) > 0 {
		// The first load has already checked where each of these is read
		// from, and this one reads the same go.mod.
		pkgs, err := packages.Load(&packages.Config{Dir: n.dir, Fset: m.Fset, Mode: mode, Tests: true},
			append([]string{"./..."}, slices.Sorted(maps.Keys(imported))...)...)
		problem = loadFailure(err)
		own = nil
		widest := map[string]*packages.Package{}
		for _, p := range pkgs {
			switch {
			case isTestMain(p):
				continue
			case m.byPath[p.PkgPath] != nil || m.byPath[p.ForTest] != nil:
				// The main modules' test variants are not what the module
				// imports, and their tests may need what its go.mod lacks.
				if p.ForTest != "" {
					continue
				}
				imp.Imported = append(imp.Imported, p)
			default:
				own = append(own, p)
				if w, ok := widest[p.PkgPath]; !ok || len(p.Syntax) > len(w.Syntax) {
					widest[p.PkgPath] = p
				}
			}
			if len(p.Errors) > 0 && problem == "" {
				problem = fmt.Sprintf("its package %s does not type-check: %s", p.PkgPath, p.Errors[0])
			}
		}
		if problem != "" {
			n.why = problem
			return
		}
		for _, path := range slices.Sorted(maps.Keys(widest)) {
			imp.Pkgs = append(imp.Pkgs, widest[path])
		}
	}
	// Its build-excluded files are read as the main modules' are, so a name
	// written under a tag this run does not set still counts.
	var xs []*excluded.File
	if err := m.walkTree(n.dir, compiledFiles(own), token.NewFileSet(), false, &xs); err != nil {
		n.why = err.Error()
		return
	}
	m.Excluded = append(m.Excluded, xs...)
	if len(imp.Pkgs) > 0 {
		m.Importers = append(m.Importers, imp)
	}
	n.why = ""
}

// loadFailure words a failed load, or is empty when the load did not fail.
// Its packages' own errors are read separately.
func loadFailure(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("loading it failed: %v", err)
}

// covers reports whether a nested module's path lies inside the tree an
// internal package of a main module may be imported from.
func (m *Module) covers(path string) bool {
	return slices.ContainsFunc(m.Paths, func(p string) bool {
		parent, ok := InternalParent(p)
		return ok && (path == parent || strings.HasPrefix(path, parent+"/"))
	})
}

// elsewhere returns where a nested module's load read p, a main module's
// package, when that is not the main module's own directory, or "".
func (m *Module) elsewhere(p *packages.Package) string {
	if p.Module != nil && p.Module.Dir != "" && sameDir(p.Module.Dir, m.owner(p.PkgPath).dir) {
		return ""
	}
	// A published version has no directory of its own to name.
	where := "outside any module"
	if p.Module != nil {
		where = cmp.Or(p.Module.Dir, p.Module.Path+"@"+p.Module.Version)
	}
	return where
}

// sameDir reports whether two directories are one, through symbolic links.
func sameDir(a, b string) bool {
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// compiledFiles is every file the packages compile.
func compiledFiles(pkgs []*packages.Package) map[string]bool {
	compiled := map[string]bool{}
	for _, p := range pkgs {
		for _, f := range slices.Concat(p.GoFiles, p.CompiledGoFiles) {
			compiled[f] = true
		}
	}
	return compiled
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
		if n.why != "" && (n.path == parent || strings.HasPrefix(n.path, parent+"/")) {
			return "", fmt.Sprintf("the nested module %s may import it, and %s", n.path, n.why)
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
	compiled := compiledFiles(m.Pkgs)
	fset := token.NewFileSet()
	for _, mm := range m.mains {
		if err := m.walkTree(mm.dir, compiled, fset, true, &m.Excluded); err != nil {
			return err
		}
	}
	return nil
}

// walkTree walks the tree of one module, rooted at root, adding the files its
// build excludes to into. record says whether the go.mod files below root are
// recorded as nested modules: a main module's walk records them, at any
// depth, so a nested module's own walk has nothing left to record.
func (m *Module) walkTree(root string, compiled map[string]bool, fset *token.FileSet, record bool, into *[]*excluded.File) error {
	ignored, err := ignoreDirectives(root)
	if err != nil {
		return err
	}
	isMain := func(dir string) bool {
		return slices.ContainsFunc(m.mains, func(mm mainModule) bool { return mm.dir == dir })
	}
	// unread holds the directories whose Go files ./... does not read.
	var unread []string
	under := func(path string) bool {
		return slices.ContainsFunc(unread, func(dir string) bool {
			return strings.HasPrefix(path, dir+string(filepath.Separator))
		})
	}
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			name := d.Name()
			if name == ".git" || isMain(path) {
				return filepath.SkipDir
			}
			if data, err := os.ReadFile(filepath.Join(path, "go.mod")); err == nil {
				modPath := modfile.ModulePath(data)
				if modPath == "" {
					return fmt.Errorf("reading %s: no module path", filepath.Join(path, "go.mod"))
				}
				if record {
					m.nested = append(m.nested, nestedModule{path: modPath, dir: path, why: "this run does not load it"})
				}
				unread = append(unread, path)
				return nil
			}
			rel, _ := filepath.Rel(root, path)
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
		*into = append(*into, f)
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
