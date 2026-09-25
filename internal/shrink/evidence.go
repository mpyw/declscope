package shrink

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
	"golang.org/x/tools/refactor/satisfy"

	"github.com/mpyw/declscope/internal/shrink/module"
	"github.com/mpyw/declscope/internal/shrink/reach"
)

// evidence is everything the module says about who uses what. Every map is
// keyed by keyOf, so the variants of one package agree on a declaration.
//
// Each is one reach path from spec/shrink.fsl: outside and extTest are the
// Outside variable, satisfied is staticSatisfaction, exposed is exposed,
// escaped is the Escapes doubt, and generated is generatedRef.
//
//declscope:package // the core judges from it, and rename.go reads its sites
type evidence struct {
	// outside is every declaration a package other than its own names,
	// external tests aside: spelled, written in an unkeyed literal, or
	// linked by name.
	outside map[string]bool
	// extTest is every declaration an external test package names.
	extTest map[string]bool
	// generated is every declaration a generated file of its own package
	// names. The rename cannot keep that file renamed.
	generated map[string]bool
	// example is every declaration an example function of its own package
	// names by its own name: ExampleF, ExampleT_M. go vet checks that the
	// name resolves, and the rename cannot rename a function's name.
	example map[string]bool
	// satisfied is every method a static interface satisfaction needs.
	satisfied map[string]bool
	// paired is every field a struct conversion or an identical unnamed
	// struct type pairs by name with a field somewhere else.
	paired map[string]bool
	// escaped is every type a value that reflection may inspect reaches.
	escaped map[string]bool
	// exposed is every method and field another module can reach through a
	// value that the exported API of an importable package hands out.
	exposed map[string]bool
	// carried is every type that a declaration used from another package
	// carries in its type: a result, a parameter, a variable's type, an
	// exported field or method of a type used there. The other package holds
	// values of it, and unexporting it would leave that API returning a type
	// its callers cannot name.
	carried map[string]bool
	// sites is every identifier naming a declaration inside its own
	// package, which is everything a rename rewrites.
	sites map[string][]evidenceSite
}

// evidenceSite is one identifier naming a declaration, in the package
// variant whose scopes it resolves in.
//
//declscope:package // rename.go rewrites and checks each one
type evidenceSite struct {
	pkg   *packages.Package
	ident *ast.Ident
}

// evidenceCollect gathers the evidence over every package of the module.
//
//declscope:package // the core collects it once per run
func evidenceCollect(m *module.Module) (ev *evidence, err error) {
	ev = &evidence{
		outside: map[string]bool{}, extTest: map[string]bool{}, generated: map[string]bool{}, example: map[string]bool{},
		satisfied: map[string]bool{}, paired: map[string]bool{}, escaped: map[string]bool{},
		exposed: map[string]bool{}, carried: map[string]bool{}, sites: map[string][]evidenceSite{},
	}
	inModule := map[string]bool{}
	for _, p := range m.Pkgs {
		inModule[p.Types.Path()] = true
	}
	// One variant per import path: the widest holds every file the others
	// do, parsed once and shared, so reading the others would find the same
	// references, satisfactions and conversions again.
	var roots []types.Type
	for _, p := range m.Widest() {
		ev.references(m, p)
		ev.examples(m, p)
		roots = append(roots, ev.instances(m, p, inModule)...)
		if err := ev.satisfactions(m, p); err != nil {
			return nil, err
		}
	}
	ev.linknames(m)
	ev.unnamedStructs(m)
	// What escapes into an interface: every conversion SSA sees, and every
	// type argument of a generic whose body it cannot see.
	roots = append(roots, reach.Conversions(m.Widest())...)
	reach.Reflect(roots, func(obj types.Object) { ev.escaped[keyOf(m.Fset, obj)] = true })
	ev.exposure(m)
	ev.carry(m)
	return ev, nil
}

// evidenceClass says where a reference stands relative to the package that
// declares what it names.
type evidenceClass int

const (
	evidenceSame evidenceClass = iota
	evidenceExtTest
	evidenceOutside
)

