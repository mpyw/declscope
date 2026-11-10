package shrink

import (
	"go/ast"
	"path/filepath"
	"slices"

	"golang.org/x/tools/go/packages"

	"github.com/mpyw/declscope/internal/directive"
	"github.com/mpyw/declscope/internal/shrink/excluded"
)

// exclusionCache holds what the build-excluded files say about each package
// asked about so far. It is built on first use.
//
//declscope:shared // run holds it
type exclusionCache struct {
	// byPath caches, per package path, what the build-excluded files say
	// about the package.
	//
	//declscope:private // only exclusionsOf reads it
	byPath map[string]*exclusions
	// byPlace holds the build-excluded files by where they belong, and
	// loadedAt holds the path of the loaded package at each place.
	//
	//declscope:private // only exclusionsOf reads it
	byPlace map[exclusionPlace][]*excluded.File
	//declscope:private // only exclusionsOf reads it
	loadedAt map[exclusionPlace]string
}

// exclusions is what the build-excluded files say about one package,
// gathered once rather than once per candidate: every candidate of a package
// asks the same files the same questions.
type exclusions struct {
	// own is every file of the package itself, and linknamed every name they
	// bind with //go:linkname or //export.
	own       []*excluded.File
	linknamed map[string]bool
	// importing is every file of another package that imports the package,
	// and dotImporting every one that imports it into its own scope.
	importing, dotImporting []*excluded.File
	// reached says whether a file of another package imports a path that
	// reaches the package (module.Reaches), or belongs to a loaded package
	// that does.
	reached bool
}

// exclusionsOf returns what the build-excluded files say about the package
// p, built on first use.
//
//declscope:shared // the core and rename.go ask it about a candidate's package
func (r *run) exclusionsOf(p *packages.Package) *exclusions {
	cache := r.exclusionCache
	if cache == nil {
		cache = &exclusionCache{
			byPath:   map[string]*exclusions{},
			byPlace:  map[exclusionPlace][]*excluded.File{},
			loadedAt: map[exclusionPlace]string{},
		}
		for _, x := range r.mod.Excluded {
			at := exclusionPlace{x.Dir, x.Package}
			cache.byPlace[at] = append(cache.byPlace[at], x)
		}
		// A nested module's excluded files are among them, so its packages
		// are siblings too.
		loaded := r.mod.Pkgs
		for _, imp := range r.mod.Importers {
			loaded = slices.Concat(loaded, imp.Pkgs)
		}
		for _, q := range loaded {
			cache.loadedAt[exclusionPlaceOf(q)] = q.PkgPath
		}
		r.exclusionCache = cache
	}
	if near, ok := cache.byPath[p.PkgPath]; ok {
		return near
	}
	own := exclusionPlaceOf(p)
	near := &exclusions{own: cache.byPlace[own]}
	syntax := make([]*ast.File, 0, len(near.own))
	for _, x := range near.own {
		syntax = append(syntax, x.Syntax)
	}
	near.linknamed = directive.Linknamed(syntax)
	for _, x := range r.mod.Excluded {
		if (exclusionPlace{x.Dir, x.Package}) == own {
			continue
		}
		paths := x.ImportPaths()
		if slices.Contains(paths, p.PkgPath) {
			near.importing = append(near.importing, x)
		}
		if x.DotImports(p.PkgPath) {
			near.dotImporting = append(near.dotImporting, x)
		}
		if !near.reached {
			// The file's compiled siblings count as much as its imports: a
			// package-level variable of theirs may hold a value of the
			// package's types.
			sibling, loaded := cache.loadedAt[exclusionPlace{x.Dir, x.Package}]
			near.reached = loaded && r.mod.Reaches(sibling, p.PkgPath) ||
				slices.ContainsFunc(paths, func(path string) bool { return r.mod.Reaches(path, p.PkgPath) })
		}
	}
	cache.byPath[p.PkgPath] = near
	return near
}

// qualifies reports whether a build-excluded file of another package imports
// the candidate's package and writes pkg.Name: a use, in a configuration
// this run does not build.
//
//declscope:shared // the core counts it as a use
func (near *exclusions) qualifies(c *candidate) bool {
	return slices.ContainsFunc(near.importing, func(x *excluded.File) bool {
		return x.Qualifies(c.pkg.PkgPath, c.pkg.Types.Name(), c.obj.Name())
	})
}

// mayUse reports whether a build-excluded file of another package may use
// the candidate, and ownMayUse whether one of its own package may, in ways
// the file's names do not show.
//
// The file is not type-checked, so for a field or a method, and for a type
// some struct embeds, what it spells is not enough. Go uses those without
// writing their names: a method satisfies an interface, a field is matched
// by name in a conversion or an assignment between identical struct types,
// a literal fills fields by position through an alias, and a selector names
// an embedded type as a field. So any file that may hold a value of a type
// of the candidate's package counts: its own package's, one importing a
// package that reaches it (module.Reaches), and one whose own package does,
// through a package-level variable of a compiled file beside it. A
// package-level name has no such use, and counts only where it is written:
// under a dot import, or pkg.Name, which usedOutside reads.
//
// usedImplicitly is run.usedImplicitly, asked only when such a file exists.
//
//declscope:shared // the core withholds the fix on it
func (near *exclusions) mayUse(c *candidate, usedImplicitly func(*candidate) bool) bool {
	if near.reached && usedImplicitly(c) {
		return true
	}
	return slices.ContainsFunc(near.dotImporting, func(x *excluded.File) bool { return x.Writes(c.obj.Name()) })
}

//declscope:shared // the core withholds the fix on it
func (near *exclusions) ownMayUse(c *candidate, usedImplicitly func(*candidate) bool) bool {
	return len(near.own) > 0 && usedImplicitly(c)
}

// ownWrites reports whether a build-excluded file of the package itself
// writes name. The rename cannot rewrite that file, which would keep the old
// name or declare the new one twice.
//
//declscope:shared // the core asks it about the name, and rename.go about the new name
func (near *exclusions) ownWrites(name string) bool {
	return slices.ContainsFunc(near.own, func(x *excluded.File) bool { return x.Writes(name) })
}

// ownLinknames reports whether a build-excluded file of the package itself
// binds name with //go:linkname or //export. The rename cannot rewrite that
// file, and the build that keeps it would fail: the directive would name a
// declaration that is gone.
//
//declscope:shared // rename.go asks it beside the package's own linknames
func (near *exclusions) ownLinknames(name string) bool {
	return near.linknamed[name]
}

// exclusionPlace is where a build-excluded file belongs: its directory and
// its package name. A file of a loaded package shares both with it.
type exclusionPlace struct {
	dir, pkg string
}

// exclusionPlaceOf returns where the files of the loaded package p are.
func exclusionPlaceOf(p *packages.Package) exclusionPlace {
	var dir string
	if files := slices.Concat(p.GoFiles, p.CompiledGoFiles); len(files) > 0 {
		dir = filepath.Dir(files[0])
	}
	return exclusionPlace{dir, p.Types.Name()}
}
