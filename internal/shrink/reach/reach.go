// Package reach answers, for declscope shrink, what a value of a type lets
// code reach: through reflection, once the value is in an interface, or by
// name, from code that holds the value.
//
// Both walks go through an instantiation's origin and its type arguments, so
// that a recursive generic type cannot expand without end.
package reach

import (
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Conversions returns the type of every value converted to an interface in
// the code of pkgs: SSA's MakeInterface, which is every such conversion,
// including the ones into any.
//
// Generic functions of pkgs are instantiated, so a conversion inside one is
// seen with the concrete type. A generic declared elsewhere has no body here,
// and the caller must treat what it is given as converted. A conversion that
// makes no value at run time, such as var _ io.Writer = T{}, is not here
// either, and needs not be: nothing escapes through it.
func Conversions(pkgs []*packages.Package) []types.Type {
	prog, _ := ssautil.Packages(pkgs, ssa.InstantiateGenerics)
	prog.Build()
	var out []types.Type
	for fn := range ssautil.AllFunctions(prog) {
		for _, b := range fn.Blocks {
			for _, instr := range b.Instrs {
				if mi, ok := instr.(*ssa.MakeInterface); ok {
					out = append(out, mi.X.Type())
				}
			}
		}
	}
	return out
}

// Reflect calls mark with every named type and every struct field that
// reflection can reach from a value of one of roots: every field, exported
// or not (fmt prints them all), every element, key and pointee, and the
// parameters and results of every method and function, which reflection can
// call.
//
// A field is marked for itself, not only through its owner. type Wire Record
// shares Record's fields, and a Wire escaping exposes them under either name.
func Reflect(roots []types.Type, mark func(types.Object)) {
	w := &walker{seen: map[*types.TypeName]bool{}}
	w.named = func(o *types.Named) {
		mark(o.Obj())
		ms := types.NewMethodSet(types.NewPointer(o))
		for i := range ms.Len() {
			w.signature(ms.At(i).Obj().(*types.Func).Signature())
		}
		w.walk(o.Underlying())
	}
	w.field = func(f *types.Var) {
		mark(f)
		w.walk(f.Type())
	}
	for _, r := range roots {
		w.walk(r)
	}
}

// ByName calls member with every exported method and field that code
// holding a value of one of roots can name, and typ with every named type and
// alias it reaches on the way. A member is followed into its type only when
// member returns true, so a caller can leave out one that will not stay
// exported. That code need not import the package: pub.Get().M()
// calls M on a type another module cannot spell.
//
// An embedded field is passed to member whether its type is exported or
// not, since the fields and methods it promotes are named through it. An
// interface's methods are followed for what they hand out. The value held in
// an interface was converted where it was put there, which Reflect covers.
func ByName(roots []types.Type, member func(types.Object) bool, typ func(*types.TypeName)) {
	w := &walker{seen: map[*types.TypeName]bool{}}
	w.named = func(o *types.Named) {
		typ(o.Obj())
		ms := types.NewMethodSet(types.NewPointer(o))
		for i := range ms.Len() {
			fn := ms.At(i).Obj().(*types.Func)
			if !fn.Exported() {
				continue
			}
			if member(fn.Origin()) {
				w.signature(fn.Signature())
			}
		}
		w.walk(o.Underlying())
	}
	w.field = func(f *types.Var) {
		if !f.Exported() && !f.Embedded() {
			return
		}
		if f.Exported() && !member(f) {
			return
		}
		w.walk(f.Type())
	}
	w.iface = func(t *types.Interface) {
		for i := range t.NumMethods() {
			w.signature(t.Method(i).Signature())
		}
	}
	w.alias = typ
	for _, r := range roots {
		w.walk(r)
	}
}

// walker is the traversal the two walks share. Each supplies what to do on a
// named type and on a field, and ByName what to do on an interface and on an
// alias.
type walker struct {
	seen  map[*types.TypeName]bool
	named func(*types.Named)
	field func(*types.Var)
	iface func(*types.Interface)
	alias func(*types.TypeName)
}

func (w *walker) walk(t types.Type) {
	// An alias is its own name for the type it denotes. Code holding a value
	// through type T = x[int] names T, so ByName reports T before following x.
	if a, ok := t.(*types.Alias); ok && w.alias != nil {
		w.alias(a.Obj())
	}
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		for i := range t.TypeArgs().Len() {
			w.walk(t.TypeArgs().At(i))
		}
		o := t.Origin()
		if w.seen[o.Obj()] {
			return
		}
		w.seen[o.Obj()] = true
		w.named(o)
	case *types.Pointer:
		w.walk(t.Elem())
	case *types.Slice:
		w.walk(t.Elem())
	case *types.Array:
		w.walk(t.Elem())
	case *types.Chan:
		w.walk(t.Elem())
	case *types.Map:
		w.walk(t.Key())
		w.walk(t.Elem())
	case *types.Struct:
		for i := range t.NumFields() {
			w.field(t.Field(i))
		}
	case *types.Signature:
		w.signature(t)
	case *types.Interface:
		if w.iface != nil {
			w.iface(t)
		}
	}
}

func (w *walker) signature(sig *types.Signature) {
	for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := range tup.Len() {
			w.walk(tup.At(i).Type())
		}
	}
}
