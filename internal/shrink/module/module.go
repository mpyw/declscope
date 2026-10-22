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
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/pattern"
	"github.com/mpyw/declscope/internal/shrink/excluded"
)

// Module is every package of the main modules, loaded with its tests, and
// what the build left out of them.
type Module struct {
	Fset *token.FileSet
	// Pkgs is every variant go/packages returned: a package, the package
	// with its in-package tests, and its external test package, each
	// separately. Paths lists their import paths in order.
	Pkgs  []*packages.Package
	Paths []string
	// Excluded is every Go file of the main modules and the loaded nested
	// ones that the build did not compile.
	Excluded []*excluded.File

	// Importers are the nested modules this run loaded to read what they use.
	// Their packages are never judged.
	Importers []Importer

	// mains are the main modules, in the order the go command lists them.
	mains  []mainModule
	byPath map[string][]*packages.Package
	// importedBy maps each loaded package's import path to the paths of the
	// loaded packages importing it, and reaching caches, per package, every
	// path that depends on it. Both are built on first use.
	importedBy map[string][]string
	reaching   map[string]map[string]bool
	// nested is every go.mod below a main module's root that is not a main
	// module itself. Go checks internal/ by import path, so a nested module
	// whose path extends an internal parent may import the package. One with
	// an unrelated path cannot, wherever it sits.
	nested []nestedModule
	// built is every module of the main modules' build list, other than a
	// main module, whose path lies in a main module's: a module that may
	// import their internal packages from wherever the build reads it.
	// unbuilt says why the build list could not be read, or is empty.
	built   []builtModule
	unbuilt string
}

