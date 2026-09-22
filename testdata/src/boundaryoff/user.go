package boundaryoff

// userCache is private to this file, and order.go reaches it. With the
// boundary rule off, that is not reported.
func userCache() int { return 1 }
