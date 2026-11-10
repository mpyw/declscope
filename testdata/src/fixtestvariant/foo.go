package fixtestvariant

// fooCount is free in this variant's package scope but declared in
// foo_test.go, which only the test variant sees. Neither variant renames: the
// test variant finds the name taken, and this one cannot see enough of the
// package to prove anything, so it defers.
var count = 10 // want `var count does not carry namespace "foo" anywhere in its name; give it a name that carries "foo", such as fooCount, or move it to the file it belongs to`

// Free in both variants. Only the test variant, which sees every file, offers
// the rename, and its edits cover this file too.
var size = 1 // want `var size does not carry namespace "foo" anywhere in its name; give it a name that carries "foo", such as fooSize, or move it to the file it belongs to`

func Exported() int { return count + size }