func evidenceClassify(p *packages.Package, declPath string) evidenceClass {
	switch {
	case p.PkgPath == declPath:
		return evidenceSame
	case p.ForTest == declPath && p.PkgPath == declPath+"_test":
		return evidenceExtTest
	}
	return evidenceOutside
}

func (ev *evidence) record(m *module.Module, p *packages.Package, obj types.Object, id *ast.Ident, gen bool) {
	if obj == nil || obj.Pkg() == nil {
		return
	}
	key := keyOf(m.Fset, obj)
	if key == "" {
		return
	}
	switch evidenceClassify(p, obj.Pkg().Path()) {
	case evidenceSame:
		ev.sites[key] = append(ev.sites[key], evidenceSite{pkg: p, ident: id})
		if gen {
			ev.generated[key] = true
		}
	case evidenceExtTest:
		ev.extTest[key] = true
	case evidenceOutside:
		ev.outside[key] = true
	}
}

// references records every identifier of one package variant, and
// every unkeyed literal and struct conversion in it.
func (ev *evidence) references(m *module.Module, p *packages.Package) {
	info := p.TypesInfo
	for _, file := range p.Syntax {
		gen := ast.IsGenerated(file)
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				// An embedded field's identifier is a definition of the field
				// and a use of the type at once, so both maps are read.
				if obj := info.Defs[n]; obj != nil {
					ev.record(m, p, origin(obj), n, gen)
				}
				if obj := info.Uses[n]; obj != nil {
					obj = origin(obj)
					ev.record(m, p, obj, n, gen)
					// s.T selects an embedded field by the type's name.
					if _, isDef := info.Defs[n]; !isDef {
						if tn := embeddedTypeName(obj); tn != nil {
							ev.record(m, p, tn, n, gen)
						}
					}
				}
			case *ast.CompositeLit:
				ev.unkeyed(m, p, n)
			case *ast.CallExpr:
				ev.conversion(m, p, n)
			}
			return true
		})
	}
}

// examples records what the example functions of a test file name.
// ExampleF names F, ExampleT names T, and ExampleT_M names T's method or
// field M, each with an optional _suffix starting with a lower-case letter.
// An example in the package itself stops the rename; one in the external
// test package is a use by external tests, like any other reference there.
func (ev *evidence) examples(m *module.Module, p *packages.Package) {
	// go vet resolves an external test's example against the package under
	// test, whether or not the file imports it. Every variant of that package
	// keys its declarations alike, so any of them answers.
	subject := p.Types
	if p.ForTest != "" && p.PkgPath != p.ForTest {
		variants := m.Variants(p.ForTest)
		if len(variants) == 0 {
			return
		}
		subject = variants[0].Types
	}
	for _, file := range p.Syntax {
		if !strings.HasSuffix(m.Fset.File(file.Pos()).Name(), "_test.go") {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			rest, ok := strings.CutPrefix(fn.Name.Name, "Example")
			if !ok || rest == "" {
				continue
			}
			parts := strings.Split(rest, "_")
			obj := subject.Scope().Lookup(parts[0])
			if obj == nil {
				continue
			}
			named := []types.Object{obj}
			if len(parts) > 1 && ast.IsExported(parts[1]) {
				if found, _, _ := types.LookupFieldOrMethod(obj.Type(), true, subject, parts[1]); found != nil {
					named = append(named, origin(found))
				}
			}
			for _, o := range named {
				if subject == p.Types {
					ev.example[keyOf(m.Fset, o)] = true
				} else {
					ev.extTest[keyOf(m.Fset, o)] = true
				}
			}
		}
	}
}

// unkeyed records the fields an unkeyed composite literal writes by
// position. Another package can write one only while every field is
// exported, so the literal is a use of each.
func (ev *evidence) unkeyed(m *module.Module, p *packages.Package, lit *ast.CompositeLit) {
	if len(lit.Elts) == 0 {
		return
	}
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
		return
	}
	st, owner := evidenceStructOf(p.TypesInfo.TypeOf(lit))
	if st == nil || owner == nil || owner.Pkg() == nil {
		return
	}
	if evidenceClassify(p, owner.Pkg().Path()) == evidenceSame {
		return
	}
	for i := range min(len(lit.Elts), st.NumFields()) {
		if evidenceClassify(p, owner.Pkg().Path()) == evidenceExtTest {
			ev.extTest[keyOf(m.Fset, st.Field(i))] = true
		} else {
			ev.outside[keyOf(m.Fset, st.Field(i))] = true
		}
	}
}

