//declscope:ignore promote

package fileignorescope

// This file stands outside the label rule.
func helper() int { return 1 }

var _ = helper
