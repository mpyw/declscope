package qualifyinflect

// The namespace's own spelling changes under -ing, so plain containment
// cannot see it: storing is stor + ing, not store + anything. The generated
// form accepts it.
func storing() int { return 1 }

// Only the generated whole form counts. A name that merely shares the stem
// does not carry the namespace: matching stor loose would accept story, which
// has nothing to do with store.
func story() int { return 2 } // want `func story does not carry namespace "store" anywhere in its name; rename it to storeStory, or to another name that carries "store"`

var _, _ = storing, story