// conversion records the fields a struct conversion pairs. The two
// struct types must name their fields alike, so renaming one side breaks it
// wherever it is written.
func (ev *evidence) conversion(m *module.Module, p *packages.Package, call *ast.CallExpr) {
	tv, ok := p.TypesInfo.Types[call.Fun]
	if !ok || !tv.IsType() || len(call.Args) != 1 {
		return
	}
	to, _ := evidenceStructOf(tv.Type)
	from, _ := evidenceStructOf(p.TypesInfo.TypeOf(call.Args[0]))
	if to == nil || from == nil || to == from {
		return
	}
	for _, st := range []*types.Struct{to, from} {
		for i := range st.NumFields() {
			ev.paired[keyOf(m.Fset, st.Field(i))] = true
		}
	}
}

// evidenceStructOf returns the declared struct type behind t, through a
// pointer and an instantiation, and the type name declaring it.
func evidenceStructOf(t types.Type) (*types.Struct, *types.TypeName) {
	if t == nil {
		return nil, nil
	}
	if ptr, ok := types.Unalias(t).(*types.Pointer); ok {
		t = ptr.Elem()
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		st, _ := t.Origin().Underlying().(*types.Struct)
		return st, t.Origin().Obj()
	case *types.Struct:
		return t, nil
	}
	return nil, nil
}

// unnamedStructs pairs the fields of every named struct type whose
// struct is identical to an unnamed struct type written somewhere: a value
// of one is assignable to the other only while the field names agree.
//
// An unnamed struct is looked for inside every type an expression has, not
// only at the top. One in a parameter of a dependency's function, loaded
// from export data, appears only inside the type of the expression naming
// that function. Identical ones are kept once, so a table-driven test
// writing the same struct in every row costs one comparison.
func (ev *evidence) unnamedStructs(m *module.Module) {
	var unnamed typeutil.Map
	var named []*types.Struct
	seenNamed := map[*types.Struct]bool{}
	for _, p := range m.Widest() {
		// The struct literal a defined type is declared with is recorded as a
		// type expression too, but it is that type's own struct, not an
		// unnamed one it could be assigned to.
		defining := map[ast.Expr]bool{}
		for _, file := range p.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if ts, ok := n.(*ast.TypeSpec); ok && !ts.Assign.IsValid() {
					defining[ts.Type] = true
				}
				return true
			})
		}
		for expr, tv := range p.TypesInfo.Types {
			if !defining[expr] {
				evidenceCollectUnnamed(tv.Type, &unnamed, map[types.Type]bool{})
			}
		}
		for _, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			if st, ok := tn.Type().Underlying().(*types.Struct); ok && !seenNamed[st] {
				seenNamed[st] = true
				named = append(named, st)
			}
		}
	}
	// Only structs with the same field names in the same order can be
	// identical, so each named struct is compared with its bucket alone.
	buckets := map[string][]*types.Struct{}
	unnamed.Iterate(func(t types.Type, _ any) {
		u := t.(*types.Struct)
		buckets[evidenceFieldNames(u)] = append(buckets[evidenceFieldNames(u)], u)
	})
	for _, st := range named {
		for _, u := range buckets[evidenceFieldNames(st)] {
			if u != st && types.IdenticalIgnoreTags(st, u) {
				for i := range st.NumFields() {
					ev.paired[keyOf(m.Fset, st.Field(i))] = true
				}
				break
			}
		}
	}
}

// evidenceFieldNames spells a struct's field names in order, which two
// identical structs share.
func evidenceFieldNames(st *types.Struct) string {
	names := make([]string, st.NumFields())
	for i := range st.NumFields() {
		names[i] = st.Field(i).Name()
	}
	return strings.Join(names, ",")
}

