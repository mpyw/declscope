package boundaryoff

// orderRun carries its namespace, so the naming rule is quiet about it. It
// reaches across a boundary that nothing is checking.
func orderRun() int { return userCache() }

// helper does not carry "order", and the naming rule still says so.
func helper() int { return 2 } // want `func helper does not carry namespace "order" anywhere in its name; give it a name that carries "order", such as orderHelper, or move it to the file it belongs to`

var _ = orderRun