// builtModule is one module of the build list: its path, and where the build
// reads it from.
type builtModule struct {
	path, dir, version string
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

// nestedModule is one module below a main module's root.
type nestedModule struct {
	path, dir string
	// why says what keeps this run from reading every use the module makes,
	// or is empty once it has read them.
	why string
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
	m.built, m.unbuilt = buildList(dir, mains)
	var failed []string
	for _, p := range pkgs {
		if IsTestMain(p) {
			continue
		}
		if len(p.Errors) > 0 {
			failed = append(failed, p.PkgPath+": "+ErrorLine(p, dir))
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
	m.loadImporters(cfg.Mode)
	return m, nil
}

// loaded is what loading one nested module read: its packages and the files
// its build excludes, or why it could not read them.
type loaded struct {
	imp      Importer
	excluded []*excluded.File
	why      string
}

// loadImporters loads every nested module that may import an internal
// package, up to GOMAXPROCS at a time. Each load runs go list and spends
// most of its time waiting on it: one after another, 40 nested modules took
// 15 to 37 s of wall time for about 1 s of CPU. Each load only reads the
// main load and writes its own slot, and the slots are folded in the order
// of m.nested, so the result does not depend on scheduling. The loads share
// the main load's file set, which is safe for concurrent use, and keyOf
// reads a file name and an offset, which do not depend on the order the
// files were added in.
func (m *Module) loadImporters(mode packages.LoadMode) {
	slots := make([]loaded, len(m.nested))
	limit := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for i, n := range m.nested {
		if !m.covers(n.path) {
			continue
		}
		limit <- struct{}{}
		wg.Go(func() {
			defer func() { <-limit }()
			slots[i] = m.loadImporter(n, mode)
		})
	}
	wg.Wait()
	for i, l := range slots {
		if !m.covers(m.nested[i].path) {
			continue
		}
		m.nested[i].why = l.why
		if l.why != "" {
			continue
		}
		m.Excluded = append(m.Excluded, l.excluded...)
		if len(l.imp.Pkgs) > 0 {
			m.Importers = append(m.Importers, l.imp)
		}
	}
}

// loadImporter loads a nested module that may import a main module's
// internal packages, so that its uses count as any other importer's. A
// nested module that no internal parent covers imports nothing that matters,
// and loadImporters leaves it alone.
//
// It takes two loads. The first asks what the module imports, the way go
// list -deps does. The second type-checks its packages together with the
// main modules' packages it imports, as roots: a dependency comes from
// export data, which keeps a declaration's file and line but not its offset,
// so only a root's declarations match the main load's by keyOf. Every
// internal package cannot be a root instead. The module's go.mod need not
// require what those import, and loading them fails.
//
// Whatever it cannot read sets why, and the ranges it may import stay
// unjudged: a load that fails, and a main module's package read from
// anywhere but that module's directory, such as a published version. Uses of
// other source say nothing about this one.
//
// Both loads run with GOWORK=off. A nested module is never one the go.work
// uses, or it would be a main module, so the workspace has nothing to say
// about how it builds: inheriting it failed the load outright, since ./...
// names no module the go.work lists. Its own go.mod is what go build in its
// directory reads with GOWORK=off, and the check that each main module's
// package is read from that module's directory still holds it to this tree.
func (m *Module) loadImporter(n nestedModule, mode packages.LoadMode) loaded {
	env := append(os.Environ(), "GOWORK=off")
	deps, err := packages.Load(&packages.Config{
		Dir:   n.dir,
		Env:   env,
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Tests: true,
	}, "./...")
	problem := loadFailure(err)
	imported := map[string]bool{}
	packages.Visit(deps, nil, func(p *packages.Package) {
		if len(p.Errors) > 0 && problem == "" {
			problem = fmt.Sprintf("loading it failed: %s: %s", p.PkgPath, ErrorLine(p, n.dir))
		}
		if m.byPath[p.PkgPath] != nil && p.PkgPath != n.path {
			if where := m.elsewhere(p); where != "" && problem == "" {
				problem = fmt.Sprintf("it reads %s from %s", p.PkgPath, where)
			}
			imported[p.PkgPath] = true
		}
	})
	if problem != "" {
		return loaded{why: problem}
	}
	var imp Importer
	own := deps
	if len(imported) > 0 {
		// The first load has already checked where each of these is read
		// from, and this one reads the same go.mod.
		pkgs, err := packages.Load(&packages.Config{Dir: n.dir, Env: env, Fset: m.Fset, Mode: mode, Tests: true},
			append([]string{"./..."}, slices.Sorted(maps.Keys(imported))...)...)
		problem = loadFailure(err)
		own = nil
		widest := map[string]*packages.Package{}
		for _, p := range pkgs {
			switch {
			case IsTestMain(p):
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
				problem = fmt.Sprintf("its package %s does not type-check: %s", p.PkgPath, ErrorLine(p, n.dir))
			}
		}
		if problem != "" {
			return loaded{why: problem}
		}
		for _, path := range slices.Sorted(maps.Keys(widest)) {
			imp.Pkgs = append(imp.Pkgs, widest[path])
		}
	}
	// Its build-excluded files are read as the main modules' are, so a name
	// written under a tag this run does not set still counts.
	var xs []*excluded.File
	if err := m.walkTree(n.dir, compiledFiles(own), token.NewFileSet(), false, &xs); err != nil {
		return loaded{why: err.Error()}
	}
	return loaded{imp: imp, excluded: xs}
}

// loadFailure words a failed load, or is empty when the load did not fail.
// Its packages' own errors are read separately.
func loadFailure(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("loading it failed: %v", err)
}

// Reaches reports whether code importing the package at from may hold a
// value of a type the package at to declares: from is to, or depends on it
// among the packages this run loaded, nested modules' included. A package of
// a module this run reads that it did not load, such as one whose every file
// the build excludes, may depend on anything, so it reaches every package.
// A package outside those modules cannot import an internal package of
// theirs, so it reaches none.
func (m *Module) Reaches(from, to string) bool {
	if m.importedBy == nil {
		m.importedBy = map[string][]string{}
		m.reaching = map[string]map[string]bool{}
		add := func(p *packages.Package) {
			// Every loaded path has an entry, imported or not.
			if _, ok := m.importedBy[p.PkgPath]; !ok {
				m.importedBy[p.PkgPath] = nil
			}
			for _, dep := range p.Imports {
				m.importedBy[dep.PkgPath] = append(m.importedBy[dep.PkgPath], p.PkgPath)
			}
		}
		for _, p := range m.Pkgs {
			add(p)
		}
		for _, imp := range m.Importers {
			for _, p := range slices.Concat(imp.Pkgs, imp.Imported) {
				add(p)
			}
		}
	}
	if from == to {
		return true
	}
	if _, loaded := m.importedBy[from]; !loaded {
		return m.inModules(from)
	}
	if m.reaching[to] == nil {
		seen := map[string]bool{to: true}
		for queue := []string{to}; len(queue) > 0; queue = queue[1:] {
			for _, dependent := range m.importedBy[queue[0]] {
				if !seen[dependent] {
					seen[dependent] = true
					queue = append(queue, dependent)
				}
			}
		}
		m.reaching[to] = seen
	}
	return m.reaching[to][from]
}

// inModules reports whether path lies in a main module or a nested one.
func (m *Module) inModules(path string) bool {
	under := func(mod string) bool { return path == mod || strings.HasPrefix(path, mod+"/") }
	return slices.ContainsFunc(m.mains, func(mm mainModule) bool { return under(mm.path) }) ||
		slices.ContainsFunc(m.nested, func(n nestedModule) bool { return under(n.path) })
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
// the main module declaring the package, and every nested module with a path
// inside it must be one this run read: that module could import the package.
// So must every module of the build list with a path inside it, since the
// build compiles the copy the list names, such as a published version, and
// not the nested directory. The reason names that module, since nothing else
// would lead a reader to it. Another main module's path may lie inside the
// tree, since every main module is loaded.
func (m *Module) Range(path string) (parent, why string) {
	parent, ok := InternalParent(path)
	if !ok {
		return "", "not inside internal/, so another module may import it"
	}
	if owner := m.owner(path); parent != owner.path && !strings.HasPrefix(parent, owner.path+"/") {
		return "", fmt.Sprintf("its internal/ parent %s lies above the module %s", parent, owner.path)
	}
	inside := func(path string) bool { return path == parent || strings.HasPrefix(path, parent+"/") }
	for _, n := range m.nested {
		if n.why != "" && inside(n.path) {
			return "", fmt.Sprintf("the nested module %s may import it, and %s", n.path, n.why)
		}
	}
	if m.unbuilt != "" {
		return "", m.unbuilt
	}
	for _, b := range m.built {
		if inside(b.path) && !m.loadedFrom(b) {
			return "", fmt.Sprintf("the build uses %s from %s", b.path, cmp.Or(b.dir, b.path+"@"+b.version))
		}
	}
	return parent, ""
}

// loadedFrom reports whether this run loaded the nested module the build
// reads b from. Uses of any other copy are of other source: a published
// version, or a replacement outside the tree.
func (m *Module) loadedFrom(b builtModule) bool {
	return b.dir != "" && slices.ContainsFunc(m.nested, func(n nestedModule) bool {
		return n.path == b.path && n.why == "" && sameDir(n.dir, b.dir)
	})
}

// buildList reads the main modules' build list, as go list -m all does, and
// keeps each module other than a main module whose path lies in one's. Go
// checks internal/ by import path, so such a module may import the main
// modules' internal packages, and the build compiles the copy the list
// names, wherever it is.
//
// -e lists the modules whose go.mod the go command cannot read, with an
// error, instead of failing: in a workspace, a requirement on a workspace
// module at v0.0.0 is one, and the build never reads it. Any other module
// with an error may hide what it requires, so the second result says why
// the list is unknown, and every range stays unjudged. In vendor mode the go
// command computes no list at all, and vendor/modules.txt names every module
// the build reads instead.
func buildList(dir string, mains []mainModule) ([]builtModule, string) {
	inMain := func(path string) bool {
		return slices.ContainsFunc(mains, func(mm mainModule) bool { return strings.HasPrefix(path, mm.path+"/") })
	}
	isMain := func(path string) bool {
		return slices.ContainsFunc(mains, func(mm mainModule) bool { return path == mm.path })
	}
	out, err := goIn(dir, "list", "-m", "-e", "-json", "all")
	if err != nil {
		if strings.Contains(err.Error(), "using the vendor directory") {
			return vendored(dir, mains, inMain, isMain)
		}
		return nil, fmt.Sprintf("reading the build list failed: %v", err)
	}
	var built []builtModule
	for dec := json.NewDecoder(bytes.NewReader(out)); dec.More(); {
		var mod struct {
			Path, Version, Dir string
			Main               bool
			Replace            *struct{ Dir string }
			Error              *struct{ Err string }
		}
		if err := dec.Decode(&mod); err != nil {
			return nil, fmt.Sprintf("reading the build list failed: %v", err)
		}
		if mod.Error != nil && !isMain(mod.Path) {
			return nil, fmt.Sprintf("reading the build list failed: %s: %s", mod.Path, mod.Error.Err)
		}
		if mod.Replace != nil {
			mod.Dir = cmp.Or(mod.Replace.Dir, mod.Dir)
		}
		if !mod.Main && inMain(mod.Path) {
			built = append(built, builtModule{path: mod.Path, dir: mod.Dir, version: mod.Version})
		}
	}
	return built, ""
}

// vendored reads the modules a vendored build reads from vendor/modules.txt,
// in the workspace's directory or else the main module's. Each is read from
// the vendor directory, which is never a nested module this run loads. A
// vendor directory with no modules.txt vendors nothing.
func vendored(dir string, mains []mainModule, inMain, isMain func(string) bool) ([]builtModule, string) {
	work, err := goIn(dir, "env", "GOWORK")
	if err != nil {
		return nil, fmt.Sprintf("reading the build list failed: %v", err)
	}
	root := mains[0].dir
	if w := strings.TrimSpace(string(work)); w != "" && w != "off" {
		root = filepath.Dir(w)
	}
	vendor := filepath.Join(root, "vendor")
	data, err := os.ReadFile(filepath.Join(vendor, "modules.txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ""
	}
	if err != nil {
		return nil, fmt.Sprintf("reading the build list failed: %v", err)
	}
	var built []builtModule
	for line := range strings.Lines(string(data)) {
		// "# path version", with "=> replacement" after it for a replace.
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "#" || isMain(fields[1]) || !inMain(fields[1]) {
			continue
		}
		built = append(built, builtModule{path: fields[1], dir: filepath.Join(vendor, filepath.FromSlash(fields[1]))})
	}
	return built, ""
}

// goIn runs the go command in dir and returns its output, or an error that
// holds what it wrote to stderr.
func goIn(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
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

// IsTestMain identifies the synthetic executable built to run a package's
// tests, which has nothing of its own to judge. Depending on the go command's
// response, its file is either the generated _testmain.go or a build-cache
// archive; a real package is loaded from .go files even when its import path
// happens to end in ".test". A package with no file at all is not called one:
// nothing shows it, and it has nothing to judge either way.
//
// shrink and the CLI's other subcommands both skip it, so one rule serves
// both.
func IsTestMain(pkg *packages.Package) bool {
	if pkg.Name != "main" || !strings.HasSuffix(pkg.PkgPath, ".test") {
		return false
	}
	paths := append(slices.Clone(pkg.GoFiles), pkg.CompiledGoFiles...)
	for _, path := range paths {
		if filepath.Base(path) == "_testmain.go" {
			return true
		}
	}
	for _, path := range paths {
		// A real package is loaded from Go source. With the go command's
		// driver, the synthetic executable instead points at its build-cache
		// archive (a path ending in -d), even when NeedSyntax is requested.
		if filepath.Ext(path) == ".go" {
			return false
		}
	}
	return len(paths) > 0
}

// ErrorLine names why a package with errors failed, on one line, as the type
// checker would. cwd is the go command's working directory: the config's Dir,
// or this process's when it sets none.
//
// The error named is the first that is not a ListError, or the first of all
// when every one is. Dependencies come from export data, so go list compiles
// the package too, and its build output comes first: a "-" position, a "# path"
// header and one line per compiler message, at positions relative to cwd. The
// type checker reports the same error after it, on one line at an absolute
// position. A type error that only says C could not be imported is passed
// over: when a cgo package's C code does not compile, the type checker has no
// metadata for C, and the C compiler's message is in the build output instead.
//
// For a package outside the patterns, the build output can be the only error:
// go/packages type-checks no function body there, so an error inside one is
// never found again. The first compiler message with a line number is kept,
// with its file made absolute: a C compiler can print a line without one
// first, such as gcc's "In function" context. Any other error has its
// newlines flattened and the "-" position dropped, since it names no place.
//
// shrink and the CLI's other subcommands both name failed packages, so one
// rule serves both.
func ErrorLine(pkg *packages.Package, cwd string) string {
	e := pkg.Errors[0]
	if i := slices.IndexFunc(pkg.Errors, func(e packages.Error) bool {
		return e.Kind != packages.ListError && !strings.HasPrefix(e.Msg, "could not import C (")
	}); i >= 0 {
		e = pkg.Errors[i]
	}
	header, rest, ok := strings.Cut(e.Msg, "\n")
	if e.Kind != packages.ListError || !ok || !strings.HasPrefix(header, "# ") {
		return strings.TrimPrefix(strings.ReplaceAll(e.Error(), "\n", " "), "-: ")
	}
	lines := strings.Split(rest, "\n")
	line := lines[0]
	for _, l := range lines {
		if _, pos, ok := strings.Cut(l, ":"); ok && pos != "" && pos[0] >= '0' && pos[0] <= '9' {
			line = l
			break
		}
	}
	if file, pos, ok := strings.Cut(line, ":"); ok && cwd != "" && strings.HasSuffix(file, ".go") && !filepath.IsAbs(file) {
		line = filepath.Join(cwd, file) + ":" + pos
	}
	return line
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