// evidenceCollectUnnamed adds every unnamed struct type inside t. A named
// type is not entered: its struct is its own, and its fields are declared
// where it is.
func evidenceCollectUnnamed(t types.Type, into *typeutil.Map, seen map[types.Type]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	switch t := types.Unalias(t).(type) {
	case *types.Struct:
		if t.NumFields() > 0 {
			into.Set(t, true)
		}
		for i := range t.NumFields() {
			evidenceCollectUnnamed(t.Field(i).Type(), into, seen)
		}
	case *types.Pointer:
		evidenceCollectUnnamed(t.Elem(), into, seen)
	case *types.Slice:
		evidenceCollectUnnamed(t.Elem(), into, seen)
	case *types.Array:
		evidenceCollectUnnamed(t.Elem(), into, seen)
	case *types.Chan:
		evidenceCollectUnnamed(t.Elem(), into, seen)
	case *types.Map:
		evidenceCollectUnnamed(t.Key(), into, seen)
		evidenceCollectUnnamed(t.Elem(), into, seen)
	case *types.Signature:
		evidenceCollectUnnamed(t.Params(), into, seen)
		evidenceCollectUnnamed(t.Results(), into, seen)
	case *types.Tuple:
		for i := range t.Len() {
			evidenceCollectUnnamed(t.At(i).Type(), into, seen)
		}
	}
}

// instances records the methods a type argument supplies to a
// constraint, and returns the type arguments given to a generic declared
// outside the module. Its body is not built here, so what it does with the
// value is unknown, and the value is taken to escape.
func (ev *evidence) instances(m *module.Module, p *packages.Package, inModule map[string]bool) []types.Type {
	var roots []types.Type
	for id, inst := range p.TypesInfo.Instances {
		obj := p.TypesInfo.Uses[id]
		var tparams *types.TypeParamList
		switch o := obj.(type) {
		case *types.Func:
			tparams = o.Signature().TypeParams()
		case *types.TypeName:
			if named, ok := o.Type().(*types.Named); ok {
				tparams = named.Origin().TypeParams()
			}
		}
		external := obj == nil || obj.Pkg() == nil || !inModule[obj.Pkg().Path()]
		for i := range inst.TypeArgs.Len() {
			targ := inst.TypeArgs.At(i)
			if external {
				roots = append(roots, targ)
			}
			if tparams == nil || i >= tparams.Len() {
				continue
			}
			if iface, ok := tparams.At(i).Constraint().Underlying().(*types.Interface); ok {
				ev.markRequired(m, iface, targ)
			}
		}
	}
	return roots
}

// satisfactions records every method a static interface satisfaction
// in the package variant needs: an assignment, an argument, a return, a
// comparison, a type assertion, anywhere the compiler checks that a type
// implements an interface.
func (ev *evidence) satisfactions(m *module.Module, p *packages.Package) (err error) {
	// satisfy requires well-typed input and may panic otherwise. The load
	// refuses a package with errors, so a panic here is a bug to report
	// rather than a finding to guess past.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("finding interface satisfactions in %s: %v", p.ID, r)
		}
	}()
	f := satisfy.Finder{Result: map[satisfy.Constraint]bool{}}
	f.Find(p.TypesInfo, p.Syntax)
	for c := range f.Result {
		if iface, ok := c.LHS.Underlying().(*types.Interface); ok {
			ev.markRequired(m, iface, c.RHS)
		}
	}
	return nil
}

// markRequired records the methods of t that iface requires.
func (ev *evidence) markRequired(m *module.Module, iface *types.Interface, t types.Type) {
	for i := range iface.NumMethods() {
		want := iface.Method(i)
		obj, _, _ := types.LookupFieldOrMethod(t, true, want.Pkg(), want.Name())
		if fn, ok := obj.(*types.Func); ok && fn != want {
			ev.satisfied[keyOf(m.Fset, origin(fn))] = true
		}
	}
}

