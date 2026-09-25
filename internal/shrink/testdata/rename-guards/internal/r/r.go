// Package r exercises the rename: what it rewrites, and every guard that
// withholds it. Nothing outside the package uses anything here.
package r

import _ "unsafe"

// Plain is fixed, and the name its doc comment opens with is rewritten too.
func Plain() int { return 1 } // want: func Plain is exported, but nothing.*uses it$

// A Thing is fixed, and so is the name after the article.
type Thing struct{} // want: type Thing is exported, but nothing.*uses it$

// Keep keeps its name, since an example names it, so KeptID keeps its own.
// KeptID then claims no name, and Kid, which would lower to the same, is
// fixed with the Toy it returns, all in one run.
func Keep() KeptID { return KeptID{} } // want: func Keep is exported.*no fix: an example function names it

type KeptID struct{} // want: type KeptID is exported.*no fix: an exported declaration that keeps its name hands it out

func KeptId() *Toy { return nil } // want: func KeptId is exported, but nothing.*uses it$

type Toy struct{} // want: type Toy is exported, but nothing.*uses it$

// NewThing returns a Thing, and is fixed in the same run, so it keeps
// nothing exported: one run of -fix settles both.
func NewThing() *Thing { return nil } // want: func NewThing is exported, but nothing.*uses it$

// Svc's method is fixed, and so is the name its doc comment opens with.
type Svc struct{} // want: type Svc is exported, but nothing.*uses it$

// Serve is fixed with its doc comment.
func (Svc) Serve() {} // want: method Serve is exported, but nothing.*uses it$

/* Block is a block comment, which the fix leaves alone. */
func Block() {} // want: func Block is exported, but nothing.*uses it$

// Loader opens this comment, which is another word, and is left alone.
func Load() {} // want: func Load is exported, but nothing.*uses it$

func Taken() int { return taken() } // want: func Taken is exported.*no fix: the unexported name is taken or would be captured

func taken() int { return 0 }

// Count is captured by the parameter of the same name at its one use.
var Count = func(count int) int { return count } // want: var Count is exported.*no fix: the unexported name is taken or would be captured

func useCount(count int) int { return count + Count(count) }

// DUP lowers to the name Dup's fix claims.
func Dup() int { return 1 } // want: func Dup is exported, but nothing.*uses it$

func DUP() int { return 2 } // want: func DUP is exported.*no fix: the unexported name is taken or would be captured

func Type() {} // want: func Type is exported.*no fix: the unexported name is not an identifier

func Init() {} // want: func Init is exported.*no fix: the unexported name means something to the toolchain

//go:linkname Pushed
func Pushed() {} // want: func Pushed is exported.*no fix: a //go:linkname or //export names it

// The unexported name is an import of another file of the package.
func Strings() {} // want: func Strings is exported.*no fix: the unexported name is taken or would be captured

// The unexported name is written by a build-excluded file of the package.
func Clash() {} // want: func Clash is exported.*no fix: a build-excluded file of its package writes the unexported name

// HTTPServer becomes httpServer. MAX_RETRIES has no spelling Go would use.
func HTTPServer() {} // want: func HTTPServer is exported, but nothing.*uses it$

const MAX_RETRIES = 3 // want: const MAX_RETRIES is exported.*no fix: the name has no unexported spelling Go would use

const Limit = 3 // want: const Limit is exported, but nothing.*uses it$

type Alias = Pair // want: type Alias is exported, but nothing.*uses it$

// Level is not a struct, so nothing embeds it to collide with.
type Level int // want: type Level is exported, but nothing.*uses it$

// Named has a method whose unexported name is a field, and a field whose
// unexported name is a method.
type Named struct { // want: type Named is exported, but nothing.*uses it$
	name string
	Size int // want: field Size is exported.*no fix: the unexported name is taken on its type
}

func (Named) Name() string { return "" } // want: method Name is exported.*no fix: the unexported name is taken on its type

func (Named) size() int { return 0 }

// Asked would start satisfying the interface of a type assertion.
type Asked struct{} // want: type Asked is exported, but nothing.*uses it$

func (Asked) Run() {} // want: method Run is exported.*no fix: the unexported name is taken on its type

// Quiet would start satisfying husher, a named interface of the package.
type Quiet struct{} // want: type Quiet is exported, but nothing.*uses it$

func (Quiet) Hush() {} // want: method Hush is exported.*no fix: the unexported name is taken on its type

