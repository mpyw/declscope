package shrink

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
	"golang.org/x/tools/refactor/satisfy"
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
func evidenceCollect(m *loadedModule) (ev *evidence, err error) {
	ev = &evidence{
		outside: map[string]bool{}, extTest: map[string]bool{}, generated: map[string]bool{}, example: map[string]bool{},
		satisfied: map[string]bool{}, paired: map[string]bool{}, escaped: map[string]bool{},
		exposed: map[string]bool{}, sites: map[string][]evidenceSite{},
	}
	inModule := map[string]bool{}
	for _, p := range m.pkgs {
		inModule[p.Types.Path()] = true
	}
	var roots []types.Type
	for _, p := range m.pkgs {
		ev.evidenceReferences(m, p)
		ev.evidenceExamples(m, p)
		roots = append(roots, ev.evidenceInstances(m, p, inModule)...)
		if err := ev.evidenceSatisfactions(m, p); err != nil {
			return nil, err
		}
	}
	ev.evidenceLinknames(m)
	ev.evidenceUnnamedStructs(m)
	roots = append(roots, evidenceInterfaceConversions(m)...)
	evidenceWalk(m, roots, false, ev.escaped)
	ev.evidenceExposure(m)
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

func (ev *evidence) evidenceRecord(m *loadedModule, p *packages.Package, obj types.Object, id *ast.Ident, gen bool) {
	if obj == nil || obj.Pkg() == nil {
		return
	}
	key := keyOf(m.fset, obj)
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

// evidenceReferences records every identifier of one package variant, and
// every unkeyed literal and struct conversion in it.
func (ev *evidence) evidenceReferences(m *loadedModule, p *packages.Package) {
	info := p.TypesInfo
	for _, file := range p.Syntax {
		gen := ast.IsGenerated(file)
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.Ident:
				// An embedded field's identifier is a definition of the field
				// and a use of the type at once, so both maps are read.
				if obj := info.Defs[n]; obj != nil {
					ev.evidenceRecord(m, p, origin(obj), n, gen)
				}
				if obj := info.Uses[n]; obj != nil {
					obj = origin(obj)
					ev.evidenceRecord(m, p, obj, n, gen)
					// s.T selects an embedded field by the type's name.
					if _, isDef := info.Defs[n]; !isDef {
						if tn := embeddedTypeName(obj); tn != nil {
							ev.evidenceRecord(m, p, tn, n, gen)
						}
					}
				}
			case *ast.CompositeLit:
				ev.evidenceUnkeyed(m, p, n)
			case *ast.CallExpr:
				ev.evidenceConversion(m, p, n)
			}
			return true
		})
	}
}

// evidenceExamples records what the example functions of a test file name.
// ExampleF names F, ExampleT names T, and ExampleT_M names T's method or
// field M, each with an optional _suffix starting with a lower-case letter.
// An example in the package itself stops the rename; one in the external
// test package is a use by external tests, like any other reference there.
func (ev *evidence) evidenceExamples(m *loadedModule, p *packages.Package) {
	// go vet resolves an external test's example against the package under
	// test, whether or not the file imports it. Every variant of that package
	// keys its declarations alike, so any of them answers.
	subject := p.Types
	if p.ForTest != "" && p.PkgPath != p.ForTest {
		variants := m.byPath[p.ForTest]
		if len(variants) == 0 {
			return
		}
		subject = variants[0].Types
	}
	for _, file := range p.Syntax {
		if !strings.HasSuffix(m.fset.Position(file.Pos()).Filename, "_test.go") {
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
					ev.example[keyOf(m.fset, o)] = true
				} else {
					ev.extTest[keyOf(m.fset, o)] = true
				}
			}
		}
	}
}

// evidenceUnkeyed records the fields an unkeyed composite literal writes by
// position. Another package can write one only while every field is
// exported, so the literal is a use of each.
func (ev *evidence) evidenceUnkeyed(m *loadedModule, p *packages.Package, lit *ast.CompositeLit) {
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
			ev.extTest[keyOf(m.fset, st.Field(i))] = true
		} else {
			ev.outside[keyOf(m.fset, st.Field(i))] = true
		}
	}
}