// linknames records every declaration a //go:linkname anywhere in the
// module pulls by its import path, compiled or not.
func (ev *evidence) linknames(m *module.Module) {
	byPath := map[string]map[string]string{}
	add := func(file *ast.File) {
		for _, g := range file.Comments {
			for _, c := range g.List {
				d, ok := ast.ParseDirective(c.Pos(), c.Text)
				if !ok || d.Tool != "go" || d.Name != "linkname" {
					continue
				}
				fields := strings.Fields(d.Args)
				if len(fields) < 2 {
					continue
				}
				target := fields[1]
				slash := strings.LastIndex(target, "/")
				dot := strings.Index(target[slash+1:], ".")
				if dot < 0 {
					continue
				}
				path, name := target[:slash+1+dot], target[slash+2+dot:]
				if byPath[path] == nil {
					byPath[path] = map[string]string{}
				}
				byPath[path][name] = name
			}
		}
	}
	for _, p := range m.Widest() {
		for _, f := range p.Syntax {
			add(f)
		}
	}
	for _, x := range m.Excluded {
		add(x.Syntax)
	}
	for path, names := range byPath {
		for _, p := range m.Variants(path) {
			scope := p.Types.Scope()
			for name := range names {
				typ, member, isMember := evidenceLinknameMember(name)
				obj := scope.Lookup(typ)
				if obj == nil {
					continue
				}
				if !isMember {
					ev.outside[keyOf(m.Fset, obj)] = true
					continue
				}
				// path.T.M spells T as well, so the type keeps its name too.
				ev.outside[keyOf(m.Fset, obj)] = true
				if found, _, _ := types.LookupFieldOrMethod(obj.Type(), true, p.Types, member); found != nil {
					ev.outside[keyOf(m.Fset, origin(found))] = true
				}
			}
		}
	}
}

// evidenceLinknameMember splits the part of a linkname target after the
// package path: F, T.M, or (*T).M for a method on a pointer receiver.
func evidenceLinknameMember(name string) (typ, member string, isMember bool) {
	if rest, ok := strings.CutPrefix(name, "(*"); ok {
		typ, member, isMember = strings.Cut(rest, ").")
		return typ, member, isMember
	}
	return strings.Cut(name, ".")
}

// exposure marks every method and field another module can reach, and every
// embedded type it can name as a field.
//
// The roots are the exported declarations of every package a module this run
// does not load may import: every package whose range the module cannot vouch
// for. That is a package outside internal/, and also an internal package
// whose parent lies above the module or holds a nested module that may import
// it. "No internal element in its path" is not the test: such a package can
// hand a value out to another module just the same. package main is always
// among the roots, inside internal/ or not, since -buildmode=plugin looks its
// exported symbols up.
func (ev *evidence) exposure(m *module.Module) {
	var roots []types.Type
	for _, p := range m.Widest() {
		if _, why := m.Range(p.PkgPath); why == "" && p.Name != "main" {
			continue
		}
		// An external test package is imported by nothing.
		if p.ForTest != "" && p.PkgPath != p.ForTest {
			continue
		}
		scope := p.Types.Scope()
		for _, name := range scope.Names() {
			if obj := scope.Lookup(name); obj.Exported() {
				roots = append(roots, obj.Type())
			}
		}
	}
	reach.ByName(roots, func(obj types.Object) {
		ev.exposed[keyOf(m.Fset, obj)] = true
		// An embedded field is named by its type, so another module selecting
		// it (v.Inner, or Inner: in a literal) spells the type's name.
		if tn := embeddedTypeName(obj); tn != nil {
			ev.exposed[keyOf(m.Fset, tn)] = true
		}
	}, func(*types.TypeName) {})
}

// carry marks every type a declaration used from another package carries out
// in its type. The roots are those declarations, found in the reference
// index. The walk follows what the other package can reach by name, as
// exposure does, but keeps the types rather than the members: every member
// the other package actually uses is already in the index, since this run
// sees the whole range.
func (ev *evidence) carry(m *module.Module) {
	var roots []types.Type
	for _, p := range m.Widest() {
		for _, obj := range p.TypesInfo.Defs {
			if obj != nil && ev.outside[keyOf(m.Fset, obj)] {
				roots = append(roots, obj.Type())
			}
		}
	}
	reach.ByName(roots, func(types.Object) {}, func(tn *types.TypeName) {
		ev.carried[keyOf(m.Fset, tn)] = true
	})
}
