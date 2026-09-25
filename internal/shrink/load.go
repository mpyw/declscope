package shrink

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
)

// loadedModule is every package of the main module, loaded with its tests,
// and what the build left out of it.
//
//declscope:package // the core and evidence.go read it
type loadedModule struct {
	// path and dir are the module's import path and root directory.
	//
	//declscope:private // only the range and the walk read them
	path, dir string
	fset      *token.FileSet
	// pkgs is every variant go/packages returned: a package, the package with
	// its in-package tests, and its external test package, each separately.
	pkgs []*packages.Package
	// byPath groups the variants of each package by import path, the widest
	// first, and paths lists the import paths in order.
	byPath map[string][]*packages.Package
	paths  []string
	// excluded is every Go file of the module the build did not compile.
	excluded []*loadExcludedFile
	// nested is the module path of every go.mod below the module root. Go
	// checks internal/ by import path, so a nested module whose path extends
	// an internal parent may import the package, and this run never loads
	// it. One with an unrelated path cannot, wherever it sits.
	//
	//declscope:private // only the range reads it
	nested []string
}

// loadModule loads every package of the main module holding dir.
//
// A load error refuses the whole run. A package that does not type-check
// yields no references, which reads exactly like nothing using a declaration.
//
//declscope:package // the core starts every run here
func loadModule(dir string) (*loadedModule, error) {
	root, modPath, err := loadModuleRoot(dir)
	if err != nil {
		return nil, err
	}
	cfg := &packages.Config{
		Dir: root,
		// No NeedDeps: dependencies come from export data. Only the module's
		// own packages are judged, and a generic declared outside the module
		// is covered by evidenceInstances rather than by its body.
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedSyntax |
			packages.NeedTypesInfo | packages.NeedForTest | packages.NeedModule,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, err
	}
	m := &loadedModule{path: modPath, dir: root, byPath: map[string][]*packages.Package{}}
	var failed []string
	for _, p := range pkgs {
		if loadIsTestMain(p) {
			continue
		}
		if len(p.Errors) > 0 {
			failed = append(failed, fmt.Sprintf("%s: %s", p.PkgPath, p.Errors[0]))
			continue
		}
		if m.fset == nil {
			m.fset = p.Fset
		}
		m.pkgs = append(m.pkgs, p)
		if _, ok := m.byPath[p.PkgPath]; !ok {
			m.paths = append(m.paths, p.PkgPath)
		}
		m.byPath[p.PkgPath] = append(m.byPath[p.PkgPath], p)
	}
	if len(failed) > 0 {
		slices.Sort(failed)
		return nil, fmt.Errorf("these packages do not type-check, so no absence of a use means anything:\n  %s",
			strings.Join(slices.Compact(failed), "\n  "))
	}
	if m.fset == nil {
		return nil, fmt.Errorf("no package found in the module at %s", root)
	}
	slices.Sort(m.paths)
	for _, path := range m.paths {
		slices.SortStableFunc(m.byPath[path], func(a, b *packages.Package) int {
			return len(b.Syntax) - len(a.Syntax)
		})
	}
	if err := m.loadExcluded(); err != nil {
		return nil, err
	}
	return m, nil
}

// loadWidest returns one variant per import path, the one with the most
// files, which holds every file the others do.
//
//declscope:package // evidence.go reads each package once through it
func (m *loadedModule) loadWidest() []*packages.Package {
	out := make([]*packages.Package, 0, len(m.paths))
	for _, path := range m.paths {
		out = append(out, m.byPath[path][0])
	}
	return out
}