// evidenceConversion records the fields a struct conversion pairs. The two
// struct types must name their fields alike, so renaming one side breaks it
// wherever it is written.
func (ev *evidence) evidenceConversion(m *loadedModule, p *packages.Package, call *ast.CallExpr) {
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
			ev.paired[keyOf(m.fset, st.Field(i))] = true
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

// evidenceUnnamedStructs pairs the fields of every named struct type whose
// struct is identical to an unnamed struct type written somewhere: a value
// of one is assignable to the other only while the field names agree.
func (ev *evidence) evidenceUnnamedStructs(m *loadedModule) {
	var unnamed []*types.Struct
	var named []*types.Struct
	for _, p := range m.pkgs {
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
			if defining[expr] {
				continue
			}
			if st, ok := types.Unalias(tv.Type).(*types.Struct); ok && st.NumFields() > 0 {
				unnamed = append(unnamed, st)
			}
		}
		for _, obj := range p.TypesInfo.Defs {
			tn, ok := obj.(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				named = append(named, st)
			}
		}
	}
	for _, st := range named {
		for _, u := range unnamed {
			if u != st && types.IdenticalIgnoreTags(st, u) {
				for i := range st.NumFields() {
					ev.paired[keyOf(m.fset, st.Field(i))] = true
				}
				break
			}
		}
	}
}

// evidenceInstances records the methods a type argument supplies to a
// constraint, and returns the type arguments given to a generic declared
// outside the module. Its body is not built here, so what it does with the
// value is unknown, and the value is taken to escape.
func (ev *evidence) evidenceInstances(m *loadedModule, p *packages.Package, inModule map[string]bool) []types.Type {
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
				ev.evidenceMarkRequired(m, iface, targ)
			}
		}
	}
	return roots
}

// evidenceSatisfactions records every method a static interface satisfaction
// in the package variant needs: an assignment, an argument, a return, a
// comparison, a type assertion, anywhere the compiler checks that a type
// implements an interface.
func (ev *evidence) evidenceSatisfactions(m *loadedModule, p *packages.Package) (err error) {
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
			ev.evidenceMarkRequired(m, iface, c.RHS)
		}
	}
	return nil
}

// evidenceMarkRequired records the methods of t that iface requires.
func (ev *evidence) evidenceMarkRequired(m *loadedModule, iface *types.Interface, t types.Type) {
	for i := range iface.NumMethods() {
		want := iface.Method(i)
		obj, _, _ := types.LookupFieldOrMethod(t, true, want.Pkg(), want.Name())
		if fn, ok := obj.(*types.Func); ok && fn != want {
			ev.satisfied[keyOf(m.fset, origin(fn))] = true
		}
	}
}

// evidenceLinknames records every declaration a //go:linkname anywhere in the
// module pulls by its import path, compiled or not.
func (ev *evidence) evidenceLinknames(m *loadedModule) {
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
	for _, p := range m.pkgs {
		for _, f := range p.Syntax {
			add(f)
		}
	}
	for _, x := range m.excluded {
		add(x.file)
	}
	for path, names := range byPath {
		for _, p := range m.byPath[path] {
			scope := p.Types.Scope()
			for name := range names {
				typ, member, isMember := strings.Cut(name, ".")
				obj := scope.Lookup(typ)
				if obj == nil {
					continue
				}
				if !isMember {
					ev.outside[keyOf(m.fset, obj)] = true
					continue
				}
				// path.T.M spells T as well, so the type keeps its name too.
				ev.outside[keyOf(m.fset, obj)] = true
				if found, _, _ := types.LookupFieldOrMethod(obj.Type(), true, p.Types, member); found != nil {
					ev.outside[keyOf(m.fset, origin(found))] = true
				}
			}
		}
	}
}

