//go:build never

package v

import "example.com/withheld/internal/y"

type runner interface{ Run() }

var (
	_ = y.Keyed{Flag: 1}
	_ = y.Filled{1, "x"}
	_ = []y.InSlice{{1}}
	_ = map[string]*y.InMap{"k": {1}}
	_ = []*y.InSlice{&y.InSlice{2}}

	_ runner = y.NewRunner()
	_        = y.Converted(struct{ Flag int }{1})
	_        = y.NewHolder().Embedded
)