// loadModuleRoot asks the go command for the module holding dir.
func loadModuleRoot(dir string) (root, path string, err error) {
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

// loadWanted resolves the patterns, from dir, to the import paths to report
// on.
//
//declscope:package // the core reports only on these
func loadWanted(dir string, patterns []string) (map[string]bool, error) {
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

// loadIsTestMain identifies the synthesized main package of a test binary,
// which has nothing of its own to judge.
func loadIsTestMain(p *packages.Package) bool {
	return p.Name == "main" && strings.HasSuffix(p.PkgPath, ".test") &&
		!slices.ContainsFunc(p.GoFiles, func(f string) bool { return strings.HasSuffix(f, ".go") && filepath.Base(f) != "_testmain.go" })
}

// rangeOf returns the import path of the tree allowed to import pkgPath, and
// whether this run loads every package that tree could hold.
//
// The deepest internal element is the one that restricts: a/internal/b/
// internal/c is importable only from a/internal/b. The tree must lie inside
// the module, and no nested module may have a path inside it: that module
// could import the package, and this run never loads it.
//
//declscope:package // the core skips a package whose importers it cannot all see
func (m *loadedModule) rangeOf(pkgPath string) (string, bool) {
	parent, ok := loadInternalParent(pkgPath)
	if !ok {
		return "", false
	}
	if parent != m.path && !strings.HasPrefix(parent, m.path+"/") {
		return "", false
	}
	for _, n := range m.nested {
		if n == parent || strings.HasPrefix(n, parent+"/") {
			return "", false
		}
	}
	return parent, true
}

// loadInternalParent returns the import path above the deepest internal
// element of pkgPath.
//
//declscope:package // the core names a skipped package only when it is internal
func loadInternalParent(pkgPath string) (string, bool) {
	elems := strings.Split(pkgPath, "/")
	for i := len(elems) - 1; i >= 0; i-- {
		if elems[i] == "internal" {
			return strings.Join(elems[:i], "/"), true
		}
	}
	return "", false
}

// loadExcludedFile is a Go file of the module that the build did not compile,
// read for the names it writes.
//
// Nothing is type-checked here. What the file can say is what it spells: the
// package it declares, what it imports under which name, and every
// identifier and selection in it.
type loadExcludedFile struct {
	path, dir, pkgName string
	// imports maps the name each import is spelled with to its path. A dot
	// import is recorded under ".", and one with no name of its own under "",
	// since the name it binds is the imported package's, which only the
	// package itself knows: internal/go-foo may well declare package foo.
	imports map[string][]string
	idents  map[string]bool
	// selected is every name written after a dot, and qualified every
	// x.Name keyed by x.
	selected  map[string]bool
	qualified map[string]map[string]bool
	//declscope:package // evidence.go reads its //go:linkname directives
	file *ast.File
}

// loadExcluded walks the module for every Go file no loaded package holds,
// and every nested module.
//
// A package whose files are all excluded under this GOOS is not in the load
// at all, so reading only IgnoredFiles would miss it. Files are read only
// where ./... reads them: not under testdata, a directory starting with . or
// _, the vendor directory at the module root, or a nested module.
//
// The walk itself goes everywhere but .git, since it is also looking for
// go.mod files. A nested module in testdata/tools or _tools/gen is no part of
// this module's build, but it can import the module's internal packages all
// the same, at any depth.
func (m *loadedModule) loadExcluded() error {
	compiled := map[string]bool{}
	for _, p := range m.pkgs {
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
		f, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return fmt.Errorf("reading %s, which the build excludes: %w", path, err)
		}
		m.excluded = append(m.excluded, loadReadExcluded(path, f))
		return nil
	})
}

func loadReadExcluded(path string, f *ast.File) *loadExcludedFile {
	x := &loadExcludedFile{
		path: path, dir: filepath.Dir(path), pkgName: f.Name.Name, file: f,
		imports: map[string][]string{}, idents: map[string]bool{},
		selected: map[string]bool{}, qualified: map[string]map[string]bool{},
	}
	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}
		x.imports[name] = append(x.imports[name], p)
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			x.idents[n.Name] = true
		case *ast.SelectorExpr:
			x.selected[n.Sel.Name] = true
			if id, ok := n.X.(*ast.Ident); ok {
				if x.qualified[id.Name] == nil {
					x.qualified[id.Name] = map[string]bool{}
				}
				x.qualified[id.Name][n.Sel.Name] = true
			}
		}
		return true
	})
	return x
}

// loadSameExcludedPackage reports whether the excluded file belongs to the
// candidate's own package: its directory and package name.
func loadSameExcludedPackage(x *loadExcludedFile, c *candidate) bool {
	return x.dir == loadPackageDir(c.pkg) && x.pkgName == c.pkg.Types.Name()
}

// loadQualifiedInExcluded reports whether a build-excluded file of another
// package imports the candidate's package and writes pkg.Name: a use, in a
// configuration this run does not build.
//
//declscope:package // the core counts it as a use from outside
func loadQualifiedInExcluded(xs []*loadExcludedFile, c *candidate) bool {
	for _, x := range xs {
		if loadSameExcludedPackage(x, c) {
			continue
		}
		for name, paths := range x.imports {
			if name == "." || name == "_" || !slices.Contains(paths, c.pkg.PkgPath) {
				continue
			}
			if name == "" {
				name = c.pkg.Types.Name()
			}
			if x.qualified[name][c.obj.Name()] {
				return true
			}
		}
	}
	return false
}

