package shrink

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
)

// candidatesOf lists every exported declaration of one package that the rule
// judges, and for every ignore it read, the ignores bound beside it.
//
// p is the widest variant, which holds every declaration the others do. Left
// out: declarations in generated files, which a regeneration would rename
// back; the test functions of a _test.go file, which go test finds by name;
// interface method names, a contract every implementation outside the
// package would have to rename too; and the fields of a struct type the
// declaration does not write itself, which are another declaration's fields.
//
// Ignores are bound where the analyzer binds them, through the one
// directive.Binder both use. The ignores of one binding are siblings, as the
// analyzer's ignoreSite.siblings are, so that an ignore naming unused answers
// the same reports in both tools.
//
//declscope:package // the core judges each of them
func candidatesOf(p *packages.Package) ([]*candidate, map[token.Pos][]directive.Ignore) {
	siblings := map[token.Pos][]directive.Ignore{}
	parse := func(groups ...*ast.CommentGroup) []directive.Ignore {
		ignores := directive.ParseDecl(groups...).Ignores
		for _, ig := range ignores {
			siblings[ig.Pos] = ignores
		}
		return ignores
	}
	var out []*candidate
	add := func(obj types.Object, k kind, owner *types.TypeName, ignores []directive.Ignore, doc *ast.CommentGroup) *candidate {
		c := &candidate{
			obj: obj, key: keyOf(p.Fset, obj), kind: k, owner: owner, pkg: p, ignores: ignores, doc: doc,
			testFile: strings.HasSuffix(p.Fset.File(obj.Pos()).Name(), "_test.go"),
		}
		out = append(out, c)
		return c
	}
	for _, file := range p.Syntax {
		if ast.IsGenerated(file) {
			continue
		}
		b := directive.NewBinder(p.Fset, file)
		testFile := strings.HasSuffix(p.Fset.File(file.Pos()).Name(), "_test.go")
		fileIgnores := directive.ParseFile(file).Ignores
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if testFile && d.Recv == nil && candidateTestEntry(d.Name.Name) {
					continue
				}
				candidateFunc(p, d, slices.Concat(parse(b.Func(d)...), fileIgnores), add)
			case *ast.GenDecl:
				block := slices.Concat(parse(d.Doc), parse(b.Block(d)...))
				for _, spec := range d.Specs {
					candidateSpec(p, d, spec, slices.Concat(parse(b.Spec(spec)...), block), fileIgnores, parse, add)
				}
			}
		}
	}
	return out, siblings
}

// candidateAdd records one candidate with the ignores covering it and its
// doc comment, and returns it.
type candidateAdd func(obj types.Object, k kind, owner *types.TypeName, ignores []directive.Ignore, doc *ast.CommentGroup) *candidate

// candidateFunc records an exported func, or an exported method of a named
// type that is not an interface.
func candidateFunc(p *packages.Package, d *ast.FuncDecl, ignores []directive.Ignore, add candidateAdd) {
	fn, ok := p.TypesInfo.Defs[d.Name].(*types.Func)
	if !ok || !d.Name.IsExported() {
		return
	}
	recv := fn.Signature().Recv()
	if recv == nil {
		add(fn, kindFunc, nil, ignores, d.Doc)
		return
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok && !types.IsInterface(named) {
		add(fn, kindMethod, named.Origin().Obj(), ignores, d.Doc)
	}
}

// candidateSpec records the exported names of one spec, and the exported
// fields a type spec writes. own holds the ignores of the spec and its
// block, and fileIgnores those of the file.
func candidateSpec(p *packages.Package, d *ast.GenDecl, spec ast.Spec, own, fileIgnores []directive.Ignore,
	parse func(...*ast.CommentGroup) []directive.Ignore, add candidateAdd,
) {
	switch s := spec.(type) {
	case *ast.ValueSpec:
		doc := candidateDoc(d, s.Doc, len(s.Names))
		for _, name := range s.Names {
			if !name.IsExported() {
				continue
			}
			switch obj := p.TypesInfo.Defs[name].(type) {
			case *types.Var:
				add(obj, kindVar, nil, slices.Concat(own, fileIgnores), doc)
			case *types.Const:
				add(obj, kindConst, nil, slices.Concat(own, fileIgnores), doc)
			}
		}
	case *ast.TypeSpec:
		tn, ok := p.TypesInfo.Defs[s.Name].(*types.TypeName)
		if !ok {
			return
		}
		if s.Name.IsExported() {
			add(tn, kindType, nil, slices.Concat(own, fileIgnores), candidateDoc(d, s.Doc, 1))
		}
		st, ok := s.Type.(*ast.StructType)
		if !ok || tn.IsAlias() {
			return
		}
		// A field is covered by its own ignores and by its type's, as the
		// analyzer's suppression chain has it.
		for _, field := range st.Fields.List {
			ignores := slices.Concat(parse(field.Doc, field.Comment), own, fileIgnores)
			for _, name := range field.Names {
				if v, ok := p.TypesInfo.Defs[name].(*types.Var); ok && name.IsExported() {
					add(v, kindField, tn, ignores, candidateDoc(nil, field.Doc, len(field.Names)))
				}
			}
		}
	}
}

// candidateDoc returns the doc comment of a declaration of names names: its
// own, or without parentheses the declaration's. A comment over `a, b int`
// opens with at most one of the names, and belongs to both, so none is
// returned for it.
func candidateDoc(d *ast.GenDecl, own *ast.CommentGroup, names int) *ast.CommentGroup {
	if names != 1 {
		return nil
	}
	if own == nil && d != nil && !d.Lparen.IsValid() {
		return d.Doc
	}
	return own
}

// candidateTestEntry reports whether go test finds a function of a _test.go
// file by this name.
func candidateTestEntry(name string) bool {
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
