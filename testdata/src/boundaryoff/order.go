package boundaryoff

// orderRun carries its namespace, so the naming rule is quiet about it. It
// reaches across a boundary that nothing is checking.
func orderRun() int { return userCache() }

// helper does not carry "order", and the naming rule still says so.
func helper() int { return 2 } // want `func helper does not carry namespace "order" anywhere in its name; rename it to orderHelper, or to another name that carries "order"`

var _ = orderRun