// loadNamedInOtherExcluded reports whether a build-excluded file of another
// package may use the candidate without pinning it to its package: a method
// or field selected by name on some value, or a name under a dot import.
//
//declscope:package // the core withholds the fix on it
func loadNamedInOtherExcluded(xs []*loadExcludedFile, c *candidate) bool {
	name := c.obj.Name()
	for _, x := range xs {
		if loadSameExcludedPackage(x, c) {
			continue
		}
		if c.kind.member() && x.selected[name] {
			return true
		}
		if slices.Contains(x.imports["."], c.pkg.PkgPath) && x.idents[name] {
			return true
		}
	}
	return false
}

// loadNamedInOwnExcluded reports whether a build-excluded file of the
// candidate's own package writes name. The rename cannot rewrite that file,
// which would keep the old name or declare the new one twice.
//
//declscope:package // the core and rename.go withhold on it
func loadNamedInOwnExcluded(xs []*loadExcludedFile, c *candidate, name string) bool {
	for _, x := range xs {
		if loadSameExcludedPackage(x, c) && x.idents[name] {
			return true
		}
	}
	return false
}

// loadPackageDir is the directory of a loaded package.
func loadPackageDir(p *packages.Package) string {
	for _, f := range slices.Concat(p.GoFiles, p.CompiledGoFiles) {
		return filepath.Dir(f)
	}
	return ""
}

// loadCandidates lists every exported declaration of one package that the
// rule judges, and for every ignore it read, the ignores parsed beside it.
//
// Its variants share their non-test files, so the widest variant (the first)
// holds every declaration. Left out: declarations in generated files, which
// a regeneration would rename back; the test functions of a _test.go file,
// which go test finds by name; interface method names, which are a contract
// every implementation outside the package would have to rename too; and the
// fields of a struct type the declaration does not write itself, which are
// the fields of another type's declaration.
//
// Ignores are bound where the analyzer binds them: a doc comment, a trailing
// comment go/parser attaches, and a comment on a declaration's first or last
// line that it attaches to nothing (looseTrailingComments), such as one after
// `struct {`, after `var (`, or after a func's closing brace. The ignores of
// one binding are siblings, the way the analyzer's ignoreSite.siblings are, so
// that an ignore naming unused answers the same reports in both tools.
//
//declscope:package // the core judges each of them
func loadCandidates(pkgs []*packages.Package) ([]*candidate, map[token.Pos][]directive.Ignore) {
	p := pkgs[0]
	l := &loadBinder{fset: p.Fset, siblings: map[token.Pos][]directive.Ignore{}}
	var out []*candidate
	add := func(obj types.Object, k kind, owner *types.TypeName, ignores []directive.Ignore) {
		testFile := strings.HasSuffix(l.fset.File(obj.Pos()).Name(), "_test.go")
		out = append(out, &candidate{obj: obj, key: keyOf(l.fset, obj), kind: k, owner: owner, pkg: p, ignores: ignores, testFile: testFile})
	}
	for _, file := range p.Syntax {
		if ast.IsGenerated(file) {
			continue
		}
		l.index(file)
		testFile := strings.HasSuffix(l.fset.File(file.Pos()).Name(), "_test.go")
		fileIgnores := directive.ParseFile(file).Ignores
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				// One binding, as the analyzer parses it: the doc comment and
				// the loose trailing comments together.
				own := l.parse(append([]*ast.CommentGroup{d.Doc}, l.loose(d)...)...)
				loadFuncCandidate(p, d, testFile, slices.Concat(own, fileIgnores), add)
			case *ast.GenDecl:
				loadGenCandidates(p, l, d, fileIgnores, add)
			}
		}
	}
	return out, l.siblings
}

// loadBinder binds comments to declarations for one package, one file at a
// time.
type loadBinder struct {
	fset *token.FileSet
	// lines is the first comment group starting on each line of the file,
	// as the analyzer's fileInfo.lineComments holds it.
	lines map[int]*ast.CommentGroup
	// siblings maps each ignore to every ignore of the same binding.
	siblings map[token.Pos][]directive.Ignore
}

func (l *loadBinder) index(file *ast.File) {
	l.lines = map[int]*ast.CommentGroup{}
	for _, g := range file.Comments {
		line := l.fset.PositionFor(g.Pos(), false).Line
		if _, seen := l.lines[line]; !seen {
			l.lines[line] = g
		}
	}
}

// parse reads the ignores of one binding and records them as siblings.
func (l *loadBinder) parse(groups ...*ast.CommentGroup) []directive.Ignore {
	ignores := directive.ParseDecl(groups...).Ignores
	for _, ig := range ignores {
		l.siblings[ig.Pos] = ignores
	}
	return ignores
}

