// Package excluded reads the Go files of a module that the build did not
// compile, for the names they write.
//
// Nothing is type-checked here. A file the build leaves out under this GOOS
// or these tags is still code that another configuration compiles, and what
// it can tell is what it spells: the package it declares, what it imports
// under which name, and every identifier and selection in it.
package excluded

import (
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
	// selected is every name written after a dot, and qualified every x.Name
	// keyed by x.
	selected  map[string]bool
	qualified map[string]map[string]bool
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
		}
		return true
	})
	return f, nil
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
	for bound, paths := range f.imports {
		if bound == "." || bound == "_" || !slices.Contains(paths, path) {
			continue
		}
		if bound == "" {
			bound = pkgName
		}
		if f.qualified[bound][name] {
			return true
		}
	}
	return false
}

// DotImports reports whether the file imports the package at path into its
// own scope, where every name of it is written bare.
func (f *File) DotImports(path string) bool {
	return slices.Contains(f.imports["."], path)
}
