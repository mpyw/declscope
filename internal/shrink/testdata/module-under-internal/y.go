// Package y is importable from example.com/x, which lies above this module,
// so its exported API is a root of the exposure walk though its path runs
// through internal/.
package y

import "example.com/x/internal/y/internal/z"

func Get() z.T { return z.T{} }
