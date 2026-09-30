// Package excluded reads the Go files of a module that the build did not
// compile, for the names they write.
//
// Nothing is type-checked here. A file the build leaves out under this GOOS
// or these tags is still code that another configuration compiles, and what
// it can tell is what it spells: the package it declares, what it imports
// under which name, and every identifier and selection in it.
package excluded

import (
	"cmp"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
)

// File is one build-excluded Go file.
type File struct {
	// Dir is the file's directory, and Package the name its clause declares.
	// Together they say whether it belongs to a loaded package.
	Dir, Package string
	// Syntax is the parsed file, comments included.
	Syntax *ast.File

	// imports maps the name each import is spelled with to its path. A dot
	// import is recorded under ".", and one with no name of its own under "",
	// since the name it binds is the imported package's, which only that
	// package knows: internal/go-foo may well declare package foo.
	imports map[string][]string
	idents  map[string]bool
	// selected is every name that may name a field or a method: written
	// after a dot, or as a key of a composite literal. qualified is every
	// x.Name keyed by x.
	selected  map[string]bool
	qualified map[string]map[string]bool
	// positional is every type a composite literal fills by position, keyed
	// like qualified: x.T under x, and a bare T under "".
	positional map[string]map[string]bool
	// read is every composite literal already recorded, since one whose type
	// is elided is recorded with the literal around it.
	read map[*ast.CompositeLit]bool
}

// Read parses the file at path.
func Read(fset *token.FileSet, path string) (*File, error) {
	syntax, err := parser.ParseFile(fset, path, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	f := &File{
		Dir: filepath.Dir(path), Package: syntax.Name.Name, Syntax: syntax,
		imports: map[string][]string{}, idents: map[string]bool{},
		selected: map[string]bool{}, qualified: map[string]map[string]bool{},
		positional: map[string]map[string]bool{}, read: map[*ast.CompositeLit]bool{},
	}
	for _, spec := range syntax.Imports {
		// The parser has checked the literal, so unquoting cannot fail.
		p, _ := strconv.Unquote(spec.Path.Value)
		name := ""
		if spec.Name != nil {
			name = spec.Name.Name
		}
		f.imports[name] = append(f.imports[name], p)
	}
	ast.Inspect(syntax, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.Ident:
			f.idents[n.Name] = true
		case *ast.SelectorExpr:
			f.selected[n.Sel.Name] = true
			if x, ok := n.X.(*ast.Ident); ok {
				if f.qualified[x.Name] == nil {
					f.qualified[x.Name] = map[string]bool{}
				}
				f.qualified[x.Name][n.Sel.Name] = true
			}
		case *ast.CompositeLit:
			if !f.read[n] {
				f.literal(n, nil)
			}
		}
		return true
	})
	f.read = nil
	return f, nil
}

// literal records what a composite literal writes, given the type the
// literal around it implies when its own is elided: each identifier key as a
// name that may be a field's, and, when an element has no key, the type it
// fills by position. A literal is typed by what the file spells, since the
// file is not type-checked: an element of a slice or a map is typed by the
// slice's or the map's, and a struct's fields are not known here, so an
// elided literal inside one is not typed.
func (f *File) literal(lit *ast.CompositeLit, implied ast.Expr) {
	f.read[lit] = true
	typ := implied
	if lit.Type != nil {
		typ = lit.Type
	}
	var key, elem ast.Expr
	switch t := ast.Unparen(typ).(type) {
	case *ast.ArrayType:
		elem = t.Elt
	case *ast.MapType:
		key, elem = t.Key, t.Value
	}
	byPosition := false
	for _, e := range lit.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				f.selected[id.Name] = true
			}
			f.element(kv.Key, key)
			f.element(kv.Value, elem)
			continue
		}
		byPosition = true
		f.element(e, elem)
	}
	if !byPosition {
		return
	}
	switch t := ast.Unparen(typ).(type) {
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			f.fillsByPosition(x.Name, t.Sel.Name)
		}
	case *ast.Ident:
		f.fillsByPosition("", t.Name)
	}
}

// element records an element of a composite literal that is a literal
// itself, typed by implied when its own type is elided: T{...}, or &T{...}
// for an element of type *T.
func (f *File) element(e, implied ast.Expr) {
	if star, ok := ast.Unparen(implied).(*ast.StarExpr); ok {
		implied = star.X
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.CompositeLit:
		f.literal(e, implied)
	case *ast.UnaryExpr:
		if lit, ok := e.X.(*ast.CompositeLit); ok && e.Op == token.AND {
			f.literal(lit, implied)
		}
	}
}

func (f *File) fillsByPosition(qualifier, typ string) {
	if f.positional[qualifier] == nil {
		f.positional[qualifier] = map[string]bool{}
	}
	f.positional[qualifier][typ] = true
}

// Writes reports whether the file writes name anywhere.
func (f *File) Writes(name string) bool { return f.idents[name] }

// Selects reports whether the file writes .name after something: a method
// or field of any value, or a name of any package.
func (f *File) Selects(name string) bool { return f.selected[name] }

// Qualifies reports whether the file imports the package at path and writes
// pkg.name, where pkg is the name the import binds. An import with no name of
// its own binds pkgName, the package's own name.
func (f *File) Qualifies(path, pkgName, name string) bool {
	return slices.ContainsFunc(f.bound(path, pkgName), func(bound string) bool { return f.qualified[bound][name] })
}

// FillsByPosition reports whether the file writes a composite literal of
// typ, a type of the package at path, with an element that has no key. Such
// a literal assigns every field of typ, so none of them can be unexported.
func (f *File) FillsByPosition(path, pkgName, typ string) bool {
	if f.DotImports(path) && f.positional[""][typ] {
		return true
	}
	return slices.ContainsFunc(f.bound(path, pkgName), func(bound string) bool { return f.positional[bound][typ] })
}

// bound returns every name the file's imports of the package at path bind,
// dot and blank imports aside.
func (f *File) bound(path, pkgName string) []string {
	var names []string
	for name, paths := range f.imports {
		if name == "." || name == "_" || !slices.Contains(paths, path) {
			continue
		}
		names = append(names, cmp.Or(name, pkgName))
	}
	return names
}

// DotImports reports whether the file imports the package at path into its
// own scope, where every name of it is written bare.
func (f *File) DotImports(path string) bool {
	return slices.Contains(f.imports["."], path)
}
