package fixcapture

// The new name is free in package scope but bound at a reference by a
// parameter. Renaming would silently retarget that reference, so no fix.
var count = 10 // want `var count does not carry namespace "user" anywhere in its name; rename it to userCount, or to another name that carries "user"`

func Add(userCount int) int { return userCount + count }

// Bound at a reference by a local declared before it.
var limit = 3 // want `var limit does not carry namespace "user" anywhere in its name; rename it to userLimit, or to another name that carries "user"`

func Limited() int {
	userLimit := 1
	return userLimit + limit
}

// Bound in file scope by an import in order.go. Go rejects a package-level
// name that any file of the package imports, whether or not that file
// references the declaration.
var total = 7 // want `var total does not carry namespace "user" anywhere in its name; rename it to userTotal, or to another name that carries "user"`

var _ = total

// A local declared after the reference does not bind it, so this one is
// renamed: the reference keeps resolving to the package-level declaration.
var size = 1 // want `var size does not carry namespace "user" anywhere in its name; rename it to userSize, or to another name that carries "user"`

func Sized() int {
	s := size
	userSize := 2
	return s + userSize
}
