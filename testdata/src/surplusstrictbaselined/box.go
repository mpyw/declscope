//declscope:package

package surplusstrictbaselined

// boxKind is read from order.go, which keeps the file's directive in use.
type boxKind int

// boxed was reported with its field n, and both are in the baseline. The
// type's finding then offers no fix to narrow extra, which came later.
type boxed struct {
	n     int
	extra int // want `field boxed.extra takes package scope from the file's //declscope:package, but no use from another namespace is visible to declscope`
}

func boxRead(b boxed) int { return b.n + b.extra }

var _ = boxRead