// evidenceInterfaceConversions returns the type of every value converted to
// an interface anywhere in the module: SSA's MakeInterface, which is every
// such conversion including the ones into any. Once a value is in an
// interface, fmt, encoding/json and reflect can find its methods and fields at
// run time, and an assertion can find it another interface to satisfy.
//
// Generic functions of the module are instantiated, so a conversion inside
// one is seen with the concrete type. One declared outside the module has no
// body here, and evidenceInstances covers it.
func evidenceInterfaceConversions(m *loadedModule) []types.Type {
	prog, _ := ssautil.Packages(m.pkgs, ssa.InstantiateGenerics)
	prog.Build()
	var roots []types.Type
	for fn := range ssautil.AllFunctions(prog) {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if mi, ok := instr.(*ssa.MakeInterface); ok {
					roots = append(roots, mi.X.Type())
				}
			}
		}
	}
	return roots
}

// evidenceWalk marks every named type reachable from roots in into.
//
// With exportedOnly false it follows everything reflection can: every field,
// every element, key and pointee, and the parameters and results of every
// method and function type. With it true it follows only what another module
// can reach by name, which is how exposure is walked: exported fields and
// methods, and fields and methods an embedding promotes. Exposure also marks
// each such member, since a member is what another module uses.
//
// An instantiation is walked through its origin and its type arguments, so
// that a recursive generic type cannot expand without end.
func evidenceWalk(m *loadedModule, roots []types.Type, exportedOnly bool, into map[string]bool) {
	seen := map[string]bool{}
	var walk func(t types.Type)
	walkSig := func(sig *types.Signature) {
		for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
			for i := range tup.Len() {
				walk(tup.At(i).Type())
			}
		}
	}
	walk = func(t types.Type) {
		switch t := types.Unalias(t).(type) {
		case *types.Named:
			for i := range t.TypeArgs().Len() {
				walk(t.TypeArgs().At(i))
			}
			o := t.Origin()
			key := keyOf(m.fset, o.Obj())
			if key == "" || seen[key] {
				return
			}
			seen[key] = true
			if !exportedOnly {
				into[key] = true
			}
			ms := types.NewMethodSet(types.NewPointer(o))
			for i := range ms.Len() {
				fn, ok := ms.At(i).Obj().(*types.Func)
				if !ok || exportedOnly && !fn.Exported() {
					continue
				}
				if exportedOnly {
					into[keyOf(m.fset, origin(fn))] = true
				}
				walkSig(fn.Signature())
			}
			walk(o.Underlying())
		case *types.Pointer:
			walk(t.Elem())
		case *types.Slice:
			walk(t.Elem())
		case *types.Array:
			walk(t.Elem())
		case *types.Chan:
			walk(t.Elem())
		case *types.Map:
			walk(t.Key())
			walk(t.Elem())
		case *types.Struct:
			for i := range t.NumFields() {
				f := t.Field(i)
				if exportedOnly && !f.Exported() && !f.Embedded() {
					continue
				}
				if exportedOnly && f.Exported() {
					into[keyOf(m.fset, f)] = true
				}
				walk(f.Type())
			}
		case *types.Signature:
			walkSig(t)
		case *types.Interface:
			// A value in an interface was converted at its own MakeInterface,
			// which is a root of its own. What another module can call on one
			// is the interface's methods, whose signatures carry values.
			if exportedOnly {
				for i := range t.NumMethods() {
					walkSig(t.Method(i).Signature())
				}
			}
		}
	}
	for _, r := range roots {
		walk(r)
	}
}

// evidenceExposure marks every method and field another module can reach.
// The roots are the exported declarations of every package another module
// can import: one with no internal element in its path. package main is
// among them, since -buildmode=plugin looks its exported symbols up.
func (ev *evidence) evidenceExposure(m *loadedModule) {
	var roots []types.Type
	for _, path := range m.paths {
		if _, internal := loadInternalParent(path); internal {
			continue
		}
		// An external test package is imported by nothing.
		p := m.byPath[path][0]
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
	evidenceWalk(m, roots, true, ev.exposed)
}
