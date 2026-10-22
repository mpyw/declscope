package coregoossibling

// foo_linux.go is in the core, but a GOOS sibling does not join it, so this
// file keeps its namespace in every build. Joining in a linux build only made a
// baseline recorded there suppress nothing elsewhere.
func fooHelper() int { return 1 } // want `func fooHelper is private to namespace "foo", but is used from the core namespace`
