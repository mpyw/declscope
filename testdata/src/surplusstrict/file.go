// A file-level directive widens every field the file declares, and the report
// names it.
//
//declscope:package

package surplusstrict

type fileCursor struct {
	pos  int
	seen int // want `field fileCursor.seen takes package scope from the file's //declscope:package, but no use from another namespace is visible to declscope`
}

func fileAdvance(c *fileCursor) { c.seen++ }
