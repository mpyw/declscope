// A file-level directive widens every field the file declares, and the report
// names it.
//
//declscope:shared

package surplusstrict

type fileCursor struct {
	pos  int
	seen int // want `field fileCursor.seen takes shared scope from the file's //declscope:shared, but no use from another namespace is visible to declscope`
}

func fileAdvance(c *fileCursor) { c.seen++ }
