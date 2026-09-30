//go:build never

package v

import "example.com/withheld/internal/w"

var (
	_ = w.Keyed{Flag: 1}
	_ = w.Filled{1, "x"}
	_ = []w.InSlice{{1}}
	_ = map[string]*w.InMap{"k": {1}}
	_ = []*w.InSlice{&w.InSlice{2}}
)