type husher interface{ hush() }

// derived promotes Hop, and already has a field of the unexported name.
type Base struct{} // want: type Base is exported, but nothing.*uses it$

func (Base) Hop() {} // want: method Hop is exported.*no fix: the unexported name is taken on its type

type derived struct {
	Base
	hop int
}

// An unnamed struct promotes Size, and has a field of the unexported name.
type Boxed struct { // want: type Boxed is exported, but nothing.*uses it$
	Size int // want: field Size is exported.*no fix: the unexported name is taken on its type
}

// amb embeds T and U, so its F is ambiguous, and renaming T's F to f would
// make its f ambiguous too.
type T struct { // want: type T is exported, but nothing.*uses it$
	F int // want: field F is exported.*no fix: the unexported name is taken on its type
}

type U struct { // want: type U is exported, but nothing.*uses it$
	F int // want: field F is exported.*no fix: the unexported name is taken on its type
	f int
}

type amb struct {
	T
	U
}

// First.ID and Second.Id both lower to id, and both embeds the two, so only
// the first is fixed.
type First struct { // want: type First is exported, but nothing.*uses it$
	ID int // want: field ID is exported, but nothing.*uses it$
}

type Second struct { // want: type Second is exported, but nothing.*uses it$
	Id int // want: field Id is exported.*no fix: the unexported name is taken on its type
}

type both struct {
	First
	Second
}

// ID and Id of one type lower to one name, so only the first is fixed.
// First.ID's claim on id above does not reach Pair.
type Pair struct { // want: type Pair is exported, but nothing.*uses it$
	ID int // want: field ID is exported, but nothing.*uses it$
	Id int // want: field Id is exported.*no fix: the unexported name is taken on its type
}

// Emb's unexported name is taken by a field of a struct embedding it, since
// the embedded field takes the type's name.
type Emb struct{} // want: type Emb is exported.*no fix: the unexported name is taken or would be captured

type Holder struct { // want: type Holder is exported, but nothing.*uses it$
	Emb
	emb int
}

// Inner is embedded and selected by its name, which the rename rewrites.
type Inner struct{} // want: type Inner is exported, but nothing.*uses it$

type outer struct{ Inner }

// Pointed and AliasOfAliased are embedded through a pointer and an alias.
type Pointed struct{} // want: type Pointed is exported, but nothing.*uses it$

type Aliased struct{} // want: type Aliased is exported, but nothing.*uses it$

type AliasOfAliased = Aliased // want: type AliasOfAliased is exported, but nothing.*uses it$

type embedder struct {
	*Pointed
	AliasOfAliased
}

// A predeclared type embedded by name is no declaration of the package.
type basic struct{ int }

// An unkeyed literal in the package itself is renamed with the fields.
type Local struct { // want: type Local is exported, but nothing.*uses it$
	A int // want: field A is exported, but nothing.*uses it$
}

// A selection on an instantiation names the origin's member.
type List[E any] struct { // want: type List is exported, but nothing.*uses it$
	Items []E // want: field Items is exported, but nothing.*uses it$
}

func (l *List[E]) Len() int { return len(l.Items) } // want: method Len is exported.*no fix: the unexported name is predeclared

func uses() int {
	var l List[int]
	_ = func(v any) bool { _, ok := v.(interface{ run() }); return ok }
	var _ husher
	var v struct {
		Boxed
		size int
	}
	return l.Len() + len(l.Items) + useCount(1) + Plain() + Dup() + DUP() + MAX_RETRIES + Limit +
		Named{}.Size + Named{}.size() + len(Named{}.name+Named{}.Name()) + derived{}.hop + v.Size + v.size +
		amb{}.f + T{}.F + U{}.F + both{}.ID + both{}.Id + Pair{}.ID + Pair{}.Id + Holder{}.emb + Local{1}.A + Local{A: 2}.A +
		basic{}.int
}

var (
	_ Thing
	_ = NewThing
	_ = Keep
	_ = KeptId
	_ Alias
	_ Level
	_ = Asked{}.Run
	_ = Quiet{}.Hush
	_ = derived{}.Hop
	_ = Holder{}.Emb
	_ = outer{}.Inner
	_ = embedder{}.Pointed
	_ = embedder{}.AliasOfAliased
	_ = uses
	_ = Svc{}.Serve
	// A conversion and a literal of types that are no struct pair nothing.
	_ = float64(Limit)
	_ = []int{1}
)