// loose returns the comment groups trailing node's first or last line that
// go/parser attached to nothing, leaving out those attached inside it.
func (l *loadBinder) loose(node ast.Node, attached ...*ast.CommentGroup) []*ast.CommentGroup {
	var out []*ast.CommentGroup
	for _, pos := range []token.Pos{node.Pos(), node.End()} {
		g, ok := l.lines[l.fset.PositionFor(pos, false).Line]
		if !ok || g.Pos() < pos || slices.Contains(attached, g) || slices.Contains(out, g) {
			continue
		}
		out = append(out, g)
	}
	return out
}

// loadAttached returns the comment groups go/parser hung on a spec or on
// anything inside it, which a loose comment never claims.
func loadAttached(spec ast.Spec) []*ast.CommentGroup {
	switch spec := spec.(type) {
	case *ast.ValueSpec:
		return []*ast.CommentGroup{spec.Doc, spec.Comment}
	case *ast.TypeSpec:
		out := []*ast.CommentGroup{spec.Doc, spec.Comment}
		var fields *ast.FieldList
		switch t := spec.Type.(type) {
		case *ast.StructType:
			fields = t.Fields
		case *ast.InterfaceType:
			fields = t.Methods
		}
		if fields != nil {
			for _, f := range fields.List {
				out = append(out, f.Doc, f.Comment)
			}
		}
		return out
	}
	return nil
}

// loadAdd records one candidate.
type loadAdd func(obj types.Object, k kind, owner *types.TypeName, ignores []directive.Ignore)

// loadFuncCandidate records a func or method with the ignores covering it.
func loadFuncCandidate(p *packages.Package, d *ast.FuncDecl, testFile bool, ignores []directive.Ignore, add loadAdd) {
	if !d.Name.IsExported() {
		return
	}
	fn, ok := p.TypesInfo.Defs[d.Name].(*types.Func)
	if !ok {
		return
	}
	if d.Recv == nil {
		if testFile && loadIsTestEntry(d.Name.Name) {
			return
		}
		add(fn, kindFunc, nil, ignores)
		return
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		return
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	if !ok || types.IsInterface(named) {
		return
	}
	add(fn, kindMethod, named.Origin().Obj(), ignores)
}

func loadGenCandidates(p *packages.Package, l *loadBinder, d *ast.GenDecl, fileIgnores []directive.Ignore, add loadAdd) {
	// The block's doc comment reaches every spec, and so does a comment on
	// the lines of `var (` and `)`. Without parentheses those lines are the
	// one spec's, and are read with it below, as the analyzer reads them.
	block := l.parse(d.Doc)
	if d.Lparen.IsValid() && len(d.Specs) > 0 {
		attached := slices.Concat(loadAttached(d.Specs[0]), loadAttached(d.Specs[len(d.Specs)-1]))
		block = slices.Concat(block, l.parse(l.loose(d, attached...)...))
	}
	for _, spec := range d.Specs {
		var docs []*ast.CommentGroup
		switch s := spec.(type) {
		case *ast.ValueSpec:
			docs = []*ast.CommentGroup{s.Doc, s.Comment}
		case *ast.TypeSpec:
			docs = []*ast.CommentGroup{s.Doc, s.Comment}
		default:
			continue
		}
		own := l.parse(slices.Concat(docs, l.loose(spec, loadAttached(spec)...))...)
		switch s := spec.(type) {
		case *ast.ValueSpec:
			ignores := slices.Concat(own, block, fileIgnores)
			for _, name := range s.Names {
				if !name.IsExported() {
					continue
				}
				switch obj := p.TypesInfo.Defs[name].(type) {
				case *types.Var:
					add(obj, kindVar, nil, ignores)
				case *types.Const:
					add(obj, kindConst, nil, ignores)
				}
			}
		case *ast.TypeSpec:
			typeIgnores := slices.Concat(own, block)
			tn, ok := p.TypesInfo.Defs[s.Name].(*types.TypeName)
			if !ok {
				continue
			}
			if s.Name.IsExported() {
				add(tn, kindType, nil, slices.Concat(typeIgnores, fileIgnores))
			}
			st, ok := s.Type.(*ast.StructType)
			if !ok || tn.IsAlias() {
				continue
			}
			for _, field := range st.Fields.List {
				fieldIgnores := slices.Concat(l.parse(field.Doc, field.Comment), typeIgnores, fileIgnores)
				for _, name := range field.Names {
					if !name.IsExported() {
						continue
					}
					if v, ok := p.TypesInfo.Defs[name].(*types.Var); ok {
						add(v, kindField, tn, fieldIgnores)
					}
				}
			}
		}
	}
}

// loadIsTestEntry reports whether go test finds a function of a _test.go file
// by this name.
func loadIsTestEntry(name string) bool {
	if name == "TestMain" {
		return true
	}
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
